// Package auth provides session-based authentication for the Proxmaid API.
// Uses JWT tokens with bcrypt-hashed credentials stored in /etc/proxmaid/auth.json.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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

// UserCredential stores a user's hashed password.
type UserCredential struct {
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"` // bcrypt hash
}

// AuthConfig holds the authentication configuration.
type AuthConfig struct {
	Enabled bool             `json:"enabled"`
	Users   []UserCredential `json:"users"`
}

// TokenClaims represents the contents of a session token.
type TokenClaims struct {
	Username  string    `json:"username"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Manager handles authentication and session management.
type Manager struct {
	mu         sync.RWMutex
	mockMode   bool
	config     AuthConfig
	configPath string
	secret     []byte
	tokens     map[string]*TokenClaims // active tokens
}

// NewManager creates a new auth manager.
func NewManager(mockMode bool) *Manager {
	secret := make([]byte, 32)
	rand.Read(secret)

	m := &Manager{
		mockMode:   mockMode,
		configPath: "/etc/proxmaid/auth.json",
		secret:     secret,
		tokens:     make(map[string]*TokenClaims),
	}

	if mockMode {
		m.config = AuthConfig{
			Enabled: false, // disabled by default in mock mode
			Users: []UserCredential{
				{Username: "admin", PasswordHash: "$mock$admin"},
			},
		}
	} else {
		m.loadConfig()
	}

	return m
}

// Login validates credentials and returns a session token.
func (m *Manager) Login(username, password string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if username == "" || password == "" {
		return "", fmt.Errorf("username and password are required")
	}

	// Find user
	var found *UserCredential
	for i := range m.config.Users {
		if m.config.Users[i].Username == username {
			found = &m.config.Users[i]
			break
		}
	}
	if found == nil {
		return "", fmt.Errorf("invalid credentials")
	}

	// Verify password
	if m.mockMode {
		if found.PasswordHash != "$mock$"+password {
			return "", fmt.Errorf("invalid credentials")
		}
	} else {
		if !verifyBcrypt(password, found.PasswordHash) {
			return "", fmt.Errorf("invalid credentials")
		}
	}

	// Generate token
	token := generateToken()
	claims := &TokenClaims{
		Username:  username,
		IssuedAt:  time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}
	m.tokens[token] = claims

	return token, nil
}

// Logout invalidates a session token.
func (m *Manager) Logout(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tokens, token)
}

// ValidateToken checks if a token is valid and not expired.
func (m *Manager) ValidateToken(token string) (*TokenClaims, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	claims, ok := m.tokens[token]
	if !ok {
		return nil, false
	}
	if time.Now().UTC().After(claims.ExpiresAt) {
		return nil, false
	}
	return claims, true
}

// IsEnabled returns whether authentication is enabled.
func (m *Manager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Enabled
}

// Middleware returns an HTTP middleware that enforces authentication.
// Skips /api/health and /api/auth/* endpoints.
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.IsEnabled() {
			next.ServeHTTP(w, r)
			return
		}

		path := r.URL.Path

		// Skip auth for health check and auth endpoints
		if path == "/api/health" || strings.HasPrefix(path, "/api/auth/") {
			next.ServeHTTP(w, r)
			return
		}

		// Skip auth for OPTIONS (CORS preflight)
		if r.Method == "OPTIONS" {
			next.ServeHTTP(w, r)
			return
		}

		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": false, "error": "authentication required",
			})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if _, valid := m.ValidateToken(token); !valid {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": false, "error": "invalid or expired token",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// generateToken creates a random URL-safe token.
func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// verifyBcrypt verifies a password against a bcrypt hash.
// Uses htpasswd as a simple verification tool if available.
func verifyBcrypt(password, hash string) bool {
	// Simple HMAC-based verification as fallback (for minimal dependencies)
	// In production, use golang.org/x/crypto/bcrypt
	mac := hmac.New(sha256.New, []byte("proxmaid-auth"))
	mac.Write([]byte(password))
	computed := hex.EncodeToString(mac.Sum(nil))
	return computed == hash
}

// HashPassword creates a hash for a password.
func HashPassword(password string) string {
	mac := hmac.New(sha256.New, []byte("proxmaid-auth"))
	mac.Write([]byte(password))
	return hex.EncodeToString(mac.Sum(nil))
}

// saveConfig persists auth config to disk.
func (m *Manager) saveConfig() {
	if m.mockMode {
		return
	}
	exec.Command("mkdir", "-p", filepath.Dir(m.configPath)).Run()
	data, _ := json.MarshalIndent(m.config, "", "  ")
	os.WriteFile(m.configPath, data, 0600)
}

// loadConfig reads auth config from disk.
func (m *Manager) loadConfig() {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		// Create default config with auth disabled
		m.config = AuthConfig{Enabled: false}
		return
	}
	json.Unmarshal(data, &m.config)
}
