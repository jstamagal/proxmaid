package auth

import (
	"testing"
	"time"
)

func TestLoginSuccess(t *testing.T) {
	m := NewManager(true)
	m.config.Enabled = true

	token, err := m.Login("admin", "admin")
	if err != nil {
		t.Fatalf("expected login to succeed: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestLoginInvalidPassword(t *testing.T) {
	m := NewManager(true)
	m.config.Enabled = true

	_, err := m.Login("admin", "wrong")
	if err == nil {
		t.Fatal("expected login to fail with wrong password")
	}
}

func TestLoginUnknownUser(t *testing.T) {
	m := NewManager(true)

	_, err := m.Login("nobody", "password")
	if err == nil {
		t.Fatal("expected login to fail for unknown user")
	}
}

func TestLoginEmptyCredentials(t *testing.T) {
	m := NewManager(true)

	_, err := m.Login("", "")
	if err == nil {
		t.Fatal("expected login to fail with empty credentials")
	}
}

func TestValidateToken(t *testing.T) {
	m := NewManager(true)

	token, err := m.Login("admin", "admin")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	claims, valid := m.ValidateToken(token)
	if !valid {
		t.Fatal("expected token to be valid")
	}
	if claims.Username != "admin" {
		t.Errorf("expected username 'admin', got %q", claims.Username)
	}
}

func TestValidateExpiredToken(t *testing.T) {
	m := NewManager(true)

	// Manually insert an expired token
	token := generateToken()
	m.tokens[token] = &TokenClaims{
		Username:  "admin",
		IssuedAt:  time.Now().Add(-48 * time.Hour),
		ExpiresAt: time.Now().Add(-24 * time.Hour),
	}

	_, valid := m.ValidateToken(token)
	if valid {
		t.Fatal("expected expired token to be invalid")
	}
}

func TestValidateInvalidToken(t *testing.T) {
	m := NewManager(true)

	_, valid := m.ValidateToken("bogus-token")
	if valid {
		t.Fatal("expected bogus token to be invalid")
	}
}

func TestLogout(t *testing.T) {
	m := NewManager(true)

	token, err := m.Login("admin", "admin")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	m.Logout(token)

	_, valid := m.ValidateToken(token)
	if valid {
		t.Fatal("expected token to be invalid after logout")
	}
}

func TestIsEnabledDefault(t *testing.T) {
	m := NewManager(true)
	if m.IsEnabled() {
		t.Fatal("expected auth to be disabled by default in mock mode")
	}
}

func TestHashPassword(t *testing.T) {
	hash1 := HashPassword("test")
	hash2 := HashPassword("test")
	if hash1 != hash2 {
		t.Fatal("expected same password to produce same hash")
	}
	hash3 := HashPassword("different")
	if hash1 == hash3 {
		t.Fatal("expected different passwords to produce different hashes")
	}
}
