# Proxmaid v2 — Architecture

## Overview

Proxmaid is a storage management sidecar for Proxmox VE. It replaces running Unraid as a VM by using the open-source **NonRAID** kernel driver (`md_nonraid`) natively on the Proxmox host.

## System Diagram

```
┌──────────────────────────────────────────────────────┐
│                   Proxmox VE Host                    │
│                                                      │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐  │
│  │  Proxmaid UI │ │ Proxmaid API │ │  md_nonraid  │  │
│  │  (Next.js)   │ │  (Go :8484)  │ │  kernel mod  │  │
│  │  :3000 dev   │ │              │ │              │  │
│  └──────┬───────┘ └──────┬───────┘ └──────┬───────┘  │
│         │                │                │          │
│         └───── REST ─────┤     proc/nmdstat          │
│                          │     proc/nmdcmd           │
│                          │───── nmdctl CLI ──────────┘
│                          │
│              ┌───────────┴───────────┐
│              │  /mnt/disk1..N        │
│              │  (NonRAID Array)      │
│              │  XFS/BTRFS per disk   │
│              └───────────────────────┘
│                                                      │
│  ┌──────────┐  ┌──────────┐  ┌──────────────────┐    │
│  │ Arch VM  │  │ Win VM   │  │ LXC / Docker     │    │
│  │ (Desktop)│  │          │  │ (Applications)   │    │
│  └──────────┘  └──────────┘  └──────────────────┘    │
└──────────────────────────────────────────────────────┘
```

## Boot Sequence

1. **Host boots** → systemd loads `md_nonraid.ko` via DKMS → array auto-starts
2. **Proxmaid API** starts via `proxmaid.service` → monitors array health
3. **Proxmox** sees storage pool (via PVE plugin) → starts VMs normally
4. **No dependency on any VM for storage**

## Component Architecture

### 1. Go API Backend (`api/`)

The backend is a single Go binary serving a REST API on port 8484.

```
api/
├── cmd/proxmaid/main.go      # Entry point, wires all managers
├── internal/
│   ├── api/router.go          # HTTP router, all endpoints
│   ├── array/manager.go       # Array state machine, nmdstat parser
│   ├── cache/manager.go       # Cache pools, mergerfs, mover daemon
│   ├── disk/manager.go        # Disk discovery (lsblk), SMART health
│   └── system/manager.go      # Kernel module, proc interface, nmdctl
```

**Manager Pattern**: Each subsystem has a `Manager` struct with a mutex for thread safety. Managers are initialized in `main.go` and injected into the router via dependency injection.

**Mock Mode**: If `/proc/nmdstat` doesn't exist (dev machine), `system.Manager` sets `MockMode = true`. All managers check this flag and return realistic simulated data instead of making real system calls.

### 2. Next.js Frontend (`ui/`)

```
ui/src/
├── app/
│   ├── page.tsx               # Dashboard (/, array + disk overview)
│   ├── array/page.tsx         # Array Manager (slot grid, drag-drop)
│   └── cache/page.tsx         # Cache & Tiering (pools, mover)
├── components/
│   ├── ArrayStatusCard.tsx    # Array status with start/stop/check
│   ├── DiskListCard.tsx       # Disk inventory grid
│   ├── Sidebar.tsx            # Navigation sidebar
│   └── StatsBar.tsx           # Top stats strip
└── lib/
    └── api.ts                 # Typed API client (fetch wrapper)
```

**Design System**: Glassmorphism dark theme with CSS variables. No component library — all custom.

**API Client**: `api.ts` exports a typed `api` object with namespaced methods matching the backend endpoints. Uses `fetch` with JSON envelope parsing.

### 3. NonRAID Kernel Driver (`nonraid/`)

The `md_nonraid` kernel module is a Linux MD personality that provides Unraid-style parity-protected storage:

- **30-slot array**: Slot 0 = P parity, Slot 29 = Q parity, Slots 1–28 = data
- **Mixed drive sizes**: Each data disk can be a different size
- **Proc interface**: `/proc/nmdstat` exposes all state as `key=value` pairs
- **Command interface**: `/proc/nmdcmd` accepts write commands
- **CLI wrapper**: `nmdctl` is a ~4800-line Bash script wrapping the proc interface

### 4. Proxmox Storage Plugin (`pve-plugin/`)

A minimal Perl module (`PVE::Storage::Plugin::NonRAID`) that tells Proxmox to treat the NonRAID array as a native storage pool. **Not yet implemented.**

## Data Flow

### Array Status Query
```
UI → GET /api/array/status → ArrayManager.Status()
                            → SystemManager.ReadNmdstat()
                            → reads /proc/nmdstat (or mock data)
                            → parseNmdstat() → ArrayStatus JSON
```

### Array Start
```
UI → POST /api/array/start → ArrayManager.Start()
                            → SystemManager.RunNmdctl("start")
                            → exec: nmdctl start (or mock)
                            → state: STARTING → STARTED
```

### Cache Mover Run
```
UI → POST /api/cache/mover/run → CacheManager.RunMover()
                                → goroutine: executeMoverRun()
                                → walk cache mount, find cold files
                                → rsync to array, verify, delete
```

## State Machine: Array

```
STOPPED → STARTING → STARTED → STOPPING → STOPPED
                  ↓                ↑
               ERROR          DEGRADED (numInvalid > 0)
```

## Configuration

| Setting | Source | Default |
|---------|--------|---------|
| API Port | `PROXMAID_PORT` env | `8484` |
| Mock Mode | Auto-detect `/proc/nmdstat` | `false` |
| API URL (UI) | `NEXT_PUBLIC_API_URL` env | `http://localhost:8484` |
| Mover Schedule | API config | `40 3 * * *` (3:40 AM daily) |
| Mover Age Threshold | API config | `1d` |

## Key Dependencies

| Dependency | Purpose | Required |
|------------|---------|----------|
| `md_nonraid.ko` | Kernel driver | Yes (prod) |
| `nmdctl` | CLI for array ops | Yes (prod) |
| `mergerfs` | Union filesystem for cache/shares | Yes (auto-installed) |
| `smartmontools` | Disk health monitoring | Yes |
| `lsblk` | Disk discovery | Yes (part of util-linux) |
| `hdparm` | Disk standby control | Future |
| `samba` | SMB file sharing | Future |
| `nfs-utils` | NFS exports | Future |
