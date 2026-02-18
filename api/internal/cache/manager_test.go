package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewManagerMockMode(t *testing.T) {
	m := NewManager(true, nil)
	if !m.mockMode {
		t.Error("expected mock mode to be enabled")
	}
	if !m.IsMergerfsInstalled() {
		t.Error("mock mode should report mergerfs as installed")
	}
}

func TestMockPoolsLoaded(t *testing.T) {
	m := NewManager(true, nil)
	pools := m.GetPools()
	if len(pools) != 2 {
		t.Errorf("expected 2 mock pools, got %d", len(pools))
	}
}

func TestGetPool(t *testing.T) {
	m := NewManager(true, nil)
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
	m := NewManager(true, nil)
	_, err := m.GetPool("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent pool")
	}
}

func TestCreatePool(t *testing.T) {
	m := NewManager(true, nil)
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
	m := NewManager(true, nil)
	_, err := m.CreatePool(CreatePoolRequest{
		Name:    "nvme-fast",
		Devices: []string{"/dev/sdf"},
	})
	if err == nil {
		t.Error("expected error for duplicate pool name")
	}
}

func TestCreatePoolValidation(t *testing.T) {
	m := NewManager(true, nil)

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
	m := NewManager(true, nil)
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
	m := NewManager(true, nil)
	err := m.DeletePool("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent pool")
	}
}

func TestMoverStatus(t *testing.T) {
	m := NewManager(true, nil)
	status := m.GetMoverStatus()
	if status.Config.Schedule != "40 3 * * *" {
		t.Errorf("expected default schedule, got %q", status.Config.Schedule)
	}
	if !status.Config.Enabled {
		t.Error("expected mover to be enabled by default")
	}
}

func TestUpdateMoverConfig(t *testing.T) {
	m := NewManager(true, nil)
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
	m := NewManager(true, nil)
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
	m := NewManager(true, nil)
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

func TestParseAgeThreshold(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{"1d", 24 * time.Hour, true},
		{"12h", 12 * time.Hour, true},
		{"30m", 30 * time.Minute, true},
		{"90m", 90 * time.Minute, true},
		{"7d", 7 * 24 * time.Hour, true},
		{"1s", 1 * time.Second, true},
		{"1D", 24 * time.Hour, true},
		{"12H", 12 * time.Hour, true},
		{"30M", 30 * time.Minute, true},
		{"", 24 * time.Hour, true},
		{"bad", 0, false},
		{"1x", 0, false},
	}
	for _, tt := range tests {
		got, err := ParseAgeThreshold(tt.in)
		if tt.ok && err != nil {
			t.Errorf("ParseAgeThreshold(%q): %v", tt.in, err)
			continue
		}
		if !tt.ok && err == nil {
			t.Errorf("ParseAgeThreshold(%q): expected error", tt.in)
			continue
		}
		if tt.ok && got != tt.want {
			t.Errorf("ParseAgeThreshold(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestFindColdFiles(t *testing.T) {
	dir := t.TempDir()

	oldFile := filepath.Join(dir, "old.txt")
	newFile := filepath.Join(dir, "new.txt")
	if err := os.WriteFile(oldFile, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newFile, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	// Make old.txt have mtime in the past
	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(oldFile, past, past); err != nil {
		t.Fatal(err)
	}

	// Threshold 1h: only old.txt is cold
	got, err := FindColdFiles(dir, 1*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 cold file, got %d: %v", len(got), got)
	}
	if len(got) > 0 && filepath.Base(got[0]) != "old.txt" {
		t.Errorf("expected old.txt, got %s", got[0])
	}

	// Threshold 30m: both files cold (old is 2h ago, new is recent but we only check mtime)
	got, err = FindColdFiles(dir, 30*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	// old.txt is 2h old, new.txt is fresh; so only old.txt is cold with 30m threshold
	if len(got) != 1 {
		t.Errorf("expected 1 cold file with 30m threshold, got %d", len(got))
	}
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name     string
		poolName string
		valid    bool
	}{
		{"valid simple", "pool0", true},
		{"valid with hyphen", "fast-pool", true},
		{"valid with underscore", "cache_pool", true},
		{"valid alphanumeric", "pool123", true},
		{"invalid empty", "", false},
		{"invalid with slash", "pool/name", false},
		{"invalid with dot", "pool.name", false},
		{"invalid with space", "pool name", false},
		{"invalid path traversal", "../pool", false},
		{"invalid special chars", "pool@name", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidPoolName(tt.poolName)
			if result != tt.valid {
				t.Errorf("isValidPoolName(%q) = %v, want %v", tt.poolName, result, tt.valid)
			}
		})
	}
}

func TestDevicePathValidation(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		valid bool
	}{
		{"valid /dev/sda", "/dev/sda", true},
		{"valid /dev/nvme0n1", "/dev/nvme0n1", true},
		{"valid /dev/sda1", "/dev/sda1", true},
		{"invalid empty", "", false},
		{"invalid relative", "dev/sda", false},
		{"invalid absolute non-dev", "/home/user/file", false},
		{"invalid path traversal", "/dev/../etc/passwd", false},
		{"invalid path traversal 2", "/dev/sda/../../../etc/passwd", false},
		{"valid with subdirs", "/dev/disk/by-id/ata-Samsung", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidDevicePath(tt.path)
			if result != tt.valid {
				t.Errorf("isValidDevicePath(%q) = %v, want %v", tt.path, result, tt.valid)
			}
		})
	}
}

func TestGetPartitionPath(t *testing.T) {
	tests := []struct {
		device   string
		expected string
	}{
		{"/dev/sda", "/dev/sda1"},
		{"/dev/sdb", "/dev/sdb1"},
		{"/dev/nvme0n1", "/dev/nvme0n1p1"},
		{"/dev/nvme1n1", "/dev/nvme1n1p1"},
		{"/dev/mmcblk0", "/dev/mmcblk0p1"},
		{"/dev/mmcblk1", "/dev/mmcblk1p1"},
	}

	for _, tt := range tests {
		t.Run(tt.device, func(t *testing.T) {
			result := getPartitionPath(tt.device)
			if result != tt.expected {
				t.Errorf("getPartitionPath(%q) = %q, want %q", tt.device, result, tt.expected)
			}
		})
	}
}

func TestCreatePoolInvalidName(t *testing.T) {
	m := NewManager(true, nil)
	_, err := m.CreatePool(CreatePoolRequest{
		Name:    "../malicious",
		Devices: []string{"/dev/sda"},
		FSType:  "xfs",
	})
	if err == nil {
		t.Error("expected error for invalid pool name with path traversal")
	}
}

func TestCreatePoolInvalidDevicePath(t *testing.T) {
	m := NewManager(true, nil)
	_, err := m.CreatePool(CreatePoolRequest{
		Name:    "valid-pool",
		Devices: []string{"/etc/passwd"},
		FSType:  "xfs",
	})
	if err == nil {
		t.Error("expected error for invalid device path")
	}
}
