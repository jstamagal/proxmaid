package array

import (
	"strings"
	"testing"

	"github.com/proxmaid/proxmaid/internal/system"
)

func TestParseNmdstat_Started(t *testing.T) {
	raw := `mdState=STARTED
sbName=/nonraid.dat
sbVersion=2.9.35
sbNumDisks=4
sbSynced=1771262613
sbSynced2=1771273972
mdNumDisks=4
mdNumInvalid=0
mdResync=0
mdResyncPos=0
mdResyncSize=976760832
diskNumber.0=0
diskName.0=
diskSize.0=976760832
diskState.0=7
rdevNumber.0=0
rdevStatus.0=DISK_OK
rdevName.0=nvme0n1p1
rdevSize.0=976760832
rdevId.0=nvme-SK_Hynix_P41_MOCK001
rdevReads.0=0
rdevWrites.0=0
rdevNumErrors.0=0
diskNumber.1=1
diskName.1=nmd1p1
diskSize.1=3906250000
diskState.1=7
rdevNumber.1=1
rdevStatus.1=DISK_OK
rdevName.1=sdb1
rdevSize.1=3906250000
rdevId.1=ata-WDC_WD20EFRX_MOCK001
rdevReads.1=163
rdevWrites.1=42
rdevNumErrors.1=0
diskNumber.2=2
diskName.2=nmd2p1
diskSize.2=3906250000
diskState.2=7
rdevNumber.2=2
rdevStatus.2=DISK_OK
rdevName.2=sdc1
rdevSize.2=3906250000
rdevId.2=ata-WDC_WD20EFRX_MOCK002
rdevReads.2=163
rdevWrites.2=38
rdevNumErrors.2=0
diskNumber.3=3
diskName.3=nmd3p1
diskSize.3=7812500000
diskState.3=7
rdevNumber.3=3
rdevStatus.3=DISK_OK
rdevName.3=sdd1
rdevSize.3=7812500000
rdevId.3=ata-Seagate_IronWolf_MOCK001
rdevReads.3=245
rdevWrites.3=120
rdevNumErrors.3=0
diskNumber.4=4
diskName.4=
diskSize.4=0
diskState.4=0
rdevNumber.4=4
rdevStatus.4=DISK_NP
rdevName.4=
rdevSize.4=0
rdevReads.4=0
rdevWrites.4=0
rdevNumErrors.4=0
`
	status, err := parseNmdstat(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.State != StateStarted {
		t.Errorf("expected state STARTED, got %s", status.State)
	}
	if status.NumDisks != 4 {
		t.Errorf("expected 4 disks, got %d", status.NumDisks)
	}
	if status.NumInvalid != 0 {
		t.Errorf("expected 0 invalid, got %d", status.NumInvalid)
	}
	if status.ResyncActive {
		t.Error("expected resync inactive")
	}
	// Should have 4 active disks (DISK_NP slots are filtered out)
	if len(status.Disks) != 4 {
		t.Fatalf("expected 4 disk entries, got %d", len(status.Disks))
	}

	// Slot 0 should be parity with real device name
	if status.Disks[0].Role != "parity" {
		t.Errorf("expected slot 0 role 'parity', got '%s'", status.Disks[0].Role)
	}
	if status.Disks[0].DeviceName != "nvme0n1p1" {
		t.Errorf("expected slot 0 device 'nvme0n1p1', got '%s'", status.Disks[0].DeviceName)
	}
	if status.Disks[0].DiskID != "nvme-SK_Hynix_P41_MOCK001" {
		t.Errorf("expected slot 0 disk_id, got '%s'", status.Disks[0].DiskID)
	}

	// Data disks
	for i := 1; i < 4; i++ {
		if status.Disks[i].Role != "data" {
			t.Errorf("expected slot %d role 'data', got '%s'", i, status.Disks[i].Role)
		}
		if status.Disks[i].SizeHuman == "" {
			t.Errorf("expected non-empty SizeHuman for slot %d", i)
		}
	}

	// Check synced
	if !status.Synced {
		t.Error("expected synced = true")
	}
}

func TestParseNmdstat_Stopped(t *testing.T) {
	raw := `mdState=STOPPED
sbName=/nonraid.dat
mdNumDisks=0
mdNumInvalid=0
mdResync=0
`
	status, err := parseNmdstat(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.State != StateStopped {
		t.Errorf("expected state STOPPED, got %s", status.State)
	}
	if status.NumDisks != 0 {
		t.Errorf("expected 0 disks, got %d", status.NumDisks)
	}
}

func TestParseNmdstat_Degraded(t *testing.T) {
	raw := `mdState=STARTED
mdNumDisks=4
mdNumInvalid=1
mdResync=0
diskNumber.0=0
diskName.0=
rdevStatus.0=DISK_OK
rdevName.0=nvme0n1p1
rdevSize.0=976760832
rdevReads.0=0
rdevWrites.0=0
rdevNumErrors.0=0
diskNumber.1=1
diskName.1=
rdevStatus.1=DISK_DSBL
rdevName.1=
rdevSize.1=0
rdevReads.1=0
rdevWrites.1=0
rdevNumErrors.1=0
diskNumber.2=2
diskName.2=nmd2p1
rdevStatus.2=DISK_OK
rdevName.2=sdc1
rdevSize.2=3906250000
rdevReads.2=0
rdevWrites.2=0
rdevNumErrors.2=0
diskNumber.3=3
diskName.3=nmd3p1
rdevStatus.3=DISK_OK
rdevName.3=sdd1
rdevSize.3=7812500000
rdevReads.3=0
rdevWrites.3=0
rdevNumErrors.3=0
`
	status, err := parseNmdstat(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.NumInvalid != 1 {
		t.Errorf("expected 1 invalid, got %d", status.NumInvalid)
	}
	// Slot 1 has DISK_DSBL — should be present in output
	found := false
	for _, d := range status.Disks {
		if d.Slot == 1 && d.Status == "DISK_DSBL" {
			found = true
		}
	}
	if !found {
		t.Error("expected slot 1 with DISK_DSBL status")
	}
}

func TestParseNmdstat_ResyncActive(t *testing.T) {
	raw := `mdState=STARTED
mdNumDisks=4
mdNumInvalid=0
mdResync=1
mdResyncPos=524288
mdResyncSize=2097152
`
	status, err := parseNmdstat(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !status.ResyncActive {
		t.Error("expected resync to be active")
	}
	if status.ResyncPct < 24.9 || status.ResyncPct > 25.1 {
		t.Errorf("expected ~25%% resync progress, got %.2f%%", status.ResyncPct)
	}
}

func TestParseNmdstat_EmptyInput(t *testing.T) {
	status, err := parseNmdstat("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.State != "" {
		t.Errorf("expected empty state, got '%s'", status.State)
	}
}

func TestParseNmdstat_MalformedLines(t *testing.T) {
	raw := `mdState=STARTED
this line has no equals
mdNumDisks=3
`
	status, err := parseNmdstat(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.State != StateStarted {
		t.Errorf("expected STARTED, got %s", status.State)
	}
	_ = strings.Contains("", "") // silence import
}

func TestAssignDiskInvalidPath(t *testing.T) {
	// This test verifies path validation happens before checking array state
	// In mock mode, AssignDisk will check validation first
	sysMgr := &system.Manager{MockMode: true}
	m := NewManager(sysMgr)
	
	// Test invalid device path - should fail validation before checking array state
	err := m.AssignDisk(0, "/etc/passwd")
	if err == nil {
		t.Error("expected error for invalid device path")
	}
	if !strings.Contains(err.Error(), "invalid device path") {
		t.Errorf("expected 'invalid device path' error, got: %v", err)
	}
	
	// Test path traversal attempt - should fail validation
	err = m.AssignDisk(0, "/dev/../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal attempt")
	}
	if !strings.Contains(err.Error(), "invalid device path") {
		t.Errorf("expected 'invalid device path' error, got: %v", err)
	}
}

func TestAssignDiskValidPath(t *testing.T) {
	// This test verifies valid paths pass validation
	// In mock mode, array is always STARTED, so this will fail with "array must be stopped"
	sysMgr := &system.Manager{MockMode: true}
	m := NewManager(sysMgr)
	
	// Test valid device path - should pass validation but fail on array state check
	err := m.AssignDisk(0, "/dev/sda1")
	if err == nil {
		t.Error("expected error (mock array is STARTED, not STOPPED)")
	}
	// Should NOT be a validation error
	if strings.Contains(err.Error(), "invalid device path") {
		t.Errorf("should not be a validation error, got: %v", err)
	}
	// Should be an array state error
	if !strings.Contains(err.Error(), "array must be stopped") {
		t.Errorf("expected 'array must be stopped' error, got: %v", err)
	}
}
