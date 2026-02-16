# Proxmaid v2 — Product Requirements Document

## Vision
**Proxmaid** is a storage management layer for **Proxmox VE** that replaces the need to run an Unraid VM inside Proxmox. It uses the open-source **NonRAID** kernel driver (`md_nonraid`) to provide Unraid-style parity-protected storage natively on the Proxmox host.

## Problem Statement
Running Unraid as a VM inside Proxmox to provide flexible, mixed-drive-size storage is:
- **Fragile**: Unraid VM must boot before any other VM that depends on its storage.
- **Expensive**: Requires USB key passthrough (Unraid license tied to USB GUID).
- **Redundant**: Two full OS stacks (Proxmox + Unraid) managing overlapping concerns.

## Solution: The Sidecar Architecture
Proxmaid runs **alongside** Proxmox as a native service, not inside a VM.

```
┌─────────────────────────────────────────────────┐
│                  Proxmox VE Host                │
│                                                 │
│  ┌───────────┐  ┌───────────┐  ┌─────────────┐ │
│  │ Proxmaid  │  │ Proxmox   │  │ md_nonraid  │ │
│  │ API+UI    │  │ mgmt API  │  │ kernel mod  │ │
│  │ :8484     │  │ :8006     │  │             │ │
│  └─────┬─────┘  └─────┬─────┘  └──────┬──────┘ │
│        │              │               │         │
│        └──────────────┼───────────────┘         │
│                       │                         │
│              ┌────────┴────────┐                │
│              │  /mnt/disk1..N  │                │
│              │  (NonRAID Array)│                │
│              └─────────────────┘                │
│                                                 │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐      │
│  │ Arch VM  │  │ Win VM   │  │ LXC/Docker│     │
│  │ (Desktop)│  │ (Testing)│  │ (Apps)    │     │
│  └──────────┘  └──────────┘  └──────────┘      │
└─────────────────────────────────────────────────┘
```

### Boot Sequence
1. **Host boots** → systemd loads `md_nonraid.ko` → array auto-starts.
2. **Proxmaid API** starts → monitors array health.
3. **Proxmox** sees storage pool → starts VMs normally.
4. **No dependency on any VM for storage.**

## Components

### 1. Proxmaid Core API (Go)
- **Location**: `api/`
- **Port**: 8484 (configurable via `PROXMAID_PORT`)
- **Responsibilities**:
  - Array lifecycle (start/stop/check)
  - Disk management (wipe/format/mount)
  - System management (kernel module load/unload)
  - Real-time status via REST + WebSocket (future)
- **Mock Mode**: Auto-enabled when `/proc/nmdstat` absent, for development without hardware.

### 2. Proxmaid UI (Next.js)
- **Location**: `ui/`
- **Goal**: A modern, beautiful web interface superior to Unraid's.
- **Features**:
  - Dashboard with real-time array stats
  - Disk management (add/remove/replace)
  - Parity check controls
  - App Store (Docker template compatibility — future)

### 3. Proxmox Storage Plugin (Perl)
- **Location**: `pve-plugin/`
- **Goal**: Minimal plugin so Proxmox treats the NonRAID array as a native storage pool.
- **Scope**: Report capacity/usage, allow VM disk image creation on the array.

### 4. NonRAID Driver (Existing)
- **Location**: `nonraid/`
- **Source**: [github.com/qvr/nonraid](https://github.com/qvr/nonraid)
- **Role**: The kernel driver that makes it all work. We consume it, not modify it.

## Target User
- Home lab enthusiasts running Proxmox who want Unraid-style storage.
- Users with mixed drive sizes who need parity protection.
- Anyone currently running "Unraid inside Proxmox" and frustrated by the fragility.

## Tech Stack
| Component      | Technology                    |
|----------------|-------------------------------|
| API Backend    | Go 1.24+                      |
| Frontend       | Next.js + Tailwind CSS        |
| PVE Plugin     | Perl (PVE::Storage::Plugin)   |
| Kernel Driver  | C (md_nonraid, DKMS)          |
| Testing        | Go `testing`, BATS (nmdctl)   |

## Development Environment
- **Dev Machine**: Any Linux box (mock mode auto-activates).
- **Test VM**: Nested Proxmox VE inside Proxmox with 4-5 virtual disks.
- **CI**: `go test ./...` for API, browser tests for UI.

## Non-Goals (v2.0)
- Running Unraid `.plg` system plugins (Slackware-dependent).
- Replacing Proxmox's VM/LXC management.
- Supporting non-Proxmox hypervisors (initially).

## Future Roadmap
- [ ] App Store: Parse Unraid Community Applications XML templates → deploy as Docker/LXC.
- [ ] SMART monitoring and disk health alerts.
- [ ] WebSocket real-time dashboard updates.
- [ ] Notification system (Discord/Pushover via Apprise).
- [ ] Scheduled parity checks via built-in scheduler.
