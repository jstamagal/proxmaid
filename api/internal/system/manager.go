// Package system manages kernel module loading, proc interface access,
// and host-level operations (mount/unmount, modprobe, etc).
package system

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Manager handles host-level system operations.
type Manager struct {
	// MockMode disables real system calls for development/testing.
	MockMode bool
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

// mockNmdstat returns simulated nmdstat output for development.
func mockNmdstat() string {
	return `mdState=STARTED
sbName=/nonraid.dat
sbVersion=2
sbNumDisks=5
sbSynced=0
sbSynced2=0
mdNumStripes=1280
mdNumDisks=5
mdNumInvalid=0
mdResync=0
mdResyncPos=0
mdResyncSize=0
mdResyncDt=0
mdResyncDb=0
diskNumber.0=0
diskStatus.0=DISK_OK
diskName.0=sdb1
diskSize.0=2097152
diskNumber.1=1
diskStatus.1=DISK_OK
diskName.1=sdc1
diskSize.1=2097152
diskNumber.2=2
diskStatus.2=DISK_OK
diskName.2=sdd1
diskSize.2=2097152
diskNumber.3=3
diskStatus.3=DISK_OK
diskName.3=sde1
diskSize.3=2097152
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
Parity: /dev/sdb1 (2.0 GB) - OK
Disk 1: /dev/sdc1 (2.0 GB) - OK
Disk 2: /dev/sdd1 (2.0 GB) - OK
Disk 3: /dev/sde1 (2.0 GB) - OK
`
	default:
		return fmt.Sprintf("[MOCK] nmdctl %s executed successfully\n", strings.Join(args, " "))
	}
}
