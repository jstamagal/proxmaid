# NonRAID Infrastructure and Packaging Research

This document provides a comprehensive analysis of the NonRAID system's infrastructure, packaging, deployment, and operational architecture.

## Table of Contents

1. [Project Overview](#project-overview)
2. [Packaging Architecture](#packaging-architecture)
3. [Build System](#build-system)
4. [Systemd Services and Timers](#systemd-services-and-timers)
5. [udev Rules](#udev-rules)
6. [CI/CD Pipeline](#cicd-pipeline)
7. [Installation Methods](#installation-methods)
8. [Configuration](#configuration)
9. [Kernel Driver Interface](#kernel-driver-interface)
10. [Manual Array Management](#manual-array-management)
11. [Development Workflow](#development-workflow)

---

## Project Overview

### What is NonRAID?

NonRAID is a fork of the unRAID system's open-source `md_unraid` kernel driver, adapted to work as a standalone DKMS module on Ubuntu, Debian, Arch Linux, and other Linux distributions. It enables UnRAID-style storage arrays with parity protection outside of the commercial UnRAID system.

**Key characteristics:**
- **Storage model:** Multiple block devices (drives) form a storage array with 1-2 parity disks and multiple data disks
- **Filesystem flexibility:** Each data disk can have independent filesystems (XFS, BTRFS, ZFS, etc.)
- **Parity protection:** Reconstructs any single (or dual, with two parity drives) disk failure
- **Mixed sizes:** Data drives can be different sizes; parity drive must be >= largest data drive
- **Architecture:** Separate kernel module (`md_nonraid`) rather than replacing the standard `md` driver

### Target Platforms

Officially supported and tested:
- **Ubuntu 24.04 LTS** (GA kernel 6.8.0, HWE kernels 6.11+)
- **Debian 12** (kernel 6.1)
- **Debian 13** (kernel 6.12)
- **Arch Linux** (LTS and stable kernels)
- **Proxmox VE 9** (kernel 6.14)

### Kernel Support Matrix

| Kernel Range | Branch | Base UnRAID Version | Tested Distros |
|---|---|---|---|
| 6.1 - 6.4 | nonraid-6.1 | unRAID 6.12.15 (6.1.126) | Debian 12 |
| 6.5 - 6.8 | nonraid-6.6 | unRAID 7.0.1 (6.6.78) | Ubuntu 24.04 GA |
| 6.11 - 6.17 | nonraid-6.12 | unRAID 7.1.2 (6.12.24) | Ubuntu 24.04 HWE, Debian 13, Arch, Proxmox VE 9 |

**Unsupported:** Kernel versions 6.9 and 6.10

---

## Packaging Architecture

NonRAID is distributed as two separate Debian packages:

### 1. nonraid-dkms (Kernel Module Package)

**Purpose:** Dynamic Kernel Module Support (DKMS) package containing kernel driver source code

**Package Details:**
- **Source:** `/home/admin/proxmaid-v2/nonraid/debian/control`
- **Architecture:** any (builds locally for target kernel)
- **Dependencies:** `dkms (>= 2.8.7)` and standard misc dependencies
- **Maintainer:** Matti Hiljanen <matti@hiljanen.com>
- **Homepage:** https://github.com/qvr/nonraid

**What it installs:**
- Kernel module source code to `/usr/src/nonraid-dkms-{version}/`
- DKMS configuration file (`dkms.conf`)
- Makefile for building modules

**Modules built:**
- `md-nonraid` (main NonRAID kernel driver)
- `nonraid6_pq` (RAID-6 parity calculation module)

**Build Configuration:**
```makefile
obj-y := md_nonraid/ raid6/
PWD := $(shell pwd)
KVERSION := $(shell uname -r)
HEADERS := /lib/modules/$(KVERSION)/build/

modules:
	make -C $(HEADERS) M=$(PWD) modules CONFIG_UBSAN=n

clean:
	make -C $(HEADERS) M=$(PWD) clean
```

Key note: Builds with `CONFIG_UBSAN=n` to avoid CONFIG_UBSAN warnings from Ubuntu's aggressive UBSAN configuration.

**DKMS Configuration:**
```conf
MAKE="make modules KVERSION=$kernelver"
CLEAN="make clean"
BUILT_MODULE_NAME[0]=md-nonraid
BUILT_MODULE_LOCATION[0]=md_nonraid
DEST_MODULE_LOCATION[0]="/updates"
BUILT_MODULE_NAME[1]=nonraid6_pq
BUILT_MODULE_LOCATION[1]=raid6
DEST_MODULE_LOCATION[1]="/updates"
AUTOINSTALL="yes"
PACKAGE_NAME=nonraid-dkms
PACKAGE_VERSION=1.3.2
```

### 2. nonraid-tools (Management Tools Package)

**Purpose:** Array management command-line tool and systemd integration

**Package Details:**
- **Source:** `/home/admin/proxmaid-v2/nonraid/tools/debian/control`
- **Architecture:** all (bash script + systemd files)
- **Dependencies:** `bash (>=5.0)`
- **Section:** admin

**What it installs:**
- `/usr/bin/nmdctl` - Main array management tool (bash script)
- `/etc/systemd/system/nonraid.service` - Main array start/stop service
- `/etc/systemd/system/nonraid-parity-check.service` - Scheduled parity check service
- `/etc/systemd/system/nonraid-parity-check.timer` - Quarterly parity check timer
- `/etc/systemd/system/nonraid-notify.service` - Status notification service
- `/etc/systemd/system/nonraid-notify.timer` - Notification timer (15-minute intervals)
- `/etc/default/nonraid` - Configuration file for systemd services
- `/etc/udev/rules.d/90-nonraid.rules` - udev rules for block device handling

**File Permissions:**
- `/etc/default/nonraid` is installed with `chmod 600` (root-only readable due to potential LUKS keyfile path)

---

## Build System

### Makefile

Location: `/home/admin/proxmaid-v2/nonraid/Makefile`

```makefile
obj-y := md_nonraid/ raid6/
PWD := $(shell pwd)
KVERSION := $(shell uname -r)
HEADERS := /lib/modules/$(KVERSION)/build/

modules:
	make -C $(HEADERS) M=$(PWD) modules CONFIG_UBSAN=n

clean:
	make -C $(HEADERS) M=$(PWD) clean

package:
	dpkg-buildpackage -b -rfakeroot -us -uc
```

**Targets:**
- `make modules` - Builds kernel modules for current kernel
- `make clean` - Cleans build artifacts
- `make package` - Creates Debian binary packages using dpkg-buildpackage

### Debian Package Build (dkms)

Location: `/home/admin/proxmaid-v2/nonraid/debian/rules`

```makefile
#!/usr/bin/make -f
export DH_VERBOSE=1
export DH_AUTOSCRIPTDIR="debian/scripts/"

include /usr/share/dpkg/pkg-info.mk
export DEB_SOURCE
export DEB_VERSION_UPSTREAM

%:
	dh $@ --parallel

# Nothing to configure, build or auto-install
override_dh_auto_configure:
override_dh_auto_build:
override_dh_auto_test:
override_dh_auto_install:
override_dh_auto_clean:
```

The DKMS package uses minimal debhelper overrides since DKMS handles the actual building. The package just distributes source code.

**Install manifest** (`debian/nonraid-dkms.install`):
```
Makefile    usr/src/${env:DEB_SOURCE}-${env:DEB_VERSION_UPSTREAM}
README.md   usr/src/${env:DEB_SOURCE}-${env:DEB_VERSION_UPSTREAM}
md_nonraid  usr/src/${env:DEB_SOURCE}-${env:DEB_VERSION_UPSTREAM}
raid6       usr/src/${env:DEB_SOURCE}-${env:DEB_VERSION_UPSTREAM}
```

### Debian Package Build (tools)

Location: `/home/admin/proxmaid-v2/nonraid/tools/debian/rules`

```makefile
#!/usr/bin/make -f
export DH_VERBOSE=1

%:
	dh $@ --parallel

override_dh_installsystemd:
	dh_installsystemd --name=nonraid --no-stop-on-upgrade --no-start
	dh_installsystemd --name=nonraid-parity-check --no-stop-on-upgrade --no-start --no-enable nonraid-parity-check.service
	dh_installsystemd --name=nonraid-parity-check nonraid-parity-check.timer
	dh_installsystemd --name=nonraid-notify --no-stop-on-upgrade --no-start --no-enable nonraid-notify.service
	dh_installsystemd --name=nonraid-notify nonraid-notify.timer

override_dh_installinit:
	dh_installinit -n --name nonraid

override_dh_installudev:
	dh_installudev --name=nonraid --priority=90

override_dh_fixperms:
	dh_fixperms
	chmod 600 debian/nonraid-tools/etc/default/nonraid
```

**Key behaviors:**
- Main `nonraid.service` is **not auto-enabled** on install (`--no-start`)
- Parity check service is **not auto-enabled** (`--no-enable`), only timer is enabled
- Notify service is **not auto-enabled**, only timer is enabled
- udev rules installed with priority 90 (high priority)
- Configuration file explicitly set to `chmod 600` for security

---

## Systemd Services and Timers

### nonraid.service

**Location:** `/home/admin/proxmaid-v2/nonraid/tools/systemd/nonraid.service`

```ini
[Unit]
Description=Start/Stop NonRAID array and manage mounts
Requires=local-fs.target
After=local-fs.target

[Service]
Type=oneshot
RemainAfterExit=yes
StateDirectory=nonraid
Environment="SUPER=/nonraid.dat"
Environment="AUTOMOUNT=yes"
Environment="MOUNT_PARAMS="
Environment="STARTUP_SETTINGS="
EnvironmentFile=-/etc/default/nonraid
ExecStart=nmdctl -u -v -s $SUPER start
ExecStart=bash -c 'if [[ -f "${STATE_DIRECTORY}/array.running" ]]; then echo "Unclean shutdown detected!"; nmdctl -u check correct; else touch "${STATE_DIRECTORY}/array.running"; fi'
ExecStart=-bash -c 'echo -n "$STARTUP_SETTINGS"|while IFS= read -rd, set || [ -n "$set" ]; do nmdctl set $set; done'
ExecStart=-bash -c '[ "$AUTOMOUNT" = "yes" ] && nmdctl -u mount $MOUNT_PARAMS || true'
ExecStop=-bash -c '[ "$AUTOMOUNT" = "yes" ] && nmdctl -u unmount || true'
ExecStop=nmdctl -u stop
ExecStop=rm -f ${STATE_DIRECTORY}/array.running

[Install]
WantedBy=multi-user.target
```

**Purpose:** Main array lifecycle management - starts array on boot, mounts disks, stops on shutdown

**Startup sequence:**
1. Start array with superblock from `$SUPER` (default `/nonraid.dat`)
2. Detect unclean shutdown:
   - If `array.running` marker exists, array didn't shutdown cleanly → run corrective parity check
   - Otherwise, create marker file for next boot
3. Apply startup settings from `$STARTUP_SETTINGS` (e.g., turbo write mode)
4. Auto-mount all data disks if `AUTOMOUNT=yes`

**Shutdown sequence:**
1. Auto-unmount all data disks if `AUTOMOUNT=yes`
2. Stop the array
3. Remove the `array.running` marker

**Key features:**
- Type `oneshot` with `RemainAfterExit=yes` - service stays active after exec
- Uses `StateDirectory=nonraid` - creates `/var/lib/nonraid/` for state tracking
- All commands run in unattended mode (`-u` flag) - no interactive prompts
- Failing commands are prefixed with `-` to prevent service failure on non-critical issues

### nonraid-notify.service and nonraid-notify.timer

**Service Location:** `/home/admin/proxmaid-v2/nonraid/tools/systemd/nonraid-notify.service`
**Timer Location:** `/home/admin/proxmaid-v2/nonraid/tools/systemd/nonraid-notify.timer`

**Timer:**
```ini
[Unit]
Description=NonRAID status notifier timer

[Timer]
OnBootSec=15m
OnUnitActiveSec=15m
Unit=nonraid-notify.service

[Install]
WantedBy=timers.target
```

**Purpose:** Periodic monitoring of array health and sending notifications on degradation

**Service behavior:**
```bash
# Runs every 15 minutes (OnBootSec=15m, OnUnitActiveSec=15m)
# Checks array status via: nmdctl --no-color status -o $NONRAID_NOTIFY_FORMAT
# If exit code != 0 and array is STARTED:
#   - Sends notification via $NONRAID_NOTIFY_CMD (e.g., email, Discord)
#   - Writes state to /var/lib/nonraid/notified.state
#   - Rate limits notifications (once per day, then daily)
# If exit code == 0 (array healthy):
#   - Sends recovery notification if previously degraded
#   - Removes state file
```

**Configuration variables** (in `/etc/default/nonraid`):
- `NONRAID_NOTIFY_CMD` - Command to send notifications (empty = disabled)
- `NONRAID_NOTIFY_FORMAT` - Output format: `terse` (one-line), `default`, `json`, `prometheus`

**Example notification command:**
```bash
# Email notifications
NONRAID_NOTIFY_CMD="mail -s 'NonRAID Status Alert' admin@example.com"

# Discord via Apprise
NONRAID_NOTIFY_CMD="apprise -t 'NonRAID Status Alert' discord://webhook_id/webhook_token"
```

### nonraid-parity-check.service and nonraid-parity-check.timer

**Service Location:** `/home/admin/proxmaid-v2/nonraid/tools/systemd/nonraid-parity-check.service`
**Timer Location:** `/home/admin/proxmaid-v2/nonraid/tools/systemd/nonraid-parity-check.timer`

**Timer:**
```ini
[Unit]
Description=NonRAID quarterly parity check timer

[Timer]
OnCalendar=quarterly
Persistent=true
RandomizedDelaySec=3600
Unit=nonraid-parity-check.service

[Install]
WantedBy=timers.target
```

**Purpose:** Scheduled parity checks to detect and correct degradation

**Schedule:**
- Runs **quarterly** (systemd calendar event: Jan, Apr, Jul, Oct at 00:00)
- `Persistent=true` - catches up on missed runs
- `RandomizedDelaySec=3600` - randomly delays by 0-3600 seconds to avoid thundering herd

**Service:**
```ini
[Unit]
Description=NonRAID scheduled parity check
Requires=nonraid.service
After=nonraid.service

[Service]
Type=oneshot
EnvironmentFile=-/etc/default/nonraid
ExecStart=-nmdctl -u check nocorrect
```

**Behavior:**
- Runs in **unattended mode** (`-u flag`)
- Defaults to **check-only mode** (`nocorrect`) - reports errors but doesn't fix
- Only runs if `nonraid.service` is active
- Failures are non-blocking (`-` prefix)

**Note:** Manual parity checks can use `correct` option to fix errors, but scheduled checks use `nocorrect` to be safe.

---

## udev Rules

**Location:** `/home/admin/proxmaid-v2/nonraid/tools/udev/nonraid.udev`

```
SUBSYSTEM!="block", GOTO="nmd_end"

# handle nmd arrays
ACTION=="remove", GOTO="nmd_end"
KERNEL!="nmd*", GOTO="nmd_end"

IMPORT{builtin}="blkid"

LABEL="nmd_end"
```

**Purpose:** Minimal udev rules for NonRAID block devices

**Behavior:**
1. Only process block devices (SUBSYSTEM=="block")
2. Ignore device removal events
3. Only process NonRAID devices (KERNEL=="nmd*")
4. Run `blkid` builtin to cache filesystem information into udev database
5. This allows udev to provide quick filesystem detection without scanning the block device

**Installation:**
- Priority: 90 (high priority, custom rules)
- Location: `/etc/udev/rules.d/90-nonraid.rules`

**Why minimal?**
- The driver itself creates `/dev/nmdXp1` block device files
- udev just caches filesystem info for performance
- No special device permissions or naming needed

---

## CI/CD Pipeline

### Workflow Files

All CI workflows are in `.github/workflows/`:

#### 1. dkms-tests.yml - Build Verification

**Trigger:**
- On PRs touching kernel module files
- On main branch pushes
- Weekly schedule (Sunday 02:00 UTC)

**Test matrix:**
```yaml
- Debian 12 with linux-headers-amd64
- Debian 13 with linux-headers-amd64
- Ubuntu 24.04 with linux-headers-generic
- Ubuntu 24.04 HWE with linux-headers-generic-hwe-24.04
- Arch Linux (experimental) with linux-headers
```

**Steps:**
1. Install build dependencies (git, build-essential, dkms, kernel headers)
2. Extract DKMS version from `dkms.conf`
3. Copy source to `/usr/src/nonraid-dkms-{version}/`
4. Run `dkms add`, `dkms build`
5. Verify both `md-nonraid` and `nonraid6_pq` modules built successfully
6. Report module info via `modinfo`

**Success criteria:** Both modules compile without errors

#### 2. dkms-integration-tests.yml - Functional Testing

**Trigger:**
- On PRs touching driver/tools/Makefile
- On main branch pushes
- Weekly schedule (Sunday 03:00 UTC)

**Test environment:**
- Creates 4 x 1GB loop devices (d1, d2, d3, d4)
- Partitions with GPT + single partition (8-sector alignment, starting at 32KB)
- Sets up device symlinks in `/dev/disk/by-id/`

**Tests:**
1. **Array creation:** Uses `nmdctl create` to create array with 1 parity + 2 data disks
2. **Parity sync:** Triggers initial parity reconstruction, waits for completion
3. **Filesystem creation:** Creates XFS on data disks
4. **Data write:** Writes 100MB random data to verify I/O
5. **Disk addition:** Adds 4th disk to array, triggers disk clear operation
6. **Disk removal:** Unassigns disk 2, zeros it to simulate failure
7. **Degraded operation:** Starts array in degraded mode with missing disk
8. **Disk rebuild:** Replaces failed disk, verifies data integrity via MD5

**Success criteria:** MD5 checksums match before/after operations

#### 3. nmdctl-tests.yml - Management Tool Testing

**Trigger:**
- On PRs touching `tools/nmdctl`
- On main branch pushes

**Test steps:**
1. **Static analysis:** ShellCheck on `tools/nmdctl`
2. **Syntax check:** `bash -n` validation
3. **Unit tests:** BATS (Bash Automated Testing System) tests in `tools/tests/`

**Success criteria:** No ShellCheck warnings, no syntax errors, all BATS tests pass

#### 4. tools-debian.yml - Tools Package Building

**Trigger:** Manual workflow dispatch

**Options:**
- `create_release` - Create GitHub release
- `draft_release` - Create as draft (no git tag)

**Steps:**
1. Extract version from `tools/nmdctl` (VERSION variable)
2. Calculate build number based on existing tags
3. Build Debian package with `dpkg-buildpackage`
4. Upload artifact (30-day retention)
5. If creating release:
   - Generate changelog from git log since last release
   - Build Debian source package
   - Sign with GPG and publish to PPA (ppa:qvr/nonraid)
   - Create GitHub release with release notes

**Artifacts:**
- Binary package: `nonraid-tools_*.deb`
- Changes file: `nonraid-tools_*.changes`
- Build info: `nonraid-tools_*.buildinfo`

#### 5. dkms-debian.yml - DKMS Package Building

**Trigger:** Manual workflow dispatch (same as tools)

**Steps:**
1. Extract version from `dkms.conf`
2. Build Debian package
3. Upload artifact
4. If creating release:
   - Generate changelog from commits since last tag
   - Build Debian source package
   - Sign and publish to PPA
   - Create GitHub release with notes

**Supported kernel versions in release notes:**
- 6.1-6.8 (tested up to 6.14)
- 6.9, 6.10 explicitly noted as unsupported

---

## Installation Methods

### Option 1: PPA Installation (Recommended for Ubuntu/Debian)

```bash
# Add PPA
sudo add-apt-repository ppa:qvr/nonraid
sudo apt update

# Install prerequisites and packages
sudo apt install linux-headers-$(uname -r) nonraid-dkms nonraid-tools
```

**For Debian:**
```bash
# Install GPG
sudo apt install gpg

# Add signing key
wget -qO- "https://keyserver.ubuntu.com/pks/lookup?op=get&search=0x0B1768BC3340D235F3A5CB25186129DABB062BFD" | sudo gpg --dearmor -o /usr/share/keyrings/nonraid-ppa.gpg

# Add repository
echo "deb [signed-by=/usr/share/keyrings/nonraid-ppa.gpg] https://ppa.launchpadcontent.net/qvr/nonraid/ubuntu noble main" | sudo tee /etc/apt/sources.list.d/nonraid-ppa.list
```

**Post-installation:**
```bash
# Verify DKMS module
sudo dkms status

# Create array
sudo nmdctl create
```

### Option 2: Manual GitHub Release Installation

```bash
# Install prerequisites
sudo apt install dkms linux-headers-$(uname -r) build-essential

# Download and install packages
sudo apt install ./nonraid-dkms_*.deb ./nonraid-tools_*.deb
```

### Option 3: Arch Linux AUR

```bash
yay -Syu nonraid-git
```

### Option 4: Manual Build from Source

```bash
# Clone repository
git clone https://github.com/qvr/nonraid.git
cd nonraid

# Extract version
DKMS_VERSION=$(grep "^PACKAGE_VERSION=" dkms.conf | cut -d= -f2)

# Install to DKMS
DKMS_SRC_DIR="/usr/src/nonraid-dkms-$DKMS_VERSION"
sudo mkdir -p "$DKMS_SRC_DIR"
sudo cp -r md_nonraid/ raid6/ dkms.conf Makefile "$DKMS_SRC_DIR/"

# Build and install
KVERSION=$(uname -r)
sudo dkms install nonraid-dkms/$DKMS_VERSION -k "$KVERSION"

# Install tools
sudo cp tools/nmdctl /usr/local/bin/
sudo chmod +x /usr/local/bin/nmdctl

# Copy systemd/udev files manually if desired
sudo cp tools/systemd/nonraid* /etc/systemd/system/
sudo cp tools/udev/nonraid.udev /etc/udev/rules.d/90-nonraid.rules
sudo cp tools/systemd/nonraid.default /etc/default/nonraid
```

**Verification:**
```bash
sudo dkms status
# Expected: nonraid-dkms/{version}, {kernel}, x86_64: installed
```

**Important:** Install kernel headers meta-package for auto-updates:
- Ubuntu: `sudo apt install linux-headers-generic` or `linux-headers-generic-hwe-24.04`
- Debian: `sudo apt install linux-headers-amd64`

---

## Configuration

### /etc/default/nonraid

**Location:** `/home/admin/proxmaid-v2/nonraid/tools/systemd/nonraid.default`

This file is sourced by `nonraid.service` and controls systemd behavior.

**Configuration variables:**

```bash
# Path to superblock file (default: /nonraid.dat)
SUPER=/nonraid.dat

# Auto-mount/unmount with service (default: yes)
AUTOMOUNT=yes

# Additional mount parameters
# Examples:
#   -k /path/to/keyfile  - LUKS keyfile location
#   /mnt/custom          - Custom mount prefix
MOUNT_PARAMS=""

# Startup settings (comma-separated list of "nmdctl set" commands)
# Examples:
#   STARTUP_SETTINGS="md_write_method 1"
#   STARTUP_SETTINGS="md_write_method 1, md_trace 2"
STARTUP_SETTINGS=""

# Notification command for degraded array (optional)
# Examples:
#   NONRAID_NOTIFY_CMD="mail -s 'Alert' admin@example.com"
#   NONRAID_NOTIFY_CMD="apprise -t 'Alert' discord://webhook"
NONRAID_NOTIFY_CMD=""

# Notification output format (default: terse)
# Options: terse, default, json, prometheus
NONRAID_NOTIFY_FORMAT="terse"
```

**Usage example:**

```bash
# Edit configuration
sudo nano /etc/default/nonraid

# Set custom mount prefix
MOUNT_PARAMS="/mnt/pool"

# Enable notifications
NONRAID_NOTIFY_CMD="apprise -t 'NonRAID Alert' discord://webhook_id/webhook_token"

# Reload systemd and restart service
sudo systemctl daemon-reload
sudo systemctl restart nonraid.service
```

**File permissions:** `chmod 600` (root-only readable) to protect LUKS keyfile paths

---

## Kernel Driver Interface

The kernel driver provides two procfs interfaces for manual management:

### /proc/nmdcmd - Command Interface

**Write-only file** for sending commands to the driver

**Common commands:**

```bash
# Import a disk into slot
echo "import 0 sdb1 0 10000000 0 VBOX_HARDDISK_xxxxx" > /proc/nmdcmd

# Start array
echo "start NEW_ARRAY" > /proc/nmdcmd

# Stop array
echo "stop" > /proc/nmdcmd

# Check/fix parity
echo "check CORRECT" > /proc/nmdcmd    # Check and fix
echo "check NOCORRECT" > /proc/nmdcmd  # Check only

# Pause parity operation
echo "nocheck PAUSE" > /proc/nmdcmd

# Resume paused operation
echo "check RESUME" > /proc/nmdcmd

# Set configuration
echo "set md_write_method 1" > /proc/nmdcmd
echo "set md_trace 2" > /proc/nmdcmd
```

**Full reference:** See `/home/admin/proxmaid-v2/nonraid/docs/nmdcmd.8`

### /proc/nmdstat - Status Interface

**Read-only file** providing real-time array and disk status

**Key fields:**

```
sbName - Superblock file path
sbVersion - Superblock format version
sbState - Superblock state (0=clean)
sbNumDisks - Number of configured slots
sbSyncErrs - Errors from last sync

mdVersion - Driver version
mdState - Array state (STARTED, STOPPED, NEW_ARRAY, etc.)
mdNumDisks - Current disk count
mdNumMissing - Number of missing disks
mdNumInvalid - Number of invalid disks
mdResyncAction - Current operation (Reconstruct, Check, etc.)
mdResync - Current sector being synced
mdResyncSize - Total size of operation

disk*Idx - Disk at this index
disk*Dev - Device name
disk*Size - Device size
disk*State - Disk status (HEALTHY, FAILED, MISSING, etc.)

rdevStatus - Overall disk status
rdevHealth - Per-disk health
```

**Example reading:**

```bash
sudo grep "mdState\|mdResync\|mdNumMissing" /proc/nmdstat
# mdState=STARTED
# mdResync=5000000
# mdResyncSize=10000000
# mdNumMissing=0
```

---

## Manual Array Management

For advanced operations or troubleshooting, arrays can be managed directly via the driver interface.

### Key Concepts

**Array States:**
- `STOPPED` - Array is stopped (normal shutdown)
- `NEW_ARRAY` - Fresh array with new disks, parity invalid
- `RECON_DISK` - Disk replacement or reconstruction needed
- `DISABLE_DISK` - Degraded mode with missing disk
- `SWAP_DSBL` - Parity disk swap in progress
- `STARTED` - Array running and ready

**Disk States:**
- `HEALTHY` - Disk operational and in sync
- `FAILED` - Disk has errors
- `MISSING` - Disk not imported
- `INVALID` - Disk needs reconstruction
- `DISABLED` - Disk disabled in degraded mode

### Example: Manual Array Creation

```bash
# 1. Load driver with superblock
sudo modprobe nonraid super=/nonraid.dat

# 2. Import disks (P=parity, 1-2=data)
sudo bash -c 'echo "import 0 sdd1 0 10000000 0 DISK_ID_P" > /proc/nmdcmd'
sudo bash -c 'echo "import 1 sdb1 0 10000000 0 DISK_ID_1" > /proc/nmdcmd'
sudo bash -c 'echo "import 2 sdc1 0 10000000 0 DISK_ID_2" > /proc/nmdcmd'

# 3. Verify import
sudo cat /proc/nmdstat | grep -E "disk|rdev"

# 4. Start array
sudo bash -c 'echo "start NEW_ARRAY" > /proc/nmdcmd'

# 5. Check array state
sudo cat /proc/nmdstat | grep mdState

# 6. Create filesystems
sudo mkfs.xfs /dev/nmd1p1
sudo mkfs.xfs /dev/nmd2p1

# 7. Start parity reconstruction
sudo bash -c 'echo "check CORRECT" > /proc/nmdcmd'

# 8. Monitor progress
watch 'sudo cat /proc/nmdstat | grep mdResync'
```

**nmdctl automates all of this:**

```bash
sudo nmdctl create  # Interactive wizard
# or
sudo nmdctl create --force P:sdd1:disk_P 1:sdb1:disk_1 2:sdc1:disk_2
```

### Handling Unclean Shutdowns

If the system crashes or loses power:

1. **Driver detects unclean shutdown** via `sbState` field in superblock
2. **nonraid.service startup** checks for `array.running` marker:
   - If marker exists → array crashed → auto-run corrective parity check
   - If marker missing → normal shutdown → start normally
3. **Marker file:** `/var/lib/nonraid/array.running` (created by service)

---

## Development Workflow

### Repository Structure

```
nonraid/
├── README.md                    # User documentation
├── DEVELOPMENT.md               # Development guide
├── CONTRIBUTING.md              # Contribution guidelines
├── LICENSE                      # GPL-2.0 license
├── dkms.conf                    # DKMS configuration
├── Makefile                     # Build rules
│
├── md_nonraid/                  # Main kernel driver source
│   ├── unraid.c                 # Core driver logic
│   ├── super.c                  # Superblock handling
│   ├── Makefile                 # Driver build
│   └── ...
│
├── raid6/                       # RAID-6 parity algorithms
│   ├── algos.c                  # Parity calculation
│   ├── Makefile                 # RAID6 build
│   └── ...
│
├── tools/
│   ├── nmdctl                   # Array management tool (bash)
│   ├── systemd/                 # Systemd service/timer files
│   │   ├── nonraid.service
│   │   ├── nonraid.default
│   │   ├── nonraid-notify.{service,timer}
│   │   └── nonraid-parity-check.{service,timer}
│   ├── udev/
│   │   └── nonraid.udev         # udev rules
│   ├── debian/                  # Tools package metadata
│   ├── tests/                   # BATS unit tests for nmdctl
│   └── Makefile
│
├── debian/                      # DKMS package metadata
│   ├── control                  # Package definition
│   ├── rules                    # Build rules (empty for DKMS)
│   ├── nonraid-dkms.install     # File installation manifest
│   └── nonraid-dkms.dkms        # DKMS configuration copy
│
├── docs/
│   ├── manual-management.md     # Driver interface docs
│   ├── nmdcmd.8                 # procfs command man page
│   ├── nmdstat.5                # procfs status man page
│   └── _research_infra.md       # This file
│
├── .github/workflows/           # CI/CD pipelines
│   ├── dkms-tests.yml           # Build tests
│   ├── dkms-integration-tests.yml # Functional tests
│   ├── nmdctl-tests.yml         # Tool tests
│   ├── dkms-debian.yml          # DKMS package build
│   └── tools-debian.yml         # Tools package build
│
└── images/                      # Documentation images
    └── status_screenshot.png    # UI screenshot for README
```

### Development Notes

**Kernel support:** From `DEVELOPMENT.md`

The driver is forked from unRAID and ports features to different kernel versions:

- **unRAID 6.12.15 (kernel 6.1.126)** → branches `nonraid-6.1`
- **unRAID 7.0.1 (kernel 6.6.78)** → branch `nonraid-6.6` (no functional diff from 6.1)
- **unRAID 7.1.2 (kernel 6.12.24)** → branch `nonraid-6.12` (7.2.0 has no driver changes)

**Known kernel issues:**

- Ubuntu has `CONFIG_UBSAN=y` enabled, causing false array-index-out-of-bounds warnings
  - Build system disables with `CONFIG_UBSAN=n` during module compilation
  - This is a driver quirk, not a real bug

**Getting upstream patches:**

```bash
wget https://unraid-dl.sfo2.cdn.digitaloceanspaces.com/stable/unRAIDServer-6.12.2-x86_64.zip
unzip unRAIDServer-6.12.2-x86_64.zip -d unRAIDServer
cd unRAIDServer
unsquashfs -d patches bzfirmware src

# Patches are in patches/src - look for changes to:
# - drivers/md/unraid.c (driver logic)
# - drivers/md/Kconfig (configuration)
# - drivers/md/raid6/algos.c (parity algorithms)
```

### Contributing

**Guidelines from `CONTRIBUTING.md`:**

1. **For questions:** Use GitHub Discussions
2. **For bugs:** Open GitHub Issues
3. **For large changes:** Discuss via issue/discussion first
4. **AI tool policy:**
   - Allowed to use (Claude, Copilot, etc.) but understand all generated code
   - **Required:** Include note in PR indicating which parts were AI-generated
   - **Not allowed:** AI-generated issue/PR descriptions - write in your own words
5. **Code style:** Follow existing conventions
6. **Documentation:** Keep verbose/AI-generated sections minimal

---

## Summary

NonRAID's infrastructure is built around:

1. **Two-package model** - Separate DKMS (kernel) and tools (management) packages
2. **Systemd integration** - Services and timers for array lifecycle and monitoring
3. **DKMS convenience** - Automatic module rebuilding on kernel updates
4. **Comprehensive testing** - Multi-distro CI testing build, functional, and unit tests
5. **Clean separation** - Driver can be used manually via procfs or via nmdctl tool
6. **PPA distribution** - Easy installation on Ubuntu/Debian systems
7. **Minimal dependencies** - Only requires bash, DKMS, and kernel headers

The architecture allows both automated array management (via systemd) and manual control (via /proc/nmdcmd) for advanced users and troubleshooting.

