// Package app manages Docker containers via the Docker socket API.
// No external Docker SDK needed — uses net/http with Unix socket transport.
package app

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Container represents a Docker container.
type Container struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	State   string            `json:"state"`   // "running", "exited", "paused", etc.
	Status  string            `json:"status"`  // human-readable status e.g. "Up 2 hours"
	Ports   []ContainerPort   `json:"ports"`
	Mounts  []ContainerMount  `json:"mounts"`
	Created time.Time         `json:"created"`
	Labels  map[string]string `json:"labels"`
}

// ContainerPort represents a port mapping.
type ContainerPort struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"` // "tcp" or "udp"
}

// ContainerMount represents a volume mount.
type ContainerMount struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Mode        string `json:"mode"` // "rw" or "ro"
}

// ContainerStats represents resource usage stats.
type ContainerStats struct {
	CPUPercent float64 `json:"cpu_percent"`
	MemUsage   int64   `json:"mem_usage"`
	MemLimit   int64   `json:"mem_limit"`
	NetIn      int64   `json:"net_in"`
	NetOut     int64   `json:"net_out"`
}

// AppTemplate represents a container application template (parsed from XML).
type AppTemplate struct {
	Name        string            `json:"name"`
	Repository  string            `json:"repository"`
	Icon        string            `json:"icon"`
	Category    string            `json:"category"`
	Description string            `json:"description"`
	Ports       []TemplatePort    `json:"ports"`
	Volumes     []TemplateVolume  `json:"volumes"`
	Env         []TemplateEnv     `json:"env"`
	Network     string            `json:"network"`
	WebUI       string            `json:"webui"`
}

// TemplatePort is a port mapping in a template.
type TemplatePort struct {
	Container int    `json:"container"`
	Host      int    `json:"host"`
	Protocol  string `json:"protocol"`
}

// TemplateVolume is a volume mapping in a template.
type TemplateVolume struct {
	Container   string `json:"container"`
	Host        string `json:"host"`
	Description string `json:"description"`
}

// TemplateEnv is an environment variable in a template.
type TemplateEnv struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

// DockerNetwork represents a Docker network.
type DockerNetwork struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Driver string `json:"driver"`
	Scope  string `json:"scope"`
}

// Manager handles Docker container operations.
type Manager struct {
	mu        sync.RWMutex
	mockMode  bool
	client    *http.Client
	templates []AppTemplate
}

// NewManager creates a new app manager.
func NewManager(mockMode bool) *Manager {
	m := &Manager{
		mockMode: mockMode,
	}

	if !mockMode {
		m.client = &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", "/var/run/docker.sock")
				},
			},
			Timeout: 30 * time.Second,
		}
	}

	return m
}

// dockerGet performs a GET request to the Docker socket API.
func (m *Manager) dockerGet(path string) ([]byte, error) {
	resp, err := m.client.Get("http://localhost" + path)
	if err != nil {
		return nil, fmt.Errorf("docker API: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("docker API read: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("docker API %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// dockerPost performs a POST request to the Docker socket API.
func (m *Manager) dockerPost(path string, body io.Reader) ([]byte, error) {
	var contentType string
	if body != nil {
		contentType = "application/json"
	}
	resp, err := m.client.Post("http://localhost"+path, contentType, body)
	if err != nil {
		return nil, fmt.Errorf("docker API: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("docker API read: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("docker API %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

// dockerDelete performs a DELETE request to the Docker socket API.
func (m *Manager) dockerDelete(path string) error {
	req, err := http.NewRequest("DELETE", "http://localhost"+path, nil)
	if err != nil {
		return err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("docker API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("docker API %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// ListContainers returns all Docker containers (including stopped).
func (m *Manager) ListContainers() ([]Container, error) {
	if m.mockMode {
		return mockContainers(), nil
	}

	data, err := m.dockerGet("/containers/json?all=true")
	if err != nil {
		return nil, err
	}

	var raw []struct {
		ID      string            `json:"Id"`
		Names   []string          `json:"Names"`
		Image   string            `json:"Image"`
		State   string            `json:"State"`
		Status  string            `json:"Status"`
		Created int64             `json:"Created"`
		Labels  map[string]string `json:"Labels"`
		Ports   []struct {
			PrivatePort int    `json:"PrivatePort"`
			PublicPort  int    `json:"PublicPort"`
			Type        string `json:"Type"`
		} `json:"Ports"`
		Mounts []struct {
			Source      string `json:"Source"`
			Destination string `json:"Destination"`
			Mode        string `json:"Mode"`
		} `json:"Mounts"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse containers: %w", err)
	}

	containers := make([]Container, 0, len(raw))
	for _, c := range raw {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		ports := make([]ContainerPort, 0, len(c.Ports))
		for _, p := range c.Ports {
			ports = append(ports, ContainerPort{
				HostPort:      p.PublicPort,
				ContainerPort: p.PrivatePort,
				Protocol:      p.Type,
			})
		}

		mounts := make([]ContainerMount, 0, len(c.Mounts))
		for _, m := range c.Mounts {
			mounts = append(mounts, ContainerMount{
				Source:      m.Source,
				Destination: m.Destination,
				Mode:        m.Mode,
			})
		}

		containers = append(containers, Container{
			ID:      c.ID[:12],
			Name:    name,
			Image:   c.Image,
			State:   c.State,
			Status:  c.Status,
			Ports:   ports,
			Mounts:  mounts,
			Created: time.Unix(c.Created, 0),
			Labels:  c.Labels,
		})
	}

	return containers, nil
}

// StartContainer starts a stopped container.
func (m *Manager) StartContainer(id string) error {
	if m.mockMode {
		fmt.Printf("[MOCK] docker start %s\n", id)
		return nil
	}
	_, err := m.dockerPost("/containers/"+id+"/start", nil)
	return err
}

// StopContainer stops a running container.
func (m *Manager) StopContainer(id string) error {
	if m.mockMode {
		fmt.Printf("[MOCK] docker stop %s\n", id)
		return nil
	}
	_, err := m.dockerPost("/containers/"+id+"/stop", nil)
	return err
}

// RemoveContainer removes a container.
func (m *Manager) RemoveContainer(id string) error {
	if m.mockMode {
		fmt.Printf("[MOCK] docker rm %s\n", id)
		return nil
	}
	return m.dockerDelete("/containers/" + id + "?force=true")
}

// GetLogs returns the last N lines of container logs.
func (m *Manager) GetLogs(id string, lines int) (string, error) {
	if m.mockMode {
		return fmt.Sprintf("[MOCK] Last %d lines of logs for %s\n2024-01-01 Starting service...\n2024-01-01 Listening on port 8080", lines, id), nil
	}

	data, err := m.dockerGet(fmt.Sprintf("/containers/%s/logs?stdout=true&stderr=true&tail=%d", id, lines))
	if err != nil {
		return "", err
	}
	// Docker log stream has 8-byte header per line, strip it
	return stripDockerLogHeaders(data), nil
}

// stripDockerLogHeaders removes the 8-byte Docker stream headers from log output.
func stripDockerLogHeaders(data []byte) string {
	var lines []string
	for len(data) > 8 {
		// First 4 bytes: stream type, next 4: size (big-endian)
		size := int(data[4])<<24 | int(data[5])<<16 | int(data[6])<<8 | int(data[7])
		if size <= 0 || 8+size > len(data) {
			break
		}
		lines = append(lines, string(data[8:8+size]))
		data = data[8+size:]
	}
	if len(lines) == 0 {
		return string(data)
	}
	return strings.Join(lines, "")
}

// GetStats returns resource usage stats for a container.
func (m *Manager) GetStats(id string) (*ContainerStats, error) {
	if m.mockMode {
		return &ContainerStats{
			CPUPercent: 2.5,
			MemUsage:   268435456,  // 256 MB
			MemLimit:   4294967296, // 4 GB
			NetIn:      104857600,  // 100 MB
			NetOut:     52428800,   // 50 MB
		}, nil
	}

	data, err := m.dockerGet("/containers/" + id + "/stats?stream=false")
	if err != nil {
		return nil, err
	}

	var raw struct {
		CPUStats struct {
			CPUUsage struct {
				TotalUsage int64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemCPUUsage int64 `json:"system_cpu_usage"`
			OnlineCPUs     int   `json:"online_cpus"`
		} `json:"cpu_stats"`
		PreCPUStats struct {
			CPUUsage struct {
				TotalUsage int64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemCPUUsage int64 `json:"system_cpu_usage"`
		} `json:"precpu_stats"`
		MemoryStats struct {
			Usage int64 `json:"usage"`
			Limit int64 `json:"limit"`
		} `json:"memory_stats"`
		Networks map[string]struct {
			RxBytes int64 `json:"rx_bytes"`
			TxBytes int64 `json:"tx_bytes"`
		} `json:"networks"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse stats: %w", err)
	}

	// Calculate CPU percentage
	cpuDelta := float64(raw.CPUStats.CPUUsage.TotalUsage - raw.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(raw.CPUStats.SystemCPUUsage - raw.PreCPUStats.SystemCPUUsage)
	cpuPct := 0.0
	if sysDelta > 0 && cpuDelta > 0 {
		cpuPct = (cpuDelta / sysDelta) * float64(raw.CPUStats.OnlineCPUs) * 100.0
	}

	var netIn, netOut int64
	for _, n := range raw.Networks {
		netIn += n.RxBytes
		netOut += n.TxBytes
	}

	return &ContainerStats{
		CPUPercent: cpuPct,
		MemUsage:   raw.MemoryStats.Usage,
		MemLimit:   raw.MemoryStats.Limit,
		NetIn:      netIn,
		NetOut:     netOut,
	}, nil
}

// ComposeUp runs docker compose up -d for a compose file.
func (m *Manager) ComposeUp(composePath string) error {
	if m.mockMode {
		fmt.Printf("[MOCK] docker compose -f %s up -d\n", composePath)
		return nil
	}
	out, err := exec.Command("docker", "compose", "-f", composePath, "up", "-d").CombinedOutput()
	if err != nil {
		return fmt.Errorf("compose up failed: %s: %w", string(out), err)
	}
	return nil
}

// ComposeDown runs docker compose down for a compose file.
func (m *Manager) ComposeDown(composePath string) error {
	if m.mockMode {
		fmt.Printf("[MOCK] docker compose -f %s down\n", composePath)
		return nil
	}
	out, err := exec.Command("docker", "compose", "-f", composePath, "down").CombinedOutput()
	if err != nil {
		return fmt.Errorf("compose down failed: %s: %w", string(out), err)
	}
	return nil
}

// ParseTemplatesFromXML parses Unraid-style CA XML templates into AppTemplate structs.
func ParseTemplatesFromXML(data []byte) ([]AppTemplate, error) {
	type xmlConfig struct {
		XMLName xml.Name `xml:"Config"`
		Name    string   `xml:"Name,attr"`
		Target  string   `xml:"Target,attr"`
		Type    string   `xml:"Type,attr"`
		Default string   `xml:"Default,attr"`
		Value   string   `xml:",chardata"`
	}
	type xmlContainer struct {
		XMLName     xml.Name    `xml:"Container"`
		Name        string      `xml:"Name"`
		Repository  string      `xml:"Repository"`
		Icon        string      `xml:"Icon"`
		Category    string      `xml:"Category"`
		Description string      `xml:"Description"`
		WebUI       string      `xml:"WebUI"`
		Network     string      `xml:"Network"`
		Configs     []xmlConfig `xml:"Config"`
	}

	var container xmlContainer
	if err := xml.Unmarshal(data, &container); err != nil {
		return nil, fmt.Errorf("parse template XML: %w", err)
	}

	tmpl := AppTemplate{
		Name:        container.Name,
		Repository:  container.Repository,
		Icon:        container.Icon,
		Category:    container.Category,
		Description: container.Description,
		Network:     container.Network,
		WebUI:       container.WebUI,
	}

	for _, cfg := range container.Configs {
		value := cfg.Value
		if value == "" {
			value = cfg.Default
		}

		switch cfg.Type {
		case "Port":
			port, _ := strconv.Atoi(cfg.Target)
			hostPort, _ := strconv.Atoi(value)
			if port > 0 {
				tmpl.Ports = append(tmpl.Ports, TemplatePort{
					Container: port,
					Host:      hostPort,
					Protocol:  "tcp",
				})
			}
		case "Path":
			tmpl.Volumes = append(tmpl.Volumes, TemplateVolume{
				Container:   cfg.Target,
				Host:        value,
				Description: cfg.Name,
			})
		case "Variable":
			tmpl.Env = append(tmpl.Env, TemplateEnv{
				Name:        cfg.Target,
				Value:       value,
				Description: cfg.Name,
			})
		}
	}

	return []AppTemplate{tmpl}, nil
}

// GetTemplates returns cached app templates.
func (m *Manager) GetTemplates(category, search string) []AppTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.mockMode && len(m.templates) == 0 {
		return mockTemplates()
	}

	results := make([]AppTemplate, 0)
	for _, t := range m.templates {
		if category != "" && !strings.EqualFold(t.Category, category) {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(t.Name), strings.ToLower(search)) &&
			!strings.Contains(strings.ToLower(t.Description), strings.ToLower(search)) {
			continue
		}
		results = append(results, t)
	}
	return results
}

// LoadTemplates loads templates from XML data.
func (m *Manager) LoadTemplates(xmlDataList [][]byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.templates = nil
	for _, data := range xmlDataList {
		if tmpls, err := ParseTemplatesFromXML(data); err == nil {
			m.templates = append(m.templates, tmpls...)
		}
	}
}

// InstallApp creates and starts a container from an app template.
func (m *Manager) InstallApp(tmpl AppTemplate, overrides map[string]string) error {
	if m.mockMode {
		fmt.Printf("[MOCK] docker pull %s && docker create %s\n", tmpl.Repository, tmpl.Name)
		return nil
	}

	// Pull image
	_, err := m.dockerPost(fmt.Sprintf("/images/create?fromImage=%s", tmpl.Repository), nil)
	if err != nil {
		return fmt.Errorf("pull image: %w", err)
	}

	// Build environment
	env := make([]string, 0, len(tmpl.Env))
	for _, e := range tmpl.Env {
		val := e.Value
		if v, ok := overrides[e.Name]; ok {
			val = v
		}
		env = append(env, fmt.Sprintf("%s=%s", e.Name, val))
	}

	// Build port bindings
	portBindings := make(map[string]interface{})
	exposedPorts := make(map[string]interface{})
	for _, p := range tmpl.Ports {
		key := fmt.Sprintf("%d/%s", p.Container, p.Protocol)
		exposedPorts[key] = struct{}{}
		hostPort := strconv.Itoa(p.Host)
		if v, ok := overrides[fmt.Sprintf("port_%d", p.Container)]; ok {
			hostPort = v
		}
		portBindings[key] = []map[string]string{{"HostPort": hostPort}}
	}

	// Build binds
	binds := make([]string, 0, len(tmpl.Volumes))
	for _, v := range tmpl.Volumes {
		host := v.Host
		if h, ok := overrides[v.Container]; ok {
			host = h
		}
		binds = append(binds, fmt.Sprintf("%s:%s:rw", host, v.Container))
	}

	// Create container
	createBody := map[string]interface{}{
		"Image":        tmpl.Repository,
		"Env":          env,
		"ExposedPorts": exposedPorts,
		"HostConfig": map[string]interface{}{
			"PortBindings": portBindings,
			"Binds":        binds,
			"RestartPolicy": map[string]interface{}{
				"Name": "unless-stopped",
			},
		},
	}

	bodyJSON, _ := json.Marshal(createBody)
	resp, err := m.dockerPost(fmt.Sprintf("/containers/create?name=%s", tmpl.Name), bytes.NewReader(bodyJSON))
	if err != nil {
		return fmt.Errorf("create container: %w", err)
	}

	var createResp struct {
		ID string `json:"Id"`
	}
	json.Unmarshal(resp, &createResp)

	// Start container
	if _, err := m.dockerPost("/containers/"+createResp.ID+"/start", nil); err != nil {
		return fmt.Errorf("start container: %w", err)
	}

	return nil
}

// CheckUpdates checks running containers for available image updates.
func (m *Manager) CheckUpdates() (map[string]bool, error) {
	if m.mockMode {
		return map[string]bool{
			"plex":   true,
			"sonarr": false,
			"nginx-proxy": false,
		}, nil
	}

	containers, err := m.ListContainers()
	if err != nil {
		return nil, err
	}

	updates := make(map[string]bool)
	for _, c := range containers {
		if c.State != "running" {
			continue
		}
		// Get current image digest
		imgData, err := m.dockerGet("/images/" + c.Image + "/json")
		if err != nil {
			updates[c.Name] = false
			continue
		}
		var imgInfo struct {
			RepoDigests []string `json:"RepoDigests"`
		}
		json.Unmarshal(imgData, &imgInfo)

		// Pull latest to check for updates (dry-run style)
		// This is a simplified check - just mark as no update by default
		// A full implementation would compare registry digests
		updates[c.Name] = false

		if len(imgInfo.RepoDigests) == 0 {
			// No digest means we can't compare, assume update available
			updates[c.Name] = true
		}
	}

	return updates, nil
}

// ListNetworks returns all Docker networks.
func (m *Manager) ListNetworks() ([]DockerNetwork, error) {
	if m.mockMode {
		return []DockerNetwork{
			{ID: "abc123", Name: "bridge", Driver: "bridge", Scope: "local"},
			{ID: "def456", Name: "host", Driver: "host", Scope: "local"},
			{ID: "ghi789", Name: "media-net", Driver: "macvlan", Scope: "local"},
		}, nil
	}

	data, err := m.dockerGet("/networks")
	if err != nil {
		return nil, err
	}

	var raw []struct {
		ID     string `json:"Id"`
		Name   string `json:"Name"`
		Driver string `json:"Driver"`
		Scope  string `json:"Scope"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse networks: %w", err)
	}

	networks := make([]DockerNetwork, 0, len(raw))
	for _, n := range raw {
		networks = append(networks, DockerNetwork{
			ID:     n.ID[:12],
			Name:   n.Name,
			Driver: n.Driver,
			Scope:  n.Scope,
		})
	}
	return networks, nil
}

// CreateNetwork creates a new Docker network.
func (m *Manager) CreateNetwork(name, driver string) error {
	if m.mockMode {
		fmt.Printf("[MOCK] docker network create --driver %s %s\n", driver, name)
		return nil
	}

	body, _ := json.Marshal(map[string]string{
		"Name":   name,
		"Driver": driver,
	})
	_, err := m.dockerPost("/networks/create", bytes.NewReader(body))
	return err
}

// DeleteNetwork removes a Docker network.
func (m *Manager) DeleteNetwork(id string) error {
	if m.mockMode {
		fmt.Printf("[MOCK] docker network rm %s\n", id)
		return nil
	}
	return m.dockerDelete("/networks/" + id)
}

func mockTemplates() []AppTemplate {
	return []AppTemplate{
		{
			Name:        "Plex Media Server",
			Repository:  "plexinc/pms-docker",
			Icon:        "https://raw.githubusercontent.com/plexinc/pms-docker/master/icon.png",
			Category:    "Media",
			Description: "Plex organizes your media and streams it to any device.",
			Ports:       []TemplatePort{{Container: 32400, Host: 32400, Protocol: "tcp"}},
			Volumes: []TemplateVolume{
				{Container: "/config", Host: "/mnt/user/appdata/plex", Description: "Config"},
				{Container: "/media", Host: "/mnt/user/media", Description: "Media"},
			},
			Env: []TemplateEnv{
				{Name: "PLEX_CLAIM", Value: "", Description: "Plex Claim Token"},
				{Name: "TZ", Value: "America/New_York", Description: "Timezone"},
			},
		},
		{
			Name:        "Sonarr",
			Repository:  "linuxserver/sonarr",
			Icon:        "https://raw.githubusercontent.com/linuxserver/docker-templates/master/linuxserver.io/img/sonarr-icon.png",
			Category:    "Media",
			Description: "Smart PVR for newsgroup and bittorrent users.",
			Ports:       []TemplatePort{{Container: 8989, Host: 8989, Protocol: "tcp"}},
			Volumes: []TemplateVolume{
				{Container: "/config", Host: "/mnt/user/appdata/sonarr", Description: "Config"},
				{Container: "/tv", Host: "/mnt/user/media/tv", Description: "TV Shows"},
			},
			Env: []TemplateEnv{
				{Name: "PUID", Value: "1000", Description: "User ID"},
				{Name: "PGID", Value: "1000", Description: "Group ID"},
			},
		},
		{
			Name:        "Pi-hole",
			Repository:  "pihole/pihole",
			Icon:        "https://pi-hole.github.io/graphics/Vortex/Vortex_Vertical_wordmark_lightbg.png",
			Category:    "Networking",
			Description: "Network-wide ad blocking via your own DNS server.",
			Ports: []TemplatePort{
				{Container: 80, Host: 8080, Protocol: "tcp"},
				{Container: 53, Host: 53, Protocol: "tcp"},
				{Container: 53, Host: 53, Protocol: "udp"},
			},
			Volumes: []TemplateVolume{
				{Container: "/etc/pihole", Host: "/mnt/user/appdata/pihole", Description: "Config"},
			},
			Env: []TemplateEnv{
				{Name: "WEBPASSWORD", Value: "", Description: "Web UI Password"},
			},
		},
	}
}

// mockContainers returns simulated container data for development.
func mockContainers() []Container {
	now := time.Now()
	return []Container{
		{
			ID:    "a1b2c3d4e5f6",
			Name:  "plex",
			Image: "plexinc/pms-docker:latest",
			State: "running",
			Status: "Up 3 days",
			Ports: []ContainerPort{
				{HostPort: 32400, ContainerPort: 32400, Protocol: "tcp"},
			},
			Mounts: []ContainerMount{
				{Source: "/mnt/user/media", Destination: "/media", Mode: "rw"},
				{Source: "/mnt/user/appdata/plex", Destination: "/config", Mode: "rw"},
			},
			Created: now.Add(-72 * time.Hour),
			Labels:  map[string]string{"com.docker.compose.project": "media"},
		},
		{
			ID:    "b2c3d4e5f6a7",
			Name:  "sonarr",
			Image: "linuxserver/sonarr:latest",
			State: "running",
			Status: "Up 3 days",
			Ports: []ContainerPort{
				{HostPort: 8989, ContainerPort: 8989, Protocol: "tcp"},
			},
			Mounts: []ContainerMount{
				{Source: "/mnt/user/media/tv", Destination: "/tv", Mode: "rw"},
				{Source: "/mnt/user/appdata/sonarr", Destination: "/config", Mode: "rw"},
			},
			Created: now.Add(-72 * time.Hour),
			Labels:  map[string]string{"com.docker.compose.project": "media"},
		},
		{
			ID:    "c3d4e5f6a7b8",
			Name:  "nginx-proxy",
			Image: "nginx:alpine",
			State: "exited",
			Status: "Exited (0) 12 hours ago",
			Ports: []ContainerPort{
				{HostPort: 80, ContainerPort: 80, Protocol: "tcp"},
				{HostPort: 443, ContainerPort: 443, Protocol: "tcp"},
			},
			Created: now.Add(-168 * time.Hour),
			Labels:  map[string]string{},
		},
	}
}
