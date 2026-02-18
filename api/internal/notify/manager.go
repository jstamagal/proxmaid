// Package notify provides an event-driven notification system
// with pluggable providers (Discord, Pushover, Email, Apprise).
package notify

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// EventType identifies the kind of event.
type EventType string

const (
	EventDiskFailure     EventType = "disk_failure"
	EventParityDone      EventType = "parity_done"
	EventMoverDone       EventType = "mover_done"
	EventSmartWarning    EventType = "smart_warning"
	EventArrayStopped    EventType = "array_stopped"
	EventCacheAlmostFull EventType = "cache_almost_full"
	EventPowerLoss       EventType = "power_loss"
	EventTest            EventType = "test"
)

// Event represents a notification event.
type Event struct {
	Type      EventType              `json:"type"`
	Message   string                 `json:"message"`
	Severity  string                 `json:"severity"` // "info", "warning", "critical"
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// NotifyConfig holds all provider configurations.
type NotifyConfig struct {
	Discord  *DiscordConfig  `json:"discord,omitempty"`
	Pushover *PushoverConfig `json:"pushover,omitempty"`
	Email    *EmailConfig    `json:"email,omitempty"`
	Apprise  *AppriseConfig  `json:"apprise,omitempty"`
}

// DiscordConfig holds Discord webhook settings.
type DiscordConfig struct {
	Enabled    bool   `json:"enabled"`
	WebhookURL string `json:"webhook_url"`
}

// PushoverConfig holds Pushover settings.
type PushoverConfig struct {
	Enabled  bool   `json:"enabled"`
	UserKey  string `json:"user_key"`
	AppToken string `json:"app_token"`
}

// EmailConfig holds SMTP email settings.
type EmailConfig struct {
	Enabled  bool   `json:"enabled"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	To       string `json:"to"`
}

// AppriseConfig holds Apprise CLI settings.
type AppriseConfig struct {
	Enabled bool     `json:"enabled"`
	URLs    []string `json:"urls"`
}

// Manager dispatches events to all configured notification providers.
type Manager struct {
	mu         sync.RWMutex
	mockMode   bool
	config     NotifyConfig
	history    []Event
	maxHistory int
	configPath string
	events     chan Event
}

// NewManager creates a new notification manager.
func NewManager(mockMode bool) *Manager {
	m := &Manager{
		mockMode:   mockMode,
		maxHistory: 100,
		configPath: "/etc/proxmaid/notifications.json",
		events:     make(chan Event, 100),
	}

	if !mockMode {
		m.loadConfig()
	}

	// Start the event dispatch goroutine
	go m.dispatchLoop()

	return m
}

// Emit sends an event to all configured providers.
func (m *Manager) Emit(event Event) {
	event.Timestamp = time.Now().UTC()

	// Store in history
	m.mu.Lock()
	m.history = append(m.history, event)
	if len(m.history) > m.maxHistory {
		m.history = m.history[len(m.history)-m.maxHistory:]
	}
	m.mu.Unlock()

	// Send to dispatch channel (non-blocking)
	select {
	case m.events <- event:
	default:
		fmt.Printf("[NOTIFY] Event queue full, dropping event: %s\n", event.Type)
	}
}

// dispatchLoop reads events from the channel and dispatches to providers.
func (m *Manager) dispatchLoop() {
	for event := range m.events {
		m.mu.RLock()
		config := m.config
		mock := m.mockMode
		m.mu.RUnlock()

		if mock {
			fmt.Printf("[MOCK] Notification [%s] %s: %s\n", event.Severity, event.Type, event.Message)
			continue
		}

		if config.Discord != nil && config.Discord.Enabled {
			m.sendDiscord(config.Discord, event)
		}
		if config.Pushover != nil && config.Pushover.Enabled {
			m.sendPushover(config.Pushover, event)
		}
		if config.Email != nil && config.Email.Enabled {
			m.sendEmail(config.Email, event)
		}
		if config.Apprise != nil && config.Apprise.Enabled {
			m.sendApprise(config.Apprise, event)
		}
	}
}

// sendDiscord sends a notification to a Discord webhook.
func (m *Manager) sendDiscord(cfg *DiscordConfig, event Event) {
	color := 0x2ecc71 // green for info
	switch event.Severity {
	case "warning":
		color = 0xf39c12
	case "critical":
		color = 0xe74c3c
	}

	body := fmt.Sprintf(`{
		"embeds": [{
			"title": "[%s] %s",
			"description": "%s",
			"color": %d,
			"timestamp": "%s"
		}]
	}`, strings.ToUpper(event.Severity), event.Type, event.Message, color, event.Timestamp.Format(time.RFC3339))

	resp, err := http.Post(cfg.WebhookURL, "application/json", strings.NewReader(body))
	if err != nil {
		fmt.Printf("[NOTIFY] Discord error: %v\n", err)
		return
	}
	resp.Body.Close()
}

// sendPushover sends a push notification via Pushover.
func (m *Manager) sendPushover(cfg *PushoverConfig, event Event) {
	priority := "0"
	switch event.Severity {
	case "warning":
		priority = "0"
	case "critical":
		priority = "1"
	}

	body := fmt.Sprintf("token=%s&user=%s&message=%s&title=Proxmaid: %s&priority=%s",
		cfg.AppToken, cfg.UserKey, event.Message, event.Type, priority)

	resp, err := http.Post("https://api.pushover.net/1/messages.json",
		"application/x-www-form-urlencoded", strings.NewReader(body))
	if err != nil {
		fmt.Printf("[NOTIFY] Pushover error: %v\n", err)
		return
	}
	resp.Body.Close()
}

// sendEmail sends a notification via SMTP.
func (m *Manager) sendEmail(cfg *EmailConfig, event Event) {
	subject := fmt.Sprintf("Proxmaid [%s]: %s", strings.ToUpper(event.Severity), event.Type)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s\r\n\r\nTimestamp: %s",
		cfg.From, cfg.To, subject, event.Message, event.Timestamp.Format(time.RFC3339))

	// Use the mail command as a simple fallback
	cmd := exec.Command("mail", "-s", subject, cfg.To)
	cmd.Stdin = strings.NewReader(msg)
	if err := cmd.Run(); err != nil {
		fmt.Printf("[NOTIFY] Email error: %v\n", err)
	}
}

// sendApprise sends a notification via the Apprise CLI.
func (m *Manager) sendApprise(cfg *AppriseConfig, event Event) {
	if _, err := exec.LookPath("apprise"); err != nil {
		fmt.Println("[NOTIFY] Apprise not installed, skipping")
		return
	}

	title := fmt.Sprintf("Proxmaid [%s]: %s", strings.ToUpper(event.Severity), event.Type)
	args := []string{"-t", title, "-b", event.Message}
	args = append(args, cfg.URLs...)

	if out, err := exec.Command("apprise", args...).CombinedOutput(); err != nil {
		fmt.Printf("[NOTIFY] Apprise error: %s: %v\n", string(out), err)
	}
}

// GetConfig returns the current notification configuration.
func (m *Manager) GetConfig() NotifyConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig updates the notification configuration.
func (m *Manager) UpdateConfig(config NotifyConfig) error {
	m.mu.Lock()
	m.config = config
	m.mu.Unlock()

	m.saveConfig()
	return nil
}

// GetHistory returns the recent event history.
func (m *Manager) GetHistory() []Event {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Event, len(m.history))
	copy(result, m.history)
	return result
}

// SendTest sends a test notification to verify configuration.
func (m *Manager) SendTest() {
	m.Emit(Event{
		Type:     EventTest,
		Message:  "This is a test notification from Proxmaid",
		Severity: "info",
	})
}

// saveConfig persists the notification configuration to disk.
func (m *Manager) saveConfig() {
	if m.mockMode {
		return
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	exec.Command("mkdir", "-p", filepath.Dir(m.configPath)).Run()
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		fmt.Printf("[NOTIFY] Failed to marshal config: %v\n", err)
		return
	}
	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		fmt.Printf("[NOTIFY] Failed to save config: %v\n", err)
	}
}

// loadConfig reads the notification configuration from disk.
func (m *Manager) loadConfig() {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return
	}
	var config NotifyConfig
	if err := json.Unmarshal(data, &config); err != nil {
		fmt.Printf("[NOTIFY] Failed to parse config: %v\n", err)
		return
	}
	m.config = config
}
