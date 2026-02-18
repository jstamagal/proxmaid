# Proxmaid v2 — Implementation Blueprint

> **This is the master task list AND architectural spec.**
> Every item explains *how* to implement it, following existing codebase patterns.
> Items marked `[x]` are done. Items marked `[ ]` are remaining.
>
> **Pattern reference**: See `docs/ARCHITECTURE.md` for the manager pattern, mock mode, and data flow.
> **API reference**: See `docs/API_REFERENCE.md` for existing endpoint schemas.
>
> Last updated: 2026-02-17

---

## Status Summary

| Category            | Done | Remaining |
|---------------------|------|-----------|
| Storage Array (API) | 8    | 7         |
| Disk Manager (API)  | 3    | 6         |
| Cache & Tiering     | 5    | 7         |
| Shares (API)        | 0    | 8         |
| Docker/Apps (API)   | 0    | 8         |
| Notifications (API) | 0    | 6         |
| API Router/Infra    | 8    | 6         |
| UI — Design System  | 5    | 5         |
| UI — Pages          | 10   | 39        |
| PVE Plugin          | 0    | 5         |
| Infra & Packaging   | 0    | 12        |
| Testing             | 5    | 5         |

---

## Existing Codebase Patterns (read this first)

Every new feature must follow these patterns for consistency:

### Manager Pattern
Each subsystem gets a `Manager` struct in `api/internal/<name>/manager.go`:
```go
type Manager struct {
    mu       sync.RWMutex    // all managers use mutex for thread safety
    mockMode bool            // gates real vs simulated behavior
    // ... subsystem-specific state
}

func NewManager(mockMode bool) *Manager { ... }
```

### Mock Mode
Every function that touches the OS must check `m.mockMode`:
```go
func (m *Manager) DoThing() error {
    if m.mockMode {
        fmt.Println("[MOCK] DoThing")
        return nil
    }
    return exec.Command("real-command", "args").Run()
}
```
Mock mode auto-activates when `/proc/nmdstat` is absent. Add mock data in `loadMock*()` methods.

### Router Pattern
Add endpoints in `api/internal/api/router.go`. Inject the manager as a parameter to `NewRouter()`:
```go
func NewRouter(arrayMgr *array.Manager, /* add new mgr here */) http.Handler {
    mux.HandleFunc("GET /api/<resource>", func(w http.ResponseWriter, r *http.Request) {
        data, err := newMgr.Method()
        if err != nil {
            writeJSON(w, http.StatusInternalServerError, response{OK: false, Error: err.Error()})
            return
        }
        writeJSON(w, http.StatusOK, response{OK: true, Data: data})
    })
}
```
All responses use: `response{OK: bool, Data: interface{}, Error: string}`

### UI Pattern
- Pages: `ui/src/app/<name>/page.tsx` (Next.js App Router, `'use client'`)
- Components: `ui/src/components/<Name>.tsx`
- API types + functions: `ui/src/lib/api.ts`
- All pages include `<Sidebar />` and follow the glassmorphism dark theme in `globals.css`
- Use `useState`/`useEffect` for data fetching with a 5-second polling interval

### Test Pattern
- Tests in `<package>_test.go`, same package (can access unexported functions)
- Use `httptest.NewRequest` + `httptest.NewRecorder` for endpoint tests
- All tests run in mock mode — no real system calls ever

---

## 1. Storage Array (`api/internal/array/`)

### 1.1 Completed ✅
- [x] Parse `/proc/nmdstat` → `ArrayStatus` with `rdevName`/`rdevStatus`/`rdevSize`
- [x] Array start/stop/check lifecycle via `nmdctl` CLI
- [x] Parity/Data/Q-Parity role detection (slot 0 = P, slot 29 = Q, rest = data)
- [x] Resync progress parsing (`mdResyncPos` / `mdResyncSize` → percentage)
- [x] Human-readable disk sizes, I/O stats per disk, disk-by-ID tracking

### 1.2 Remaining

- [ ] **Disk assignment API** — Assign a physical disk to an array slot
  - Add `Manager.AssignDisk(slot int, devicePath string) error`
  - Wraps `nmdctl assign <slot> <device>` (see `docs/nonraid/_research_array_mgmt.md`)
  - In mock mode: update internal state, validate slot is empty (`DISK_NP`), device exists
  - Add endpoint: `POST /api/array/assign` with body `{"slot": 1, "device": "/dev/sdb"}`
  - Router: decode JSON body, call `arrayMgr.AssignDisk()`, return status

- [ ] **Disk unassignment API** — Remove a disk from a slot
  - Add `Manager.UnassignDisk(slot int) error`
  - Wraps `nmdctl unassign <slot>`
  - Precondition: array must be STOPPED (unassign not allowed while running)
  - Add endpoint: `POST /api/array/unassign` with body `{"slot": 1}`

- [ ] **Array import** — Import existing array from superblock
  - Add `Manager.Import(superblockPath string) error`
  - Wraps `nmdctl import <path>` — reads superblock, discovers disk assignments
  - Response should return the imported `ArrayStatus`
  - Add endpoint: `POST /api/array/import` with body `{"superblock_path": "/boot/config/nonraid.dat"}`

- [ ] **New array creation** — Format superblock on new disks
  - Add `Manager.CreateArray(parityDevice string) error`
  - Wraps `nmdctl new <parity-device>` — writes fresh superblock
  - Precondition: no existing array loaded
  - Add endpoint: `POST /api/array/new` with body `{"parity_device": "/dev/nvme0n1"}`

- [ ] **Disk replacement** — Replace a failed disk and trigger rebuild
  - Add `Manager.ReplaceDisk(slot int, newDevice string) error`
  - Wraps `nmdctl replace <slot> <new-device>`
  - Kernel automatically begins parity rebuild after replace
  - Add endpoint: `POST /api/array/replace` with body `{"slot": 1, "device": "/dev/sdf"}`

- [ ] **Array expansion** — Add a data disk to next empty slot
  - Add `Manager.AddDisk(devicePath string) error`
  - Wraps `nmdctl add <device>` — finds first empty data slot (1–28)
  - Add endpoint: `POST /api/array/add` with body `{"device": "/dev/sde"}`

- [ ] **Per-disk filesystem** — Format and mount each data disk
  - Add `Manager.FormatSlot(slot int, fsType string) error`
  - Uses `disk.Manager.FormatPartition()` (already exists) on the slot's device
  - Add `Manager.MountSlot(slot int) error` — mount to `/mnt/disk<slot>`
  - Create mount point `mkdir -p /mnt/disk<slot>`, run `mount /dev/nmd<slot>p1 /mnt/disk<slot>`
  - Add endpoints: `POST /api/array/format` and `POST /api/array/mount`

---

## 2. Disk Manager (`api/internal/disk/`)

### 2.1 Completed ✅
- [x] `ListDisks()` via `lsblk -J -b -o NAME,PATH,SIZE,MODEL,SERIAL,TYPE,MOUNTPOINT,FSTYPE,ROTA`
- [x] `GetSmartHealth()` via `smartctl -a <device>` — parses healthy/temp/power-on hours
- [x] Mock mode with 5 simulated drives, `WipeDisk()`, `PartitionDisk()`, `FormatPartition()`

### 2.2 Remaining

- [ ] **Mount/unmount** — Mount a formatted partition
  - Add `Manager.Mount(partitionPath, mountPoint string) error`
  - Wraps `mount <partition> <mountpoint>` with `mkdir -p` for mount point
  - Add `Manager.Unmount(mountPoint string) error` — wraps `umount <mountpoint>`
  - Mock mode: print `[MOCK] mount ...`

- [ ] **Disk temperature monitoring** — Periodic background polling
  - Add a background goroutine in `Manager` that runs every 30 minutes
  - For each known disk: call `GetSmartHealth()`, cache the result in `map[string]*SmartHealth`
  - Add `Manager.GetCachedHealth() map[string]*SmartHealth` to return latest cached data
  - Add endpoint: `GET /api/disks/health` — returns all cached SMART data
  - Start the goroutine from `NewManager()` (skip in mock mode, use static mock data)

- [ ] **Disk standby management** — Spin down idle HDDs to save power
  - Add `Manager.SetStandbyTimeout(device string, minutes int) error`
  - Wraps `hdparm -S <value> <device>` (value = minutes/5, max 252)
  - Add `Manager.GetPowerState(device string) (string, error)`
  - Wraps `hdparm -C <device>` — parse output for "active/idle" or "standby"
  - Add `Manager.Spindown(device string) error` — wraps `hdparm -Y <device>`
  - Add endpoints: `PUT /api/disks/standby`, `GET /api/disks/power-state`
  - Only applies to rotational disks (`Rotational: true`)

- [ ] **Disk wipe/format endpoints** — Expose existing functions via API
  - `WipeDisk()`, `PartitionDisk()`, `FormatPartition()` already exist but have no endpoints
  - Add endpoints: `POST /api/disks/wipe`, `POST /api/disks/partition`, `POST /api/disks/format`
  - Request body: `{"device": "/dev/sdb", "fs_type": "xfs"}` (fs_type for format only)
  - These are **destructive** — add confirmation field: `{"confirm": true}`

- [ ] **Disk identification** — Blink a disk's LED
  - Add `Manager.IdentifyDisk(device string) error`
  - Try `ledctl locate=<device>` (for SES enclosures), fall back to no-op with warning
  - Add endpoint: `POST /api/disks/identify` with body `{"device": "/dev/sdb"}`

---

## 3. Cache & Tiering (`api/internal/cache/`)

### 3.1 Completed ✅
- [x] Pool CRUD (create/delete/list/get) with mock pools ("nvme-fast", "ssd-warm")
- [x] Mover daemon with config (schedule, age threshold, enabled), manual trigger
- [x] mergerfs auto-install check
- [x] All API endpoints wired in router

### 3.2 Remaining

- [ ] **Real pool creation** — Actually partition, format, and mount devices
  - Modify `CreatePool()`: after validation, for each device in `req.Devices`:
    1. Call `disk.Manager.WipeDisk(device)`
    2. Call `disk.Manager.PartitionDisk(device)`
    3. Call `disk.Manager.FormatPartition(device+"1", req.FSType)`
  - If single device: `mount <device>1 /mnt/cache/<name>`
  - If multiple devices: use `mergerfs` union mount (see mergerfs mount below)
  - Inject `disk.Manager` into `cache.Manager` constructor

- [ ] **mergerfs mount** — Union-mount multiple cache devices
  - When pool has 2+ devices, create individual mount points `/mnt/cache/<name>/dev0`, `/mnt/cache/<name>/dev1`
  - Mount each device partition to its individual mount point
  - Run: `mergerfs -o defaults,allow_other,use_ino,category.create=mfs,moveonenospc=true /mnt/cache/<name>/dev0:/mnt/cache/<name>/dev1 /mnt/cache/<name>/merged`
  - The `mfs` (most free space) policy spreads writes across devices
  - Store the merged path as `Pool.MountPoint`

- [ ] **Real mover execution** — Move cold files from cache to array
  - Implement `executeMoverRun()` (currently a TODO placeholder):
    1. `findColdFiles()` — walk `Pool.MountPoint`, find files with `mtime > AgeThreshold`
    2. For each file: `rsync --checksum --remove-source-files <src> <dest>`
    3. Dest path: mirror the directory structure under `/mnt/user/<share>/`
    4. Track progress: increment `BytesMoved` and `FilesMoved` after each file
    5. Publish event on completion (for notification system)

- [ ] **Find cold files** — Walk cache mount, check mtime vs threshold
  - Add `findColdFiles(poolMount string, threshold time.Duration) ([]string, error)`
  - Use `filepath.Walk()`, check `info.ModTime().Before(time.Now().Add(-threshold))`
  - Parse `AgeThreshold` string: `"1d"` → 24h, `"12h"` → 12h, `"30m"` → 30m
  - Return list of absolute paths to move

- [ ] **Scheduled mover** — Cron-style timer goroutine
  - Parse `MoverConfig.Schedule` cron expression (use `github.com/robfig/cron/v3`)
  - Start a goroutine in `NewManager()` that sleeps until next cron trigger
  - On trigger: call `RunMover()` (reuse existing logic)
  - Update `MoverStatus.NextRun` after each run

- [ ] **Pool resize** — Add or remove devices from an existing pool
  - Add `Manager.AddDevice(poolName, device string) error`
  - Format + mount the new device, then update mergerfs branches:
    `mount -o remount,add=/mnt/cache/<name>/dev2 /mnt/cache/<name>/merged`
  - Add `Manager.RemoveDevice(poolName, device string) error` — move data off first
  - Add endpoints: `POST /api/cache/pools/{name}/devices`, `DELETE /api/cache/pools/{name}/devices/{dev}`

- [ ] **Cache pool health monitoring** — Alert when cache is almost full
  - Add a background goroutine (like disk temp polling) that checks `Pool.UsedPct` every 5 minutes
  - If `UsedPct > 85%`: emit a warning event (for notification system)
  - If `UsedPct > 95%`: trigger an emergency mover run
  - Read real usage via `df -B1 <mountpoint>` and parse output

---

## 4. Share Manager (`api/internal/share/` — NEW PACKAGE)

> Create `api/internal/share/manager.go`. Follow the manager pattern exactly.

- [ ] **Share CRUD** — Create, delete, list, get named shares
  - Define struct:
    ```go
    type Share struct {
        Name           string           `json:"name"`           // e.g. "media"
        Path           string           `json:"path"`           // /mnt/user/<name>
        CachePolicy    string           `json:"cache_policy"`   // "yes", "no", "only", "prefer"
        AllocMethod    string           `json:"alloc_method"`   // "mfs", "lfs", "ff"
        ExportSMB      bool             `json:"export_smb"`
        ExportNFS      bool             `json:"export_nfs"`
        SecurityMode   string           `json:"security_mode"`  // "public", "private", "secure"
        IncludedDisks  []int            `json:"included_disks"` // slot numbers, empty = all
        ExcludedDisks  []int            `json:"excluded_disks"`
        MinFreeSpace   string           `json:"min_free_space"` // e.g. "10GB"
        RecycleBin     bool             `json:"recycle_bin"`
    }
    ```
  - `CreateShare()`: `mkdir -p /mnt/user/<name>`, store config in `map[string]*Share`
  - `DeleteShare()`: unmount, remove directory, clean up SMB/NFS exports
  - Persist share config to `/etc/proxmaid/shares.json` (JSON file)
  - Add endpoints: `GET /api/shares`, `POST /api/shares`, `GET /api/shares/{name}`, `DELETE /api/shares/{name}`, `PUT /api/shares/{name}`

- [ ] **SMB export** — Generate Samba configuration per share
  - For each share with `ExportSMB: true`, write to `/etc/samba/smb.d/<name>.conf`:
    ```ini
    [media]
        path = /mnt/user/media
        browseable = yes
        read only = no
        guest ok = yes  ; if security_mode == "public"
        valid users = @proxmaid  ; if security_mode == "secure"
    ```
  - Include from main `smb.conf` via `include = /etc/samba/smb.d/*.conf`
  - After writing config: `exec.Command("smbcontrol", "smbd", "reload-config")`
  - Mock mode: just log the generated config

- [ ] **NFS export** — Generate `/etc/exports` entries per share
  - For each share with `ExportNFS: true`, append to `/etc/exports`:
    ```
    /mnt/user/media *(rw,sync,no_subtree_check,no_root_squash)
    ```
  - After updating: `exec.Command("exportfs", "-ra")`
  - Mock mode: log the generated exports

- [ ] **mergerfs union mount** — Combine array disks (+ cache) into share path
  - Build branches string from share config: `/mnt/cache/<pool>/<share>:/mnt/disk1/<share>:/mnt/disk2/<share>:...`
  - If `IncludedDisks` set: only include those disk mount points
  - If `ExcludedDisks` set: exclude those
  - Cache policy determines branch ordering:
    - `"yes"`: cache first, then array disks (new writes go to cache, mover moves to array)
    - `"prefer"`: cache first with `category.create=epff` (keep on cache)
    - `"only"`: cache mount only, no array branches
    - `"no"`: array disks only, no cache branch
  - Allocation method maps to mergerfs create policy: `mfs`=most-free, `lfs`=least-free, `ff`=first-found

- [ ] **User management** — Create system users for share access
  - Add `Manager.CreateUser(username, password string) error`
  - Run: `useradd -M -s /usr/sbin/nologin <username>` (no home dir, no shell)
  - Then: `echo '<password>\n<password>' | smbpasswd -a -s <username>`
  - Add `Manager.DeleteUser(username string) error` — `userdel`, `smbpasswd -x`
  - Add `Manager.ListUsers() []string` — parse `/etc/samba/smbpasswd` or `pdbedit -L`
  - Add endpoints: `GET /api/users`, `POST /api/users`, `DELETE /api/users/{name}`

- [ ] **Per-share ACLs** — Control read/write access per user per share
  - Extend `Share` struct with: `ReadUsers []string`, `WriteUsers []string`
  - Generate Samba config:
    ```ini
    read list = user1, user2
    write list = user3
    ```
  - When `security_mode == "private"`: `guest ok = no`, `valid users` required
  - When `security_mode == "secure"`: `valid users` + per-user read/write lists

- [ ] **Recycle bin** — Deleted files go to recycle bin instead of permanent delete
  - For shares with `RecycleBin: true`, add to Samba config:
    ```ini
    vfs objects = recycle
    recycle:repository = .Recycle.Bin/%U
    recycle:keeptree = yes
    recycle:versions = yes
    ```
  - Add purge cron: `find /mnt/user/<share>/.Recycle.Bin -mtime +30 -delete`
  - Run purge via scheduled task system (see section 9)

- [ ] **Allocation methods** — Control which disk gets new files
  - `"mfs"` (Most Free Space): mergerfs `category.create=mfs` — writes to disk with most free space
  - `"lfs"` (Least Free Space / Fill-Up): mergerfs `category.create=lfs` — fills one disk before moving to next
  - `"ff"` (First Found / High-Water): mergerfs `category.create=ff` — writes to first disk with enough space
  - Set via mergerfs mount option, changeable at runtime: `xattr -w user.mergerfs.category.create mfs /mnt/user/<share>`

---

## 5. App Manager (`api/internal/app/` — NEW PACKAGE)

> Create `api/internal/app/manager.go`. Communicates with Docker via its Unix socket API.

- [ ] **Docker container management** — List, inspect, start, stop, remove containers
  - Use Go's `net/http` with Unix socket transport (no external Docker SDK needed):
    ```go
    client := &http.Client{
        Transport: &http.Transport{
            DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
                return net.Dial("unix", "/var/run/docker.sock")
            },
        },
    }
    resp, _ := client.Get("http://localhost/containers/json")
    ```
  - `Manager.ListContainers() ([]Container, error)` → `GET /containers/json`
  - `Manager.StartContainer(id string)` → `POST /containers/{id}/start`
  - `Manager.StopContainer(id string)` → `POST /containers/{id}/stop`
  - `Manager.RemoveContainer(id string)` → `DELETE /containers/{id}`
  - Define `Container` struct matching Docker API: name, image, state, ports, mounts
  - Add endpoints: `GET /api/apps`, `POST /api/apps/{id}/start`, `POST /api/apps/{id}/stop`, `DELETE /api/apps/{id}`

- [ ] **Container logs** — Stream container logs
  - `Manager.GetLogs(id string, lines int) (string, error)` → `GET /containers/{id}/logs?stdout=true&stderr=true&tail=<lines>`
  - Add endpoint: `GET /api/apps/{id}/logs?lines=100`

- [ ] **Container stats** — CPU, memory, network usage
  - `Manager.GetStats(id string) (*ContainerStats, error)` → `GET /containers/{id}/stats?stream=false`
  - Parse Docker stats response into simple struct: `CPUPercent`, `MemUsage`, `MemLimit`, `NetIn`, `NetOut`
  - Add endpoint: `GET /api/apps/{id}/stats`

- [ ] **Parse Unraid CA templates** — Convert XML app templates to JSON catalog
  - Unraid Community Applications templates are XML files in public GitHub repos
  - Clone/download templates repo, parse each template XML:
    ```xml
    <Container>
      <Name>Plex</Name>
      <Repository>plexinc/pms-docker</Repository>
      <Icon>https://...</Icon>
      <Category>Media</Category>
      <Config Name="Media" Target="/media" Type="Path" ... />
      <Config Name="PLEX_CLAIM" Target="PLEX_CLAIM" Type="Variable" ... />
    </Config>
    ```
  - Convert to `AppTemplate` struct: `Name`, `Image`, `Icon`, `Category`, `Ports[]`, `Volumes[]`, `Env[]`
  - Cache parsed templates in memory, refresh periodically
  - Add endpoint: `GET /api/apps/templates?category=Media&search=plex`

- [ ] **Install from template** — Create container from template config
  - `Manager.InstallApp(template AppTemplate, overrides map[string]string) error`
  - Build Docker create request from template:
    ```json
    POST /containers/create
    {
      "Image": "plexinc/pms-docker",
      "Env": ["PLEX_CLAIM=xxx"],
      "HostConfig": {
        "PortBindings": {"32400/tcp": [{"HostPort": "32400"}]},
        "Binds": ["/mnt/user/media:/media:rw"]
      }
    }
    ```
  - Then `POST /containers/{id}/start`
  - Pull image first if not present: `POST /images/create?fromImage=plexinc/pms-docker`

- [ ] **App update detection** — Compare running image digest vs registry
  - For each running container: get image digest (`GET /images/{id}/json` → `RepoDigests`)
  - Compare against registry: `GET https://registry-1.docker.io/v2/<repo>/manifests/latest`
  - If digests differ, mark container as `update_available: true`
  - Add endpoint: `GET /api/apps/updates`

- [ ] **Docker Compose support** — Run compose files
  - `Manager.ComposeUp(composePath string) error` → `exec.Command("docker", "compose", "-f", composePath, "up", "-d")`
  - `Manager.ComposeDown(composePath string) error` → `docker compose -f ... down`
  - Store compose file paths in config
  - Add endpoints: `POST /api/apps/compose/up`, `POST /api/apps/compose/down`

- [ ] **Network management** — Create Docker networks
  - `Manager.CreateNetwork(name, driver string) error` → `POST /networks/create {"Name": "...", "Driver": "bridge|host|macvlan"}`
  - Required for macvlan setups (giving containers their own IP)
  - Add endpoint: `POST /api/apps/networks`

---

## 6. Notification System (`api/internal/notify/` — NEW PACKAGE)

> Create `api/internal/notify/manager.go`. Event-driven architecture.

- [ ] **Event framework** — Internal event emitter with typed events
  - Define event types as constants:
    ```go
    type EventType string
    const (
        EventDiskFailure    EventType = "disk_failure"
        EventParityDone     EventType = "parity_done"
        EventMoverDone      EventType = "mover_done"
        EventSmartWarning   EventType = "smart_warning"
        EventArrayStopped   EventType = "array_stopped"
        EventCacheAlmostFull EventType = "cache_almost_full"
    )
    type Event struct {
        Type      EventType
        Message   string
        Severity  string  // "info", "warning", "critical"
        Timestamp time.Time
        Data      map[string]interface{}
    }
    ```
  - `Manager.Emit(event Event)` — dispatches to all configured providers
  - Use Go channels: `Manager` has `chan Event`, providers read from it
  - Other managers call `notifyMgr.Emit(...)` when things happen
  - Inject `notify.Manager` into other managers that need to emit events

- [ ] **Discord webhook** — Send notifications to a Discord channel
  - Add provider `DiscordProvider` with `WebhookURL string`
  - Send via HTTP POST:
    ```go
    body := fmt.Sprintf(`{"content": "[%s] %s: %s"}`, event.Severity, event.Type, event.Message)
    http.Post(webhookURL, "application/json", strings.NewReader(body))
    ```
  - Support rich embeds with color coding (green=info, yellow=warning, red=critical)

- [ ] **Pushover** — Push notifications to mobile
  - Add provider `PushoverProvider` with `UserKey, AppToken string`
  - POST to `https://api.pushover.net/1/messages.json` with `token`, `user`, `message`, `priority`

- [ ] **Email (SMTP)** — Send email notifications
  - Add provider `EmailProvider` with `Host, Port, Username, Password, From, To string`
  - Use Go's `net/smtp` package: `smtp.SendMail(host+":"+port, auth, from, []string{to}, msg)`
  - Support TLS via `tls.Config`

- [ ] **Apprise wrapper** — Multi-provider via CLI tool
  - Add provider `AppriseProvider` with `URLs []string`
  - Shell out: `exec.Command("apprise", "-t", title, "-b", body, urls...)`
  - Apprise supports 80+ notification services in one CLI tool
  - Install check: `exec.LookPath("apprise")`

- [ ] **Notification settings API** — Configure providers via REST
  - Store config in `/etc/proxmaid/notifications.json`
  - Define `NotifyConfig` struct with provider configs
  - Add endpoints:
    - `GET /api/notifications/config` — current settings
    - `PUT /api/notifications/config` — update settings
    - `POST /api/notifications/test` — send test notification to verify config
    - `GET /api/notifications/history` — recent events (keep last 100 in memory)

---

## 7. System & User Management

### 7.1 UPS Integration (NUT)
- [ ] **UPS status via Network UPS Tools (NUT)**
  - Add to `system.Manager`: `GetUPSStatus() (*UPSStatus, error)`
  - Parse output of `upsc <upsname>@localhost` — key fields: `ups.status`, `battery.charge`, `battery.runtime`
  - Define `UPSStatus` struct: `Online bool`, `BatteryPct int`, `RuntimeSec int`, `Load float64`
  - On power loss (`ups.status = OB`): emit `EventType("power_loss")` notification
  - On low battery (`battery.charge < 20`): trigger graceful shutdown:
    `exec.Command("nmdctl", "stop")` then `exec.Command("shutdown", "-h", "now")`
  - Add endpoint: `GET /api/system/ups`
  - Mock mode: return `{Online: true, BatteryPct: 100, RuntimeSec: 3600}`

### 7.2 Syslog Viewer
- [ ] **Tail system logs via journalctl**
  - Add to `system.Manager`: `GetLogs(lines int, unit string) ([]LogEntry, error)`
  - Run: `journalctl --no-pager -n <lines> -o json` (optionally `-u <unit>` for specific service)
  - Parse JSON output into `LogEntry` struct: `Timestamp`, `Unit`, `Priority`, `Message`
  - Add endpoint: `GET /api/system/logs?lines=100&unit=proxmaid`
  - Mock mode: return sample log entries

### 7.3 Timezone Configuration
- [ ] **Get/set timezone via timedatectl**
  - Add `system.Manager.GetTimezone() (string, error)` — parse `timedatectl show -p Timezone --value`
  - Add `system.Manager.SetTimezone(tz string) error` — `timedatectl set-timezone <tz>`
  - Add endpoint: `GET /api/system/timezone`, `PUT /api/system/timezone` with body `{"timezone": "America/New_York"}`
  - Mock mode: store timezone in memory

### 7.4 Web UI Authentication
- [ ] **Session-based auth for the Proxmaid UI**
  - Add `api/internal/auth/manager.go` with JWT or session cookie approach
  - Store credentials in `/etc/proxmaid/auth.json` (bcrypt hashed)
  - Middleware: check `Authorization: Bearer <token>` header on all `/api/*` routes except `/api/health`
  - Add endpoints:
    - `POST /api/auth/login` with body `{"username": "admin", "password": "..."}` → returns JWT
    - `POST /api/auth/logout` — invalidate token
    - `GET /api/auth/me` — current user info
  - UI: add login page, store token in `localStorage`, include in API headers

### 7.5 User Management
- [ ] **System users for share access** (see Share Manager section 4 for details)
  - This is part of the Share Manager — `share.Manager.CreateUser()`, etc.
  - Web UI user management page is in Settings (section 10)

---

## 8. API Router & Infrastructure (`api/internal/api/`)

### 8.1 Completed ✅
- [x] Health, array, disk, system, cache, mover endpoints
- [x] CORS middleware, JSON envelope

### 8.2 Remaining

- [ ] **Share CRUD endpoints** — Wire `share.Manager` into router
  - Add `shareMgr *share.Manager` param to `NewRouter()`
  - Register: `GET/POST /api/shares`, `GET/PUT/DELETE /api/shares/{name}`
  - Initialize in `main.go`: `shareMgr := share.NewManager(sysMgr.MockMode)`

- [ ] **App management endpoints** — Wire `app.Manager` into router
  - Add `appMgr *app.Manager` param to `NewRouter()`
  - Register: `GET /api/apps`, `POST /api/apps/{id}/start`, etc.
  - Initialize in `main.go`: `appMgr := app.NewManager(sysMgr.MockMode)`

- [ ] **Notification config endpoints** — Wire `notify.Manager`
  - As described in section 6

- [ ] **WebSocket endpoint** — Real-time dashboard updates
  - Add `GET /api/ws` using `golang.org/x/net/websocket` or `gorilla/websocket`
  - On connect: subscribe client to event stream
  - Broadcast: array status changes, mover progress, notification events
  - UI: replace 5-second polling with WebSocket connection in `api.ts`

- [ ] **Authentication middleware** — Protect endpoints
  - As described in section 7.4
  - Apply as middleware wrapping the existing mux, skip `/api/health` and `/api/auth/*`

- [ ] **Rate limiting** — Prevent API abuse
  - Use `golang.org/x/time/rate` — token bucket per IP
  - Default: 100 requests/minute
  - Apply as middleware (outermost layer): if `!limiter.Allow()` → `429 Too Many Requests`

---

## 9. Scheduled Tasks

> Implement a generic scheduler in `api/internal/scheduler/manager.go`.

- [ ] **Task scheduler framework**
  - Use `github.com/robfig/cron/v3` for cron expression parsing
  - Define `ScheduledTask` struct: `Name`, `Schedule (cron)`, `Enabled`, `LastRun`, `NextRun`, `Action func()`
  - `Manager.AddTask(task ScheduledTask)` — registers with cron runner
  - `Manager.ListTasks() []ScheduledTask` — returns all registered tasks
  - Add endpoints: `GET /api/tasks`, `PUT /api/tasks/{name}` (update schedule/enabled)

- [ ] **Scheduled parity check** — Run `nmdctl check` on schedule
  - Default schedule: `0 0 1 * *` (1st of each month at midnight)
  - Task action: `arrayMgr.Check("CORRECT")`
  - Emit `EventParityDone` when complete

- [ ] **Parity check tuning** — Pause during peak hours
  - Write `pause` to `/proc/nmdcmd` to pause, `resume` to resume via `system.Manager.WriteNmdcmd()`
  - Add schedule config: `pause_hours: "08:00-22:00"` — pause during business hours
  - Background goroutine checks current time against pause window

- [ ] **Scheduled SMART tests** — Run short/long SMART tests
  - Task action: `exec.Command("smartctl", "-t", "short", device)` for each disk
  - Short test: weekly. Long test: monthly.

- [ ] **Recycle bin purge** — Clean old deleted files
  - Task action: `exec.Command("find", "/mnt/user", "-path", "*/.Recycle.Bin/*", "-mtime", "+30", "-delete")`
  - Default: daily purge of files older than 30 days

---

## 10. Frontend UI (`ui/`)

### 10.1 Design System — Remaining
- [ ] **Toast/notification component** — Slide-in notification for API actions
  - Create `ui/src/components/Toast.tsx`
  - Use React context/provider pattern: `<ToastProvider>` wraps app, `useToast()` hook
  - Auto-dismiss after 5 seconds, support success/error/warning variants
  - Show on: array start/stop, mover trigger, pool create/delete, errors

- [ ] **Modal/dialog component** — Confirmation dialogs, detail drawers
  - Create `ui/src/components/Modal.tsx`
  - Props: `isOpen`, `onClose`, `title`, `children`
  - Glassmorphism styling: `backdrop-filter: blur(20px)`, dark overlay
  - Use for: delete confirmations, disk detail view, array creation wizard

- [ ] **Form input components** — Consistent form controls
  - Create `ui/src/components/FormInputs.tsx`
  - Components: `TextInput`, `Select`, `Toggle`, `Slider`, `NumberInput`
  - All use CSS variables from `globals.css` for colors
  - Include label, error state, disabled state

- [ ] **Loading skeleton** — Placeholder while data loads
  - Create `ui/src/components/Skeleton.tsx`
  - Animated pulse effect on glass card backgrounds
  - Variants: `SkeletonCard`, `SkeletonTable`, `SkeletonText`

- [ ] **Responsive breakpoints** — Tablet and mobile support
  - Add to `globals.css`: media queries at `768px` and `480px`
  - Sidebar: collapsible on mobile (hamburger menu)
  - Grid layouts: stack to single column on mobile

### 10.2 Dashboard (`/`) — Remaining
- [ ] **Real-time updates** — Replace polling with WebSocket (after backend WS is done)
- [ ] **Cache usage widget** — Small card showing each pool's usage bar
- [ ] **Mover status widget** — Shows last run, next run, or progress if running
- [ ] **Recent activity feed** — Last 10 events from notification system
- [ ] **System health overview** — CPU, RAM, disk temps from `/proc/stat` + SMART cache

### 10.3 Array Manager (`/array`) — Remaining
- [ ] **Show real device names** from `rdevName` field (already in API response)
- [ ] **Show disk sizes** (`size_human`) and I/O stats in slot cards
- [ ] **Slot detail drawer** — Click slot → slide-in panel with full SMART data, history
- [ ] **Disk replacement dialog** — Select failed slot → pick replacement disk → confirm
- [ ] **Array creation wizard** — Step-by-step: select parity disk → add data disks → create
- [ ] **Resync speed graph** — Plot resync progress over time using simple SVG line chart

### 10.4 Cache Page (`/cache`) — Remaining
- [ ] **Edit mover config** — Inline editable fields (schedule, threshold, toggle)
- [ ] **Create pool dialog** — Modal with device picker, filesystem dropdown, name input
- [ ] **Delete pool confirm** — Modal with "type pool name to confirm" pattern
- [ ] **Pool device management** — Add/remove devices from existing pool

### 10.5 Disks Page (`/disks` — NEW)
- [ ] Create `ui/src/app/disks/page.tsx`
- [ ] **Disk inventory table** — Columns: device, model, serial, size, temp, health, mount, type badge
- [ ] **SMART detail drawer** — Click row → slide-in with full SMART attributes
- [ ] **Wipe/format actions** — Buttons with confirmation modal (destructive action)
- [ ] **Disk identification** — "Blink LED" button per disk
- [ ] **Filters** — Toggle: show unassigned only, filter by type (HDD/SSD/NVMe)

### 10.6 Shares Page (`/shares` — NEW)
- [ ] Create `ui/src/app/shares/page.tsx`
- [ ] **Share list** — Cards with usage bars, export badges (SMB/NFS), cache policy badge
- [ ] **Create share dialog** — Name, export type checkboxes, cache policy dropdown, allocation method
- [ ] **Share settings editor** — Inline edit ACLs, included/excluded disks, min free space
- [ ] **User management panel** — List users, add/remove, set per-share permissions
- [ ] **Connected clients** — List active SMB/NFS connections per share (`smbstatus` / `showmount`)

### 10.7 Apps Page (`/apps` — NEW)
- [ ] Create `ui/src/app/apps/page.tsx`
- [ ] **Installed apps dashboard** — Cards: app icon, name, status indicator, start/stop/restart buttons
- [ ] **App store browser** — Grid of template cards with icon, name, category badge, install button
- [ ] **Category sidebar** — Filter: Media, Networking, Productivity, Tools, etc.
- [ ] **Search** — Filter templates by name, description, repository
- [ ] **Install wizard** — Multi-step: configure ports, volumes (point to shares), env vars → create
- [ ] **App detail view** — Logs tab, stats tab (CPU/mem chart), settings tab

### 10.8 Settings Page (`/settings` — NEW)
- [ ] Create `ui/src/app/settings/page.tsx`
- [ ] **Array config** — Stripe cache size, auto-start on boot toggle
- [ ] **Network settings** — Display hostname, IP (read from system)
- [ ] **Notification providers** — Add/edit/test Discord/email/Pushover configs
- [ ] **Scheduled tasks** — Table of tasks with schedule, enabled toggle, last run
- [ ] **System info card** — Kernel version, NonRAID module version, API version, uptime
- [ ] **Backup/restore** — Download `/etc/proxmaid/*.json` as ZIP, upload to restore
- [ ] **Theme preference** — Light/dark toggle (dark is default)

---

## 11. Proxmox Storage Plugin (`pve-plugin/`)

- [ ] **Perl plugin** — `PVE::Storage::Plugin::NonRAID`
  - Subclass `PVE::Storage::Plugin`, override:
    - `type()` → `"nonraid"`
    - `properties()` → define `mount_point` property
    - `status()` → read array mount, return `(total, used, free, active)`
    - `activate_storage()` → verify array is started (call Proxmaid API)
  - See Proxmox plugin examples: `PVE::Storage::DirPlugin` is the simplest reference

- [ ] **Register with storage subsystem**
  - Install script: copy `.pm` file to `/usr/share/perl5/PVE/Storage/`
  - Register in `/etc/pve/storage.cfg`:
    ```
    nonraid: array-pool
        path /mnt/user
        content images,iso,backup
    ```

- [ ] **VM disk image support** — Allow creating VM disks on array
  - Implement `alloc_image()`, `free_image()`, `list_images()`
  - Store as raw/qcow2 files under `/mnt/user/vms/` share

- [ ] **ISO/template storage** — Point Proxmox at array for ISOs
  - Implement `list_volumes()` with `content => 'iso'`
  - Path: `/mnt/user/isos/`

- [ ] **Installation script** — Automate plugin setup
  - Bash: copy `.pm`, add storage config entry, restart `pvedaemon`

---

## 12. Infrastructure & Packaging

### 12.1 Systemd
- [ ] **`proxmaid.service`** — Main API daemon
  ```ini
  [Unit]
  Description=Proxmaid Storage Management API
  After=network.target nonraid.service
  Wants=nonraid.service

  [Service]
  ExecStart=/usr/bin/proxmaid
  Restart=on-failure
  RestartSec=5
  Environment=PROXMAID_PORT=8484

  [Install]
  WantedBy=multi-user.target
  ```
- [ ] **Watchdog** — Restart on failure with exponential backoff
  - Use `RestartSec=5` with `StartLimitIntervalSec=300`, `StartLimitBurst=5`

### 12.2 Build System
- [ ] **Top-level Makefile**
  ```makefile
  build: build-api build-ui
  build-api:
      cd api && CGO_ENABLED=0 go build -ldflags "-X main.version=$(VERSION)" -o ../dist/proxmaid ./cmd/proxmaid/
  build-ui:
      cd ui && npm run build && cp -r out ../dist/ui
  test:
      cd api && go test ./... -v
  package:
      dpkg-deb --build dist/ proxmaid_$(VERSION).deb
  ```
- [ ] **Embed UI in Go binary** — Use `embed.FS` to serve static files from the Go binary, no separate web server needed
- [ ] **Version injection** — `-ldflags "-X main.version=$(git describe --tags)"`

### 12.3 Debian Packaging
- [ ] **`.deb` package structure**
  ```
  proxmaid_0.1.0/
  ├── DEBIAN/
  │   ├── control          # package metadata
  │   ├── postinst         # enable + start service
  │   └── prerm            # stop service
  ├── usr/bin/proxmaid     # Go binary
  ├── usr/share/proxmaid/ui/  # static frontend
  └── lib/systemd/system/proxmaid.service
  ```

### 12.4 Configuration
- [ ] **`/etc/proxmaid/config.yaml`** — Central config file
  ```yaml
  port: 8484
  log_level: info
  mock_mode: false
  data_dir: /etc/proxmaid
  shares_config: /etc/proxmaid/shares.json
  notify_config: /etc/proxmaid/notifications.json
  ```
- [ ] **Config reload** — `SIGHUP` handler: re-read config without restart
  - Or API endpoint: `POST /api/system/reload`

### 12.5 CI/CD
- [ ] **GitHub Actions: Go** — `go test`, `go vet`, `golangci-lint` on push/PR
- [ ] **GitHub Actions: UI** — `npm run build`, `npm run lint` on push/PR
- [ ] **Release workflow** — On tag push: build `.deb`, create GitHub release with changelog
- [ ] **Automated `.deb` builds** — GitHub Actions builds and uploads `.deb` artifacts

---

## 13. Testing — Remaining

- [ ] **Share manager tests** — `api/internal/share/manager_test.go`
  - Test CRUD, SMB config generation, NFS export generation, user management
  - Mock: don't actually call `useradd` or write to `/etc/samba/`

- [ ] **App manager tests** — `api/internal/app/manager_test.go`
  - Mock Docker socket responses with `httptest.NewServer()`
  - Test: list containers, start/stop, template parsing

- [ ] **Integration tests** — Full API workflow
  - Create pool → create share → assign disks → start array → run mover
  - All in mock mode, verify state transitions

- [ ] **Frontend component tests** — React Testing Library
  - Test: Sidebar nav highlights, ArrayStatusCard renders states, form validation

- [ ] **E2E tests** — Playwright
  - Start API (mock mode) + UI dev server
  - Test: navigate all pages, trigger actions (start array, create pool), verify UI updates
