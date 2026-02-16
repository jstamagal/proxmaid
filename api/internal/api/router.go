// Package api provides the HTTP REST API for Proxmaid.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/proxmaid/proxmaid/internal/array"
	"github.com/proxmaid/proxmaid/internal/disk"
	"github.com/proxmaid/proxmaid/internal/system"
)

// response is the standard API response envelope.
type response struct {
	OK    bool        `json:"ok"`
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

// NewRouter creates the HTTP router with all API routes.
func NewRouter(arrayMgr *array.Manager, sysMgr *system.Manager, diskMgr *disk.Manager) http.Handler {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, response{OK: true, Data: "proxmaid is running"})
	})

	// Array status
	mux.HandleFunc("GET /api/array/status", func(w http.ResponseWriter, r *http.Request) {
		status, err := arrayMgr.Status()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: status})
	})

	// Start array
	mux.HandleFunc("POST /api/array/start", func(w http.ResponseWriter, r *http.Request) {
		if err := arrayMgr.Start(); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "array started"})
	})

	// Stop array
	mux.HandleFunc("POST /api/array/stop", func(w http.ResponseWriter, r *http.Request) {
		if err := arrayMgr.Stop(); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "array stopped"})
	})

	// Parity check
	mux.HandleFunc("POST /api/array/check", func(w http.ResponseWriter, r *http.Request) {
		mode := r.URL.Query().Get("mode")
		if mode == "" {
			mode = "CORRECT"
		}
		if err := arrayMgr.Check(mode); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "check started"})
	})

	// System info
	mux.HandleFunc("GET /api/system/module", func(w http.ResponseWriter, r *http.Request) {
		loaded := sysMgr.IsModuleLoaded()
		writeJSON(w, http.StatusOK, response{OK: true, Data: map[string]bool{"loaded": loaded}})
	})

	// List all disks
	mux.HandleFunc("GET /api/disks", func(w http.ResponseWriter, r *http.Request) {
		disks, err := diskMgr.ListDisks()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: disks})
	})

	// Get SMART health for a disk
	mux.HandleFunc("GET /api/disks/smart", func(w http.ResponseWriter, r *http.Request) {
		device := r.URL.Query().Get("device")
		if device == "" {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "device parameter required"})
			return
		}
		health, err := diskMgr.GetSmartHealth(device)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: health})
	})

	// CORS middleware for development
	return corsMiddleware(mux)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
