package app

import "testing"

func newMockManager() *Manager {
	return NewManager(true)
}

func TestListContainers(t *testing.T) {
	m := newMockManager()
	containers, err := m.ListContainers()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(containers) != 3 {
		t.Errorf("expected 3 mock containers, got %d", len(containers))
	}
}

func TestContainerStates(t *testing.T) {
	m := newMockManager()
	containers, _ := m.ListContainers()

	running := 0
	exited := 0
	for _, c := range containers {
		switch c.State {
		case "running":
			running++
		case "exited":
			exited++
		}
	}
	if running != 2 {
		t.Errorf("expected 2 running, got %d", running)
	}
	if exited != 1 {
		t.Errorf("expected 1 exited, got %d", exited)
	}
}

func TestStartContainer(t *testing.T) {
	m := newMockManager()
	if err := m.StartContainer("a1b2c3d4e5f6"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStopContainer(t *testing.T) {
	m := newMockManager()
	if err := m.StopContainer("a1b2c3d4e5f6"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveContainer(t *testing.T) {
	m := newMockManager()
	if err := m.RemoveContainer("a1b2c3d4e5f6"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetLogs(t *testing.T) {
	m := newMockManager()
	logs, err := m.GetLogs("a1b2c3d4e5f6", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if logs == "" {
		t.Error("expected non-empty logs")
	}
}

func TestGetStats(t *testing.T) {
	m := newMockManager()
	stats, err := m.GetStats("a1b2c3d4e5f6")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.CPUPercent <= 0 {
		t.Error("expected non-zero CPU percent")
	}
	if stats.MemUsage <= 0 {
		t.Error("expected non-zero memory usage")
	}
}

func TestComposeUp(t *testing.T) {
	m := newMockManager()
	if err := m.ComposeUp("/opt/stacks/media/docker-compose.yml"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestComposeDown(t *testing.T) {
	m := newMockManager()
	if err := m.ComposeDown("/opt/stacks/media/docker-compose.yml"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
