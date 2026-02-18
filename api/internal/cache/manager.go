// Package cache manages cache pools and the mover daemon.
// Provides mergerfs-based hot/cold tiering inspired by Unraid's cache system.
package cache

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/proxmaid/proxmaid/internal/disk"
)

func humanSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.1f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// ShareCachePolicy controls how a share uses the cache pool.
type ShareCachePolicy string

const (
	// CachePolicyYes writes new files to cache, mover moves them to array.
	CachePolicyYes ShareCachePolicy = "yes"
	// CachePolicyPrefer keeps files on cache, overflows to array. Mover pulls back.
	CachePolicyPrefer ShareCachePolicy = "prefer"
	// CachePolicyOnly writes only to cache; never moves to array.
	CachePolicyOnly ShareCachePolicy = "only"
	// CachePolicyNo bypasses cache entirely, writes directly to array.
	CachePolicyNo ShareCachePolicy = "no"
)

// PoolDevice represents a single device in a cache pool.
type PoolDevice struct {
	Path      string `json:"path"`       // e.g. "/dev/nvme0n1p1"
	Model     string `json:"model"`      // drive model
	Size      int64  `json:"size"`       // bytes
	SizeHuman string `json:"size_human"` // e.g. "2.0 TB"
}

// Pool represents a cache pool configuration and status.
type Pool struct {
	Name         string       `json:"name"`          // e.g. "pool0"
	Devices      []PoolDevice `json:"devices"`       // devices in the pool
	MountPoint   string      `json:"mount_point"`   // e.g. "/mnt/cache/pool0" or "/mnt/cache/pool0/merged"
	BranchMounts []string    `json:"-"`              // branch mount paths for mergerfs (unexported, for unmount)
	FSType       string      `json:"fs_type"`       // e.g. "xfs", "btrfs"
	TotalBytes   int64       `json:"total_bytes"`   // total capacity
	UsedBytes    int64       `json:"used_bytes"`    // used space
	FreeBytes    int64       `json:"free_bytes"`    // free space
	UsedPct      float64     `json:"used_pct"`      // usage percentage
	Status       string      `json:"status"`        // "active", "degraded", "stopped"
}

// MoverConfig holds the mover daemon configuration.
type MoverConfig struct {
	Schedule     string `json:"schedule"`      // cron expression, e.g. "40 3 * * *"
	AgeThreshold string `json:"age_threshold"` // e.g. "1d", "12h", "30m"
	Enabled      bool   `json:"enabled"`       // whether the mover is active
}

// MoverStatus holds runtime state of the mover.
type MoverStatus struct {
	Running    bool        `json:"running"`
	LastRun    string      `json:"last_run"`    // ISO timestamp or ""
	NextRun    string      `json:"next_run"`    // ISO timestamp or ""
	BytesMoved int64       `json:"bytes_moved"` // bytes moved in last run
	FilesMoved int         `json:"files_moved"` // files moved in last run
	Progress   float64     `json:"progress"`    // 0-100 during active run
	Config     MoverConfig `json:"config"`
}

// Manager handles cache pool operations and the mover daemon.
type Manager struct {
	mu       sync.RWMutex
	mockMode bool
	diskMgr  *disk.Manager
	pools    map[string]*Pool
	mover    *MoverStatus
}

// NewManager creates a new cache manager. diskMgr can be nil in mock mode.
func NewManager(mockMode bool, diskMgr *disk.Manager) *Manager {
	m := &Manager{
		mockMode: mockMode,
		diskMgr:  diskMgr,
		pools:    make(map[string]*Pool),
		mover: &MoverStatus{
			Config: MoverConfig{
				Schedule:     "40 3 * * *",
				AgeThreshold: "1d",
				Enabled:      true,
			},
		},
	}

	if !mockMode {
		m.ensureMergerfs()
	}

	if mockMode {
		m.loadMockPools()
	} else {
		go m.startHealthMonitor()
		go m.startScheduledMover()
	}

	return m
}

// IsMergerfsInstalled checks if mergerfs is available on the system.
func (m *Manager) IsMergerfsInstalled() bool {
	if m.mockMode {
		return true
	}
	_, err := exec.LookPath("mergerfs")
	return err == nil
}

// ensureMergerfs installs mergerfs if it is not present.
func (m *Manager) ensureMergerfs() {
	if m.IsMergerfsInstalled() {
		fmt.Println("[CACHE] mergerfs found")
		return
	}
	fmt.Println("[CACHE] mergerfs not found, installing...")
	cmd := exec.Command("apt-get", "install", "-y", "mergerfs")
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("[CACHE] WARNING: failed to install mergerfs: %s: %v\n", string(out), err)
	} else {
		fmt.Println("[CACHE] mergerfs installed successfully")
	}
}

// GetPools returns all configured cache pools.
func (m *Manager) GetPools() []*Pool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pools := make([]*Pool, 0, len(m.pools))
	for _, p := range m.pools {
		pools = append(pools, p)
	}
	return pools
}

// GetPool returns a single cache pool by name.
func (m *Manager) GetPool(name string) (*Pool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, ok := m.pools[name]
	if !ok {
		return nil, fmt.Errorf("pool %q not found", name)
	}
	return pool, nil
}

// CreatePoolRequest is the input for creating a new cache pool.
type CreatePoolRequest struct {
	Name    string   `json:"name"`
	Devices []string `json:"devices"` // device paths, e.g. ["/dev/nvme0n1"]
	FSType  string   `json:"fs_type"` // "xfs", "btrfs", "ext4"
}

// CreatePool creates a new cache pool from the given devices.
func (m *Manager) CreatePool(req CreatePoolRequest) (*Pool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("pool name is required")
	}
	if _, exists := m.pools[req.Name]; exists {
		return nil, fmt.Errorf("pool %q already exists", req.Name)
	}
	if len(req.Devices) == 0 {
		return nil, fmt.Errorf("at least one device is required")
	}
	if req.FSType == "" {
		req.FSType = "xfs"
	}

	baseDir := fmt.Sprintf("/mnt/cache/%s", req.Name)

	if m.mockMode {
		fmt.Printf("[MOCK] Creating cache pool %q with devices %v, fs=%s, mount=%s\n",
			req.Name, req.Devices, req.FSType, baseDir)
	} else if m.diskMgr != nil {
		// Real implementation: wipe, partition, format each device
		var partitionPaths []string
		var branchMounts []string
		for i, devicePath := range req.Devices {
			if err := m.diskMgr.WipeDisk(devicePath); err != nil {
				return nil, fmt.Errorf("wipe %s: %w", devicePath, err)
			}
			if err := m.diskMgr.PartitionDisk(devicePath); err != nil {
				return nil, fmt.Errorf("partition %s: %w", devicePath, err)
			}
			partPath := devicePath + "1"
			if err := m.diskMgr.FormatPartition(partPath, req.FSType); err != nil {
				return nil, fmt.Errorf("format %s: %w", partPath, err)
			}
			partitionPaths = append(partitionPaths, partPath)
			if len(req.Devices) >= 2 {
				branchMounts = append(branchMounts, fmt.Sprintf("%s/dev%d", baseDir, i))
			}
		}

		if len(req.Devices) == 1 {
			if err := exec.Command("mkdir", "-p", baseDir).Run(); err != nil {
				return nil, fmt.Errorf("mkdir mount point: %w", err)
			}
			if err := exec.Command("mount", partitionPaths[0], baseDir).Run(); err != nil {
				return nil, fmt.Errorf("mount %s: %w", partitionPaths[0], err)
			}
		} else {
			// mergerfs: create branch dirs and merged dir
			for _, b := range branchMounts {
				if err := exec.Command("mkdir", "-p", b).Run(); err != nil {
					return nil, fmt.Errorf("mkdir branch %s: %w", b, err)
				}
			}
			mergedDir := baseDir + "/merged"
			if err := exec.Command("mkdir", "-p", mergedDir).Run(); err != nil {
				return nil, fmt.Errorf("mkdir merged: %w", err)
			}
			for i, partPath := range partitionPaths {
				if err := exec.Command("mount", partPath, branchMounts[i]).Run(); err != nil {
					return nil, fmt.Errorf("mount branch %s: %w", partPath, err)
				}
			}
			opts := "defaults,allow_other,use_ino,category.create=mfs,moveonenospc=true"
			mergerfsArgs := []string{"-o", opts, strings.Join(branchMounts, ":"), mergedDir}
			if err := exec.Command("mergerfs", mergerfsArgs...).Run(); err != nil {
				// Unmount branches on failure
				for _, b := range branchMounts {
					exec.Command("umount", b).Run()
				}
				return nil, fmt.Errorf("mergerfs: %w", err)
			}
			baseDir = mergedDir
		}
	} else {
		// No disk manager: just create directory (e.g. tests)
		if err := exec.Command("mkdir", "-p", baseDir).Run(); err != nil {
			return nil, fmt.Errorf("failed to create mount point: %w", err)
		}
	}

	// Build pool device list (sizes from disk list when available)
	poolDevices := make([]PoolDevice, 0, len(req.Devices))
	var totalSize int64
	if m.diskMgr != nil && !m.mockMode {
		disks, _ := m.diskMgr.ListDisks()
		for _, d := range req.Devices {
			size := int64(0)
			model := ""
			for _, info := range disks {
				if info.Path == d || strings.HasPrefix(d, info.Path) {
					size = info.Size
					model = info.Model
					break
				}
			}
			if size == 0 {
				size = 1000204886016
				model = "unknown"
			}
			poolDevices = append(poolDevices, PoolDevice{
				Path:      d,
				Model:     model,
				Size:      size,
				SizeHuman: humanSize(size),
			})
			totalSize += size
		}
	} else {
		for _, d := range req.Devices {
			dev := PoolDevice{Path: d, Size: 1000204886016, SizeHuman: "931.5 GB"}
			totalSize += dev.Size
			poolDevices = append(poolDevices, dev)
		}
	}

	pool := &Pool{
		Name:       req.Name,
		Devices:    poolDevices,
		MountPoint: baseDir,
		FSType:     req.FSType,
		TotalBytes: totalSize,
		UsedBytes:  0,
		FreeBytes:  totalSize,
		UsedPct:    0,
		Status:     "active",
	}
	if len(req.Devices) >= 2 && !m.mockMode && m.diskMgr != nil {
		pool.BranchMounts = make([]string, len(req.Devices))
		for i := range req.Devices {
			pool.BranchMounts[i] = fmt.Sprintf("/mnt/cache/%s/dev%d", req.Name, i)
		}
	}

	m.pools[req.Name] = pool
	return pool, nil
}

// DeletePool removes a cache pool.
func (m *Manager) DeletePool(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, ok := m.pools[name]
	if !ok {
		return fmt.Errorf("pool %q not found", name)
	}

	if m.mockMode {
		fmt.Printf("[MOCK] Deleting cache pool %q, unmounting %s\n", name, pool.MountPoint)
	} else {
		// Unmount mergerfs merged mount first, then branch mounts
		if err := exec.Command("umount", pool.MountPoint).Run(); err != nil {
			return fmt.Errorf("failed to unmount %s: %w", pool.MountPoint, err)
		}
		for _, branch := range pool.BranchMounts {
			if err := exec.Command("umount", branch).Run(); err != nil {
				return fmt.Errorf("failed to unmount branch %s: %w", branch, err)
			}
		}
	}

	delete(m.pools, name)
	return nil
}

// GetMoverStatus returns a copy of the current mover status.
// Returns a copy (not a pointer) to avoid data races with concurrent
// modifications from executeMoverRun or UpdateMoverConfig.
func (m *Manager) GetMoverStatus() MoverStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.mover == nil {
		return MoverStatus{}
	}
	return *m.mover
}

// UpdateMoverConfig updates the mover configuration.
func (m *Manager) UpdateMoverConfig(config MoverConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.Schedule == "" {
		return fmt.Errorf("schedule is required")
	}
	if config.AgeThreshold == "" {
		return fmt.Errorf("age threshold is required")
	}

	m.mover.Config = config
	return nil
}

// RunMover triggers an immediate mover run.
func (m *Manager) RunMover() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.mover.Running {
		return fmt.Errorf("mover is already running")
	}

	m.mover.Running = true

	if m.mockMode {
		fmt.Println("[MOCK] Mover triggered manually")
		// Simulate a mover run in mock mode
		go func() {
			time.Sleep(2 * time.Second)
			m.mu.Lock()
			defer m.mu.Unlock()
			m.mover.Running = false
			m.mover.LastRun = time.Now().UTC().Format(time.RFC3339)
			m.mover.BytesMoved = 1073741824 // 1 GB
			m.mover.FilesMoved = 42
			m.mover.Progress = 100
		}()
		return nil
	}

	// Real implementation: start mover in background goroutine
	go m.executeMoverRun()
	return nil
}

// executeMoverRun performs the actual data movement from hot to cold tier.
func (m *Manager) executeMoverRun() {
	defer func() {
		m.mu.Lock()
		m.mover.Running = false
		m.mover.LastRun = time.Now().UTC().Format(time.RFC3339)
		m.mu.Unlock()
	}()

	fmt.Println("[CACHE] Mover run started")

	m.mu.RLock()
	threshold := m.mover.Config.AgeThreshold
	pools := make([]*Pool, 0, len(m.pools))
	for _, p := range m.pools {
		pools = append(pools, p)
	}
	m.mu.RUnlock()

	dur, err := ParseAgeThreshold(threshold)
	if err != nil {
		fmt.Printf("[CACHE] Invalid age threshold %q: %v\n", threshold, err)
		return
	}

	var totalFiles int
	var totalBytes int64
	for _, pool := range pools {
		files, err := FindColdFiles(pool.MountPoint, dur)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			fmt.Printf("[CACHE] Error scanning %s: %v\n", pool.MountPoint, err)
			continue
		}
		for _, path := range files {
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			if !info.IsDir() {
				totalFiles++
				totalBytes += info.Size()
			}
		}
		// rsync each cold file to the array, mirroring directory structure
		for _, path := range files {
			info, err := os.Stat(path)
			if err != nil || info.IsDir() {
				continue
			}

			// Mirror path: /mnt/cache/pool/share/dir/file → /mnt/user/share/dir/file
			relPath, err := filepath.Rel(pool.MountPoint, path)
			if err != nil {
				continue
			}
			destPath := filepath.Join("/mnt/user", relPath)
			destDir := filepath.Dir(destPath)

			// Create destination directory
			if err := exec.Command("mkdir", "-p", destDir).Run(); err != nil {
				fmt.Printf("[CACHE] Failed to create dest dir %s: %v\n", destDir, err)
				continue
			}

			// rsync with checksum verification, then remove source
			cmd := exec.Command("rsync", "--checksum", "--remove-source-files", path, destPath)
			if out, err := cmd.CombinedOutput(); err != nil {
				fmt.Printf("[CACHE] rsync failed for %s: %s: %v\n", path, string(out), err)
				continue
			}

			totalFiles++
			totalBytes += info.Size()

			m.mu.Lock()
			m.mover.FilesMoved = totalFiles
			m.mover.BytesMoved = totalBytes
			if totalFiles > 0 {
				// Rough progress estimate based on files processed vs found
				m.mover.Progress = float64(totalFiles) / float64(len(files)) * 100
			}
			m.mu.Unlock()
		}
	}

	m.mu.Lock()
	m.mover.FilesMoved = totalFiles
	m.mover.BytesMoved = totalBytes
	m.mover.Progress = 100
	m.mu.Unlock()

	fmt.Printf("[CACHE] Mover run completed: %d files, %d bytes (move to array not yet implemented)\n", totalFiles, totalBytes)
}

// ParseAgeThreshold parses an age threshold string into a duration.
// Supported: "1d", "12h", "30m", "90m", "7d".
func ParseAgeThreshold(s string) (time.Duration, error) {
	if s == "" {
		return 24 * time.Hour, nil
	}
	re := regexp.MustCompile(`^(?i)(\d+)(d|h|m|s)$`)
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return 0, fmt.Errorf("invalid age threshold %q (use e.g. 1d, 12h, 30m)", s)
	}
	n, _ := strconv.Atoi(matches[1])
	unit := strings.ToLower(matches[2])
	switch unit {
	case "d":
		return time.Duration(n) * 24 * time.Hour, nil
	case "h":
		return time.Duration(n) * time.Hour, nil
	case "m":
		return time.Duration(n) * time.Minute, nil
	case "s":
		return time.Duration(n) * time.Second, nil
	default:
		return 0, fmt.Errorf("unknown unit %q", unit)
	}
}

// FindColdFiles walks poolMount and returns absolute paths of files
// whose mtime is older than (now - threshold). Directories are not included.
func FindColdFiles(poolMount string, threshold time.Duration) ([]string, error) {
	cutoff := time.Now().Add(-threshold)
	var out []string
	err := filepath.Walk(poolMount, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsPermission(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
		}
		if info.ModTime().Before(cutoff) {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}

// AddDevice adds a new device to an existing pool.
// Formats the device and adds it as a new mergerfs branch.
func (m *Manager) AddDevice(poolName, devicePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, ok := m.pools[poolName]
	if !ok {
		return fmt.Errorf("pool %q not found", poolName)
	}

	if m.mockMode {
		fmt.Printf("[MOCK] Adding device %s to pool %s\n", devicePath, poolName)
		pool.Devices = append(pool.Devices, PoolDevice{
			Path:      devicePath,
			Model:     "mock",
			Size:      1000204886016,
			SizeHuman: "931.5 GB",
		})
		pool.TotalBytes += 1000204886016
		pool.FreeBytes += 1000204886016
		return nil
	}

	if m.diskMgr == nil {
		return fmt.Errorf("disk manager not available")
	}

	// Format the new device
	if err := m.diskMgr.WipeDisk(devicePath); err != nil {
		return fmt.Errorf("wipe %s: %w", devicePath, err)
	}
	if err := m.diskMgr.PartitionDisk(devicePath); err != nil {
		return fmt.Errorf("partition %s: %w", devicePath, err)
	}
	partPath := devicePath + "1"
	if err := m.diskMgr.FormatPartition(partPath, pool.FSType); err != nil {
		return fmt.Errorf("format %s: %w", partPath, err)
	}

	// Create branch mount point
	branchIdx := len(pool.Devices)
	baseDir := fmt.Sprintf("/mnt/cache/%s", poolName)
	branchDir := fmt.Sprintf("%s/dev%d", baseDir, branchIdx)
	if err := exec.Command("mkdir", "-p", branchDir).Run(); err != nil {
		return fmt.Errorf("mkdir branch: %w", err)
	}

	// Mount the new device
	if err := exec.Command("mount", partPath, branchDir).Run(); err != nil {
		return fmt.Errorf("mount branch: %w", err)
	}

	// Add to mergerfs by remounting with the new branch
	if out, err := exec.Command("mount", "-o", fmt.Sprintf("remount,add=%s", branchDir), pool.MountPoint).CombinedOutput(); err != nil {
		return fmt.Errorf("mergerfs remount: %s: %w", string(out), err)
	}

	// Update pool state
	pool.Devices = append(pool.Devices, PoolDevice{Path: devicePath})
	pool.BranchMounts = append(pool.BranchMounts, branchDir)

	return nil
}

// RemoveDevice removes a device from an existing pool.
// Data should be moved off first (not enforced here).
func (m *Manager) RemoveDevice(poolName, devicePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, ok := m.pools[poolName]
	if !ok {
		return fmt.Errorf("pool %q not found", poolName)
	}

	if len(pool.Devices) <= 1 {
		return fmt.Errorf("cannot remove the last device from a pool")
	}

	if m.mockMode {
		fmt.Printf("[MOCK] Removing device %s from pool %s\n", devicePath, poolName)
		for i, d := range pool.Devices {
			if d.Path == devicePath {
				pool.Devices = append(pool.Devices[:i], pool.Devices[i+1:]...)
				break
			}
		}
		return nil
	}

	// Find and unmount the branch
	found := false
	for i, d := range pool.Devices {
		if d.Path == devicePath {
			if i < len(pool.BranchMounts) {
				branch := pool.BranchMounts[i]
				exec.Command("umount", branch).Run()
				pool.BranchMounts = append(pool.BranchMounts[:i], pool.BranchMounts[i+1:]...)
			}
			pool.Devices = append(pool.Devices[:i], pool.Devices[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("device %s not found in pool %s", devicePath, poolName)
	}

	return nil
}

// RefreshPoolUsage reads real disk usage for all pools via df.
func (m *Manager) RefreshPoolUsage() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, pool := range m.pools {
		if m.mockMode {
			continue
		}
		out, err := exec.Command("df", "-B1", pool.MountPoint).Output()
		if err != nil {
			continue
		}
		lines := strings.Split(string(out), "\n")
		if len(lines) < 2 {
			continue
		}
		fields := strings.Fields(lines[1])
		if len(fields) >= 4 {
			total, _ := strconv.ParseInt(fields[1], 10, 64)
			used, _ := strconv.ParseInt(fields[2], 10, 64)
			free, _ := strconv.ParseInt(fields[3], 10, 64)
			pool.TotalBytes = total
			pool.UsedBytes = used
			pool.FreeBytes = free
			if total > 0 {
				pool.UsedPct = float64(used) / float64(total) * 100
			}
		}
	}
}

// startHealthMonitor starts a goroutine that checks pool usage every 5 minutes.
// Triggers emergency mover if usage > 95%.
func (m *Manager) startHealthMonitor() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.RefreshPoolUsage()

		m.mu.RLock()
		for _, pool := range m.pools {
			if pool.UsedPct > 95 {
				fmt.Printf("[CACHE] CRITICAL: Pool %s at %.0f%% — triggering emergency mover\n", pool.Name, pool.UsedPct)
				m.mu.RUnlock()
				m.RunMover()
				m.mu.RLock()
			} else if pool.UsedPct > 85 {
				fmt.Printf("[CACHE] WARNING: Pool %s at %.0f%% usage\n", pool.Name, pool.UsedPct)
			}
		}
		m.mu.RUnlock()
	}
}

// startScheduledMover parses the mover cron schedule and runs on schedule.
// Uses a simple minute-level ticker approach for cron-like scheduling.
func (m *Manager) startScheduledMover() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for now := range ticker.C {
		m.mu.RLock()
		enabled := m.mover.Config.Enabled
		schedule := m.mover.Config.Schedule
		running := m.mover.Running
		m.mu.RUnlock()

		if !enabled || running {
			continue
		}

		if matchesCron(schedule, now) {
			fmt.Println("[CACHE] Scheduled mover trigger")
			m.RunMover()

			m.mu.Lock()
			m.mover.NextRun = nextCronMatch(schedule, now).Format(time.RFC3339)
			m.mu.Unlock()
		}
	}
}

// matchesCron checks if the current time matches a simple cron expression.
// Supports: "MIN HOUR * * *" format (minute, hour, day-of-month, month, day-of-week).
func matchesCron(schedule string, t time.Time) bool {
	fields := strings.Fields(schedule)
	if len(fields) != 5 {
		return false
	}

	minute := t.Minute()
	hour := t.Hour()

	// Check minute field
	if fields[0] != "*" {
		m, err := strconv.Atoi(fields[0])
		if err != nil || m != minute {
			return false
		}
	}
	// Check hour field
	if fields[1] != "*" {
		h, err := strconv.Atoi(fields[1])
		if err != nil || h != hour {
			return false
		}
	}

	return true
}

// nextCronMatch calculates the next time the cron schedule will match.
func nextCronMatch(schedule string, after time.Time) time.Time {
	fields := strings.Fields(schedule)
	if len(fields) < 2 {
		return after.Add(24 * time.Hour)
	}

	minute, _ := strconv.Atoi(fields[0])
	hour, _ := strconv.Atoi(fields[1])

	next := time.Date(after.Year(), after.Month(), after.Day(), hour, minute, 0, 0, after.Location())
	if !next.After(after) {
		next = next.Add(24 * time.Hour)
	}
	return next
}

// loadMockPools creates realistic mock cache pools for development.
func (m *Manager) loadMockPools() {
	m.pools["nvme-fast"] = &Pool{
		Name: "nvme-fast",
		Devices: []PoolDevice{
			{Path: "/dev/nvme0n1", Model: "SK Hynix P41", Size: 2000398934016, SizeHuman: "1.8 TB"},
			{Path: "/dev/nvme1n1", Model: "SK Hynix P41", Size: 2000398934016, SizeHuman: "1.8 TB"},
			{Path: "/dev/nvme2n1", Model: "SK Hynix P41", Size: 2000398934016, SizeHuman: "1.8 TB"},
		},
		MountPoint: "/mnt/cache/nvme-fast",
		FSType:     "xfs",
		TotalBytes: 6001196802048,
		UsedBytes:  1800359040614,
		FreeBytes:  4200837761434,
		UsedPct:    30.0,
		Status:     "active",
	}

	m.pools["ssd-warm"] = &Pool{
		Name: "ssd-warm",
		Devices: []PoolDevice{
			{Path: "/dev/sda", Model: "Samsung 870 EVO", Size: 1000204886016, SizeHuman: "931.5 GB"},
			{Path: "/dev/sdb", Model: "Samsung 870 EVO", Size: 1000204886016, SizeHuman: "931.5 GB"},
		},
		MountPoint: "/mnt/cache/ssd-warm",
		FSType:     "xfs",
		TotalBytes: 2000409772032,
		UsedBytes:  1200245863219,
		FreeBytes:  800163908813,
		UsedPct:    60.0,
		Status:     "active",
	}

	m.mover.LastRun = time.Now().Add(-8 * time.Hour).UTC().Format(time.RFC3339)
	m.mover.NextRun = time.Now().Add(16 * time.Hour).UTC().Format(time.RFC3339)
	m.mover.BytesMoved = 5368709120 // 5 GB
	m.mover.FilesMoved = 127
}
