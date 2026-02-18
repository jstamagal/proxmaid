// Package array manages the NonRAID array state machine:
// Stopped -> Starting -> Started -> Stopping -> Stopped
// Also handles degraded states, parity checks, and disk operations.
package array

import (
	"fmt"
	"strings"
	"sync"

	"github.com/proxmaid/proxmaid/internal/system"
)

// State represents the current array state.
type State string

const (
	StateStopped  State = "STOPPED"
	StateStarting State = "STARTING"
	StateStarted  State = "STARTED"
	StateStopping State = "STOPPING"
	StateDegraded State = "DEGRADED"
	StateError    State = "ERROR"
)

// DiskInfo holds parsed status for a single disk in the array.
type DiskInfo struct {
	Slot       int    `json:"slot"`
	Status     string `json:"status"`      // rdevStatus: "DISK_OK", "DISK_NP", etc.
	DeviceName string `json:"device_name"` // rdevName: actual device e.g. "nvme3n1p1"
	VirtName   string `json:"virt_name"`   // diskName: virtual name e.g. "nmd1p1"
	SizeBytes  int64  `json:"size_bytes"`  // rdevSize in sectors (* 512)
	SizeHuman  string `json:"size_human"`  // human-readable size
	Role       string `json:"role"`        // "parity", "data", "q-parity"
	DiskID     string `json:"disk_id"`     // rdevId: disk-by-id link
	Reads      int64  `json:"reads"`       // rdevReads
	Writes     int64  `json:"writes"`      // rdevWrites
	Errors     int    `json:"errors"`      // rdevNumErrors
}

// ArrayStatus holds the full parsed state of the array.
type ArrayStatus struct {
	State        State      `json:"state"`
	NumDisks     int        `json:"num_disks"`
	NumInvalid   int        `json:"num_invalid"`
	Synced       bool       `json:"synced"`
	SyncedTime   string     `json:"synced_time"`
	ResyncActive bool       `json:"resync_active"`
	ResyncPct    float64    `json:"resync_pct"`
	Disks        []DiskInfo `json:"disks"`
}

// Manager controls the NonRAID array lifecycle.
type Manager struct {
	mu     sync.RWMutex
	sysMgr *system.Manager
	state  State
}

// NewManager creates a new array manager.
func NewManager(sysMgr *system.Manager) *Manager {
	return &Manager{
		sysMgr: sysMgr,
		state:  StateStopped,
	}
}

// Status returns the current array status by reading /proc/nmdstat.
func (m *Manager) Status() (*ArrayStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	raw, err := m.sysMgr.ReadNmdstat()
	if err != nil {
		return nil, fmt.Errorf("failed to read array status: %w", err)
	}

	return parseNmdstat(raw)
}

// Start starts the array.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == StateStarted {
		return fmt.Errorf("array is already started")
	}

	m.state = StateStarting
	out, err := m.sysMgr.RunNmdctl("start")
	if err != nil {
		m.state = StateError
		return fmt.Errorf("failed to start array: %s: %w", out, err)
	}

	m.state = StateStarted
	return nil
}

// Stop stops the array.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == StateStopped {
		return fmt.Errorf("array is already stopped")
	}

	m.state = StateStopping
	out, err := m.sysMgr.RunNmdctl("stop")
	if err != nil {
		m.state = StateError
		return fmt.Errorf("failed to stop array: %s: %w", out, err)
	}

	m.state = StateStopped
	return nil
}

// Check starts a parity check.
func (m *Manager) Check(mode string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.state != StateStarted {
		return fmt.Errorf("array must be started to run a check")
	}

	out, err := m.sysMgr.RunNmdctl("check", mode)
	if err != nil {
		return fmt.Errorf("failed to start check: %s: %w", out, err)
	}
	return nil
}

// humanSizeKB converts sectors (512 bytes) to human-readable.
func humanSizeKB(sectors int64) string {
	bytes := sectors * 512
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

// parseNmdstat parses the key=value output from /proc/nmdstat.
func parseNmdstat(raw string) (*ArrayStatus, error) {
	kv := make(map[string]string)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			kv[parts[0]] = parts[1]
		}
	}

	status := &ArrayStatus{
		State: State(kv["mdState"]),
	}

	// Parse disk count
	fmt.Sscanf(kv["mdNumDisks"], "%d", &status.NumDisks)
	fmt.Sscanf(kv["mdNumInvalid"], "%d", &status.NumInvalid)

	// Parse resync state
	if kv["mdResync"] == "1" {
		status.ResyncActive = true
		var pos, size int64
		fmt.Sscanf(kv["mdResyncPos"], "%d", &pos)
		fmt.Sscanf(kv["mdResyncSize"], "%d", &size)
		if size > 0 {
			status.ResyncPct = float64(pos) / float64(size) * 100
		}
	}

	// Parse synced state — sbSynced is a unix timestamp, nonzero = synced
	if kv["sbSynced"] != "" && kv["sbSynced"] != "0" {
		status.Synced = true
		status.SyncedTime = kv["sbSynced"]
	}

	// Parse all 30 disk slots using rdev* fields (the actual device info)
	for i := 0; i < 30; i++ {
		rdevStatus := kv[fmt.Sprintf("rdevStatus.%d", i)]

		// Skip empty slots (DISK_NP = not present)
		if rdevStatus == "" || rdevStatus == "DISK_NP" {
			continue
		}

		var rdevSize int64
		fmt.Sscanf(kv[fmt.Sprintf("rdevSize.%d", i)], "%d", &rdevSize)

		var reads, writes int64
		fmt.Sscanf(kv[fmt.Sprintf("rdevReads.%d", i)], "%d", &reads)
		fmt.Sscanf(kv[fmt.Sprintf("rdevWrites.%d", i)], "%d", &writes)

		var numErrors int
		fmt.Sscanf(kv[fmt.Sprintf("rdevNumErrors.%d", i)], "%d", &numErrors)

		disk := DiskInfo{
			Slot:       i,
			Status:     rdevStatus,
			DeviceName: kv[fmt.Sprintf("rdevName.%d", i)],
			VirtName:   kv[fmt.Sprintf("diskName.%d", i)],
			SizeBytes:  rdevSize * 512,
			SizeHuman:  humanSizeKB(rdevSize),
			DiskID:     kv[fmt.Sprintf("rdevId.%d", i)],
			Reads:      reads,
			Writes:     writes,
			Errors:     numErrors,
		}

		// Slot 0 = Parity, Slot 29 = Q Parity, rest = Data
		switch i {
		case 0:
			disk.Role = "parity"
		case 29:
			disk.Role = "q-parity"
		default:
			disk.Role = "data"
		}

		status.Disks = append(status.Disks, disk)
	}

	return status, nil
}
