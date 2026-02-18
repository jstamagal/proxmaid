# Unraid Feature Parity Analysis for Proxmaid

> What NonRAID handles vs what Proxmaid must implement in userspace.
> All features use open-source tools only — no Unraid-proprietary code.

---

## Legend

| Icon | Meaning |
|------|---------|
| 🟢 | Handled by NonRAID kernel driver |
| 🔵 | Already implemented in Proxmaid |
| 🟡 | Needs Proxmaid implementation (userspace) |
| ⚪ | Not applicable / out of scope for Proxmaid |

---

## 1. Storage Array

| Feature | Status | Implementation |
|---------|--------|----------------|
| Parity-protected array (single/dual) | 🟢 | `md_nonraid` kernel module |
| Mixed drive sizes | 🟢 | Kernel driver handles mismatched disks |
| Array start/stop | 🔵 | `nmdctl start/stop` via API |
| Parity check (correct/nocorrect) | 🔵 | `nmdctl check` via API |
| Parity sync/rebuild | 🟢 | Kernel auto-rebuilds on disk replace |
| Resync progress tracking | 🔵 | Parsed from `/proc/nmdstat` |
| Disk add (expand array) | 🟡 | `nmdctl add` — needs API + UI |
| Disk replace (failed → new) | 🟡 | `nmdctl replace` — needs API + UI |
| Disk unassign (remove from array) | 🟡 | `nmdctl unassign` — needs API + UI |
| Array import (from existing superblock) | 🟡 | `nmdctl import` — needs API + UI |
| New array creation (format superblock) | 🟡 | `nmdctl new` — needs API + UI |
| XFS/BTRFS/Ext4 per data disk | 🟡 | `mkfs.*` + mount — needs API + UI |
| Disk spin-up/spin-down control | 🟡 | `hdparm -S` / `hdparm -Y` |
| Disk standby monitoring | 🟡 | Poll `hdparm -C` or `/sys/block/*/device/power` |
| Turbo write mode (all disks up for writes) | 🟢 | Kernel driver feature |

---

## 2. Cache & Tiering

| Feature | Status | Implementation |
|---------|--------|----------------|
| Cache pool CRUD | 🔵 | `cache.Manager` — done |
| Multiple cache pools | 🔵 | Supports named pools — done |
| mergerfs union mount (cache + array) | 🟡 | `mergerfs` — auto-installed, needs mount logic |
| Mover daemon (scheduled) | 🔵 | Cron-style scheduler — done |
| Mover: manual trigger | 🔵 | API endpoint — done |
| Mover: age-based threshold | 🔵 | Config — done |
| Mover: per-share cache preference | 🟡 | Share → cache mapping (Yes/No/Only/Prefer) |
| Mover Tuning (usage threshold) | 🟡 | Trigger mover when cache usage > X% |
| Pool BTRFS RAID1/RAID5 | 🟡 | `mkfs.btrfs -m raid1` for multi-device pools |
| Real file moving (rsync + verify) | 🟡 | `rsync --checksum` + cleanup |

---

## 3. Shares (User Shares)

| Feature | Status | Implementation |
|---------|--------|----------------|
| Create/delete named shares | 🟡 | mkdir + manage via API |
| SMB export (Samba) | 🟡 | Generate `smb.conf` includes per share |
| NFS export | 🟡 | Generate `/etc/exports` entries per share |
| Share security (Public/Private/Secure) | 🟡 | Samba `valid users` / `guest ok` directives |
| Per-share user ACLs (read/write) | 🟡 | Samba `read list` / `write list` |
| mergerfs aggregate view | 🟡 | Union mount: array disks → `/mnt/user/<share>` |
| Allocation method (Most-Free, Fill-Up, High-Water) | 🟡 | mergerfs policy: `mfs`, `lfs`, `ff` |
| Split level (keep related dirs on same disk) | 🟡 | mergerfs `minfreespace` + custom policy |
| Included/Excluded disks per share | 🟡 | mergerfs branches configuration |
| Minimum free space per share | 🟡 | mergerfs `minfreespace` attribute |
| Cache usage per share (Yes/No/Only/Prefer) | 🟡 | Mover + mergerfs branch ordering |
| Recycle bin (per share) | 🟡 | Samba `vfs_recycle` module + purge cron |
| Disk shares (expose raw disks) | 🟡 | SMB share pointing at `/mnt/disk<N>` |

---

## 4. Docker / Applications

| Feature | Status | Implementation |
|---------|--------|----------------|
| Docker container management | 🟡 | Docker Engine API (`/var/run/docker.sock`) |
| Container start/stop/restart | 🟡 | Docker API |
| Container logs viewer | 🟡 | Docker API `logs` endpoint |
| Container resource usage | 🟡 | Docker API `stats` endpoint |
| App template browser (like CA plugin) | 🟡 | Parse Unraid CA XML templates → JSON catalog |
| Install from template (ports, volumes, env) | 🟡 | `docker create` with template params |
| App update detection | 🟡 | Compare running image digest vs registry |
| Network modes (bridge, host, macvlan) | 🟡 | Docker network create/attach |
| App icon and category display | 🟡 | Fetch from template `Icon` and `Category` fields |
| Custom Docker Compose support | 🟡 | `docker compose up -d` wrapper |

> **License note**: Unraid's Community Applications (CA) plugin is GPL-licensed. The XML
> template format is community-maintained public data. We parse the XML templates (public
> data), not reuse CA's Slackware plugin code.

---

## 5. Virtual Machines

| Feature | Status | Implementation |
|---------|--------|----------------|
| VM management | ⚪ | **Proxmox handles this** — no need to duplicate |
| GPU passthrough | ⚪ | Proxmox's native feature |
| VM snapshots | ⚪ | Proxmox's native feature |
| ISO storage on array | 🟡 | Share `/mnt/user/isos` via SMB to Proxmox |

> **Key insight**: Proxmox already provides KVM/QEMU management, so Proxmaid does NOT
> need to reimplement VM features. This is a major advantage over Unraid.

---

## 6. Disk Health & Monitoring

| Feature | Status | Implementation |
|---------|--------|----------------|
| SMART data readout | 🔵 | `smartctl` via `disk.Manager` — done |
| SMART health summary | 🔵 | `smart_overall` field — done |
| Periodic SMART polling | 🟡 | Background goroutine, 30-min interval |
| Temperature monitoring | 🟡 | Parse SMART temp attributes |
| Disk failure prediction | 🟡 | Check critical SMART attrs (Reallocated, Pending, Uncorrectable) |
| SMART notifications (disk warning/failure) | 🟡 | Notify subsystem on threshold breach |
| Disk identification (LED blink) | 🟡 | `ledctl` if SES enclosure, or `hdparm` locate |

---

## 7. Scheduled Tasks

| Feature | Status | Implementation |
|---------|--------|----------------|
| Scheduled parity check | 🟡 | Cron-style scheduler → `nmdctl check` |
| Incremental parity check | 🟡 | Resume from `mdResyncPos` with `nmdctl check` |
| Parity check tuning (pause during hours) | 🟡 | Pause/resume syscall on `/proc/nmdstat` |
| Scheduled mover runs | 🔵 | Mover config with cron schedule — done |
| Scheduled SMART tests | 🟡 | `smartctl -t short/long` on schedule |
| Recycle bin auto-purge | 🟡 | Cron to `find .Recycle.Bin -mtime +X -delete` |

---

## 8. Notifications

| Feature | Status | Implementation |
|---------|--------|----------------|
| Notification framework | 🟡 | Event system → provider dispatch |
| Email (SMTP) | 🟡 | Go `net/smtp` or `gomail` |
| Discord webhook | 🟡 | HTTP POST to webhook URL |
| Pushover | 🟡 | HTTP POST to Pushover API |
| Telegram | 🟡 | HTTP POST to Telegram Bot API |
| Apprise wrapper (multi-provider) | 🟡 | Shell out to `apprise` CLI |
| Event types: disk fail, parity done, mover done, SMART warning, array stopped | 🟡 | Internal event emitter |

---

## 9. User Management

| Feature | Status | Implementation |
|---------|--------|----------------|
| Create/delete users | 🟡 | `useradd`/`userdel` + `smbpasswd` |
| Per-share permissions | 🟡 | Samba `valid users`, `write list` |
| Web UI authentication | 🟡 | Session/token auth for Proxmaid UI |
| Root SSH access | ⚪ | Proxmox already manages this |

---

## 10. System & UI

| Feature | Status | Implementation |
|---------|--------|----------------|
| Web UI (dashboard, storage) | 🔵 | Next.js — Dashboard, Array, Cache pages |
| Syslog viewer | 🟡 | Tail `journalctl` via API |
| UPS integration (NUT) | 🟡 | Read NUT status → graceful shutdown |
| NTP / timezone config | 🟡 | `timedatectl` wrapper |
| Registration/license | ⚪ | Proxmaid is open-source, no license needed |
| Flash drive boot | ⚪ | Proxmox boots from disk, not flash |
| Plugin system (.plg) | ⚪ | Slackware-specific, we use Docker instead |

---

## Summary: What's Left

| Category | Done | Remaining |
|----------|------|-----------|
| Storage Array | 6 | 9 |
| Cache & Tiering | 5 | 5 |
| Shares | 0 | 13 |
| Docker/Apps | 0 | 10 |
| VMs | 0 | 1 (just ISO share) |
| Monitoring | 2 | 5 |
| Scheduled Tasks | 1 | 5 |
| Notifications | 0 | 7 |
| User Management | 0 | 3 |
| System/UI | 1 | 3 |
| **Total** | **15** | **61** |

### Key open-source tools needed
- **mergerfs** — union filesystem for share aggregation (already auto-installed)
- **Samba** — SMB file sharing
- **NFS utils** — NFS exports
- **Docker Engine** — container runtime (likely already installed)
- **smartmontools** — disk health (already used)
- **hdparm** — disk standby control
- **Apprise** — multi-provider notifications
- **NUT** — UPS monitoring
