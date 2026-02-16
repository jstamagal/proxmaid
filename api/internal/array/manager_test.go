package array

import (
	"strings"
	"testing"
)

func TestParseNmdstat_Started(t *testing.T) {
	raw := `mdState=STARTED
sbName=/nonraid.dat
sbVersion=2
sbNumDisks=4
sbSynced=1
sbSynced2=0
mdNumStripes=1280
mdNumDisks=4
mdNumInvalid=0
mdResync=0
mdResyncPos=0
mdResyncSize=0
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
	if len(status.Disks) != 4 {
		t.Fatalf("expected 4 disk entries, got %d", len(status.Disks))
	}

	// Slot 0 should be parity
	if status.Disks[0].Role != "parity" {
		t.Errorf("expected slot 0 role 'parity', got '%s'", status.Disks[0].Role)
	}
	if status.Disks[0].DeviceName != "sdb1" {
		t.Errorf("expected slot 0 device 'sdb1', got '%s'", status.Disks[0].DeviceName)
	}

	// Remaining slots should be data
	for i := 1; i < 4; i++ {
		if status.Disks[i].Role != "data" {
			t.Errorf("expected slot %d role 'data', got '%s'", i, status.Disks[i].Role)
		}
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
diskStatus.0=DISK_OK
diskName.0=sdb1
diskSize.0=2097152
diskNumber.1=1
diskStatus.1=DISK_DSBL
diskName.1=
diskSize.1=0
diskNumber.2=2
diskStatus.2=DISK_OK
diskName.2=sdd1
diskSize.2=2097152
diskNumber.3=3
diskStatus.3=DISK_OK
diskName.3=sde1
diskSize.3=2097152
`
	status, err := parseNmdstat(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.NumInvalid != 1 {
		t.Errorf("expected 1 invalid, got %d", status.NumInvalid)
	}
	if status.Disks[1].Status != "DISK_DSBL" {
		t.Errorf("expected slot 1 status 'DISK_DSBL', got '%s'", status.Disks[1].Status)
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
