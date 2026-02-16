package disk

import (
	"testing"
)

func TestMockListDisks(t *testing.T) {
	m := NewManager(true)
	disks, err := m.ListDisks()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(disks) != 5 {
		t.Errorf("expected 5 mock disks, got %d", len(disks))
	}

	// First disk should be SSD
	if disks[0].Rotational {
		t.Error("expected first disk to be SSD (non-rotational)")
	}
	if disks[0].SizeHuman == "" {
		t.Error("expected non-empty human size")
	}
}

func TestMockSmartHealth(t *testing.T) {
	m := NewManager(true)
	health, err := m.GetSmartHealth("/dev/sdb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !health.Healthy {
		t.Error("expected mock disk to be healthy")
	}
	if health.Temperature <= 0 {
		t.Error("expected positive temperature")
	}
}

func TestMockWipeDisk(t *testing.T) {
	m := NewManager(true)
	if err := m.WipeDisk("/dev/sdb"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMockPartitionDisk(t *testing.T) {
	m := NewManager(true)
	if err := m.PartitionDisk("/dev/sdb"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMockFormatPartition(t *testing.T) {
	m := NewManager(true)

	for _, fs := range []string{"xfs", "btrfs", "ext4"} {
		if err := m.FormatPartition("/dev/sdb1", fs); err != nil {
			t.Errorf("unexpected error formatting %s: %v", fs, err)
		}
	}
}

func TestMockFormatUnsupported(t *testing.T) {
	m := NewManager(true)
	err := m.FormatPartition("/dev/sdb1", "ntfs")
	if err == nil {
		t.Error("expected error for unsupported filesystem")
	}
}

func TestHumanSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{500, "500 B"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{2000398934016, "1.8 TB"},
	}

	for _, tc := range tests {
		got := humanSize(tc.bytes)
		if got != tc.expected {
			t.Errorf("humanSize(%d) = %q, want %q", tc.bytes, got, tc.expected)
		}
	}
}
