package share

import "testing"

func newMockManager() *Manager {
	return NewManager(true)
}

func TestMockSharesLoaded(t *testing.T) {
	m := newMockManager()
	shares := m.ListShares()
	if len(shares) != 3 {
		t.Errorf("expected 3 mock shares, got %d", len(shares))
	}
}

func TestGetShare(t *testing.T) {
	m := newMockManager()
	s, err := m.GetShare("media")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Name != "media" {
		t.Errorf("expected name 'media', got %q", s.Name)
	}
	if s.Path != "/mnt/user/media" {
		t.Errorf("expected path '/mnt/user/media', got %q", s.Path)
	}
}

func TestGetShareNotFound(t *testing.T) {
	m := newMockManager()
	_, err := m.GetShare("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent share")
	}
}

func TestCreateShare(t *testing.T) {
	m := newMockManager()
	s, err := m.CreateShare(Share{Name: "photos", ExportSMB: true, SecurityMode: "public"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Path != "/mnt/user/photos" {
		t.Errorf("expected path '/mnt/user/photos', got %q", s.Path)
	}
	if s.CachePolicy != "yes" {
		t.Errorf("expected default cache policy 'yes', got %q", s.CachePolicy)
	}
}

func TestCreateShareDuplicate(t *testing.T) {
	m := newMockManager()
	_, err := m.CreateShare(Share{Name: "media"})
	if err == nil {
		t.Error("expected error for duplicate share name")
	}
}

func TestCreateShareValidation(t *testing.T) {
	m := newMockManager()
	_, err := m.CreateShare(Share{})
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestUpdateShare(t *testing.T) {
	m := newMockManager()
	updated, err := m.UpdateShare("media", Share{CachePolicy: "prefer", ExportNFS: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.CachePolicy != "prefer" {
		t.Errorf("expected cache policy 'prefer', got %q", updated.CachePolicy)
	}
	if updated.Name != "media" {
		t.Errorf("expected name preserved as 'media', got %q", updated.Name)
	}
}

func TestUpdateShareNotFound(t *testing.T) {
	m := newMockManager()
	_, err := m.UpdateShare("nonexistent", Share{})
	if err == nil {
		t.Error("expected error for nonexistent share")
	}
}

func TestDeleteShare(t *testing.T) {
	m := newMockManager()
	if err := m.DeleteShare("media"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err := m.GetShare("media")
	if err == nil {
		t.Error("expected share to be deleted")
	}
}

func TestDeleteShareNotFound(t *testing.T) {
	m := newMockManager()
	err := m.DeleteShare("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent share")
	}
}

func TestListUsers(t *testing.T) {
	m := newMockManager()
	users := m.ListUsers()
	if len(users) != 3 {
		t.Errorf("expected 3 mock users, got %d", len(users))
	}
}

func TestCreateUser(t *testing.T) {
	m := newMockManager()
	if err := m.CreateUser("testuser", "password123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateUserValidation(t *testing.T) {
	m := newMockManager()
	if err := m.CreateUser("", "password"); err == nil {
		t.Error("expected error for empty username")
	}
	if err := m.CreateUser("user", ""); err == nil {
		t.Error("expected error for empty password")
	}
}

func TestDeleteUser(t *testing.T) {
	m := newMockManager()
	if err := m.DeleteUser("testuser"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteUserValidation(t *testing.T) {
	m := newMockManager()
	if err := m.DeleteUser(""); err == nil {
		t.Error("expected error for empty username")
	}
}
