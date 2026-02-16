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
	Status     string `json:"status"`
	DeviceName string `json:"device_name"`
	SizeBytes  int64  `json:"size_bytes"`
	Role       string `json:"role"` // "parity", "data", "q-parity"
}

// ArrayStatus holds the full parsed state of the array.
type ArrayStatus struct {
	State        State      `json:"state"`
	NumDisks     int        `json:"num_disks"`
	NumInvalid   int        `json:"num_invalid"`
	Synced       bool       `json:"synced"`
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

	// Parse resync
	if kv["mdResync"] == "1" {
		status.ResyncActive = true
	}

	// Parse synced state
	status.Synced = kv["sbSynced"] == "1" || kv["sbSynced"] == "0"

	// Parse individual disks
	for i := 0; i < status.NumDisks; i++ {
		disk := DiskInfo{
			Slot:       i,
			Status:     kv[fmt.Sprintf("diskStatus.%d", i)],
			DeviceName: kv[fmt.Sprintf("diskName.%d", i)],
		}
		fmt.Sscanf(kv[fmt.Sprintf("diskSize.%d", i)], "%d", &disk.SizeBytes)

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
