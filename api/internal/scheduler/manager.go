// Package scheduler provides a cron-style task scheduler.
// Uses minute-level polling to check if any registered tasks should run.
package scheduler

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ScheduledTask represents a periodic task with a cron schedule.
type ScheduledTask struct {
	Name     string    `json:"name"`
	Schedule string    `json:"schedule"` // cron expression: "min hour dom month dow"
	Enabled  bool      `json:"enabled"`
	LastRun  time.Time `json:"last_run"`
	NextRun  time.Time `json:"next_run"`
	Action   func()    `json:"-"` // the function to execute
}

// Manager handles scheduled task registration and execution.
type Manager struct {
	mu         sync.RWMutex
	mockMode   bool
	tasks      map[string]*ScheduledTask
	configPath string
	stopCh     chan struct{}
}

// NewManager creates a new scheduler manager and starts the polling loop.
func NewManager(mockMode bool) *Manager {
	m := &Manager{
		mockMode:   mockMode,
		tasks:      make(map[string]*ScheduledTask),
		configPath: "/etc/proxmaid/scheduler.json",
		stopCh:     make(chan struct{}),
	}

	if !mockMode {
		m.loadConfig()
		go m.runLoop()
	}

	return m
}

// RegisterTask adds a task to the scheduler.
// If the task already exists (from config), its action is updated.
func (m *Manager) RegisterTask(task ScheduledTask) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.tasks[task.Name]; ok {
		// Preserve config (enabled, schedule) but update action
		existing.Action = task.Action
		if existing.Schedule == "" {
			existing.Schedule = task.Schedule
		}
		existing.NextRun = nextCronMatch(existing.Schedule, time.Now())
		return
	}

	task.NextRun = nextCronMatch(task.Schedule, time.Now())
	m.tasks[task.Name] = &task
}

// ListTasks returns all registered tasks.
func (m *Manager) ListTasks() []ScheduledTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]ScheduledTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, *t)
	}
	return tasks
}

// GetTask returns a single task by name.
func (m *Manager) GetTask(name string) (*ScheduledTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[name]
	if !ok {
		return nil, fmt.Errorf("task %q not found", name)
	}
	return task, nil
}

// UpdateTask updates a task's schedule and enabled state.
func (m *Manager) UpdateTask(name string, schedule string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[name]
	if !ok {
		return fmt.Errorf("task %q not found", name)
	}

	task.Schedule = schedule
	task.Enabled = enabled
	task.NextRun = nextCronMatch(schedule, time.Now())

	m.saveConfig()
	return nil
}

// TriggerTask manually runs a task immediately.
func (m *Manager) TriggerTask(name string) error {
	m.mu.RLock()
	task, ok := m.tasks[name]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("task %q not found", name)
	}
	if task.Action == nil {
		return fmt.Errorf("task %q has no action", name)
	}

	go func() {
		task.Action()
		m.mu.Lock()
		task.LastRun = time.Now()
		task.NextRun = nextCronMatch(task.Schedule, time.Now())
		m.mu.Unlock()
	}()

	return nil
}

// Stop gracefully stops the scheduler loop.
func (m *Manager) Stop() {
	close(m.stopCh)
}

// runLoop checks every minute if any tasks should fire.
func (m *Manager) runLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkAndRun()
		case <-m.stopCh:
			return
		}
	}
}

// checkAndRun fires any tasks whose NextRun has passed.
func (m *Manager) checkAndRun() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for _, task := range m.tasks {
		if !task.Enabled || task.Action == nil {
			continue
		}
		if !now.Before(task.NextRun) {
			go task.Action()
			task.LastRun = now
			task.NextRun = nextCronMatch(task.Schedule, now)
			m.saveConfig()
		}
	}
}

// nextCronMatch returns the next time a cron expression matches after the given time.
// Cron format: "min hour dom month dow" (5 fields, like standard cron).
func nextCronMatch(expr string, after time.Time) time.Time {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return after.Add(24 * time.Hour) // fallback: next day
	}

	// Start checking from the next minute
	t := after.Truncate(time.Minute).Add(time.Minute)

	// Check up to 366 days worth of minutes (worst case for a yearly cron)
	for i := 0; i < 527040; i++ {
		if matchesCronField(fields[0], t.Minute()) &&
			matchesCronField(fields[1], t.Hour()) &&
			matchesCronField(fields[2], t.Day()) &&
			matchesCronField(fields[3], int(t.Month())) &&
			matchesCronField(fields[4], int(t.Weekday())) {
			return t
		}
		t = t.Add(time.Minute)
	}

	return after.Add(24 * time.Hour) // fallback
}

// matchesCronField checks if a value matches a cron field expression.
// Supports: * (any), specific number, comma-separated, ranges (N-M), and step (*/N).
func matchesCronField(field string, value int) bool {
	if field == "*" {
		return true
	}

	// Handle step: */N
	if strings.HasPrefix(field, "*/") {
		step, err := strconv.Atoi(field[2:])
		if err != nil || step <= 0 {
			return false
		}
		return value%step == 0
	}

	// Handle comma-separated values
	for _, part := range strings.Split(field, ",") {
		// Handle range: N-M
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			lo, _ := strconv.Atoi(bounds[0])
			hi, _ := strconv.Atoi(bounds[1])
			if value >= lo && value <= hi {
				return true
			}
			continue
		}

		// Exact match
		if n, err := strconv.Atoi(part); err == nil && n == value {
			return true
		}
	}

	return false
}

// saveConfig persists task schedules (not actions) to disk.
func (m *Manager) saveConfig() {
	if m.mockMode {
		return
	}

	type taskConfig struct {
		Schedule string    `json:"schedule"`
		Enabled  bool      `json:"enabled"`
		LastRun  time.Time `json:"last_run"`
	}

	configs := make(map[string]taskConfig)
	for name, task := range m.tasks {
		configs[name] = taskConfig{
			Schedule: task.Schedule,
			Enabled:  task.Enabled,
			LastRun:  task.LastRun,
		}
	}

	exec.Command("mkdir", "-p", filepath.Dir(m.configPath)).Run()
	data, _ := json.MarshalIndent(configs, "", "  ")
	os.WriteFile(m.configPath, data, 0644)
}

// loadConfig reads saved task schedules from disk.
func (m *Manager) loadConfig() {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return
	}

	type taskConfig struct {
		Schedule string    `json:"schedule"`
		Enabled  bool      `json:"enabled"`
		LastRun  time.Time `json:"last_run"`
	}

	var configs map[string]taskConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return
	}

	for name, cfg := range configs {
		m.tasks[name] = &ScheduledTask{
			Name:     name,
			Schedule: cfg.Schedule,
			Enabled:  cfg.Enabled,
			LastRun:  cfg.LastRun,
			NextRun:  nextCronMatch(cfg.Schedule, time.Now()),
		}
	}
}
