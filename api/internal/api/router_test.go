package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/proxmaid/proxmaid/internal/array"
	"github.com/proxmaid/proxmaid/internal/disk"
	"github.com/proxmaid/proxmaid/internal/system"
)

func setupTestRouter() http.Handler {
	sysMgr := &system.Manager{MockMode: true}
	arrayMgr := array.NewManager(sysMgr)
	diskMgr := disk.NewManager(true)
	return NewRouter(arrayMgr, sysMgr, diskMgr)
}

func TestHealthEndpoint(t *testing.T) {
	router := setupTestRouter()
	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.OK {
		t.Error("expected ok=true")
	}
}

func TestArrayStatusEndpoint(t *testing.T) {
	router := setupTestRouter()
	req := httptest.NewRequest("GET", "/api/array/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.OK {
		t.Error("expected ok=true")
	}
}

func TestArrayStartEndpoint(t *testing.T) {
	router := setupTestRouter()
	req := httptest.NewRequest("POST", "/api/array/start", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestArrayStopEndpoint(t *testing.T) {
	router := setupTestRouter()

	// Start first
	req := httptest.NewRequest("POST", "/api/array/start", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Then stop
	req = httptest.NewRequest("POST", "/api/array/stop", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestArrayDoubleStartReturnsError(t *testing.T) {
	router := setupTestRouter()

	// Start once
	req := httptest.NewRequest("POST", "/api/array/start", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Start again — should fail
	req = httptest.NewRequest("POST", "/api/array/start", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for double start, got %d", w.Code)
	}
}

func TestModuleStatusEndpoint(t *testing.T) {
	router := setupTestRouter()
	req := httptest.NewRequest("GET", "/api/system/module", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestCorsHeaders(t *testing.T) {
	router := setupTestRouter()
	req := httptest.NewRequest("OPTIONS", "/api/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected CORS origin header")
	}
}
