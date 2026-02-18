// Package disk manages physical disk operations:
// discovery, SMART health, wiping, partitioning, formatting, and mounting.
package disk

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Info holds discovered information about a block device.
type Info struct {
	Name       string `json:"name"`       // e.g. "sdb"
	Path       string `json:"path"`       // e.g. "/dev/sdb"
	Size       int64  `json:"size"`       // bytes
	SizeHuman  string `json:"size_human"` // e.g. "2.0 TB"
	Model      string `json:"model"`      // drive model string
	Serial     string `json:"serial"`     // drive serial number
	Type       string `json:"type"`       // "disk", "part", etc.
	Mountpoint string `json:"mountpoint"` // if mounted
	FSType     string `json:"fstype"`     // filesystem type if any
	Rotational bool   `json:"rotational"` // true = HDD, false = SSD/NVMe
	DiskID     string `json:"disk_id"`    // /dev/disk/by-id/ link
}

// SmartHealth holds parsed SMART data for a disk.
type SmartHealth struct {
	Device      string `json:"device"`
	Healthy     bool   `json:"healthy"`
	Temperature int    `json:"temperature"` // Celsius
	PowerOnHrs  int    `json:"power_on_hours"`
	RawOutput   string `json:"raw_output"`
}

// Manager handles disk discovery and operations.
type Manager struct {
	MockMode    bool
	healthCache map[string]*SmartHealth
	mu          sync.RWMutex
}

// NewManager creates a new disk manager.
func NewManager(mockMode bool) *Manager {
	m := &Manager{
		MockMode:    mockMode,
		healthCache: make(map[string]*SmartHealth),
	}
	if mockMode {
		m.loadMockHealthCache()
	} else {
		go m.healthPollingLoop()
	}
	return m
}

// lsblkDevice matches the JSON output from lsblk.
// Size and Rota use interface{} because lsblk returns different JSON types
// depending on version (number, string, bool).
type lsblkDevice struct {
	Name       string        `json:"name"`
	Path       string        `json:"path"`
	Size       interface{}   `json:"size"`
	Model      string        `json:"model"`
	Serial     string        `json:"serial"`
	Type       string        `json:"type"`
	Mountpoint string        `json:"mountpoint"`
	FSType     string        `json:"fstype"`
	Rota       interface{}   `json:"rota"`
	Children   []lsblkDevice `json:"children"`
}

type lsblkOutput struct {
	BlockDevices []lsblkDevice `json:"blockdevices"`
}

// toInt64 converts an interface{} (string, float64, json.Number) to int64.
func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case float64:
		return int64(val)
	case string:
		n, _ := strconv.ParseInt(val, 10, 64)
		return n
	case json.Number:
		n, _ := val.Int64()
		return n
	default:
		return 0
	}
}

// toBool converts an interface{} (bool, string, float64) to bool.
func toBool(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case float64:
		return val == 1
	case string:
		return val == "1" || val == "true"
	case json.Number:
		return val.String() == "1"
	default:
		return false
	}
}

// ListDisks discovers all block devices on the system.
func (m *Manager) ListDisks() ([]Info, error) {
	if m.MockMode {
		return mockDisks(), nil
	}

	out, err := exec.Command("lsblk", "-J", "-b", "-o",
		"NAME,PATH,SIZE,MODEL,SERIAL,TYPE,MOUNTPOINT,FSTYPE,ROTA").Output()
	if err != nil {
		return nil, fmt.Errorf("lsblk failed: %w", err)
	}

	var parsed lsblkOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse lsblk output: %w", err)
	}

	var disks []Info
	for _, dev := range parsed.BlockDevices {
		if dev.Type != "disk" {
			continue
		}
		size := toInt64(dev.Size)
		disks = append(disks, Info{
			Name:       dev.Name,
			Path:       dev.Path,
			Size:       size,
			SizeHuman:  humanSize(size),
			Model:      strings.TrimSpace(dev.Model),
			Serial:     strings.TrimSpace(dev.Serial),
			Type:       dev.Type,
			Mountpoint: dev.Mountpoint,
			FSType:     dev.FSType,
			Rotational: toBool(dev.Rota),
		})
	}

	return disks, nil
}

// GetSmartHealth retrieves SMART health for a given device path.
func (m *Manager) GetSmartHealth(devicePath string) (*SmartHealth, error) {
	if m.MockMode {
		return mockSmartHealth(devicePath), nil
	}

	out, err := exec.Command("smartctl", "-a", devicePath).CombinedOutput()
	if err != nil {
		// smartctl returns non-zero for various reasons (not just errors)
		// so we still try to parse the output
	}

	health := &SmartHealth{
		Device:    devicePath,
		RawOutput: string(out),
		Healthy:   strings.Contains(string(out), "PASSED"),
	}

	// Parse temperature
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "Temperature_Celsius") || strings.Contains(line, "Current Temperature") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				temp, _ := strconv.Atoi(fields[len(fields)-1])
				if temp > 0 && temp < 150 {
					health.Temperature = temp
				}
			}
		}
		if strings.Contains(line, "Power_On_Hours") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				hrs, _ := strconv.Atoi(fields[len(fields)-1])
				health.PowerOnHrs = hrs
			}
		}
	}

	return health, nil
}

// WipeDisk erases the partition table on a device.
func (m *Manager) WipeDisk(devicePath string) error {
	if m.MockMode {
		fmt.Printf("[MOCK] wipefs -a %s\n", devicePath)
		return nil
	}
	return exec.Command("wipefs", "-a", devicePath).Run()
}

// PartitionDisk creates a single GPT partition aligned for NonRAID.
func (m *Manager) PartitionDisk(devicePath string) error {
	if m.MockMode {
		fmt.Printf("[MOCK] sgdisk -o -a 8 -n 1:32K:0 %s\n", devicePath)
		return nil
	}
	return exec.Command("sgdisk", "-o", "-a", "8", "-n", "1:32K:0", devicePath).Run()
}

// FormatPartition formats a partition with the specified filesystem.
func (m *Manager) FormatPartition(partitionPath string, fsType string) error {
	switch fsType {
	case "xfs", "btrfs", "ext4":
		// supported
	default:
		return fmt.Errorf("unsupported filesystem type: %s", fsType)
	}

	if m.MockMode {
		fmt.Printf("[MOCK] mkfs.%s %s\n", fsType, partitionPath)
		return nil
	}

	var cmd *exec.Cmd
	switch fsType {
	case "xfs":
		cmd = exec.Command("mkfs.xfs", "-f", partitionPath)
	case "btrfs":
		cmd = exec.Command("mkfs.btrfs", "-f", partitionPath)
	case "ext4":
		cmd = exec.Command("mkfs.ext4", "-F", partitionPath)
	}
	return cmd.Run()
}

// humanSize converts bytes to a human-readable string.
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

// mockDisks returns simulated disk data for development.
func mockDisks() []Info {
	return []Info{
		{Name: "sda", Path: "/dev/sda", Size: 256060514304, SizeHuman: "238.5 GB", Model: "Samsung SSD 870", Serial: "S1234", Type: "disk", Rotational: false},
		{Name: "sdb", Path: "/dev/sdb", Size: 2000398934016, SizeHuman: "1.8 TB", Model: "WDC WD20EFRX", Serial: "WD-1234", Type: "disk", Rotational: true},
		{Name: "sdc", Path: "/dev/sdc", Size: 2000398934016, SizeHuman: "1.8 TB", Model: "WDC WD20EFRX", Serial: "WD-5678", Type: "disk", Rotational: true},
		{Name: "sdd", Path: "/dev/sdd", Size: 4000787030016, SizeHuman: "3.6 TB", Model: "Seagate IronWolf", Serial: "ST-9012", Type: "disk", Rotational: true},
		{Name: "sde", Path: "/dev/sde", Size: 4000787030016, SizeHuman: "3.6 TB", Model: "Seagate IronWolf", Serial: "ST-3456", Type: "disk", Rotational: true},
	}
}

// Mount mounts a formatted partition to the given mount point.
func (m *Manager) Mount(partitionPath, mountPoint string) error {
	if m.MockMode {
		fmt.Printf("[MOCK] mount %s %s\n", partitionPath, mountPoint)
		return nil
	}
	if err := exec.Command("mkdir", "-p", mountPoint).Run(); err != nil {
		return fmt.Errorf("failed to create mount point %s: %w", mountPoint, err)
	}
	if out, err := exec.Command("mount", partitionPath, mountPoint).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to mount %s to %s: %s: %w", partitionPath, mountPoint, string(out), err)
	}
	return nil
}

// GetCachedHealth returns all cached SMART health data.
func (m *Manager) GetCachedHealth() map[string]*SmartHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]*SmartHealth, len(m.healthCache))
	for k, v := range m.healthCache {
		result[k] = v
	}
	return result
}

// healthPollingLoop runs every 30 minutes, refreshing SMART data for all disks.
func (m *Manager) healthPollingLoop() {
	m.refreshHealthCache()
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		m.refreshHealthCache()
	}
}

// refreshHealthCache polls SMART health for all discovered disks.
func (m *Manager) refreshHealthCache() {
	disks, err := m.ListDisks()
	if err != nil {
		fmt.Printf("[DISK] Failed to list disks for health poll: %v\n", err)
		return
	}
	for _, d := range disks {
		health, err := m.GetSmartHealth(d.Path)
		if err != nil {
			continue
		}
		m.mu.Lock()
		m.healthCache[d.Path] = health
		m.mu.Unlock()
	}
}

// loadMockHealthCache populates the cache with mock SMART data.
func (m *Manager) loadMockHealthCache() {
	for _, d := range mockDisks() {
		m.healthCache[d.Path] = mockSmartHealth(d.Path)
	}
}

// SetStandbyTimeout sets the idle spindown timeout for an HDD.
// Value is in minutes. Wraps hdparm -S.
func (m *Manager) SetStandbyTimeout(device string, minutes int) error {
	if m.MockMode {
		fmt.Printf("[MOCK] hdparm -S %d %s\n", minutes/5, device)
		return nil
	}
	// hdparm -S value: timeout = value * 5 seconds, max 252 (21 minutes)
	// For longer timeouts, use values 241-251 (30min to 5.5hrs in 30min steps)
	val := minutes * 60 / 5 // convert minutes to 5-second units
	if val > 252 {
		val = 252
	}
	out, err := exec.Command("hdparm", "-S", strconv.Itoa(val), device).CombinedOutput()
	if err != nil {
		return fmt.Errorf("hdparm -S failed: %s: %w", string(out), err)
	}
	return nil
}

// GetPowerState returns the current power state of a disk ("active/idle" or "standby").
func (m *Manager) GetPowerState(device string) (string, error) {
	if m.MockMode {
		return "active/idle", nil
	}
	out, err := exec.Command("hdparm", "-C", device).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("hdparm -C failed: %s: %w", string(out), err)
	}
	output := string(out)
	if strings.Contains(output, "standby") {
		return "standby", nil
	}
	return "active/idle", nil
}

// Spindown forces a disk into standby (spin down).
func (m *Manager) Spindown(device string) error {
	if m.MockMode {
		fmt.Printf("[MOCK] hdparm -Y %s\n", device)
		return nil
	}
	out, err := exec.Command("hdparm", "-Y", device).CombinedOutput()
	if err != nil {
		return fmt.Errorf("hdparm -Y failed: %s: %w", string(out), err)
	}
	return nil
}

// IdentifyDisk blinks the LED on a disk enclosure (if supported).
func (m *Manager) IdentifyDisk(device string) error {
	if m.MockMode {
		fmt.Printf("[MOCK] ledctl locate=%s\n", device)
		return nil
	}
	if _, err := exec.LookPath("ledctl"); err != nil {
		return fmt.Errorf("ledctl not found — disk identification not supported on this system")
	}
	out, err := exec.Command("ledctl", "locate="+device).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ledctl failed: %s: %w", string(out), err)
	}
	return nil
}

// Unmount unmounts the given mount point.
func (m *Manager) Unmount(mountPoint string) error {
	if m.MockMode {
		fmt.Printf("[MOCK] umount %s\n", mountPoint)
		return nil
	}
	if out, err := exec.Command("umount", mountPoint).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to unmount %s: %s: %w", mountPoint, string(out), err)
	}
	return nil
}

// mockSmartHealth returns simulated SMART data.
func mockSmartHealth(device string) *SmartHealth {
	return &SmartHealth{
		Device:      device,
		Healthy:     true,
		Temperature: 34,
		PowerOnHrs:  12450,
		RawOutput:   "[MOCK] SMART data for " + device,
	}
}
