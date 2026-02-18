// Package system manages kernel module loading, proc interface access,
// and host-level operations (mount/unmount, modprobe, etc).
package system

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// UPSStatus holds UPS state from Network UPS Tools (NUT).
type UPSStatus struct {
	Online     bool    `json:"online"`
	BatteryPct int     `json:"battery_pct"`
	RuntimeSec int     `json:"runtime_sec"`
	Load       float64 `json:"load"`
}

// LogEntry holds a parsed journalctl log entry.
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Unit      string `json:"unit"`
	Priority  int    `json:"priority"`
	Message   string `json:"message"`
}

// Manager handles host-level system operations.
type Manager struct {
	// MockMode disables real system calls for development/testing.
	MockMode bool
	// mockTimezone stores the timezone when in mock mode.
	mockTimezone string
}

// NewManager creates a new system manager.
// Automatically enables MockMode if the NonRAID proc interface is not present.
func NewManager() *Manager {
	m := &Manager{}
	if _, err := os.Stat("/proc/nmdstat"); os.IsNotExist(err) {
		fmt.Println("[WARN] /proc/nmdstat not found. Running in MOCK MODE.")
		m.MockMode = true
	}
	return m
}

// IsModuleLoaded checks if the md_nonraid kernel module is loaded.
func (m *Manager) IsModuleLoaded() bool {
	if m.MockMode {
		return true
	}
	out, err := exec.Command("lsmod").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "md_nonraid")
}

// LoadModule loads the md_nonraid kernel module via modprobe.
func (m *Manager) LoadModule(superblockPath string) error {
	if m.MockMode {
		fmt.Printf("[MOCK] modprobe md_nonraid super=%s\n", superblockPath)
		return nil
	}
	cmd := exec.Command("modprobe", "md_nonraid", fmt.Sprintf("super=%s", superblockPath))
	return cmd.Run()
}

// UnloadModule removes the md_nonraid kernel module.
func (m *Manager) UnloadModule() error {
	if m.MockMode {
		fmt.Println("[MOCK] modprobe -r md_nonraid")
		return nil
	}
	return exec.Command("modprobe", "-r", "md_nonraid").Run()
}

// ReadNmdstat reads and returns the full contents of /proc/nmdstat.
func (m *Manager) ReadNmdstat() (string, error) {
	if m.MockMode {
		return mockNmdstat(), nil
	}
	data, err := os.ReadFile("/proc/nmdstat")
	if err != nil {
		return "", fmt.Errorf("failed to read /proc/nmdstat: %w", err)
	}
	return string(data), nil
}

// WriteNmdcmd writes a command string to /proc/nmdcmd.
func (m *Manager) WriteNmdcmd(command string) error {
	if m.MockMode {
		fmt.Printf("[MOCK] echo '%s' > /proc/nmdcmd\n", command)
		return nil
	}
	return os.WriteFile("/proc/nmdcmd", []byte(command), 0644)
}

// RunNmdctl executes an nmdctl command and returns its output.
func (m *Manager) RunNmdctl(args ...string) (string, error) {
	if m.MockMode {
		fmt.Printf("[MOCK] nmdctl %s\n", strings.Join(args, " "))
		return mockNmdctlOutput(args), nil
	}
	cmd := exec.Command("nmdctl", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// GetUPSStatus queries NUT (upsc) for UPS state.
func (m *Manager) GetUPSStatus() (*UPSStatus, error) {
	if m.MockMode {
		return &UPSStatus{
			Online:     true,
			BatteryPct: 100,
			RuntimeSec: 3600,
			Load:       12.5,
		}, nil
	}

	out, err := exec.Command("upsc", "ups@localhost").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("upsc failed: %s: %w", string(out), err)
	}

	kv := make(map[string]string)
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) == 2 {
			kv[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	status := &UPSStatus{
		Online: kv["ups.status"] == "OL",
	}
	if v, err := strconv.Atoi(kv["battery.charge"]); err == nil {
		status.BatteryPct = v
	}
	if v, err := strconv.Atoi(kv["battery.runtime"]); err == nil {
		status.RuntimeSec = v
	}
	if v, err := strconv.ParseFloat(kv["ups.load"], 64); err == nil {
		status.Load = v
	}

	return status, nil
}

// GetLogs returns recent system log entries from journalctl.
func (m *Manager) GetLogs(lines int, unit string) ([]LogEntry, error) {
	if m.MockMode {
		now := time.Now().UTC().Format(time.RFC3339)
		return []LogEntry{
			{Timestamp: now, Unit: "proxmaid.service", Priority: 6, Message: "Proxmaid API started on :8484"},
			{Timestamp: now, Unit: "proxmaid.service", Priority: 6, Message: "Mock mode enabled"},
			{Timestamp: now, Unit: "kernel", Priority: 4, Message: "md_nonraid: module loaded"},
			{Timestamp: now, Unit: "systemd", Priority: 6, Message: "Started Proxmaid Storage Management API"},
		}, nil
	}

	args := []string{"--no-pager", "-n", strconv.Itoa(lines), "-o", "json"}
	if unit != "" {
		args = append(args, "-u", unit)
	}

	out, err := exec.Command("journalctl", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("journalctl failed: %w", err)
	}

	var entries []LogEntry
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		entry := LogEntry{
			Message: fmt.Sprintf("%v", raw["MESSAGE"]),
		}
		if v, ok := raw["__REALTIME_TIMESTAMP"].(string); ok {
			if usec, err := strconv.ParseInt(v, 10, 64); err == nil {
				entry.Timestamp = time.UnixMicro(usec).UTC().Format(time.RFC3339)
			}
		}
		if v, ok := raw["_SYSTEMD_UNIT"].(string); ok {
			entry.Unit = v
		}
		if v, ok := raw["PRIORITY"].(string); ok {
			entry.Priority, _ = strconv.Atoi(v)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// GetTimezone returns the current system timezone.
func (m *Manager) GetTimezone() (string, error) {
	if m.MockMode {
		if m.mockTimezone == "" {
			return "America/New_York", nil
		}
		return m.mockTimezone, nil
	}

	out, err := exec.Command("timedatectl", "show", "-p", "Timezone", "--value").Output()
	if err != nil {
		return "", fmt.Errorf("timedatectl failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// SetTimezone sets the system timezone.
func (m *Manager) SetTimezone(tz string) error {
	if tz == "" {
		return fmt.Errorf("timezone is required")
	}

	if m.MockMode {
		fmt.Printf("[MOCK] timedatectl set-timezone %s\n", tz)
		m.mockTimezone = tz
		return nil
	}

	out, err := exec.Command("timedatectl", "set-timezone", tz).CombinedOutput()
	if err != nil {
		return fmt.Errorf("timedatectl failed: %s: %w", string(out), err)
	}
	return nil
}

// mockNmdstat returns simulated nmdstat output for development.
func mockNmdstat() string {
	return `sbName=/nonraid.dat
sbVersion=2.9.35
sbCreated=1771262354
sbUpdated=1771273972
sbEvents=3
sbState=1
sbNumDisks=4
sbSynced=1771262613
sbSynced2=1771273972
sbSyncErrs=0
mdVersion=2.9.35
mdState=STARTED
mdNumDisks=4
mdNumDisabled=0
mdNumReplaced=0
mdNumInvalid=0
mdNumMissing=0
mdResyncAction=check P
mdResyncSize=976760832
mdResync=0
mdResyncPos=0
mdResyncDt=0
mdResyncDb=0
diskNumber.0=0
diskName.0=
diskSize.0=976760832
diskState.0=7
diskId.0=nvme-SK_Hynix_P41_MOCK001
rdevNumber.0=0
rdevStatus.0=DISK_OK
rdevName.0=nvme0n1p1
rdevOffset.0=0
rdevSize.0=976760832
rdevId.0=nvme-SK_Hynix_P41_MOCK001
rdevReads.0=0
rdevWrites.0=0
rdevNumErrors.0=0
diskNumber.1=1
diskName.1=nmd1p1
diskSize.1=3906250000
diskState.1=7
diskId.1=ata-WDC_WD20EFRX_MOCK001
rdevNumber.1=1
rdevStatus.1=DISK_OK
rdevName.1=sdb1
rdevOffset.1=0
rdevSize.1=3906250000
rdevId.1=ata-WDC_WD20EFRX_MOCK001
rdevReads.1=163
rdevWrites.1=42
rdevNumErrors.1=0
diskNumber.2=2
diskName.2=nmd2p1
diskSize.2=3906250000
diskState.2=7
diskId.2=ata-WDC_WD20EFRX_MOCK002
rdevNumber.2=2
rdevStatus.2=DISK_OK
rdevName.2=sdc1
rdevOffset.2=0
rdevSize.2=3906250000
rdevId.2=ata-WDC_WD20EFRX_MOCK002
rdevReads.2=163
rdevWrites.2=38
rdevNumErrors.2=0
diskNumber.3=3
diskName.3=nmd3p1
diskSize.3=7812500000
diskState.3=7
diskId.3=ata-Seagate_IronWolf_MOCK001
rdevNumber.3=3
rdevStatus.3=DISK_OK
rdevName.3=sdd1
rdevOffset.3=0
rdevSize.3=7812500000
rdevId.3=ata-Seagate_IronWolf_MOCK001
rdevReads.3=245
rdevWrites.3=120
rdevNumErrors.3=0
diskNumber.4=4
diskName.4=
diskSize.4=0
diskState.4=0
diskId.4=
rdevNumber.4=4
rdevStatus.4=DISK_NP
rdevName.4=
rdevOffset.4=0
rdevSize.4=0
rdevId.4=
rdevReads.4=0
rdevWrites.4=0
rdevNumErrors.4=0
`
}

// mockNmdctlOutput returns simulated nmdctl output for development.
func mockNmdctlOutput(args []string) string {
	if len(args) == 0 {
		return ""
	}
	switch args[0] {
	case "status":
		return `Array Status: STARTED
Parity: /dev/nvme0n1p1 (931.5 GB) - OK
Disk 1: /dev/sdb1 (1.8 TB) - OK
Disk 2: /dev/sdc1 (1.8 TB) - OK
Disk 3: /dev/sdd1 (3.6 TB) - OK
`
	default:
		return fmt.Sprintf("[MOCK] nmdctl %s executed successfully\n", strings.Join(args, " "))
	}
}
