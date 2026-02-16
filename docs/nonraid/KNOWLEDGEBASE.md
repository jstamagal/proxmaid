# NonRAID Codebase Knowledgebase

> Comprehensive technical reference for the NonRAID project. Intended for LLM consumption and developer onboarding.

## Table of Contents

1. [Project Overview](#1-project-overview)
2. [Architecture](#2-architecture)
3. [nmdctl Command Reference](#3-nmdctl-command-reference)
4. [Array Lifecycle](#4-array-lifecycle)
5. [Kernel Module (md-nonraid)](#5-kernel-module-md-nonraid)
6. [RAID6 Parity Engine](#6-raid6-parity-engine)
7. [Data Layout and I/O Paths](#7-data-layout-and-io-paths)
8. [Status and Monitoring](#8-status-and-monitoring)
9. [Systemd Integration](#9-systemd-integration)
10. [Packaging and Installation](#10-packaging-and-installation)
11. [CI/CD Pipeline](#11-cicd-pipeline)
12. [Configuration Reference](#12-configuration-reference)
13. [Key Data Structures](#13-key-data-structures)
14. [Error Handling and Recovery](#14-error-handling-and-recovery)
15. [Internal Function Reference](#15-internal-function-reference)

---

## 1. Project Overview

**NonRAID** is a Linux storage array system implementing UnRAID-compatible disk arrays with parity protection. It consists of:

- **md-nonraid** -- A DKMS kernel module implementing the "nonraid" md personality
- **nonraid6_pq** -- An embedded RAID6 parity calculation library (auto-selects AVX-512/AVX2/SSE2/integer)
- **nmdctl** -- A ~4800-line Bash 4.0+ management CLI (`tools/nmdctl`)
- **Systemd services** -- Array lifecycle, scheduled parity checks, and health monitoring
- **Debian packaging** -- Two packages: `nonraid-dkms` (kernel module) and `nonraid-tools` (CLI + services)

**Key design principles:**
- Data and parity distributed asymmetrically (not striped like standard RAID6)
- Each data disk can have its own independent filesystem (XFS, Btrfs, ZFS, ext4, LUKS)
- Parity disk(s) must be >= largest data disk
- Supports up to 28 data disks + 2 parity disks (P + Q)
- Can survive loss of any 1 disk (single parity) or any 2 disks (dual parity)

**Supported platforms:** Ubuntu 24.04, Debian 12/13, Arch Linux, Proxmox VE 9. Kernel versions 6.1-6.8, 6.11+ (6.9-6.10 unsupported).

---

## 2. Architecture

```
User Space                          Kernel Space
-----------                         ------------
nmdctl (Bash CLI)                   md-nonraid.ko
  |                                   |
  |-- writes to /proc/nmdcmd ------->| (command interface)
  |-- reads from /proc/nmdstat <-----| (status interface)
  |                                   |
  |-- modprobe nonraid super=...      |-- nonraid6_pq.ko (RAID6 lib)
  |-- mount/umount /dev/nmdXp1       |-- block devices: /dev/nmd{1..28}p1
  |                                   |-- stripe cache (in-memory)
  |                                   |-- per-disk kernel threads
systemd services
  |-- nonraid.service (start/stop)
  |-- nonraid-notify.timer (15min health check)
  |-- nonraid-parity-check.timer (quarterly)
```

**Communication model:** All array control flows through two proc files:
- `/proc/nmdcmd` -- Write-only command interface (e.g., `echo "start" > /proc/nmdcmd`)
- `/proc/nmdstat` -- Read-only status interface (key=value format, one per line)

**Slot model:** Fixed 30-slot architecture:
- Slot 0: Primary parity disk (P)
- Slots 1-28: Data disks
- Slot 29: Secondary parity disk (Q, optional)

---

## 3. nmdctl Command Reference

**Version:** 1.22.0 | **Location:** `tools/nmdctl` | **Requires:** root, Bash 4.0+

### Global Options

| Flag | Long | Description |
|------|------|-------------|
| `-s` | `--super PATH` | Superblock file path (default: `/nonraid.dat`) |
| `-k` | `--keyfile PATH` | LUKS keyfile path (default: `/etc/nonraid/luks-keyfile`) |
| `-u` | `--unattended` | No interactive prompts; stricter error handling |
| `-v` | `--verbose` | Detailed output |
| | `--no-color` | Disable ANSI color codes |
| `-V` | `--version` | Show version |
| `-h` | `--help` | Show help |

### Commands

#### `status` -- Display array status
```
nmdctl status [-v] [--no-fs] [-o FORMAT] [-m [INTERVAL]]
```
- `-o FORMAT`: `default` (colored terminal), `json`, `prometheus`/`prom`, `terse`
- `-m [INTERVAL]`: Monitor mode with live refresh (default 2s, press `q` to quit)
- `--no-fs`: Skip filesystem info collection

#### `create` -- Create a new array
```
nmdctl create                           # Interactive wizard
nmdctl create [-f] SLOT:DEV[:ID] ...    # Direct layout
```
- Slots: `P` or `0` (parity), `Q` or `29` (second parity), `1-28` (data)
- `-f`: Skip device availability checks

#### `start` -- Start the array
```
nmdctl start [STATE]
```
- Loads module, imports disks, starts array
- Optional STATE: `NEW_ARRAY`, `RECON_DISK`, `DISABLE_DISK`, `SWAP_DSBL`
- Unattended mode cannot start with missing disks or abnormal states without explicit STATE

#### `stop` -- Stop the array
```
nmdctl stop
```
- Unmounts filesystems (interactive) or fails if mounted (unattended)
- Reloads module to clear stale state

#### `import` -- Import disks without starting
```
nmdctl import
```

#### `add` -- Add a disk to the array
```
nmdctl add                              # Interactive
nmdctl add [-f] SLOT:DEV[:ID]          # Direct
nmdctl add [-f] SLOT                   # Replace in slot
```

#### `replace` -- Replace a disk (alias for `add` with slot)
```
nmdctl replace SLOT[:DEV[:ID]]
```

#### `unassign` -- Remove a disk from a slot
```
nmdctl unassign SLOT
```
- Array must be stopped
- Enforces parity preservation: can't unassign more disks than parity count

#### `reload` -- Reload the kernel module
```
nmdctl reload
```
- Stops array, unloads module, reloads with current superblock

#### `check` -- Parity check/sync operations
```
nmdctl check [CORRECT|NOCORRECT|PAUSE|RESUME|CANCEL]
```
- Default: `CORRECT` (interactive) or `NOCORRECT` (unattended)
- `PAUSE`/`RESUME`: Suspend/continue operations
- `CANCEL`: Stop and discard progress
- Legacy: `nmdctl nocheck` maps to `check CANCEL`

#### `mount` / `unmount` -- Filesystem operations
```
nmdctl mount [-k KEYFILE] [MOUNTPREFIX]   # Default prefix: /mnt/disk
nmdctl unmount
```
- Auto-detects filesystem type (XFS, ext4, Btrfs, ZFS, LUKS)
- Creates mount points: `/mnt/disk1`, `/mnt/disk2`, etc.
- Reads `/etc/nonraid/fstab` for custom mount options

#### `set` -- Configure array settings
```
nmdctl set [SETTING] [VALUE]
```

| Setting | Alias | Range | Default | Description |
|---------|-------|-------|---------|-------------|
| `md_write_method` | `turbo` | 0-1 | 0 | 0=RMW (standard), 1=Reconstruct (turbo, needs all disks) |
| `md_trace` | `debug` | 0-4 | 1 | Debug trace level |
| `md_queue_limit` | | 1-100 | 80 | Normal I/O queue limit % |
| `md_sync_limit` | | 0-100 | 5 | Sync/parity operation queue limit % |
| `md_num_stripes` | | int | 1280 | Stripe cache entries |
| `label` | | string(32) | "" | Array label (stopped only) |
| `invalidslot` | | "S1 S2" | | Mark slots as invalid |
| `resync_start` | | sectors | 0 | Parity sync start position |
| `resync_end` | | sectors | 0 | Parity sync end position (0=auto) |
| `rderror` | | SLOT | | Simulate read error (testing) |
| `wrerror` | | SLOT | | Simulate write error (testing) |

### Command Dispatch

Main dispatch at `main()` (line ~4694):
1. Parse global options
2. Extract command name
3. Route via case statement to handler functions:

```
status    -> show_status()
create    -> create_array()
start     -> start_array()
stop      -> stop_array()
import    -> import_disks()
add       -> add_disk()
replace   -> add_disk() (with validation)
unassign  -> unassign_disk()
reload    -> reload_module()
check     -> handle_check()
nocheck   -> handle_check("CANCEL")
mount     -> mount_array_disks()
unmount   -> unmount_array_disks()
set       -> set_array_setting()
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success / HEALTHY |
| 1 | Warning / degraded / validation error |
| 2 | Error / offline / critical |

---

## 4. Array Lifecycle

### State Machine

```
STOPPED ──create──> NEW_ARRAY
   │                    │
   │    <──start────────┘ (builds parity)
   │
   ├──start──> STARTED (normal operation)
   │              │
   │    <──stop───┘
   │
   ├──start──> RECON_DISK (disk reconstruction needed)
   ├──start──> DISABLE_DISK (degraded, missing disks)
   ├──start──> SWAP_DSBL (parity swap in progress)
   └──────────> ERROR:* (critical failure)
```

### Typical Workflow

```bash
# 1. Create array
nmdctl create P:/dev/sda1 1:/dev/sdb1 2:/dev/sdc1

# 2. Start (triggers parity build for NEW_ARRAY)
nmdctl start

# 3. Mount filesystems
nmdctl mount

# 4. Monitor
nmdctl status -m

# 5. Periodic parity check
nmdctl check NOCORRECT

# 6. Stop for maintenance
nmdctl unmount
nmdctl stop
```

### Disk Import Process (`import_disks()`, line 2199)

For each slot defined in superblock:
1. Check if already imported (skip if so)
2. Check if intentionally unassigned (skip if DISK_NP_MISSING/DISK_NP_DSBL)
3. Look up disk by ID in `/dev/disk/by-id/`
4. Find largest unmounted partition on the device
5. Validate exclusive access (Python `os.O_WRONLY|os.O_EXCL`)
6. Validate size against superblock configuration
7. Send import command: `import SLOT PARTITION OFFSET SIZE ERASED ID`

### Disk Replacement Scenarios

| Scenario | Condition | Action |
|----------|-----------|--------|
| Replace missing data disk | Slot has size but no ID | Import new disk, kernel rebuilds from parity |
| Replace missing parity disk | Parity slot has size but no ID | Import new disk, kernel rebuilds parity |
| Parity swap | Parity slot already assigned + unassigned data slot exists | Manual copy required before start |
| Add new data disk | Empty slot | Import disk, clear operation runs |

### Unattended Mode Restrictions

- Cannot start array with missing disks
- Cannot start in abnormal states without explicit STATE parameter
- Cannot stop with mounted filesystems
- Parity check defaults to NOCORRECT
- Paused operations block new operations (error instead of prompt)
- Insufficient parity for unassign returns error

---

## 5. Kernel Module (md-nonraid)

### Overview

The `md-nonraid` kernel module (`md_nonraid/`) implements a Linux md personality for UnRAID-compatible arrays. It is built via DKMS and automatically selects the correct source for the running kernel:

| Kernel | Source Dir | Based On |
|--------|-----------|----------|
| >= 6.9 | `6.12/` | UnRAID 7.1.2 (6.12.24) |
| 6.5-6.8 | `6.6/` | UnRAID 7.0.1 (6.6.78) |
| < 6.5 | `6.1/` | UnRAID 6.12.15 (6.1.126) |

### Source Files

- `md_unraid.c` -- md subsystem integration: superblock handling, array state management, thread management, proc/sysfs interface
- `md_unraid.h` -- Data structures and constants
- `unraid.c` -- Core stripe engine: stripe cache, I/O processing, parity calculation, sync operations

### Superblock Format (4096 bytes)

```c
typedef struct mdp_superblock_s {
    __u32 md_magic;           // 0xb92b4efc
    __u32 major_version;      // 2
    __u32 minor_version;      // 9
    __u32 patch_version;      // 35
    __u32 sb_csum;            // Checksum
    __u32 ctime, utime;       // Create/update time
    __u32 events;             // Update counter
    __u32 state;              // Array state flags
    __u32 num_disks;          // Active disk slots
    __u32 stime, stime2;      // Last sync start/end
    __u32 sync_errs;          // Sync errors
    __u32 sync_exit;          // Sync exit code
    __u8  label[32];          // Array label
    mdp_disk_t disks[30];     // Disk descriptors (128 bytes each)
} mdp_super_t;
```

### Virtual Block Devices

When started, the driver creates `/dev/nmd{1..28}p1` devices (major 127). Parity disks (P/Q) are NOT exposed as block devices -- only data disks are user-accessible.

### Kernel Threads

The module spawns N+1 threads:
- `nmdrecoveryd` -- Parity check/reconstruction, sync operations
- `nonraidd{1..N}` -- Per-data-disk I/O handling

Each thread processes stripes from its `handle_list[]` work queue.

### Module Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `super` | (required) | Superblock file path |
| `md_num_stripes` | 1280 | Stripe cache entries |
| `md_queue_limit` | 80 | Normal I/O queue % |
| `md_sync_limit` | 5 | Sync queue % |
| `md_write_method` | 0 | 0=RMW, 1=Reconstruct |
| `md_restrict` | 1 | bit0: sector limit, bit1: fail read-ahead |

### Proc/Sysfs Interface

**`/proc/nmdcmd`** (write-only): Commands sent as strings
```
import SLOT DEVICE OFFSET SIZE ERASED ID
start [STATE]
stop
check CORRECT|NOCORRECT|RESUME
nocheck PAUSE|CANCEL
set PARAM VALUE
```

**`/proc/nmdstat`** (read-only): Key=value status output

**Array-level keys:**
`mdState`, `mdResync`, `mdResyncAction`, `mdResyncCorr`, `mdResyncPos`, `mdResyncSize`, `mdResyncDt`, `mdResyncDb`, `mdNumDisks`, `mdNumMissing`, `mdNumInvalid`, `mdNumWrong`, `mdNumDisabled`, `mdNumReplaced`, `mdNumNew`, `mdHealth`, `mdLastSync`

**Superblock keys:**
`sbName`, `sbVersion`, `sbLabel`, `sbSynced`, `sbSynced2`, `sbSyncErrs`, `sbSyncExit`

**Per-slot keys (0-29):**
`diskId.N`, `diskSize.N`, `diskState.N`, `diskName.N`, `rdevName.N`, `rdevStatus.N`, `rdevSize.N`, `rdevId.N`, `rdevReads.N`, `rdevWrites.N`, `rdevNumErrors.N`

**rdevStatus values:** `DISK_OK`, `DISK_INVALID`, `DISK_NP_MISSING`, `DISK_NP_DSBL`, `DISK_WRONG`, `DISK_DSBL`, `DISK_NEW`, `DISK_DSBL_NEW`

---

## 6. RAID6 Parity Engine

### How Parity Works

**P Parity (XOR):**
```
P = D1 XOR D2 XOR D3 ... XOR Dn
```
Recovers any single failed disk.

**Q Parity (Reed-Solomon):**
```
Q = (D1 * g^0) XOR (D2 * g^1) XOR ... (Dn * g^(n-1))
```
Where `g = 0x02` in GF(2^8). Combined with P, recovers any two failed disks.

### Embedded Library (`raid6/`)

```
raid6/
  algos.c          -- Algorithm selection and benchmark
  recov.c          -- Recovery algorithms
  tables.c         -- Galois field lookup tables
  int.uc           -- Pure C implementation
  sse2.c           -- SSE2 SIMD
  avx2.c           -- AVX2 SIMD
  avx512.c         -- AVX-512 SIMD
  nonraid_pq.h     -- Symbol namespace mapping
  nonraid_raid6.h  -- API header
```

Auto-selects fastest algorithm at module load: AVX-512 > AVX2 > SSE2 > C integer.

### Key Functions

| Function | Purpose |
|----------|---------|
| `raid6_gen_syndrome()` | Compute P and Q from all data blocks |
| `raid6_xor_syndrome()` | Incremental P/Q update (for RMW writes) |
| `raid6_2data_recov()` | Recover 2 failed data disks from P+Q |
| `raid6_datap_recov()` | Recover data disk when P is missing (using Q) |
| `raid5_generate_d()` | Single-disk recovery via XOR |

### Two Modules Built

1. `nonraid6_pq.ko` -- RAID6 parity library
2. `md-nonraid.ko` -- Main personality module (depends on nonraid6_pq)

---

## 7. Data Layout and I/O Paths

### Stripe Definition

- Stripe size = PAGE_SIZE (4096 bytes = 8 x 512-byte sectors)
- Stripe number = sector_address / 8
- Each stripe contains one block from each disk in the array

### Column Layout (in stripe cache)

```
col[0]    = Data disk 1
col[1]    = Data disk 2
...
col[N-1]  = Data disk N
col[N]    = P parity (always slot 0)
col[N+1]  = Q parity (always slot 29)
```

### Disk Size Rules

- Parity disk(s) must be >= largest data disk
- Data disks can be different sizes
- Virtual block device size = physical disk size

### Read Path

1. **Cache hit:** Return data from stripe cache
2. **Cache miss, all disks OK:** Read from target data disk
3. **1 disk failed:** Read all valid disks, reconstruct via XOR or Q recovery
4. **2 disks failed:** Read all valid disks, RAID6 2-data recovery
5. **>2 failed:** Return I/O error

### Write Path

**RMW (Read-Modify-Write) -- default, `md_write_method=0`:**
1. Read old data from target disk + old P + old Q
2. Compute: `P_new = P_old XOR D_old XOR D_new`
3. Compute: `Q_new` via incremental syndrome update
4. Write D_new, P_new, Q_new
5. Only needs target disk + parity disks spinning

**Reconstruct Write -- `md_write_method=1` (turbo):**
1. Read all other data disks
2. Compute P = XOR of all data, Q = syndrome of all data
3. Write all modified columns
4. Needs ALL disks spinning

### Flush/Barrier Handling

For writes with `REQ_PREFLUSH`: flush target + P + Q disks before write. For `REQ_FUA`: flush again after write.

---

## 8. Status and Monitoring

### Health Status Levels

| Status | Code | Condition |
|--------|------|-----------|
| HEALTHY | 0 | All disks present, no errors |
| WARNING | 1 | I/O errors but all disks present |
| DEGRADED | 1 | Missing/invalid/wrong/disabled disks or sync errors |
| PARTIAL | 1 | Not all disks imported (array stopped) |
| READY | 1 | All disks imported but array stopped |
| NEW | 1 | New array, parity not yet built |
| NEW_DISK | 1 | New disk added, needs clearing |
| OFFLINE | 2 | No disks imported |
| ERROR | 2 | Array in error state |

### Output Formats

**Human (default):** Colored terminal output with tables, progress bars, disk inventory.

**JSON:** Machine-parseable with fields: `timestamp`, `code`, `state`, `status`, `label`, `superblock`, `array`, `disks[]`, `resync`, `messages[]`.

**Prometheus:** Metrics including:
```
nonraid_array_state{label="..."}
nonraid_array_health{label="..."}
nonraid_disks_present{label="..."}
nonraid_resync_active{label="..."}
nonraid_resync_progress_percent{label="..."}
nonraid_resync_rate_mb_per_sec{label="..."}
nonraid_disk_errors_total{label="..."}
nonraid_last_sync_timestamp{label="..."}
```

**Terse:** Single line: `NonRAID Array State: STARTED, Health: HEALTHY`

### Monitor Mode

`nmdctl status -m [INTERVAL]` -- TUI with real-time refresh:
- Uses alternative screen buffer
- Keyboard: `q`=quit, `r`=refresh, `?`=help
- Calculates I/O rates from counter deltas between refreshes
- Caches filesystem info to reduce stat() overhead

### Resync Progress Tracking

Variables from `/proc/nmdstat`:
- `mdResync`: 0/1 (running flag)
- `mdResyncAction`: Operation type (check, recon, clear)
- `mdResyncCorr`: 0/1 (correcting mode)
- `mdResyncPos`: Current position (blocks)
- `mdResyncSize`: Total size (blocks)
- `mdResyncDt`/`mdResyncDb`: Rate calculation (time delta / blocks delta)

Computed: progress %, rate (KB/s), ETA, elapsed time.

---

## 9. Systemd Integration

### nonraid.service -- Main Array Lifecycle

```ini
Type=oneshot, RemainAfterExit=yes
After=local-fs.target
```

**Startup sequence:**
1. `nmdctl -u -v -s $SUPER start`
2. Detect unclean shutdown via `/var/lib/nonraid/array.running` marker
   - Marker exists -> auto-run corrective parity check
   - Marker missing -> create marker
3. Apply `$STARTUP_SETTINGS` (e.g., `md_write_method 1`)
4. Auto-mount if `$AUTOMOUNT=yes`

**Shutdown sequence:**
1. Auto-unmount if `$AUTOMOUNT=yes`
2. `nmdctl -u stop`
3. Remove `array.running` marker

### nonraid-notify.timer -- Health Monitoring

- Runs every 15 minutes (`OnBootSec=15m, OnUnitActiveSec=15m`)
- Checks array health via `nmdctl status`
- If degraded: sends notification via `$NONRAID_NOTIFY_CMD`
- Rate-limits notifications; sends recovery notice when healthy again

### nonraid-parity-check.timer -- Scheduled Checks

- Quarterly (`OnCalendar=quarterly`)
- `Persistent=true` (catches up missed runs)
- `RandomizedDelaySec=3600`
- Runs `nmdctl -u check nocorrect` (verify-only, safe default)

---

## 10. Packaging and Installation

### Two Debian Packages

**nonraid-dkms** -- Kernel module source
- Installs to `/usr/src/nonraid-dkms-{version}/`
- DKMS auto-rebuilds on kernel updates
- Builds: `md-nonraid.ko` + `nonraid6_pq.ko`
- Dependencies: `dkms (>= 2.8.7)`
- Build note: `CONFIG_UBSAN=n` to avoid Ubuntu false positives

**nonraid-tools** -- Management tools
- Installs: `/usr/bin/nmdctl`, systemd services/timers, udev rules, `/etc/default/nonraid`
- Dependencies: `bash (>=5.0)`
- `/etc/default/nonraid` installed with `chmod 600` (contains potential LUKS paths)

### Installation

```bash
# PPA (Ubuntu/Debian)
sudo add-apt-repository ppa:qvr/nonraid
sudo apt install linux-headers-$(uname -r) nonraid-dkms nonraid-tools

# Arch Linux
yay -Syu nonraid-git

# Manual
sudo dkms install nonraid-dkms/$VERSION -k $(uname -r)
sudo cp tools/nmdctl /usr/local/bin/
```

### udev Rules (`90-nonraid.rules`)

Minimal: only runs `blkid` builtin on `nmd*` block devices to cache filesystem info.

---

## 11. CI/CD Pipeline

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `dkms-tests.yml` | PR/push/weekly | Build kernel modules across Debian 12/13, Ubuntu 24.04, Arch |
| `dkms-integration-tests.yml` | PR/push/weekly | Functional tests with loop devices: create, sync, write, replace, rebuild, verify MD5 |
| `nmdctl-tests.yml` | PR/push | ShellCheck, `bash -n`, BATS unit tests |
| `tools-debian.yml` | Manual | Build nonraid-tools .deb, optional PPA publish |
| `dkms-debian.yml` | Manual | Build nonraid-dkms .deb, optional PPA publish |

### Integration Test Flow

Creates 4x1GB loop devices, then:
1. Create array (1P + 2D)
2. Build parity, create XFS, write 100MB data
3. Add 4th disk, clear
4. Unassign disk 2, zero it
5. Start degraded, replace disk 2
6. Verify MD5 integrity

### nmdctl Unit Tests (`tools/tests/test_nmdctl_basic.bats`)

Uses BATS with mock functions. Covers:
- Version/help output
- `format_kbytes()` conversions
- `format_time_duration()`
- `get_visible_length()` (ANSI stripping)
- Health status detection (HEALTHY, STOPPED, DEGRADED)
- Array calculations (size, parity detection)
- Resync progress parsing
- All output formats (default, JSON, Prometheus, terse)
- Array creation parameter parsing (P/Q/numeric slots)

---

## 12. Configuration Reference

### /etc/default/nonraid

```bash
SUPER=/nonraid.dat                    # Superblock path
AUTOMOUNT=yes                         # Auto-mount/unmount with service
MOUNT_PARAMS=""                       # Extra mount args (-k keyfile, prefix)
STARTUP_SETTINGS=""                   # Comma-separated: "md_write_method 1, md_trace 2"
NONRAID_NOTIFY_CMD=""                 # Notification command (empty=disabled)
NONRAID_NOTIFY_FORMAT="terse"         # terse|default|json|prometheus
```

### Key File Paths

| Path | Purpose |
|------|---------|
| `/nonraid.dat` | Default superblock (binary, managed by kernel module) |
| `/etc/nonraid/luks-keyfile` | Default LUKS keyfile |
| `/etc/nonraid/fstab` | Custom mount options per disk |
| `/proc/nmdstat` | Kernel status interface |
| `/proc/nmdcmd` | Kernel command interface |
| `/var/lib/nonraid/array.running` | Unclean shutdown marker |
| `/var/lib/nonraid/notified.state` | Notification rate-limit state |

### Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `PROC_NMDSTAT` | `/proc/nmdstat` | Override for testing |

---

## 13. Key Data Structures

### nmdctl Global Arrays (Bash)

```bash
declare -g -A NMDSTAT_VALUES       # Raw key=value from /proc/nmdstat
declare -g -A ARRAY_STATUS_DATA    # Processed health, sync info
declare -g -A DISK_STATUS_DATA     # Per-slot disk info
declare -g -A RESYNC_STATUS_DATA   # Active/paused/pending operation info
```

### Kernel: stripe_head

```c
struct stripe_head {
    struct hlist_node hash;       // Hash table entry
    struct list_head lru;         // Inactive or handle list
    sector_t sector;              // Stripe start sector
    atomic_t count;               // Reference counter
    unsigned long state;          // STRIPE_HANDLE|SYNCING|CLEARING|INSYNC
    int unit;                     // Target disk thread
    int write_method;             // RMW or RECONSTRUCT
    void *srcs[30];               // Buffer pointers for parity calc
    column_t col[0];              // Flex array of per-disk columns
};
```

### Kernel: column_t

```c
typedef struct column_s {
    unsigned long state;          // Buffer flags (UPTODATE, LOCKED, READ, WRITE)
    struct bio *read_bi;          // Pending read
    struct bio *write_bi;         // Pending write
    struct bio *written_bi;       // Write awaiting completion
    struct bio bio;               // Bio for disk I/O
    struct page *page;            // Page buffer
} column_t;
```

### Kernel: unraid_conf

```c
typedef struct unraid_conf {
    struct hlist_head *stripe_hashtbl;
    int disks;                        // num_data + 2
    mdp_disk_t *disk[30];
    mdk_rdev_t *rdev[30];
    void *p_scribble, *q_scribble;    // Temp parity buffers
    int num_stripes;
    mdk_thread_t *thread[29];
    struct list_head handle_list[29];  // Per-thread work queues
    struct list_head inactive_list;
    atomic_t active_stripes[29];
    spinlock_t device_lock;
} unraid_conf_t;
```

---

## 14. Error Handling and Recovery

### Disk Error Tracking

- Per-disk: `rdevNumErrors.N` -- cumulative I/O errors
- Array-wide: `mdNumMissing`, `mdNumInvalid`, `mdNumWrong`, `mdNumDisabled`
- Sync: `sbSyncErrs` (errors during last sync), `sbSyncExit` (exit code)

### Kernel Error Handling

- **Read error:** Mark disk invalid, reconstruct data from parity
- **Write error:** If disk was valid, mark invalid+disabled; if replacement, disable again
- **1 failure:** XOR or Q recovery, degraded operation continues
- **2 failures:** RAID6 dual recovery, degraded operation continues
- **>2 failures:** I/O errors returned to userspace

### nmdctl Error Handling Layers

1. **Input validation:** Parameter types, ranges, format
2. **Precondition checks:** Root, module loaded, nmdstat exists
3. **Operation validation:** Device exists, size matches, no conflicts
4. **Execution validation:** `/proc/nmdcmd` write succeeded
5. **Error format:** `${RED}Error: message${NC}` with remediation hints

### Unclean Shutdown Recovery

1. `nonraid.service` checks for `/var/lib/nonraid/array.running` marker
2. If marker exists at startup: previous shutdown was unclean
3. Auto-runs `nmdctl -u check correct` to fix any parity inconsistencies
4. Marker removed on clean shutdown

### Parity Preservation Rule

Cannot unassign more disks than parity count:
- Single parity (P only): max 1 unassigned disk
- Dual parity (P+Q): max 2 unassigned disks

---

## 15. Internal Function Reference

### Core Functions (nmdctl)

| Function | Line | Purpose |
|----------|------|---------|
| `main()` | ~4694 | Entry point, global option parsing, command dispatch |
| `run_nmd_command()` | 117 | Write command to `/proc/nmdcmd` |
| `check_root()` | 109 | Verify root privileges |
| `check_module_loaded()` | 127 | Load/verify nonraid module |
| `check_nmdstat_exists()` | 189 | Verify `/proc/nmdstat` available |
| `get_all_nmdstat_values()` | 214 | Read all status into NMDSTAT_VALUES |
| `get_nmdstat_value()` | 235 | Get single status value |
| `get_defined_slots()` | 425 | List slots with disks defined |

### Array Operations

| Function | Line | Purpose |
|----------|------|---------|
| `create_array()` | 2762 | Create array (dispatch to layout/interactive) |
| `create_array_layout()` | 2794 | Non-interactive creation from parameters |
| `create_array_interactive()` | 2989 | Interactive creation wizard |
| `start_array()` | 2359 | Start array (import, validate, send start) |
| `stop_array()` | 2498 | Stop array (unmount, send stop, reload) |
| `import_disks()` | 2199 | Import all configured disks |
| `add_disk()` | 3314 | Add/replace disk |
| `unassign_disk()` | 3909 | Remove disk from slot |
| `handle_check()` | 2572 | Parity check/pause/resume/cancel |
| `reload_module()` | 4071 | Unload and reload kernel module |
| `mount_array_disks()` | 4171 | Mount all data disk filesystems |
| `unmount_array_disks()` | 4349 | Unmount all data disk filesystems |
| `set_array_setting()` | 4485 | Configure array parameter |

### Status Collection

| Function | Line | Purpose |
|----------|------|---------|
| `collect_array_summary()` | 463 | Basic array ID and state |
| `collect_array_health()` | 472 | Health determination with all counters |
| `collect_array_size_and_parity()` | 629 | Capacity and parity config |
| `collect_resync_status()` | 686 | Active/paused/pending operation tracking |
| `collect_disk_status()` | 772 | Per-disk info including filesystem |

### Output Formatters

| Function | Line | Purpose |
|----------|------|---------|
| `format_human_output()` | 864 | Colored terminal display |
| `format_prometheus_output()` | 1138 | Prometheus metrics |
| `format_json_output()` | 1305 | JSON output |
| `format_terse_output()` | 1401 | Single-line summary |
| `format_resync_action()` | 1919 | Friendly operation name |

### Disk Utilities

| Function | Line | Purpose |
|----------|------|---------|
| `find_matching_disk()` | 2167 | Find disk by ID in `/dev/disk/by-id/` |
| `find_partition()` | 2141 | Find largest unmounted partition |
| `get_disk_size_kb()` | 2121 | Get partition size via blockdev |
| `get_fs_type()` | 1960 | Detect filesystem type |
| `get_mountpoint()` | 1998 | Find mount point for device |
| `get_fs_usage()` | 2071 | Get filesystem usage stats |
| `list_available_devices()` | 2725 | Enumerate available block devices |
| `validate_device_path()` | 2686 | Check device path validity |

### Formatting Helpers

| Function | Line | Purpose |
|----------|------|---------|
| `format_kbytes()` | 1657 | KB to human-readable size (auto-units) |
| `format_time_duration()` | 444 | Seconds to human time |
| `format_io_rate()` | 1632 | KB/s to human speed |
| `format_array_state()` | 268 | Color-code array state |
| `format_disk_status()` | 240 | Color-code disk status |
| `format_health_status()` | 326 | Color-code health status |
| `get_visible_length()` | 1413 | String length excluding ANSI codes |

### Monitor Mode

| Function | Line | Purpose |
|----------|------|---------|
| `monitor_status_loop()` | 1749 | Main TUI event loop |
| `redraw_monitor_status()` | 1713 | Full screen redraw |
| `setup_monitor_terminal()` | 1515 | Init alt screen buffer |
| `reset_monitor_terminal()` | 1527 | Restore normal terminal |
| `calculate_disk_io_rates()` | 1564 | Delta-based I/O rate calculation |
| `read_key_with_timeout()` | 1737 | Non-blocking keyboard input |

---

*Generated from comprehensive analysis of the nonraid codebase at `/home/admin/proxmaid-v2/nonraid/`. nmdctl version 1.22.0, nonraid-dkms version 1.3.2.*
