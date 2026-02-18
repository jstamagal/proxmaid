package scheduler

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestRegisterAndListTasks(t *testing.T) {
	m := NewManager(true)
	m.RegisterTask(ScheduledTask{
		Name:     "test_task",
		Schedule: "0 * * * *",
		Enabled:  true,
	})

	tasks := m.ListTasks()
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Name != "test_task" {
		t.Errorf("expected name 'test_task', got %q", tasks[0].Name)
	}
}

func TestGetTask(t *testing.T) {
	m := NewManager(true)
	m.RegisterTask(ScheduledTask{
		Name:     "my_task",
		Schedule: "*/5 * * * *",
		Enabled:  true,
	})

	task, err := m.GetTask("my_task")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.Schedule != "*/5 * * * *" {
		t.Errorf("expected schedule '*/5 * * * *', got %q", task.Schedule)
	}
}

func TestGetTaskNotFound(t *testing.T) {
	m := NewManager(true)
	_, err := m.GetTask("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestUpdateTask(t *testing.T) {
	m := NewManager(true)
	m.RegisterTask(ScheduledTask{
		Name:     "updatable",
		Schedule: "0 * * * *",
		Enabled:  true,
	})

	err := m.UpdateTask("updatable", "30 2 * * *", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	task, _ := m.GetTask("updatable")
	if task.Schedule != "30 2 * * *" {
		t.Errorf("expected updated schedule, got %q", task.Schedule)
	}
	if task.Enabled {
		t.Error("expected task to be disabled")
	}
}

func TestTriggerTask(t *testing.T) {
	m := NewManager(true)
	var ran atomic.Int32

	m.RegisterTask(ScheduledTask{
		Name:     "trigger_me",
		Schedule: "0 0 1 1 *", // yearly, won't fire on its own
		Enabled:  true,
		Action:   func() { ran.Add(1) },
	})

	err := m.TriggerTask("trigger_me")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for async execution
	time.Sleep(50 * time.Millisecond)

	if ran.Load() != 1 {
		t.Errorf("expected action to run once, ran %d times", ran.Load())
	}
}

func TestTriggerTaskNotFound(t *testing.T) {
	m := NewManager(true)
	err := m.TriggerTask("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestMatchesCronField(t *testing.T) {
	tests := []struct {
		field string
		value int
		want  bool
	}{
		{"*", 5, true},
		{"5", 5, true},
		{"5", 6, false},
		{"*/5", 0, true},
		{"*/5", 5, true},
		{"*/5", 10, true},
		{"*/5", 3, false},
		{"1,5,10", 5, true},
		{"1,5,10", 3, false},
		{"1-5", 3, true},
		{"1-5", 6, false},
		{"1-5", 1, true},
		{"1-5", 5, true},
	}

	for _, tt := range tests {
		got := matchesCronField(tt.field, tt.value)
		if got != tt.want {
			t.Errorf("matchesCronField(%q, %d) = %v, want %v", tt.field, tt.value, got, tt.want)
		}
	}
}

func TestNextCronMatch(t *testing.T) {
	// "0 12 * * *" = every day at noon
	ref := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	next := nextCronMatch("0 12 * * *", ref)

	if next.Hour() != 12 || next.Minute() != 0 {
		t.Errorf("expected 12:00, got %02d:%02d", next.Hour(), next.Minute())
	}
	if next.Day() != 15 {
		t.Errorf("expected day 15, got %d", next.Day())
	}
}

func TestNextCronMatchSpecificDay(t *testing.T) {
	// "0 0 1 * *" = 1st of each month at midnight
	ref := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	next := nextCronMatch("0 0 1 * *", ref)

	if next.Day() != 1 {
		t.Errorf("expected day 1, got %d", next.Day())
	}
	if next.Month() != time.April {
		t.Errorf("expected April, got %v", next.Month())
	}
}
