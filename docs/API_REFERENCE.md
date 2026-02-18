# Proxmaid v2 — API Reference

**Base URL**: `http://localhost:8484`
**Version**: `0.1.0`

## Response Envelope

All endpoints return a standard JSON envelope:

```json
{
    "ok": true,
    "data": { ... },
    "error": "string (only present on failure)"
}
```

- `ok` — `true` on success, `false` on error
- `data` — response payload (type varies per endpoint)
- `error` — human-readable error message (only on failure)

## CORS

All responses include `Access-Control-Allow-Origin: *` for development. Preflight `OPTIONS` requests return `200 OK`.

---

## Health

### `GET /api/health`

Health check endpoint.

**Response**: `data: "proxmaid is running"`

```bash
curl http://localhost:8484/api/health
```

---

## Array

### `GET /api/array/status`

Returns full array status parsed from `/proc/nmdstat`.

**Response Body** (`data: ArrayStatus`):

```json
{
    "state": "STARTED",
    "num_disks": 4,
    "num_invalid": 0,
    "synced": true,
    "synced_time": "1771262613",
    "resync_active": false,
    "resync_pct": 0,
    "disks": [
        {
            "slot": 0,
            "status": "DISK_OK",
            "device_name": "nvme0n1p1",
            "virt_name": "",
            "size_bytes": 500101545984,
            "size_human": "465.8 GB",
            "role": "parity",
            "disk_id": "nvme-SK_Hynix_P41_MOCK001",
            "reads": 0,
            "writes": 0,
            "errors": 0
        }
    ]
}
```

**Array States**: `STOPPED`, `STARTING`, `STARTED`, `STOPPING`, `DEGRADED`, `ERROR`

**Disk Statuses**: `DISK_OK`, `DISK_NP` (not present), `DISK_DSBL` (disabled/failed)

**Disk Roles**: `parity` (slot 0), `data` (slots 1–28), `q-parity` (slot 29)

---

### `POST /api/array/start`

Starts the array (`nmdctl start`). Fails if array is already started.

**Response**: `data: "array started"` or `error: "array is already started"`

---

### `POST /api/array/stop`

Stops the array (`nmdctl stop`). Fails if array is already stopped.

**Response**: `data: "array stopped"` or `error: "array is already stopped"`

---

### `POST /api/array/check`

Starts a parity check.

**Query Parameters**:
- `mode` — `CORRECT` (default) or `NOCORRECT`. Determines whether mismatches are corrected.

**Response**: `data: "check started"`

**Precondition**: Array must be in `STARTED` state.

```bash
curl -X POST "http://localhost:8484/api/array/check?mode=CORRECT"
```

---

## System

### `GET /api/system/module`

Checks if the `md_nonraid` kernel module is loaded.

**Response**: `data: { "loaded": true }`

---

## Disks

### `GET /api/disks`

Lists all block devices on the system via `lsblk`.

**Response Body** (`data: SystemDisk[]`):

```json
[
    {
        "name": "sda",
        "path": "/dev/sda",
        "size": 256060514304,
        "size_human": "238.5 GB",
        "model": "Samsung SSD 870",
        "serial": "S1234",
        "type": "disk",
        "mountpoint": "",
        "fstype": "",
        "rotational": false,
        "disk_id": ""
    }
]
```

---

### `GET /api/disks/smart`

Returns SMART health data for a specific disk.

**Query Parameters**:
- `device` — **(required)** Device path, e.g. `/dev/sda`

**Response Body** (`data: SmartHealth`):

```json
{
    "device": "/dev/sda",
    "healthy": true,
    "temperature": 34,
    "power_on_hours": 12450,
    "raw_output": "..."
}
```

```bash
curl "http://localhost:8484/api/disks/smart?device=/dev/sda"
```

---

## Cache Pools

### `GET /api/cache/pools`

Lists all configured cache pools.

**Response Body** (`data: CachePool[]`):

```json
[
    {
        "name": "nvme-fast",
        "devices": [
            { "path": "/dev/nvme0n1", "model": "SK Hynix P41", "size": 2000398934016, "size_human": "1.8 TB" }
        ],
        "mount_point": "/mnt/cache/nvme-fast",
        "fs_type": "xfs",
        "total_bytes": 6001196802048,
        "used_bytes": 1800359040614,
        "free_bytes": 4200837761434,
        "used_pct": 30.0,
        "status": "active"
    }
]
```

**Pool Statuses**: `active`, `degraded`, `stopped`

---

### `GET /api/cache/pools/{name}`

Returns a single cache pool by name.

**Path Parameters**: `name` — pool name (e.g. `nvme-fast`)

**Error**: `404` if pool not found

---

### `POST /api/cache/pools`

Creates a new cache pool.

**Request Body**:

```json
{
    "name": "my-pool",
    "devices": ["/dev/nvme0n1"],
    "fs_type": "xfs"
}
```

- `name` — unique pool name (required)
- `devices` — array of device paths (at least one required)
- `fs_type` — `"xfs"`, `"btrfs"`, or `"ext4"` (defaults to `"xfs"`)

**Response**: `201 Created` with the new pool object

---

### `DELETE /api/cache/pools/{name}`

Deletes a cache pool. Unmounts the pool in real mode.

**Path Parameters**: `name` — pool name

**Response**: `data: "pool deleted"`

---

## Mover

### `GET /api/cache/mover`

Returns the mover daemon status.

**Response Body** (`data: MoverStatus`):

```json
{
    "running": false,
    "last_run": "2026-02-16T22:00:00Z",
    "next_run": "2026-02-17T03:40:00Z",
    "bytes_moved": 5368709120,
    "files_moved": 127,
    "progress": 100,
    "config": {
        "schedule": "40 3 * * *",
        "age_threshold": "1d",
        "enabled": true
    }
}
```

---

### `POST /api/cache/mover/run`

Triggers an immediate mover run. Fails if mover is already running.

**Response**: `data: "mover started"`

In mock mode, the mover simulates a 2-second run and reports 1 GB / 42 files moved.

---

### `PUT /api/cache/mover/config`

Updates the mover configuration.

**Request Body**:

```json
{
    "schedule": "40 3 * * *",
    "age_threshold": "12h",
    "enabled": true
}
```

- `schedule` — cron expression (required)
- `age_threshold` — duration string, e.g. `"1d"`, `"12h"`, `"30m"` (required)
- `enabled` — boolean

**Response**: `data: "mover config updated"`

---

## Planned Endpoints (Not Yet Implemented)

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/api/array/assign` | Assign disk to array slot |
| POST | `/api/array/unassign` | Remove disk from slot |
| GET | `/api/shares` | List shares |
| POST | `/api/shares` | Create share |
| DELETE | `/api/shares/{name}` | Delete share |
| GET | `/api/apps` | List Docker containers |
| POST | `/api/apps` | Install app from template |
| GET | `/api/notifications/config` | Notification settings |
| WS | `/api/ws` | Real-time WebSocket |

## Go Struct Reference

### `array.DiskInfo`
```go
type DiskInfo struct {
    Slot       int    `json:"slot"`
    Status     string `json:"status"`
    DeviceName string `json:"device_name"`
    VirtName   string `json:"virt_name"`
    SizeBytes  int64  `json:"size_bytes"`
    SizeHuman  string `json:"size_human"`
    Role       string `json:"role"`
    DiskID     string `json:"disk_id"`
    Reads      int64  `json:"reads"`
    Writes     int64  `json:"writes"`
    Errors     int    `json:"errors"`
}
```

### `array.ArrayStatus`
```go
type ArrayStatus struct {
    State        State      `json:"state"`
    NumDisks     int        `json:"num_disks"`
    NumInvalid   int        `json:"num_invalid"`
    Synced       bool       `json:"synced"`
    SyncedTime   string     `json:"synced_time"`
    ResyncActive bool       `json:"resync_active"`
    ResyncPct    float64    `json:"resync_pct"`
    Disks        []DiskInfo `json:"disks"`
}
```

### `cache.Pool`
```go
type Pool struct {
    Name       string       `json:"name"`
    Devices    []PoolDevice `json:"devices"`
    MountPoint string       `json:"mount_point"`
    FSType     string       `json:"fs_type"`
    TotalBytes int64        `json:"total_bytes"`
    UsedBytes  int64        `json:"used_bytes"`
    FreeBytes  int64        `json:"free_bytes"`
    UsedPct    float64      `json:"used_pct"`
    Status     string       `json:"status"`
}
```

### `cache.MoverConfig`
```go
type MoverConfig struct {
    Schedule     string `json:"schedule"`
    AgeThreshold string `json:"age_threshold"`
    Enabled      bool   `json:"enabled"`
}
```

### `cache.MoverStatus`
```go
type MoverStatus struct {
    Running    bool        `json:"running"`
    LastRun    string      `json:"last_run"`
    NextRun    string      `json:"next_run"`
    BytesMoved int64       `json:"bytes_moved"`
    FilesMoved int         `json:"files_moved"`
    Progress   float64     `json:"progress"`
    Config     MoverConfig `json:"config"`
}
```

### `disk.Info`
```go
type Info struct {
    Name       string `json:"name"`
    Path       string `json:"path"`
    Size       int64  `json:"size"`
    SizeHuman  string `json:"size_human"`
    Model      string `json:"model"`
    Serial     string `json:"serial"`
    Type       string `json:"type"`
    Mountpoint string `json:"mountpoint"`
    FSType     string `json:"fstype"`
    Rotational bool   `json:"rotational"`
    DiskID     string `json:"disk_id"`
}
```

### `disk.SmartHealth`
```go
type SmartHealth struct {
    Device      string `json:"device"`
    Healthy     bool   `json:"healthy"`
    Temperature int    `json:"temperature"`
    PowerOnHrs  int    `json:"power_on_hours"`
    RawOutput   string `json:"raw_output"`
}
```
