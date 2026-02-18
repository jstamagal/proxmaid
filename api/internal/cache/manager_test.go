package cache

import (
	"testing"
)

func TestNewManagerMockMode(t *testing.T) {
	m := NewManager(true)
	if !m.mockMode {
		t.Error("expected mock mode to be enabled")
	}
	if !m.IsMergerfsInstalled() {
		t.Error("mock mode should report mergerfs as installed")
	}
}

func TestMockPoolsLoaded(t *testing.T) {
	m := NewManager(true)
	pools := m.GetPools()
	if len(pools) != 2 {
		t.Errorf("expected 2 mock pools, got %d", len(pools))
	}
}

func TestGetPool(t *testing.T) {
	m := NewManager(true)
	pool, err := m.GetPool("nvme-fast")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pool.Name != "nvme-fast" {
		t.Errorf("expected pool name 'nvme-fast', got %q", pool.Name)
	}
	if len(pool.Devices) != 3 {
		t.Errorf("expected 3 devices in nvme-fast pool, got %d", len(pool.Devices))
	}
}

func TestGetPoolNotFound(t *testing.T) {
	m := NewManager(true)
	_, err := m.GetPool("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent pool")
	}
}

func TestCreatePool(t *testing.T) {
	m := NewManager(true)
	pool, err := m.CreatePool(CreatePoolRequest{
		Name:    "test-pool",
		Devices: []string{"/dev/sdf"},
		FSType:  "xfs",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pool.Name != "test-pool" {
		t.Errorf("expected pool name 'test-pool', got %q", pool.Name)
	}
	if pool.MountPoint != "/mnt/cache/test-pool" {
		t.Errorf("unexpected mount point: %s", pool.MountPoint)
	}
}

func TestCreatePoolDuplicate(t *testing.T) {
	m := NewManager(true)
	_, err := m.CreatePool(CreatePoolRequest{
		Name:    "nvme-fast",
		Devices: []string{"/dev/sdf"},
	})
	if err == nil {
		t.Error("expected error for duplicate pool name")
	}
}

func TestCreatePoolValidation(t *testing.T) {
	m := NewManager(true)

	// Empty name
	_, err := m.CreatePool(CreatePoolRequest{
		Devices: []string{"/dev/sdf"},
	})
	if err == nil {
		t.Error("expected error for empty pool name")
	}

	// No devices
	_, err = m.CreatePool(CreatePoolRequest{
		Name: "bad-pool",
	})
	if err == nil {
		t.Error("expected error for no devices")
	}
}

func TestDeletePool(t *testing.T) {
	m := NewManager(true)
	err := m.DeletePool("ssd-warm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = m.GetPool("ssd-warm")
	if err == nil {
		t.Error("expected pool to be deleted")
	}
}

func TestDeletePoolNotFound(t *testing.T) {
	m := NewManager(true)
	err := m.DeletePool("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent pool")
	}
}

func TestMoverStatus(t *testing.T) {
	m := NewManager(true)
	status := m.GetMoverStatus()
	if status == nil {
		t.Fatal("expected non-nil mover status")
	}
	if status.Config.Schedule != "40 3 * * *" {
		t.Errorf("expected default schedule, got %q", status.Config.Schedule)
	}
	if !status.Config.Enabled {
		t.Error("expected mover to be enabled by default")
	}
}

func TestUpdateMoverConfig(t *testing.T) {
	m := NewManager(true)
	err := m.UpdateMoverConfig(MoverConfig{
		Schedule:     "0 4 * * *",
		AgeThreshold: "2d",
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	status := m.GetMoverStatus()
	if status.Config.Schedule != "0 4 * * *" {
		t.Errorf("expected updated schedule, got %q", status.Config.Schedule)
	}
}

func TestUpdateMoverConfigValidation(t *testing.T) {
	m := NewManager(true)
	err := m.UpdateMoverConfig(MoverConfig{
		AgeThreshold: "1d",
	})
	if err == nil {
		t.Error("expected error for empty schedule")
	}

	err = m.UpdateMoverConfig(MoverConfig{
		Schedule: "40 3 * * *",
	})
	if err == nil {
		t.Error("expected error for empty age threshold")
	}
}

func TestRunMover(t *testing.T) {
	m := NewManager(true)
	err := m.RunMover()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	status := m.GetMoverStatus()
	if !status.Running {
		t.Error("expected mover to be running after trigger")
	}

	// Double run should fail
	err = m.RunMover()
	if err == nil {
		t.Error("expected error when mover is already running")
	}
}
