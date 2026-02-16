package system

import (
	"testing"
)

func TestNewManager_MockMode(t *testing.T) {
	// On a dev machine without /proc/nmdstat, mock mode should be enabled
	m := NewManager()
	// We can't guarantee /proc/nmdstat exists or doesn't exist,
	// so just verify the manager initializes without panic
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestMockNmdstat(t *testing.T) {
	m := &Manager{MockMode: true}
	raw, err := m.ReadNmdstat()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if raw == "" {
		t.Error("expected non-empty mock nmdstat output")
	}
}

func TestMockIsModuleLoaded(t *testing.T) {
	m := &Manager{MockMode: true}
	if !m.IsModuleLoaded() {
		t.Error("mock mode should report module as loaded")
	}
}

func TestMockRunNmdctl(t *testing.T) {
	m := &Manager{MockMode: true}
	out, err := m.RunNmdctl("status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Error("expected non-empty mock nmdctl output")
	}
}

func TestMockLoadModule(t *testing.T) {
	m := &Manager{MockMode: true}
	err := m.LoadModule("/nonraid.dat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMockUnloadModule(t *testing.T) {
	m := &Manager{MockMode: true}
	err := m.UnloadModule()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMockWriteNmdcmd(t *testing.T) {
	m := &Manager{MockMode: true}
	err := m.WriteNmdcmd("importdisk 1 sdc1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
