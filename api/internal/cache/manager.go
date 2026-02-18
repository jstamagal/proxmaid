// Package cache manages cache pools and the mover daemon.
// Provides mergerfs-based hot/cold tiering inspired by Unraid's cache system.
package cache

import (
	"fmt"
	"os/exec"
	"sync"
	"time"
)

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
	Name       string       `json:"name"`        // e.g. "pool0"
	Devices    []PoolDevice `json:"devices"`     // devices in the pool
	MountPoint string       `json:"mount_point"` // e.g. "/mnt/cache/pool0"
	FSType     string       `json:"fs_type"`     // e.g. "xfs", "btrfs"
	TotalBytes int64        `json:"total_bytes"` // total capacity
	UsedBytes  int64        `json:"used_bytes"`  // used space
	FreeBytes  int64        `json:"free_bytes"`  // free space
	UsedPct    float64      `json:"used_pct"`    // usage percentage
	Status     string       `json:"status"`      // "active", "degraded", "stopped"
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
	pools    map[string]*Pool
	mover    *MoverStatus
}

// NewManager creates a new cache manager.
func NewManager(mockMode bool) *Manager {
	m := &Manager{
		mockMode: mockMode,
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

	mountPoint := fmt.Sprintf("/mnt/cache/%s", req.Name)

	if m.mockMode {
		fmt.Printf("[MOCK] Creating cache pool %q with devices %v, fs=%s, mount=%s\n",
			req.Name, req.Devices, req.FSType, mountPoint)
	} else {
		// Real implementation:
		// 1. Partition each device
		// 2. Format each partition
		// 3. Create mount directory
		// 4. Mount (or use mergerfs if multiple devices)
		// For now, create the mount point
		if err := exec.Command("mkdir", "-p", mountPoint).Run(); err != nil {
			return nil, fmt.Errorf("failed to create mount point: %w", err)
		}
	}

	// Build pool device list
	devices := make([]PoolDevice, 0, len(req.Devices))
	var totalSize int64
	for _, d := range req.Devices {
		dev := PoolDevice{
			Path:      d,
			Size:      1000204886016, // placeholder in mock
			SizeHuman: "931.5 GB",
		}
		totalSize += dev.Size
		devices = append(devices, dev)
	}

	pool := &Pool{
		Name:       req.Name,
		Devices:    devices,
		MountPoint: mountPoint,
		FSType:     req.FSType,
		TotalBytes: totalSize,
		UsedBytes:  0,
		FreeBytes:  totalSize,
		UsedPct:    0,
		Status:     "active",
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
		// Unmount the pool
		if err := exec.Command("umount", pool.MountPoint).Run(); err != nil {
			return fmt.Errorf("failed to unmount %s: %w", pool.MountPoint, err)
		}
	}

	delete(m.pools, name)
	return nil
}

// GetMoverStatus returns the current mover status.
func (m *Manager) GetMoverStatus() *MoverStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.mover
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

	// TODO: Real mover implementation
	// 1. Walk each cache pool mount point
	// 2. Find files older than age threshold
	// 3. rsync each file to the corresponding array share
	// 4. Verify checksum
	// 5. Delete from cache
	// 6. Update progress
	fmt.Println("[CACHE] Mover run started")
	fmt.Println("[CACHE] Mover run completed")
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
