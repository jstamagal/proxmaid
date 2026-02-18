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
