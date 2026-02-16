// Package disk manages physical disk operations:
// discovery, SMART health, wiping, partitioning, formatting, and mounting.
package disk

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
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
	MockMode bool
}

// NewManager creates a new disk manager.
func NewManager(mockMode bool) *Manager {
	return &Manager{MockMode: mockMode}
}

// lsblkDevice matches the JSON output from lsblk.
type lsblkDevice struct {
	Name       string        `json:"name"`
	Path       string        `json:"path"`
	Size       string        `json:"size"`
	Model      string        `json:"model"`
	Serial     string        `json:"serial"`
	Type       string        `json:"type"`
	Mountpoint string        `json:"mountpoint"`
	FSType     string        `json:"fstype"`
	Rota       string        `json:"rota"`
	Children   []lsblkDevice `json:"children"`
}

type lsblkOutput struct {
	BlockDevices []lsblkDevice `json:"blockdevices"`
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
		size, _ := strconv.ParseInt(dev.Size, 10, 64)
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
			Rotational: dev.Rota == "1",
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
