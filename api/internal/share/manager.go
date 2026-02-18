// Package share manages user shares: CRUD, SMB/NFS exports,
// mergerfs union mounts, and allocation methods.
package share

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// Share represents a named user share with export and cache configuration.
type Share struct {
	Name          string   `json:"name"`           // e.g. "media"
	Path          string   `json:"path"`           // /mnt/user/<name>
	CachePolicy   string   `json:"cache_policy"`   // "yes", "no", "only", "prefer"
	AllocMethod   string   `json:"alloc_method"`   // "mfs", "lfs", "ff"
	ExportSMB     bool     `json:"export_smb"`
	ExportNFS     bool     `json:"export_nfs"`
	SecurityMode  string   `json:"security_mode"`  // "public", "private", "secure"
	IncludedDisks []int    `json:"included_disks"` // slot numbers, empty = all
	ExcludedDisks []int    `json:"excluded_disks"`
	MinFreeSpace  string   `json:"min_free_space"` // e.g. "10GB"
	RecycleBin    bool     `json:"recycle_bin"`
	ReadUsers     []string `json:"read_users"`
	WriteUsers    []string `json:"write_users"`
}

// Manager handles share CRUD and export configuration.
type Manager struct {
	mu         sync.RWMutex
	mockMode   bool
	shares     map[string]*Share
	configPath string // path to persist share config
	arraySlots int    // number of data disk slots (default 28)
	cacheMount string // cache pool mount base path
}

// NewManager creates a new share manager.
func NewManager(mockMode bool) *Manager {
	m := &Manager{
		mockMode:   mockMode,
		shares:     make(map[string]*Share),
		configPath: "/etc/proxmaid/shares.json",
		arraySlots: 28,
		cacheMount: "/mnt/cache",
	}
	if mockMode {
		m.loadMockShares()
	} else {
		m.loadConfig()
	}
	return m
}

// ListShares returns all configured shares.
func (m *Manager) ListShares() []*Share {
	m.mu.RLock()
	defer m.mu.RUnlock()

	shares := make([]*Share, 0, len(m.shares))
	for _, s := range m.shares {
		shares = append(shares, s)
	}
	return shares
}

// GetShare returns a single share by name.
func (m *Manager) GetShare(name string) (*Share, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	share, ok := m.shares[name]
	if !ok {
		return nil, fmt.Errorf("share %q not found", name)
	}
	return share, nil
}

// CreateShare creates a new share with the given configuration.
func (m *Manager) CreateShare(share Share) (*Share, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if share.Name == "" {
		return nil, fmt.Errorf("share name is required")
	}
	if _, exists := m.shares[share.Name]; exists {
		return nil, fmt.Errorf("share %q already exists", share.Name)
	}

	// Set defaults
	if share.CachePolicy == "" {
		share.CachePolicy = "yes"
	}
	if share.AllocMethod == "" {
		share.AllocMethod = "mfs"
	}
	if share.SecurityMode == "" {
		share.SecurityMode = "public"
	}
	share.Path = "/mnt/user/" + share.Name

	if m.mockMode {
		fmt.Printf("[MOCK] mkdir -p %s\n", share.Path)
	} else {
		if err := exec.Command("mkdir", "-p", share.Path).Run(); err != nil {
			return nil, fmt.Errorf("failed to create share directory: %w", err)
		}
	}

	m.shares[share.Name] = &share

	// Generate exports
	m.generateSMBConfig(&share)
	m.generateNFSExport(&share)

	m.saveConfig()
	return &share, nil
}

// UpdateShare updates an existing share's configuration.
func (m *Manager) UpdateShare(name string, updated Share) (*Share, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.shares[name]
	if !ok {
		return nil, fmt.Errorf("share %q not found", name)
	}

	// Preserve the name and path
	updated.Name = existing.Name
	updated.Path = existing.Path

	m.shares[name] = &updated

	m.generateSMBConfig(&updated)
	m.generateNFSExport(&updated)

	m.saveConfig()
	return &updated, nil
}

// DeleteShare removes a share and cleans up exports.
func (m *Manager) DeleteShare(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	share, ok := m.shares[name]
	if !ok {
		return fmt.Errorf("share %q not found", name)
	}

	if m.mockMode {
		fmt.Printf("[MOCK] Deleting share %q, removing exports\n", name)
	} else {
		// Remove SMB config
		smbConf := fmt.Sprintf("/etc/samba/smb.d/%s.conf", name)
		os.Remove(smbConf)
		exec.Command("smbcontrol", "smbd", "reload-config").Run()

		// Unmount and remove directory
		exec.Command("umount", share.Path).Run()
	}

	delete(m.shares, name)
	m.saveConfig()
	return nil
}

// generateSMBConfig writes the Samba config for a share.
func (m *Manager) generateSMBConfig(share *Share) {
	if !share.ExportSMB {
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s]\n", share.Name))
	sb.WriteString(fmt.Sprintf("    path = %s\n", share.Path))
	sb.WriteString("    browseable = yes\n")
	sb.WriteString("    read only = no\n")

	switch share.SecurityMode {
	case "public":
		sb.WriteString("    guest ok = yes\n")
	case "private":
		sb.WriteString("    guest ok = no\n")
		if len(share.WriteUsers) > 0 {
			sb.WriteString(fmt.Sprintf("    valid users = %s\n", strings.Join(share.WriteUsers, ", ")))
		}
	case "secure":
		sb.WriteString("    guest ok = no\n")
		if len(share.ReadUsers) > 0 {
			sb.WriteString(fmt.Sprintf("    read list = %s\n", strings.Join(share.ReadUsers, ", ")))
		}
		if len(share.WriteUsers) > 0 {
			sb.WriteString(fmt.Sprintf("    write list = %s\n", strings.Join(share.WriteUsers, ", ")))
		}
		allUsers := append(share.ReadUsers, share.WriteUsers...)
		if len(allUsers) > 0 {
			sb.WriteString(fmt.Sprintf("    valid users = %s\n", strings.Join(allUsers, ", ")))
		}
	}

	if share.RecycleBin {
		sb.WriteString("    vfs objects = recycle\n")
		sb.WriteString("    recycle:repository = .Recycle.Bin/%U\n")
		sb.WriteString("    recycle:keeptree = yes\n")
		sb.WriteString("    recycle:versions = yes\n")
	}

	config := sb.String()

	if m.mockMode {
		fmt.Printf("[MOCK] SMB config for %q:\n%s\n", share.Name, config)
		return
	}

	confDir := "/etc/samba/smb.d"
	exec.Command("mkdir", "-p", confDir).Run()
	confPath := filepath.Join(confDir, share.Name+".conf")
	os.WriteFile(confPath, []byte(config), 0644)
	exec.Command("smbcontrol", "smbd", "reload-config").Run()
}

// generateNFSExport updates /etc/exports for a share.
func (m *Manager) generateNFSExport(share *Share) {
	if !share.ExportNFS {
		return
	}

	exportLine := fmt.Sprintf("%s *(rw,sync,no_subtree_check,no_root_squash)\n", share.Path)

	if m.mockMode {
		fmt.Printf("[MOCK] NFS export: %s", exportLine)
		return
	}

	// Append to /etc/exports if not already present
	data, _ := os.ReadFile("/etc/exports")
	if !strings.Contains(string(data), share.Path) {
		f, err := os.OpenFile("/etc/exports", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			f.WriteString(exportLine)
			f.Close()
		}
	}
	exec.Command("exportfs", "-ra").Run()
}

// MountUnionFS creates the mergerfs union mount for a share.
// Combines cache and array disk branches according to the share's cache policy.
func (m *Manager) MountUnionFS(share *Share) error {
	branches := m.buildBranches(share)
	if len(branches) == 0 {
		return fmt.Errorf("no branches available for share %q", share.Name)
	}

	createPolicy := share.AllocMethod
	if createPolicy == "" {
		createPolicy = "mfs"
	}

	branchStr := strings.Join(branches, ":")
	mountPoint := share.Path

	if m.mockMode {
		fmt.Printf("[MOCK] mergerfs %s → %s (create=%s)\n", branchStr, mountPoint, createPolicy)
		return nil
	}

	// Ensure mount point exists
	exec.Command("mkdir", "-p", mountPoint).Run()

	// Ensure each branch directory exists
	for _, branch := range branches {
		exec.Command("mkdir", "-p", branch).Run()
	}

	opts := fmt.Sprintf("defaults,allow_other,use_ino,category.create=%s,moveonenospc=true,minfreespace=%s",
		createPolicy, parseFreeSpace(share.MinFreeSpace))

	cmd := exec.Command("mergerfs", "-o", opts, branchStr, mountPoint)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mergerfs mount failed: %s: %w", string(out), err)
	}

	return nil
}

// UnmountUnionFS unmounts the mergerfs union mount for a share.
func (m *Manager) UnmountUnionFS(share *Share) error {
	if m.mockMode {
		fmt.Printf("[MOCK] umount %s\n", share.Path)
		return nil
	}

	if out, err := exec.Command("umount", share.Path).CombinedOutput(); err != nil {
		return fmt.Errorf("umount failed: %s: %w", string(out), err)
	}
	return nil
}

// SetAllocMethod changes the allocation method for a mounted share at runtime.
func (m *Manager) SetAllocMethod(name, method string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	share, ok := m.shares[name]
	if !ok {
		return fmt.Errorf("share %q not found", name)
	}

	validMethods := map[string]bool{"mfs": true, "lfs": true, "ff": true, "epff": true}
	if !validMethods[method] {
		return fmt.Errorf("invalid allocation method %q (valid: mfs, lfs, ff, epff)", method)
	}

	share.AllocMethod = method

	if m.mockMode {
		fmt.Printf("[MOCK] xattr -w user.mergerfs.category.create %s %s\n", method, share.Path)
	} else {
		// Change at runtime via xattr on the mergerfs mount
		cmd := exec.Command("xattr", "-w", "user.mergerfs.category.create", method, share.Path)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to set allocation method: %s: %w", string(out), err)
		}
	}

	m.saveConfig()
	return nil
}

// buildBranches constructs the mergerfs branch list based on cache policy and disk inclusion.
func (m *Manager) buildBranches(share *Share) []string {
	var branches []string

	// Build cache branch path
	cacheBranch := fmt.Sprintf("%s/default/%s", m.cacheMount, share.Name)

	// Build array disk branches
	var diskBranches []string
	for slot := 1; slot <= m.arraySlots; slot++ {
		if !m.isSlotIncluded(share, slot) {
			continue
		}
		diskBranches = append(diskBranches, fmt.Sprintf("/mnt/disk%d/%s", slot, share.Name))
	}

	// Arrange branches based on cache policy
	switch share.CachePolicy {
	case "yes":
		// Cache first, then array disks — new writes go to cache, mover moves to array
		branches = append(branches, cacheBranch)
		branches = append(branches, diskBranches...)
	case "prefer":
		// Cache first with keep — data stays on cache
		branches = append(branches, cacheBranch)
		branches = append(branches, diskBranches...)
	case "only":
		// Cache mount only, no array branches
		branches = append(branches, cacheBranch)
	case "no":
		// Array disks only, no cache branch
		branches = diskBranches
	default:
		// Default to "yes" behavior
		branches = append(branches, cacheBranch)
		branches = append(branches, diskBranches...)
	}

	return branches
}

// isSlotIncluded checks if a disk slot should be included in the share's branches.
func (m *Manager) isSlotIncluded(share *Share, slot int) bool {
	// If included list is specified, only include those slots
	if len(share.IncludedDisks) > 0 {
		for _, s := range share.IncludedDisks {
			if s == slot {
				return true
			}
		}
		return false
	}

	// If excluded list is specified, exclude those slots
	for _, s := range share.ExcludedDisks {
		if s == slot {
			return false
		}
	}

	return true
}

// parseFreeSpace converts a human-readable free space string to bytes for mergerfs.
func parseFreeSpace(s string) string {
	if s == "" {
		return "4G"
	}
	// mergerfs accepts values like "4G", "10G", "500M"
	s = strings.TrimSpace(s)
	s = strings.Replace(s, "GB", "G", 1)
	s = strings.Replace(s, "MB", "M", 1)
	s = strings.Replace(s, "TB", "T", 1)
	return s
}

// CreateUser creates a system user for share access.
func (m *Manager) CreateUser(username, password string) error {
	if username == "" || password == "" {
		return fmt.Errorf("username and password are required")
	}

	if m.mockMode {
		fmt.Printf("[MOCK] useradd -M -s /usr/sbin/nologin %s && smbpasswd -a %s\n", username, username)
		return nil
	}

	// Create system user (no home dir, no shell)
	if out, err := exec.Command("useradd", "-M", "-s", "/usr/sbin/nologin", username).CombinedOutput(); err != nil {
		return fmt.Errorf("useradd failed: %s: %w", string(out), err)
	}

	// Add to samba
	cmd := exec.Command("smbpasswd", "-a", "-s", username)
	cmd.Stdin = strings.NewReader(password + "\n" + password + "\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("smbpasswd failed: %s: %w", string(out), err)
	}
	return nil
}

// DeleteUser removes a system user and their samba credentials.
func (m *Manager) DeleteUser(username string) error {
	if username == "" {
		return fmt.Errorf("username is required")
	}

	if m.mockMode {
		fmt.Printf("[MOCK] userdel %s && smbpasswd -x %s\n", username, username)
		return nil
	}

	exec.Command("smbpasswd", "-x", username).Run()
	if out, err := exec.Command("userdel", username).CombinedOutput(); err != nil {
		return fmt.Errorf("userdel failed: %s: %w", string(out), err)
	}
	return nil
}

// ListUsers returns all samba users.
func (m *Manager) ListUsers() []string {
	if m.mockMode {
		return []string{"admin", "media-user", "backup-svc"}
	}

	out, err := exec.Command("pdbedit", "-L", "-d", "0").Output()
	if err != nil {
		return nil
	}
	var users []string
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) >= 1 && parts[0] != "" {
			users = append(users, parts[0])
		}
	}
	return users
}

// saveConfig persists the share configuration to disk.
func (m *Manager) saveConfig() {
	if m.mockMode {
		return
	}
	exec.Command("mkdir", "-p", filepath.Dir(m.configPath)).Run()
	data, err := json.MarshalIndent(m.shares, "", "  ")
	if err != nil {
		fmt.Printf("[SHARE] Failed to marshal config: %v\n", err)
		return
	}
	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		fmt.Printf("[SHARE] Failed to save config: %v\n", err)
	}
}

// loadConfig reads share configuration from disk.
func (m *Manager) loadConfig() {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return // no config file yet, that's OK
	}
	var shares map[string]*Share
	if err := json.Unmarshal(data, &shares); err != nil {
		fmt.Printf("[SHARE] Failed to parse config: %v\n", err)
		return
	}
	m.shares = shares
}

// loadMockShares creates realistic mock shares for development.
func (m *Manager) loadMockShares() {
	m.shares["media"] = &Share{
		Name:         "media",
		Path:         "/mnt/user/media",
		CachePolicy:  "yes",
		AllocMethod:  "mfs",
		ExportSMB:    true,
		ExportNFS:    false,
		SecurityMode: "public",
		RecycleBin:   true,
	}
	m.shares["backups"] = &Share{
		Name:         "backups",
		Path:         "/mnt/user/backups",
		CachePolicy:  "no",
		AllocMethod:  "lfs",
		ExportSMB:    true,
		ExportNFS:    true,
		SecurityMode: "private",
		WriteUsers:   []string{"backup-svc"},
		RecycleBin:   false,
	}
	m.shares["appdata"] = &Share{
		Name:         "appdata",
		Path:         "/mnt/user/appdata",
		CachePolicy:  "prefer",
		AllocMethod:  "mfs",
		ExportSMB:    false,
		ExportNFS:    false,
		SecurityMode: "private",
		RecycleBin:   false,
	}
}
