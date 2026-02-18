// Package api provides the HTTP REST API for Proxmaid.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/proxmaid/proxmaid/internal/app"
	"github.com/proxmaid/proxmaid/internal/array"
	"github.com/proxmaid/proxmaid/internal/auth"
	"github.com/proxmaid/proxmaid/internal/cache"
	"github.com/proxmaid/proxmaid/internal/disk"
	"github.com/proxmaid/proxmaid/internal/notify"
	"github.com/proxmaid/proxmaid/internal/scheduler"
	"github.com/proxmaid/proxmaid/internal/share"
	"github.com/proxmaid/proxmaid/internal/system"
)

// response is the standard API response envelope.
type response struct {
	OK    bool        `json:"ok"`
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

// NewRouter creates the HTTP router with all API routes.
func NewRouter(arrayMgr *array.Manager, sysMgr *system.Manager, diskMgr *disk.Manager, cacheMgr *cache.Manager, shareMgr *share.Manager, appMgr *app.Manager, notifyMgr *notify.Manager, authMgr *auth.Manager, schedMgr *scheduler.Manager) http.Handler {
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

	// Import existing array from superblock
	mux.HandleFunc("POST /api/array/import", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SuperblockPath string `json:"superblock_path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		status, err := arrayMgr.Import(req.SuperblockPath)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: status})
	})

	// Create a new array
	mux.HandleFunc("POST /api/array/new", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ParityDevice string `json:"parity_device"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		if err := arrayMgr.CreateArray(req.ParityDevice); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "array created"})
	})

	// Replace a failed disk
	mux.HandleFunc("POST /api/array/replace", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Slot   int    `json:"slot"`
			Device string `json:"device"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		if err := arrayMgr.ReplaceDisk(req.Slot, req.Device); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "disk replaced, rebuild will begin automatically"})
	})

	// Add a data disk to next empty slot
	mux.HandleFunc("POST /api/array/add", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Device string `json:"device"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		if err := arrayMgr.AddDisk(req.Device); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "disk added to array"})
	})

	// Format a data disk's partition in the array
	mux.HandleFunc("POST /api/array/format", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Slot   int    `json:"slot"`
			FSType string `json:"fs_type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		if req.FSType == "" {
			req.FSType = "xfs"
		}
		if err := arrayMgr.FormatSlot(req.Slot, req.FSType, diskMgr); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "slot formatted"})
	})

	// Mount a data disk
	mux.HandleFunc("POST /api/array/mount", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Slot int `json:"slot"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		if err := arrayMgr.MountSlot(req.Slot, diskMgr); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: fmt.Sprintf("disk mounted to /mnt/disk%d", req.Slot)})
	})

	// System info
	mux.HandleFunc("GET /api/system/module", func(w http.ResponseWriter, r *http.Request) {
		loaded := sysMgr.IsModuleLoaded()
		writeJSON(w, http.StatusOK, response{OK: true, Data: map[string]bool{"loaded": loaded}})
	})

	// UPS status
	mux.HandleFunc("GET /api/system/ups", func(w http.ResponseWriter, r *http.Request) {
		status, err := sysMgr.GetUPSStatus()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: status})
	})

	// System logs
	mux.HandleFunc("GET /api/system/logs", func(w http.ResponseWriter, r *http.Request) {
		lines := 100
		if l := r.URL.Query().Get("lines"); l != "" {
			fmt.Sscanf(l, "%d", &lines)
		}
		unit := r.URL.Query().Get("unit")
		entries, err := sysMgr.GetLogs(lines, unit)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: entries})
	})

	// Get timezone
	mux.HandleFunc("GET /api/system/timezone", func(w http.ResponseWriter, r *http.Request) {
		tz, err := sysMgr.GetTimezone()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: map[string]string{"timezone": tz}})
	})

	// Set timezone
	mux.HandleFunc("PUT /api/system/timezone", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Timezone string `json:"timezone"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		if err := sysMgr.SetTimezone(req.Timezone); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "timezone updated"})
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

	// --- Disk Health & Power Endpoints ---

	// Get cached SMART health for all disks
	mux.HandleFunc("GET /api/disks/health", func(w http.ResponseWriter, r *http.Request) {
		health := diskMgr.GetCachedHealth()
		writeJSON(w, http.StatusOK, response{OK: true, Data: health})
	})

	// Set standby timeout for a disk
	mux.HandleFunc("PUT /api/disks/standby", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Device  string `json:"device"`
			Minutes int    `json:"minutes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		if req.Device == "" {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "device is required"})
			return
		}
		if err := diskMgr.SetStandbyTimeout(req.Device, req.Minutes); err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "standby timeout set"})
	})

	// Get power state of a disk
	mux.HandleFunc("GET /api/disks/power-state", func(w http.ResponseWriter, r *http.Request) {
		device := r.URL.Query().Get("device")
		if device == "" {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "device parameter required"})
			return
		}
		state, err := diskMgr.GetPowerState(device)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: map[string]string{"device": device, "state": state}})
	})

	// Spin down a disk
	mux.HandleFunc("POST /api/disks/spindown", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Device string `json:"device"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		if req.Device == "" {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "device is required"})
			return
		}
		if err := diskMgr.Spindown(req.Device); err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "disk spun down"})
	})

	// Identify disk (blink LED)
	mux.HandleFunc("POST /api/disks/identify", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Device string `json:"device"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		if req.Device == "" {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "device is required"})
			return
		}
		if err := diskMgr.IdentifyDisk(req.Device); err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "disk identification triggered"})
	})

	// --- Disk Operation Endpoints (destructive) ---

	// Wipe a disk (erase partition table)
	mux.HandleFunc("POST /api/disks/wipe", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Device  string `json:"device"`
			Confirm bool   `json:"confirm"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		if !req.Confirm {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "confirm must be true for destructive operations"})
			return
		}
		if req.Device == "" {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "device is required"})
			return
		}
		if err := diskMgr.WipeDisk(req.Device); err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "disk wiped"})
	})

	// Partition a disk (GPT, single partition)
	mux.HandleFunc("POST /api/disks/partition", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Device  string `json:"device"`
			Confirm bool   `json:"confirm"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		if !req.Confirm {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "confirm must be true for destructive operations"})
			return
		}
		if req.Device == "" {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "device is required"})
			return
		}
		if err := diskMgr.PartitionDisk(req.Device); err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "disk partitioned"})
	})

	// Format a partition
	mux.HandleFunc("POST /api/disks/format", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Device  string `json:"device"`
			FSType  string `json:"fs_type"`
			Confirm bool   `json:"confirm"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		if !req.Confirm {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "confirm must be true for destructive operations"})
			return
		}
		if req.Device == "" {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "device is required"})
			return
		}
		if req.FSType == "" {
			req.FSType = "xfs"
		}
		if err := diskMgr.FormatPartition(req.Device, req.FSType); err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "partition formatted"})
	})

	// Mount a partition
	mux.HandleFunc("POST /api/disks/mount", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Device     string `json:"device"`
			MountPoint string `json:"mount_point"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		if req.Device == "" || req.MountPoint == "" {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "device and mount_point are required"})
			return
		}
		if err := diskMgr.Mount(req.Device, req.MountPoint); err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "partition mounted"})
	})

	// Unmount a mount point
	mux.HandleFunc("POST /api/disks/unmount", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			MountPoint string `json:"mount_point"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		if req.MountPoint == "" {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "mount_point is required"})
			return
		}
		if err := diskMgr.Unmount(req.MountPoint); err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "unmounted"})
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

	// Add device to a pool
	mux.HandleFunc("POST /api/cache/pools/{name}/devices", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		var req struct {
			Device string `json:"device"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		if err := cacheMgr.AddDevice(name, req.Device); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "device added to pool"})
	})

	// Remove device from a pool
	mux.HandleFunc("DELETE /api/cache/pools/{name}/devices/{dev}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		dev := r.PathValue("dev")
		if err := cacheMgr.RemoveDevice(name, "/dev/"+dev); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "device removed from pool"})
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

	// --- Share Endpoints ---

	// List all shares
	mux.HandleFunc("GET /api/shares", func(w http.ResponseWriter, r *http.Request) {
		shares := shareMgr.ListShares()
		writeJSON(w, http.StatusOK, response{OK: true, Data: shares})
	})

	// Get a single share
	mux.HandleFunc("GET /api/shares/", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Path[len("/api/shares/"):]
		if name == "" {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "share name required"})
			return
		}
		s, err := shareMgr.GetShare(name)
		if err != nil {
			writeJSON(w, http.StatusNotFound, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: s})
	})

	// Create a share
	mux.HandleFunc("POST /api/shares", func(w http.ResponseWriter, r *http.Request) {
		var req share.Share
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		s, err := shareMgr.CreateShare(req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, response{OK: true, Data: s})
	})

	// Update a share
	mux.HandleFunc("PUT /api/shares/", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Path[len("/api/shares/"):]
		if name == "" {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "share name required"})
			return
		}
		var req share.Share
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		s, err := shareMgr.UpdateShare(name, req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: s})
	})

	// Delete a share
	mux.HandleFunc("DELETE /api/shares/", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Path[len("/api/shares/"):]
		if name == "" {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "share name required"})
			return
		}
		if err := shareMgr.DeleteShare(name); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "share deleted"})
	})

	// Mount a share's union filesystem
	mux.HandleFunc("POST /api/shares/{name}/mount", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		s, err := shareMgr.GetShare(name)
		if err != nil {
			writeJSON(w, http.StatusNotFound, response{OK: false, Error: err.Error()})
			return
		}
		if err := shareMgr.MountUnionFS(s); err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "share mounted"})
	})

	// Unmount a share's union filesystem
	mux.HandleFunc("POST /api/shares/{name}/unmount", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		s, err := shareMgr.GetShare(name)
		if err != nil {
			writeJSON(w, http.StatusNotFound, response{OK: false, Error: err.Error()})
			return
		}
		if err := shareMgr.UnmountUnionFS(s); err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "share unmounted"})
	})

	// Set allocation method for a share
	mux.HandleFunc("PUT /api/shares/{name}/alloc", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		var req struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		if err := shareMgr.SetAllocMethod(name, req.Method); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "allocation method updated"})
	})

	// --- User Management Endpoints ---

	// List users
	mux.HandleFunc("GET /api/users", func(w http.ResponseWriter, r *http.Request) {
		users := shareMgr.ListUsers()
		writeJSON(w, http.StatusOK, response{OK: true, Data: users})
	})

	// Create user
	mux.HandleFunc("POST /api/users", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		if err := shareMgr.CreateUser(req.Username, req.Password); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, response{OK: true, Data: "user created"})
	})

	// Delete user
	mux.HandleFunc("DELETE /api/users/", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Path[len("/api/users/"):]
		if name == "" {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "username required"})
			return
		}
		if err := shareMgr.DeleteUser(name); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "user deleted"})
	})

	// --- App (Docker) Endpoints ---

	// List all containers
	mux.HandleFunc("GET /api/apps", func(w http.ResponseWriter, r *http.Request) {
		containers, err := appMgr.ListContainers()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: containers})
	})

	// Start a container
	mux.HandleFunc("POST /api/apps/{id}/start", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := appMgr.StartContainer(id); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "container started"})
	})

	// Stop a container
	mux.HandleFunc("POST /api/apps/{id}/stop", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := appMgr.StopContainer(id); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "container stopped"})
	})

	// Remove a container
	mux.HandleFunc("DELETE /api/apps/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := appMgr.RemoveContainer(id); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "container removed"})
	})

	// Get container logs
	mux.HandleFunc("GET /api/apps/{id}/logs", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		lines := 100
		if l := r.URL.Query().Get("lines"); l != "" {
			fmt.Sscanf(l, "%d", &lines)
		}
		logs, err := appMgr.GetLogs(id, lines)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: logs})
	})

	// Get container stats
	mux.HandleFunc("GET /api/apps/{id}/stats", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		stats, err := appMgr.GetStats(id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: stats})
	})

	// Docker Compose up
	mux.HandleFunc("POST /api/apps/compose/up", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		if req.Path == "" {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "path is required"})
			return
		}
		if err := appMgr.ComposeUp(req.Path); err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "compose up completed"})
	})

	// Docker Compose down
	mux.HandleFunc("POST /api/apps/compose/down", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		if req.Path == "" {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "path is required"})
			return
		}
		if err := appMgr.ComposeDown(req.Path); err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "compose down completed"})
	})

	// App templates
	mux.HandleFunc("GET /api/apps/templates", func(w http.ResponseWriter, r *http.Request) {
		category := r.URL.Query().Get("category")
		search := r.URL.Query().Get("search")
		templates := appMgr.GetTemplates(category, search)
		writeJSON(w, http.StatusOK, response{OK: true, Data: templates})
	})

	// Install app from template
	mux.HandleFunc("POST /api/apps/install", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Template  app.AppTemplate   `json:"template"`
			Overrides map[string]string `json:"overrides"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		if err := appMgr.InstallApp(req.Template, req.Overrides); err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "app installed"})
	})

	// Check for app updates
	mux.HandleFunc("GET /api/apps/updates", func(w http.ResponseWriter, r *http.Request) {
		updates, err := appMgr.CheckUpdates()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: updates})
	})

	// List Docker networks
	mux.HandleFunc("GET /api/apps/networks", func(w http.ResponseWriter, r *http.Request) {
		networks, err := appMgr.ListNetworks()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: networks})
	})

	// Create Docker network
	mux.HandleFunc("POST /api/apps/networks", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name   string `json:"name"`
			Driver string `json:"driver"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		if req.Name == "" || req.Driver == "" {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "name and driver are required"})
			return
		}
		if err := appMgr.CreateNetwork(req.Name, req.Driver); err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, response{OK: true, Data: "network created"})
	})

	// Delete Docker network
	mux.HandleFunc("DELETE /api/apps/networks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := appMgr.DeleteNetwork(id); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "network deleted"})
	})

	// --- Notification Endpoints ---

	// Get notification config
	mux.HandleFunc("GET /api/notifications/config", func(w http.ResponseWriter, r *http.Request) {
		config := notifyMgr.GetConfig()
		writeJSON(w, http.StatusOK, response{OK: true, Data: config})
	})

	// Update notification config
	mux.HandleFunc("PUT /api/notifications/config", func(w http.ResponseWriter, r *http.Request) {
		var config notify.NotifyConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		if err := notifyMgr.UpdateConfig(config); err != nil {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "config updated"})
	})

	// Send test notification
	mux.HandleFunc("POST /api/notifications/test", func(w http.ResponseWriter, r *http.Request) {
		notifyMgr.SendTest()
		writeJSON(w, http.StatusOK, response{OK: true, Data: "test notification sent"})
	})

	// Get notification history
	mux.HandleFunc("GET /api/notifications/history", func(w http.ResponseWriter, r *http.Request) {
		history := notifyMgr.GetHistory()
		writeJSON(w, http.StatusOK, response{OK: true, Data: history})
	})

	// --- Scheduled Task Endpoints ---

	// List all scheduled tasks
	mux.HandleFunc("GET /api/tasks", func(w http.ResponseWriter, r *http.Request) {
		tasks := schedMgr.ListTasks()
		writeJSON(w, http.StatusOK, response{OK: true, Data: tasks})
	})

	// Update a task's schedule/enabled state
	mux.HandleFunc("PUT /api/tasks/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		var req struct {
			Schedule string `json:"schedule"`
			Enabled  bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		if err := schedMgr.UpdateTask(name, req.Schedule, req.Enabled); err != nil {
			writeJSON(w, http.StatusNotFound, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "task updated"})
	})

	// Trigger a task manually
	mux.HandleFunc("POST /api/tasks/{name}/run", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if err := schedMgr.TriggerTask(name); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "task triggered"})
	})

	// --- SSE (Server-Sent Events) Endpoint ---
	eventHub := NewEventHub()

	mux.HandleFunc("GET /api/events", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: "streaming not supported"})
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		ch := eventHub.Subscribe()
		defer eventHub.Unsubscribe(ch)

		// Send initial connected event
		fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"ok\"}\n\n")
		flusher.Flush()

		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprint(w, msg)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})

	// Store event hub reference for external use (e.g., by notification manager)
	_ = eventHub

	// --- Auth Endpoints ---

	// Login
	mux.HandleFunc("POST /api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{OK: false, Error: "invalid request body"})
			return
		}
		token, err := authMgr.Login(req.Username, req.Password)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, response{OK: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: map[string]string{"token": token}})
	})

	// Logout
	mux.HandleFunc("POST /api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			authMgr.Logout(token)
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: "logged out"})
	})

	// Current user info
	mux.HandleFunc("GET /api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			writeJSON(w, http.StatusUnauthorized, response{OK: false, Error: "not authenticated"})
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")
		claims, valid := authMgr.ValidateToken(token)
		if !valid {
			writeJSON(w, http.StatusUnauthorized, response{OK: false, Error: "invalid token"})
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: map[string]interface{}{
			"username":   claims.Username,
			"expires_at": claims.ExpiresAt.Format(time.RFC3339),
		}})
	})

	// Apply middleware: Rate Limit → CORS → Auth → routes
	limiter := newRateLimiter(100) // 100 requests per minute per IP
	return rateLimitMiddleware(limiter)(corsMiddleware(authMgr.Middleware(mux)))
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

// --- Server-Sent Events (SSE) Hub ---

// EventHub manages SSE client connections and broadcasts events.
type EventHub struct {
	mu      sync.RWMutex
	clients map[chan string]struct{}
}

// NewEventHub creates a new SSE event hub.
func NewEventHub() *EventHub {
	return &EventHub{
		clients: make(map[chan string]struct{}),
	}
}

// Subscribe adds a new client and returns its event channel.
func (h *EventHub) Subscribe() chan string {
	ch := make(chan string, 16)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

// Unsubscribe removes a client.
func (h *EventHub) Unsubscribe(ch chan string) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
	close(ch)
}

// Broadcast sends an event to all connected SSE clients.
func (h *EventHub) Broadcast(eventType, data string) {
	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, data)
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients {
		select {
		case ch <- msg:
		default:
			// Client channel full, skip
		}
	}
}

// --- Rate Limiter ---

type rateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	limit    int           // requests per window
	window   time.Duration // window duration
}

type visitor struct {
	count    int
	windowAt time.Time
}

func newRateLimiter(requestsPerMinute int) *rateLimiter {
	rl := &rateLimiter{
		visitors: make(map[string]*visitor),
		limit:    requestsPerMinute,
		window:   time.Minute,
	}
	// Clean up stale entries every 5 minutes
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			rl.cleanup()
		}
	}()
	return rl
}

func (rl *rateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	for ip, v := range rl.visitors {
		if now.Sub(v.windowAt) > rl.window*2 {
			delete(rl.visitors, ip)
		}
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	v, ok := rl.visitors[ip]
	if !ok || now.Sub(v.windowAt) > rl.window {
		rl.visitors[ip] = &visitor{count: 1, windowAt: now}
		return true
	}
	v.count++
	return v.count <= rl.limit
}

func rateLimitMiddleware(limiter *rateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
				ip = strings.Split(fwd, ",")[0]
			}
			if !limiter.allow(strings.TrimSpace(ip)) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(response{OK: false, Error: "rate limit exceeded"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
