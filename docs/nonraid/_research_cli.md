# nmdctl CLI Interface - Comprehensive Research Document

## Overview

**nmdctl** is the NonRAID array management utility - a command-line interface for managing NonRAID RAID-like arrays. It provides a complete command dispatch system with global options, status monitoring, array operations, disk management, maintenance commands, and filesystem operations.

- **Version**: 1.22.0
- **Script Location**: `/nonraid/tools/nmdctl`
- **Written in**: Bash 4.0+
- **Requires**: Root privileges for most operations

---

## Global Options and Flags

Global options are parsed before the main command and apply to the entire operation:

### Option Syntax
```
nmdctl [GLOBAL OPTIONS] COMMAND [COMMAND OPTIONS]
```

### All Global Options

| Short | Long | Argument | Default | Description |
|-------|------|----------|---------|-------------|
| `-s` | `--super` | PATH | `/nonraid.dat` | Superblock file path for array configuration |
| `-k` | `--keyfile` | PATH | `/etc/nonraid/luks-keyfile` | LUKS keyfile path for encrypted disks |
| `-u` | `--unattended` | (none) | disabled | Enable unattended mode (no interactive prompts) |
| `-v` | `--verbose` | (none) | disabled | Enable verbose output (detailed information) |
| `--no-color` | (none) | (none) | disabled | Disable colored output in terminal |
| `-V` | `--version` | (none) | - | Display version information and exit |
| `-h` | `--help` | (none) | - | Display help message and exit |

#### Special Global Options
- `help` (without dash) - Also displays help message
- All global options must appear **before** the command name

---

## Complete Command Reference

### 1. STATUS AND MONITORING

#### Command: `status`
Display current array status with comprehensive information.

**Syntax:**
```bash
nmdctl status [OPTIONS]
```

**Options:**
| Option | Argument | Default | Description |
|--------|----------|---------|-------------|
| `-v, --verbose` | (none) | disabled | Show detailed status information (disk names, sizes in 1K blocks, reads/writes) |
| `--no-fs` | (none) | disabled | Don't show filesystem information |
| `-o, --output` | FORMAT | `default` | Output format: `default`, `prometheus` (or `prom`), `json`, `terse` |
| `-m, --monitor` | [INTERVAL] | 2 seconds | Monitor mode: refresh status every INTERVAL seconds (default: 2) |

**Output Formats:**
- **default**: Human-readable colored terminal output with full status details
- **prometheus**: Prometheus metrics format (enables `--no-fs` automatically)
- **prom**: Alias for `prometheus`
- **json**: JSON structured output for parsing by scripts
- **terse**: Minimal text output (enables `--no-fs` automatically)

**Monitor Mode Details:**
- Press `q` to quit monitor mode
- Refreshes status at specified interval
- Only works with default output format
- Shows live updates with status bars

**Examples:**
```bash
# Default status display
nmdctl status

# Verbose status with detailed disk info
nmdctl status -v

# Monitor mode with 5-second refresh
nmdctl status -m 5

# JSON output for scripts
nmdctl status -o json

# Prometheus metrics for monitoring
nmdctl status -o prometheus

# Skip filesystem info but stay verbose
nmdctl status -v --no-fs
```

---

### 2. ARRAY MANAGEMENT COMMANDS

#### Command: `create`
Create a new NonRAID array with interactive disk assignment or direct layout specification.

**Syntax (Interactive Mode):**
```bash
nmdctl create
```

**Syntax (Direct Mode):**
```bash
nmdctl create [-f] [SLOT:DEV[:ID] ..]
```

**Options:**
| Option | Description |
|--------|-------------|
| `-f, --force` | Skip device availability checks; allow any block device |

**Direct Mode Format:**
- `SLOT:DEV[:ID]` where:
  - `SLOT`: Slot number (0=P/Parity, 1-28=Data, 29=Q/Second Parity)
  - `DEV`: Device path (e.g., `/dev/sdb1`)
  - `ID` (optional): Disk identifier string for tracking

**Interactive Mode Flow:**
1. Asks for array label
2. Lists available disks with size and ID
3. For each disk, prompts for slot assignment:
   - `P` or `0`: Parity disk
   - `Q` or `29`: Second parity disk (optional)
   - `1-28`: Data disk slots
   - `o`: Use old slot (if recreating)
   - `s`: Skip this disk
   - `f`: Finish array creation
4. Offers to mark parity as valid (if rebuilding)
5. Shows array status
6. Asks if user wants to start the array now

**Examples:**
```bash
# Interactive creation
nmdctl create

# Direct layout creation
nmdctl create P:/dev/sdb1 1:/dev/sdc1 2:/dev/sdd1

# Force creation without device checks
nmdctl create -f P:/dev/sdb1:WD-ABC123 1:/dev/sdc1:WD-DEF456

# Direct with disk IDs
nmdctl create P:/dev/sda1:samsung-001 1:/dev/sdb1:wd-002 2:/dev/sdc1:seagate-003
```

---

#### Command: `start`
Start the NonRAID array and make it operational.

**Syntax:**
```bash
nmdctl start [STATE]
```

**Parameters:**
- `STATE` (optional): Explicit array state to start in (e.g., `NEW_ARRAY`, `RECON_DISK`)

**Behavior:**
1. Loads module if not already loaded
2. Runs import_disks to discover and import disks
3. Validates all defined disks are imported
4. Handles special states:
   - `NEW_ARRAY`: Starting a new array for the first time (rebuilds parity)
   - `RECON_DISK`: Starting with disk reconstruction needed
   - `DISABLE_DISK`: Starting in degraded mode (missing disks)
   - `SWAP_DSBL`: Performing parity disk swap (requires manual data copy first)
   - `ERROR:*`: Array in error state (may require recovery)

**Unattended Mode Restrictions:**
- Cannot start array with missing disks
- Cannot start with 0 imported disks
- Cannot start in abnormal states without explicit STATE parameter

**Examples:**
```bash
# Start array (normal mode)
nmdctl start

# Start in specific state
nmdctl start NEW_ARRAY

# Start in unattended mode
nmdctl -u start

# Start after handling missing disks
nmdctl start DISABLE_DISK
```

---

#### Command: `stop`
Stop the running NonRAID array.

**Syntax:**
```bash
nmdctl stop
```

**Behavior:**
1. Checks if any filesystems are mounted on data disks
2. If mounted, offers to unmount them (interactive mode) or fails (unattended mode)
3. Stops the array
4. Reloads module to clear stale state (interactive mode only)

**Unattended Mode Restrictions:**
- Fails if any filesystems are mounted

**Examples:**
```bash
# Stop array
nmdctl stop

# Stop in unattended mode (fails if mounted)
nmdctl -u stop
```

---

#### Command: `import`
Import all disks in the array without starting it.

**Syntax:**
```bash
nmdctl import
```

**Behavior:**
- Scans for all configured disks and imports them
- Does not start the array
- Useful for preparing array before starting
- Used internally by `start` command

**Examples:**
```bash
# Import all disks
nmdctl import
```

---

### 3. DISK MANAGEMENT COMMANDS

#### Command: `add`
Add a new disk to the array or replace an existing disk.

**Syntax (Interactive Mode):**
```bash
nmdctl add
```

**Syntax (Direct Mode):**
```bash
nmdctl add [-f] SLOT:DEV[:ID]
nmdctl add [-f] SLOT
```

**Options:**
| Option | Description |
|--------|-------------|
| `-f, --force` | Skip device availability checks |

**Formats:**
- `SLOT:DEVICE`: Add disk to specific slot
- `SLOT:DEVICE:ID`: Add disk with specific identifier
- `SLOT`: Replace/unassign disk in slot (interactive mode)

**Slot Notation:**
- `P` or `0`: Parity disk
- `Q` or `29`: Second parity disk
- `1-28`: Data disk slots

**Interactive Mode Flow:**
1. Lists available disks
2. Prompts for slot assignment
3. Optionally generates preclear data for new disks
4. Imports disk and handles reconstruction

**Examples:**
```bash
# Interactive add
nmdctl add

# Add disk to specific slot
nmdctl add 1:/dev/sdb1

# Add with disk ID
nmdctl add 1:/dev/sdb1:WD-12345ABC

# Replace parity disk
nmdctl add P:/dev/sda1

# Force add without checks
nmdctl add -f 2:/dev/sdd1
```

---

#### Command: `replace`
Replace a disk in a specific slot (alias for `add` with slot parameter).

**Syntax:**
```bash
nmdctl replace SLOT[:DEV[:ID]]
```

**Parameters:**
- `SLOT`: Slot number (0, 1-28, or 29; P/Q also supported)
- `DEV` (optional): Device path to use
- `ID` (optional): Disk identifier

**Examples:**
```bash
# Interactive replace (choose disk)
nmdctl replace 1

# Replace with specific disk
nmdctl replace 1:/dev/sdb1

# Replace parity disk
nmdctl replace P:/dev/sda1
```

---

#### Command: `unassign`
Unassign a disk from a specific slot without removing it physically.

**Syntax:**
```bash
nmdctl unassign SLOT
```

**Parameters:**
- `SLOT`: Slot number to unassign (0-29)

**Behavior:**
- Marks slot as not imported
- Does not physically remove the disk
- Allows starting array in degraded mode
- Can later reassign the disk

**Examples:**
```bash
# Unassign slot 3
nmdctl unassign 3

# Unassign parity disk (slot 0)
nmdctl unassign 0

# Unassign second parity (slot 29)
nmdctl unassign 29
```

---

### 4. MAINTENANCE COMMANDS

#### Command: `reload`
Reload the NonRAID kernel module with specified superblock.

**Syntax:**
```bash
nmdctl reload
```

**Behavior:**
1. Gets current superblock path (from running module or global `--super` option)
2. Checks if superblock file exists (confirms if not)
3. Stops running array if active
4. Unloads nonraid module
5. Loads module with specified superblock

**Default Superblock:** `/nonraid.dat`

**Uses:**
- Switch between different superblocks
- Recover from module issues
- Reinitialize with new superblock

**Examples:**
```bash
# Reload with current superblock
nmdctl reload

# Reload with specific superblock
nmdctl --super /path/to/backup.dat reload

# Force reload new superblock
nmdctl reload
```

---

#### Command: `check`
Start, pause, resume, or cancel parity check and reconstruction operations.

**Syntax:**
```bash
nmdctl check [OPTION]
```

**Options:**
| Option | Description |
|--------|-------------|
| `CORRECT` (default) | Start corrective parity check (detects and fixes errors) |
| `NOCORRECT` | Start check-only mode (detects errors without fixing) |
| `PAUSE` | Pause the current resync operation |
| `RESUME` | Resume a previously paused operation |
| `CANCEL` | Cancel the current resync operation |

**Array States:**
- Array must be STARTED to run parity check
- Detects if operation is already running
- Detects if operation is paused
- Detects if other sync operations are pending

**Default Behavior:**
- Without option: Uses `CORRECT` mode (interactive confirmation required)
- Unattended mode: Defaults to `NOCORRECT` if no option provided

**Related Legacy Command:**
- `nocheck`: Separate command for backwards compatibility (maps to `check CANCEL`)

**Examples:**
```bash
# Start corrective parity check
nmdctl check

# Start check-only mode
nmdctl check NOCORRECT

# Check in unattended mode (defaults to NOCORRECT)
nmdctl -u check

# Pause running operation
nmdctl check PAUSE

# Resume paused operation
nmdctl check RESUME

# Cancel operation
nmdctl check CANCEL

# Legacy command (same as check CANCEL)
nmdctl nocheck
```

---

#### Command: `set`
Configure array settings and parameters.

**Syntax:**
```bash
nmdctl set [SETTING] [VALUE]
```

**Parameters:**
- `SETTING`: Configuration parameter name
- `VALUE`: New value (omit to reset to default)

**Common Settings:**
| Setting | Values | Default | Description |
|---------|--------|---------|-------------|
| `md_write_method` or `turbo` | 0 or 1 | 0 | Write algorithm (0=READ_MODIFY_WRITE standard, 1=RECONSTRUCT_WRITE turbo) |
| `md_trace` or `debug` | 0-4 | 1 | Debug trace level (0=off, 1=cmd, 2=debug, 3=IO, 4=detailed) |
| `md_queue_limit` | 1-100 | 80 | I/O queue limit percentage (throttles normal I/O) |
| `md_sync_limit` | 0-100 | 5 | Sync queue limit percentage (throttles parity operations) |
| `md_num_stripes` | (integer) | 1280 | Stripe cache entries count |
| `label` | (string, max 32) | (empty) | Array label (alphanumeric, underscore, dash only) |

**Advanced Settings:**
| Setting | Values | Description |
|---------|--------|-------------|
| `invalidslot` | SLOT1 SLOT2 | Sets slots to start as invalid (format: "0 29") |
| `resync_start` | (sector number) | Sets parity sync start position in sectors |
| `resync_end` | (sector number) | Sets parity sync end position in sectors (0=auto) |
| `rderror` | SLOT | Simulates read error on next operation for specified slot |
| `wrerror` | SLOT | Simulates write error on next operation for specified slot |
| `md_restrict` | (integer) | Restriction flags bitfield controlling array behavior |

**Restrictions:**
- `label`: Can only be set when array is STOPPED
- Turbo mode (`md_write_method=1`): Requires all disks spinning
- Standard mode (`md_write_method=0`): Only needs parity and target data disk

**Examples:**
```bash
# Show all available settings
nmdctl set

# Enable turbo write mode
nmdctl set turbo 1

# Set debug trace level
nmdctl set md_trace 3

# Set array label
nmdctl set label myarray

# Throttle I/O queue to 50%
nmdctl set md_queue_limit 50

# Reset write method to standard
nmdctl set md_write_method 0

# Mark invalid slots
nmdctl set invalidslot 0 29
```

---

### 5. FILESYSTEM COMMANDS

#### Command: `mount`
Mount all active data disks with identified filesystems.

**Syntax:**
```bash
nmdctl mount [MOUNTPREFIX]
nmdctl mount [-k|--key-file PATH] [MOUNTPREFIX]
```

**Parameters:**
- `MOUNTPREFIX`: Directory prefix for mount points (default: `/mnt/disk`)
- `-k, --key-file PATH`: LUKS keyfile path (overrides global `--keyfile`)

**Mount Point Naming:**
- Slot 1 → `/mnt/disk1`
- Slot 2 → `/mnt/disk2`
- Slot N → `/mnt/diskN`

**Supported Filesystems:**
- **ZFS**: Imports pool with name `diskN` to mountpoint
- **LUKS**: Opens encrypted container and mounts inner filesystem
- **Standard**: XFS, Ext4, Ext3, Ext2, Btrfs (mounted normally)

**Behavior:**
- Creates mount directories if they don't exist
- Skips already-mounted filesystems
- Handles LUKS containers (requires keyfile)
- Reads `/etc/nonraid/fstab` for custom mount options

**Array Requirements:**
- Array must be STARTED

**Examples:**
```bash
# Mount all disks with default prefix
nmdctl mount

# Mount with custom prefix
nmdctl mount /data/disk

# Mount with specific LUKS keyfile
nmdctl mount -k /etc/nonraid/luks-keyfile /mnt/disk

# Mount in unattended mode (fails if array not started)
nmdctl -u mount /data
```

---

#### Command: `unmount` or `umount`
Unmount all active data disks.

**Syntax:**
```bash
nmdctl unmount
nmdctl umount
```

**Behavior:**
- Unmounts all mounted data disks
- Closes LUKS containers if they were opened
- Skips unmounted disks

**Examples:**
```bash
# Unmount all disks
nmdctl unmount

# Alias version
nmdctl umount
```

---

## Command Dispatch Logic

The main command dispatch logic is implemented in the `main()` function starting at line 4694:

### Flow:
1. **Parse Global Options** (lines 4701-4749)
   - Loop through arguments until non-option found
   - Extract `-s`, `-k`, `-v`, `-u`, `--no-color`, `-V`, `-h`

2. **Validate Command Presence** (lines 4752-4754)
   - Require at least one command
   - Exit with usage if no command provided

3. **Extract Command** (lines 4756-4758)
   - Get first remaining argument as command name
   - Shift remaining arguments to command handler

4. **Dispatch to Handler** (lines 4760-4829)
   - Case statement routes command to appropriate function
   - Handlers receive remaining arguments as parameters

### Command Routing Table:

```bash
case "$command" in
    "status")           → show_status "$@"
    "create")           → create_array "$@"
    "start")            → start_array "$1"
    "stop")             → stop_array
    "import")           → import_disks
    "add")              → add_disk "$@"
    "replace")          → add_disk "$@"  (with validation)
    "unassign")         → unassign_disk "$1"
    "reload")           → reload_module
    "check")            → handle_check "$1"
    "nocheck")          → handle_check "${1:-CANCEL}"
    "mount")            → mount_array_disks "$1" (with -k option parsing)
    "unmount"|"umount") → unmount_array_disks
    "set")              → set_array_setting "$@"
    *)                  → Error: Unknown command
esac
```

---

## Configuration Files and Environment Variables

### Configuration Files:

| File | Purpose | Default |
|------|---------|---------|
| `/nonraid.dat` | Default superblock file | Used unless `--super` specified |
| `/etc/nonraid/luks-keyfile` | LUKS encryption keyfile | Used unless `--keyfile` specified |
| `/etc/nonraid/fstab` | Mount options for data disks | Optional, consulted during mount |

### Kernel Interface:

| Path | Purpose | Access |
|------|---------|--------|
| `/proc/nmdstat` | Read-only driver status interface | Queried for array state |
| `/proc/nmdcmd` | Command interface to driver | Write-only, root required |

### Module Parameters:

When loading nonraid kernel module:
```bash
modprobe nonraid super="<path-to-superblock>"
```

### Environment Variables:

| Variable | Default | Override |
|----------|---------|----------|
| `PROC_NMDSTAT` | `/proc/nmdstat` | Can be overridden for testing |

---

## Usage Examples

### Complete Array Lifecycle:

```bash
# 1. Create new array with two data disks and parity
nmdctl create P:/dev/sda1 1:/dev/sdb1 2:/dev/sdc1

# 2. Start the array
nmdctl start

# 3. Check status
nmdctl status -v

# 4. Mount filesystems
nmdctl mount /data

# 5. Monitor array (live updates)
nmdctl status -m 5

# 6. Run parity check
nmdctl check

# 7. Unmount before maintenance
nmdctl unmount

# 8. Stop array
nmdctl stop
```

### Advanced Operations:

```bash
# Add new disk to slot 3 with specific ID
nmdctl add 3:/dev/sdd1:WD-ABC123XYZ

# Replace parity disk
nmdctl replace P:/dev/sda1

# Replace second parity with specific device
nmdctl replace 29:/dev/sde1

# Unassign missing disk from slot 5
nmdctl unassign 5

# Reload module with backup superblock
nmdctl -s /backups/array.dat reload

# Mount with custom LUKS keyfile
nmdctl -k /secure/keyfile mount /mnt/nonraid

# Run check in unattended mode
nmdctl -u check

# Export status as JSON for processing
nmdctl status -o json | jq '.array_health'

# Get Prometheus metrics
nmdctl status -o prometheus
```

---

## Version Information

- **Current Version**: 1.22.0
- **Version Command**: `nmdctl -V` or `nmdctl --version`
- **Bash Requirement**: Version 4.0 or higher (for associative arrays)

---

## Module Loading and Root Access

### Root Requirement:
All commands that modify array state or access `/proc/nmdstat` and `/proc/nmdcmd` require root privileges.

### Automatic Module Loading:
- `check_module_loaded()` function attempts to automatically load the nonraid module
- Uses superblock path from global `--super` option or defaults to `/nonraid.dat`
- Creates new superblock if file doesn't exist (for new arrays)

### Module Unload Behavior:
- Module can be unloaded with `reload_module` command
- Cannot unload if array is currently started
- Module load/unload cycles used during array recreation

---

## Return Codes and Exit Status

### Exit Codes:
- `0`: Success
- `1`: Command failed or error encountered
- Status commands return health code: `0` (healthy), `1` (warning/degraded), `2` (offline/error)

### Health Status Levels:
- **HEALTHY** (code 0): All disks present and functioning
- **WARNING** (code 1): Some I/O errors but all disks present
- **DEGRADED** (code 1): Missing/replaced disks or sync errors
- **PARTIAL** (code 1): Some disks imported, array not started
- **READY** (code 1): All disks imported but array stopped
- **NEW** (code 1): New array needs parity rebuild
- **NEW_DISK** (code 1): New disk added, needs clearing/reconstruction
- **OFFLINE** (code 2): No disks imported or array unconfigured
- **ERROR** (code 2): Array in error state

---

## Color Codes and Output Formatting

### Colors Used:
- `RED` (\033[0;31m): Errors, problems, critical issues
- `GREEN` (\033[0;32m): Success, good status, healthy
- `YELLOW` (\033[0;33m): Warnings, attention needed, degraded status
- `BLUE` (\033[0;34m): Information, new items, progress

### Color Control:
- Enabled by default in terminal
- Disabled with `--no-color` flag
- Automatically disabled when piping output

### Monitor Mode Display:
- Uses background colors for status bars
- Header: Blue background with white text
- Footer: Dark gray background with bright white text

---

## Key Functions and Their Purposes

### Utility Functions:
- `check_root()`: Verify root privileges
- `check_module_loaded()`: Verify/load nonraid kernel module
- `check_nmdstat_exists()`: Verify `/proc/nmdstat` accessible
- `get_all_nmdstat_values()`: Fetch all array status values
- `get_nmdstat_value()`: Get single status value
- `validate_device_path()`: Check device availability
- `run_nmd_command()`: Execute command via `/proc/nmdcmd`

### Status Collection:
- `collect_array_summary()`: Get array name, label, disk count
- `collect_array_health()`: Analyze health status and issues
- `collect_array_size_and_parity()`: Calculate capacity metrics
- `collect_resync_status()`: Get parity operation progress
- `collect_disk_status()`: Gather per-disk information

### Output Formatters:
- `format_output()`: Route to appropriate formatter
- `format_human_output()`: Terminal-friendly display
- `format_prometheus_output()`: Prometheus metrics
- `format_json_output()`: JSON structured output
- `format_terse_output()`: Minimal text format

### Filesystem Utilities:
- `get_fs_type()`: Detect filesystem (XFS, ZFS, LUKS, etc.)
- `get_mountpoint()`: Find where filesystem is mounted
- `get_fs_usage()`: Get disk usage percentage
- `get_disk_size_kb()`: Get disk capacity

---

## Argument Parsing Patterns

### Option Patterns:
1. **Short flag** with space: `-v value` or `-v=value`
2. **Long flag** with space: `--verbose value` or `--verbose=value`
3. **Flag without argument**: `-v` (sets flag to 1)
4. **Multi-letter shorthand**: `-m 5` (monitor with 5 second interval)

### Device Parameter Patterns:
1. **Slot only**: `add 1` → interactive mode for slot 1
2. **Slot:Device**: `add 1:/dev/sdb1`
3. **Slot:Device:ID**: `add 1:/dev/sdb1:WD-ABC123`
4. **Letter notation**: `add P:/dev/sda1` (parity), `add Q:/dev/sde1` (second parity)

### Setting Parameter Patterns:
1. **Setting only**: `set md_trace` → shows help
2. **Setting and value**: `set md_trace 3`
3. **Space-separated values**: `set invalidslot 0 29`
4. **Alias names**: `set turbo 1` → alias for `md_write_method`

---

## Notes on Command Validation

### Slot Numbers:
- Valid range: 0-29 (30 total slots)
- 0 or P: Parity disk
- 1-28: Data disks (28 available)
- 29 or Q: Second parity disk (optional)

### Device Path Validation:
- Must start with `/dev/`
- Should be actual block device or partition
- Force mode (`-f`) skips validation

### State Machine Constraints:
- Some operations only valid in specific states (STOPPED, STARTED, etc.)
- Module loading triggers state transitions
- Unattended mode restricts state transitions that need confirmation

---

## Documentation Map

This document comprehensively covers:
1. ✅ All CLI commands and subcommands
2. ✅ How argument parsing works (global then command-specific)
3. ✅ Usage/help text and command formatting
4. ✅ Version information (1.22.0)
5. ✅ Global options and flags
6. ✅ Main command dispatch logic
7. ✅ Configuration files and environment variables
8. ✅ Return codes and exit status
9. ✅ Color output and formatting
10. ✅ Complete examples for each command

---

## See Also

- Kernel Module Interface: `/nonraid/kernel/` (driver interface docs)
- RAID6 Implementation: `/nonraid/raid6/` (parity algorithm docs)
- System Integration: `/etc/nonraid/` (configuration directory)
- Service Files: `/etc/systemd/system/nonraid.service` (startup integration)
