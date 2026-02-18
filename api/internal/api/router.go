// Package api provides the HTTP REST API for Proxmaid.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/proxmaid/proxmaid/internal/array"
	"github.com/proxmaid/proxmaid/internal/cache"
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
func NewRouter(arrayMgr *array.Manager, sysMgr *system.Manager, diskMgr *disk.Manager, cacheMgr *cache.Manager) http.Handler {
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

	// Assign disk to array slot (array must be stopped)
	mux.HandleFunc("POST /api/array/assign", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Slot   int    `json:"slot"`
			Device string `json:"device"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		if err := arrayMgr.AssignDisk(req.Slot, req.Device); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "disk assigned"})
	})

	// Unassign disk from slot (array must be stopped)
	mux.HandleFunc("POST /api/array/unassign", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Slot int `json:"slot"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		if err := arrayMgr.UnassignDisk(req.Slot); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "disk unassigned"})
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

	// --- Cache Pool Endpoints ---

	// List all cache pools
	mux.HandleFunc("GET /api/cache/pools", func(w http.ResponseWriter, r *http.Request) {
		pools := cacheMgr.GetPools()
		writeJSON(w, http.StatusOK, response{OK: true, Data: pools})
	})

	// Get a single cache pool
	mux.HandleFunc("GET /api/cache/pools/", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Path[len("/api/cache/pools/"):]
		if name == "" {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "pool name required"})
			return
		}
		pool, err := cacheMgr.GetPool(name)
		if err != nil {
			writeJSON(w, http.StatusNotFound, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: pool})
	})

	// Create a cache pool
	mux.HandleFunc("POST /api/cache/pools", func(w http.ResponseWriter, r *http.Request) {
		var req cache.CreatePoolRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		pool, err := cacheMgr.CreatePool(req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, response{OK: true, Data: pool})
	})

	// Delete a cache pool
	mux.HandleFunc("DELETE /api/cache/pools/", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Path[len("/api/cache/pools/"):]
		if name == "" {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "pool name required"})
			return
		}
		if err := cacheMgr.DeletePool(name); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "pool deleted"})
	})

	// --- Mover Endpoints ---

	// Get mover status
	mux.HandleFunc("GET /api/cache/mover", func(w http.ResponseWriter, r *http.Request) {
		status := cacheMgr.GetMoverStatus()
		writeJSON(w, http.StatusOK, response{OK: true, Data: status})
	})

	// Trigger mover run
	mux.HandleFunc("POST /api/cache/mover/run", func(w http.ResponseWriter, r *http.Request) {
		if err := cacheMgr.RunMover(); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "mover started"})
	})

	// Update mover config
	mux.HandleFunc("PUT /api/cache/mover/config", func(w http.ResponseWriter, r *http.Request) {
		var config cache.MoverConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		if err := cacheMgr.UpdateMoverConfig(config); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "mover config updated"})
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
