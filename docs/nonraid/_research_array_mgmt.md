# NonRAID Array Management in nmdctl - Comprehensive Research

## Overview

The `nmdctl` tool is a 4837-line bash script that serves as the primary user interface for NonRAID array management. It provides functionality for creating, starting, stopping, and maintaining NonRAID arrays through direct interaction with the Linux kernel module and the `/proc/nmdstat` interface.

**File:** `/home/admin/proxmaid-v2/nonraid/tools/nmdctl`
**Version:** 1.22.0

---

## Core System Architecture

### Module Loading and Initialization

**Key Functions:**
- `check_module_loaded()` (line 127-186)
- `check_nmdstat_exists()` (line 189-209)
- `reload_module()` (line 4071-4168)

#### Module Loading Flow

1. **Automatic Module Detection (line 137-160)**
   - Check if `nonraid` module is already loaded with `lsmod | grep -q nonraid`
   - For `create_array` commands, verify the loaded module uses the correct superblock
   - If superblocks differ, error out to prevent accidental modification of wrong array

2. **Superblock Path Resolution**
   - Default path: `/nonraid.dat` (DEFAULT_SUPERBLOCK, line 25)
   - Can be overridden with `-s` or `--super PATH` flag
   - If not found but needed, attempts to load from current running module

3. **Module Loading (line 164-178)**
   - Command: `modprobe nonraid super="$superblock"`
   - For new arrays, allows loading with non-existent superblock files
   - Superblock will be created when array is first started
   - Provides appropriate user warnings for new superblock creation

4. **Module Reloading (line 4109-4167)**
   - Stops running array if active
   - Unloads current module with `modprobe -r nonraid`
   - Reloads with new superblock path specified

### Status Interface: /proc/nmdstat

**Key Functions:**
- `get_all_nmdstat_values()` (line 214-232)
- `get_nmdstat_value()` (line 235-237)

#### Data Structures

The kernel module exposes array state through `/proc/nmdstat` with key=value format. Values are cached in associative array `NMDSTAT_VALUES`:

**Array State Keys:**
- `mdState` - Current array state (see Array States section)
- `mdResyncAction` - Active sync operation type
- `mdResyncCorr` - Whether resync is corrective (1) or non-corrective (0)
- `mdResyncPos` - Current resync position in bytes
- `mdResyncSize` - Total size to resync in bytes
- `mdResync` - Resync progress flag (1=running, 0=stopped/paused)
- `sbName` - Path to superblock file

**Disk Keys (per slot, 0-29):**
- `diskId.$slot` - Disk identifier string
- `diskSize.$slot` - Configured disk size in KB
- `diskState.$slot` - Disk state code (0=new, 1=ready, 2=failed, 3=assigned, 4=disabled)
- `diskName.$slot` - Linux device name (e.g., "md0")
- `rdevName.$slot` - Real device name (e.g., "sda1")
- `rdevStatus.$slot` - Device status (DISK_OK, DISK_INVALID, DISK_NP_MISSING, DISK_NP_DSBL, DISK_WRONG)
- `rdevSize.$slot` - Actual device size in KB
- `rdevId.$slot` - Device ID from superblock

**Health and Statistics:**
- `mdNumMissing`, `mdNumInvalid`, `mdNumWrong`, `mdNumDisabled`, `mdNumReplaced`, `mdNumNew` - Counters
- `mdHealth` - Overall array health status
- `mdLastSync` - Last sync position (for paused operations)

#### Kernel Interface: /proc/nmdcmd

**Function:** `run_nmd_command()` (line 117-124)

Commands are issued to the kernel module by writing to `/proc/nmdcmd`:

```bash
echo -n "$command" > /proc/nmdcmd
```

This is the primary mechanism for all control operations (start, stop, import, check, etc.).

---

## Array State Machine

### Array States (line 268-290)

NonRAID implements a comprehensive state machine:

**STOPPED**
- Array not running
- Disks not imported
- Safe state for administrative operations

**STARTED**
- Array operational
- Disks imported and functioning
- Data accessible

**NEW_ARRAY**
- First-time array startup
- Parity being initialized
- User confirmation required (unless explicit state provided)

**RECON_DISK**
- One or more disks need reconstruction
- Disk data lost or corrupted, recovery required
- Requires user confirmation to proceed

**DISABLE_DISK**
- Running in degraded mode with missing disk(s)
- Parity providing data for missing disks
- Limited redundancy

**SWAP_DSBL**
- Parity swap in progress
- Old parity disk will be moved to data slot
- New parity disk being initialized
- **Critical:** User must manually copy old parity to new parity disk before array start

**ERROR: (prefix)**
- Critical error state
- Array operation may have failed
- Data integrity at risk

### State Transitions

```
STOPPED --create/import--> NEW_ARRAY
   |                            |
   |          <-start (forced)---
   |                            |
   +--> RECON_DISK/DISABLE_DISK/SWAP_DSBL
   |                            |
   +-- (start after import) --> STARTED
   |                            |
   |          <--stop-----------
   |
   +-- ERROR: (critical failure)
```

---

## Array Creation

### Workflow Overview

**Functions:**
- `create_array()` (line 2762-2791)
- `create_array_layout()` (line 2794-2988)
- `create_array_interactive()` (line 2989-3312)

### Create Array - Two Modes

#### 1. Non-Interactive (Layout Mode)

**Invocation:** `nmdctl create [SLOT:DEVICE[:ID] ...]`

**Flow (line 2784-2786):**
1. Caller provides disk parameters: `P:/dev/sdb1 1:/dev/sdc1 2:/dev/sdd1`
2. Parse all SLOT:DEVICE[:ID] parameters
3. Validate each device and disk ID

**Parameter Validation (line 2845-2988):**
- Check superblock exists or is new (line 2802-2806)
- Verify array in STOPPED state (line 2819-2823)
- Pre-fetch available devices once (line 2840-2842)
- For each parameter:
  - Parse slot (P=0, Q=29, or 1-28)
  - Parse device path
  - Optionally parse disk ID string
  - Validate format: `^([0-9P-Q]|[12][0-9]|29):.*$`
  - Check device exists and is available
  - Extract disk size and disk ID from device

**Create with Force Flag (-f):**
- Skips device availability checks
- Allows any block device to be used
- Useful for recovery scenarios

#### 2. Interactive Mode

**Invocation:** `nmdctl create` (no parameters)

**User Prompts (line 2989-3312):**
1. Confirm new superblock creation if needed (line 2995-3000)
2. Scan available disks
3. Ask for array label (line 3068-3073)
4. For each available disk:
   - Display device, partition, size, disk ID
   - Show previous assignment if recreating (line 3103-3130)
   - Request slot assignment (P, Q, or 1-28)
5. Request confirmation before proceeding

### Array Creation Process

**Superblock Creation:**
- File created on first `start` command
- Path specified by `-s` flag or default `/nonraid.dat`
- Contains disk IDs and slot assignments

**Disk Layout:**
- Slot 0: Primary parity disk (P notation)
- Slot 29: Secondary parity disk (Q notation) - optional
- Slots 1-28: Data disks (up to 28 data slots)

**New Array State:**
- After creation, array is in NEW_ARRAY state
- No parity has been computed yet
- Array must be started with `nmdctl start` to build parity

---

## Array Startup

### Start Array Workflow

**Function:** `start_array()` (line 2359-2495)

### Step-by-Step Startup Process

#### 1. Preconditions Check (line 2361-2371)
```bash
check_root          # Require root privileges
check_module_loaded # Load kernel module if needed
check_nmdstat_exists # Verify /proc/nmdstat available
```

#### 2. Array State Check (line 2373-2379)
- If already STARTED, return success
- If STOPPED, proceed to normal startup
- If abnormal state (NEW_ARRAY, RECON_DISK, etc.), may need confirmation

#### 3. Disk Import (line 2381-2382)
- Call `import_disks` to load all configured disks
- Scans for physical disks matching superblock disk IDs
- Finds available partitions and loads them

#### 4. Import Verification (line 2386-2413)
- Iterate through all defined slots
- Check `rdevName.$slot` for each disk (is it imported?)
- Count successfully imported disks
- Identify missing disk slots
- **Error if any defined slots missing and not unassigned**

#### 5. State-Specific Startup (line 2425-2486)

**STOPPED State:**
```
STOPPED -> run_nmd_command "start" -> STARTED
```

**Abnormal States:**
- NEW_ARRAY: First-time parity initialization needed
- RECON_DISK: Disk reconstruction will occur
- DISABLE_DISK: Running degraded with missing disks
- SWAP_DSBL: Parity swap continuation
- ERROR state: May fail or cause data loss

User confirmation required unless:
- `--unattended` flag provided (error in abnormal state)
- Explicit state parameter passed: `nmdctl start <STATE>` (line 2437-2444)

#### 6. Post-Startup (line 2488-2492)
- Display success message
- Show detailed status with `show_status`

---

## Array Shutdown

### Stop Array Workflow

**Function:** `stop_array()` (line 2498-2569)

### Shutdown Sequence

#### 1. Preconditions (line 2499-2503)
```bash
check_root
check_nmdstat_exists
```

#### 2. State Check (line 2510-2515)
- If not STARTED, return success
- Otherwise proceed with shutdown

#### 3. Filesystem Unmounting (line 2517-2552)

**Safety Check:**
- Iterate all slots (line 2519-2530)
- For each slot, get `diskName.$slot`
- Check filesystem type with `get_fs_type`
- Determine mount point with `get_mountpoint`
- If any filesystem mounted, prompt for unmounting

**Error in Unattended Mode:**
- Cannot stop with mounted filesystems in unattended mode
- User must explicitly unmount or authorize unmounting

#### 4. Array Stop Command (line 2554-2558)
```bash
run_nmd_command "stop"  # Send to kernel module
```

#### 5. Module Reload (line 2560-2565)
- If not unattended mode, reload module to clear stale state
- Warning if reload fails (non-critical)

#### 6. Completion (line 2567-2568)
- Display success message

---

## Disk Import Operations

### Import Disks Workflow

**Function:** `import_disks()` (line 2199-2356)

### Import Process

#### 1. Initialization (line 2201-2211)
```bash
check_root
check_module_loaded  # Load kernel module
check_nmdstat_exists # Verify /proc/nmdstat
```

#### 2. Disk Discovery (line 2220-2312)

For each defined slot in superblock:

**Check If Already Imported (line 2229-2234):**
```bash
rdevname=$(get_nmdstat_value "rdevName.$slot")
if [ -n "$rdevname" ] && [ "$rdevname" != "none" ]; then
    skip "Already has device $rdevname imported"
fi
```

**Check If Intentionally Unassigned (line 2236-2246):**
```bash
rdevstatus=$(get_nmdstat_value "rdevStatus.$slot")
if [ "$rdevstatus" = "DISK_NP_MISSING" ] || [ "$rdevstatus" = "DISK_NP_DSBL" ]; then
    skip "Disk has been unassigned"
fi
```

**Check Disk State (line 2248-2252):**
```bash
disk_state=$(get_nmdstat_value "diskState.$slot")
if [ "$disk_state" -eq 4 ]; then
    skip "Disk disabled due to errors"
fi
```

**Physical Disk Lookup (line 2254-2311):**

1. Get disk ID from superblock:
   ```bash
   disk_id=$(get_nmdstat_value "diskId.$slot")
   ```

2. Search `/dev/disk/by-id/` for matching disk:
   ```bash
   found_device=$(find_matching_disk "$disk_id")
   ```
   - Function `find_matching_disk()` (line 2167-2195)
   - Searches for symlinks in `/dev/disk/by-id/`
   - Returns matching device path

3. Find available partition:
   ```bash
   partition=$(find_partition "$found_device")
   ```
   - Function `find_partition()` (line 2141-2164)
   - Uses `lsblk -bpno kname,size,mountpoint` to list partitions
   - Sorts by size, returns largest unmounted partition
   - Checks not parent of LVM/ZFS with `lsblk -pno pkname`
   - Validates exclusive access with Python: `os.O_WRONLY|os.O_EXCL`

4. Get partition size:
   ```bash
   disk_size=$(get_disk_size_kb "$partition")  # line 2266
   ```

5. Validate against configured size (line 2268-2293):
   - Get expected size from superblock: `diskSize.$slot`
   - If mismatch and not unattended mode, ask user for confirmation
   - In unattended mode, error on mismatch

#### 3. Import List Construction (line 2300-2301)

Create import list entry: `slot|partition|offset|size|erased|id`

**Fields:**
- `slot`: Disk slot number (0-29)
- `partition`: Device name (e.g., "sdb1")
- `offset`: Usually 0 (could support partial disk usage)
- `size`: Partition size in KB
- `erased`: 1 if disk pre-cleared, 0 otherwise
- `id`: Disk ID string from superblock

#### 4. Import Execution (line 2336-2346)

For each disk in import list:
```bash
run_nmd_command "import $slot $partition $offset $size $erased $id"
```

**Import Command Format:**
- Slot: 0-29 integer
- Partition: Device name without /dev/ (e.g., "sdb1")
- Offset: 0 for standard usage
- Size: Disk size in KB
- Erased: 1 if pre-cleared (skips driver clearing), 0 otherwise
- ID: Disk identifier string

---

## Disk Management: Add, Replace, Unassign

### Add Disk Workflow

**Function:** `add_disk()` (line 3314-3906)

#### Invocation Modes

**Interactive Mode:**
```bash
nmdctl add
```
- User selects device from available list
- User selects slot interactively
- User confirms preclear status

**Parameter Mode:**
```bash
nmdctl add SLOT:DEVICE[:ID]
nmdctl add P:/dev/sdb1:WD-12345
nmdctl add 1:/dev/sdc1
```

**Replace Mode:**
```bash
nmdctl add SLOT_NUMBER
```
- Replaces existing disk in slot

**Force Mode:**
```bash
nmdctl add -f SLOT:DEVICE[:ID]
```
- Skips device availability checks

#### Parameter Parsing (line 3344-3394)

**Format Validation:**
```
SLOT:DEVICE[:ID]
Where SLOT can be: P, Q, 0-29
```

**Slot Normalization (line 3364-3371):**
- P → 0 (primary parity)
- Q → 29 (secondary parity)

**Conditional Detection (line 3429-3464):**
- If slot has size but no disk ID → Replace operation
- If slot has disk ID and is parity (0 or 29) → Parity swap (requires unassigned data slot)
- If slot empty (size=0) → Add operation

#### Preconditions (line 3396-3426)

```bash
check_root
check_module_loaded
check_nmdstat_exists
```

**Array State Validation:**
- NEW_ARRAY: Error, start array first
- STARTED: Error, stop array first
- Other: Allowed if explicit state parameter or confirmation

#### Device Selection

**Candidate Discovery (line 3600-3680):**

Uses `list_available_devices()` (line 2725-2761):

1. Scan `/dev/disk/by-id/` for disk devices
2. For each disk:
   - Find largest partition
   - Get partition size in KB/GB
   - Extract disk ID
   - Create entry: `device|partition|size_kb|size_gb|id`

3. Filter already-in-array devices (line 3701-3715):
   - Compare disk ID against existing `diskId.$slot` values
   - Error if disk already in array (unless -f/--force)

**Parameter Mode Selection (line 3676-3700):**
- Parse device from SLOT:DEVICE[:ID] parameter
- Find matching device by exact device path
- Validate device is available for use

**Interactive Mode Selection (line 3724-3760):**
- Display available devices in table format
- User enters choice (1-N)
- Validate choice is in range

#### Slot Selection

**Fixed Slot (line 3764-3769):**
- When replacing, use provided replace_slot

**Parameter Mode (line 3767-3769):**
- Use slot from SLOT:DEVICE[:ID] parameter

**Interactive Mode (line 3771-3808):**
- Display slot options:
  - P or 0: Parity disk
  - Q or 29: Second parity disk
  - 1-28: Data disk
- Validate selected slot is available

#### Preclear Status (line 3827-3846)

**Interactive Mode Query (line 3833-3839):**
- Ask if disk has been pre-cleared
- Pre-clear is optional optimization: `dd if=/dev/zero of=PARTITION bs=1M status=progress`
- If pre-cleared: set `preclear_done=1` (driver skips clearing)
- If not pre-cleared: driver clears disk when array started

**Parameter Mode (line 3842-3845):**
- Assume disk NOT pre-cleared by default

#### Confirmation and Import (line 3848-3870)

**Interactive Mode Confirmation (line 3848-3856):**
```bash
read -r -p "Proceed with the operation? (y/N): " confirm
```

**Parameter Mode Display (line 3857-3860):**
- Show action without confirming

**Import Execution (line 3865-3866):**
```bash
run_nmd_command "import $selected_slot $(basename "$selected_partition") 0 $selected_size_kb $preclear_done $selected_disk_id"
```

#### Post-Add Workflow (line 3874-3903)

**Normal Add (data disk, line 3896-3902):**
1. Start the array: `nmdctl start`
2. Clear the new disk: `nmdctl check`

**Parity Disk Add (slot 0 or 29, line 3898-3899):**
1. Start the array: `nmdctl start`
2. Reconstruct new parity: `nmdctl check`

**Parity Swap Case (line 3878-3894):**
- State becomes SWAP_DSBL
- **Critical manual step:** Copy old parity disk contents to new parity disk
  ```bash
  ( dd if=$old_parity_partition bs=1M status=progress ; \
    dd if=/dev/zero bs=1M status=progress ) > /dev/$new_parity_rdevname
  ```
- Then replace old parity to unassigned data slot
- Finally start array

### Replace Disk Workflow

Subset of add_disk workflow:
- Slot has size but no disk ID (recently unassigned)
- Or parity swap case (unassign data slot, swap parity)

### Unassign Disk Workflow

**Function:** `unassign_disk()` (line 3909-4068)

#### Invocation
```bash
nmdctl unassign SLOT
nmdctl unassign P      # Slot 0
nmdctl unassign Q      # Slot 29
nmdctl unassign 5      # Data slot
```

#### Preconditions (line 3943-3965)

```bash
check_root
check_module_loaded
check_nmdstat_exists
```

**Array Must Be Stopped (line 3960-3966):**
```bash
mdstate=$(get_nmdstat_value "mdState")
if [ "$mdstate" = "STARTED" ]; then
    error "Array must be stopped"
fi
```

#### Validation (line 3968-4030)

**Get Disk Information (line 3968-3981):**
```bash
disk_id=$(get_nmdstat_value "diskId.$slot")
rdev_name=$(get_nmdstat_value "rdevName.$slot")
rdev_status=$(get_nmdstat_value "rdevStatus.$slot")
disk_size=$(get_nmdstat_value "diskSize.$slot")
rdev_size=$(get_nmdstat_value "rdevSize.$slot")
```

**Check Not Already Unassigned (line 3992-4001):**
- If `diskId` is empty and `diskSize` is 0: Already unassigned
- If `rdevStatus` is DISK_NP_DSBL: Already unassigned

**Parity Preservation Check (line 4033-4050):**

```bash
unassigned_disks = count slots with DISK_NP_MISSING or DISK_NP_DSBL
parity_count = count of slots 0 and 29 with diskSize > 0

if (unassigned_disks + 1) > parity_count:
    error "Not enough parity disks to unassign"
```

This ensures:
- Single parity: max 1 disk can be unassigned at once
- Dual parity: max 2 disks can be unassigned
- Unassigned disks are reconstructed via parity

#### Unassign Execution (line 4052-4058)

```bash
run_nmd_command "import $slot '' 0 0 0 ''"
```

**Import Command with Empty Values:**
- Slot: Target slot
- Partition: Empty string
- Offset: 0
- Size: 0
- Erased: 0
- ID: Empty string

This marks the slot as having no physical device but maintains size configuration.

#### Post-Unassign (line 4060-4065)

- Display success message
- Note: Changes committed on array start
- Show current status

---

## Parity Check and Maintenance

### Parity Check Workflow

**Function:** `handle_check()` (line 2572-2683)

#### Invocation

```bash
nmdctl check                # Default: CORRECT (corrective check)
nmdctl check CORRECT        # Corrective check (fixes errors)
nmdctl check NOCORRECT      # Non-corrective (read-only check)
nmdctl check PAUSE          # Pause current operation
nmdctl check RESUME         # Resume paused operation
nmdctl check CANCEL         # Stop and cancel operation
```

#### Preconditions (line 2574-2578)

- Array must be STARTED
- Access `/proc/nmdstat` for operation status

#### State Variables (line 2584-2589)

```bash
mdstate=$(get_nmdstat_value "mdState")           # Array state
mdresyncaction=$(get_nmdstat_value "mdResyncAction")  # Operation type
mdresynccorr=$(get_nmdstat_value "mdResyncCorr")      # Corrective flag
mdresyncpos=$(get_nmdstat_value "mdResyncPos")        # Current position
mdresyncsize=$(get_nmdstat_value "mdResyncSize")      # Total size
mdresync=$(get_nmdstat_value "mdResync")              # Running flag
```

#### Operation Control (line 2598-2612)

**PAUSE/CANCEL Commands (line 2599-2612):**

```bash
if [ "$mdresync" -eq 0 ]; then
    warning "No resync operation running"
else
    run_nmd_command "nocheck PAUSE|CANCEL"
fi
```

**RESUME Command (line 2619-2622):**
```bash
if [ "$mdresyncpos" -eq 0 ]; then
    error "No paused operation to resume"
else
    run_nmd_command "check RESUME"
fi
```

#### Conflict Detection (line 2614-2656)

**Already Running (line 2614-2617):**
- If mdresync > 0 and mdresyncpos > 0: Operation in progress
- Error out

**Paused Operation Pending (line 2624-2637):**
- If mdresync=0 but mdresyncpos > 0: Operation paused
- Show pause percentage: `(mdresyncpos * 100 / mdresyncsize)%`
- Prompt to resume (unless RESUME requested)
- In unattended mode, error if paused operation exists

**Other Sync Operation Pending (line 2638-2657):**
- If mdresyncaction is not "check*"
- Show warning about other operation
- Allow proceeding if explicitly requested
- In unattended mode, error

#### Default Behavior (line 2658-2672)

**Unattended Mode Default:**
- Use NOCORRECT mode (read-only, no corrections)

**Interactive Mode Default:**
- Prompt user for confirmation before starting CORRECT

#### Execution (line 2675-2680)

```bash
run_nmd_command "check CORRECT|NOCORRECT|RESUME"
```

#### Operations Triggered by Check

**New Array Initialization:**
- First check after array creation
- Builds initial parity data

**Disk Clearing (Add New Data Disk):**
- New disk sectors cleared to ensure parity correctness
- Happens when array started with new uncleared disk

**Parity Reconstruction (Add Parity Disk):**
- Calculates and writes parity for new parity slot

**Disk Recovery (RECON_DISK state):**
- Reconstructs data from remaining disks and parity
- May be triggered automatically if disk fails during operation

**Corrective Check (CORRECT option):**
- Reads all data
- Verifies parity
- Corrects any mismatches found
- Time-consuming operation

**Non-Corrective Check (NOCORRECT option):**
- Reads and verifies parity
- Does not write corrections
- Faster, suitable for monitoring

---

## Device Path and Disk Identification

### Disk Identification

**Key Concept:** NonRAID uses disk IDs (not device names) to identify disks persistently.

#### Disk ID Extraction

**From /dev/disk/by-id/ (line 2167-2195):**

```bash
find_matching_disk() {
    local disk_id="$1"
    if [ -d "/dev/disk/by-id" ]; then
        for id_path in /dev/disk/by-id/*; do
            # Skip partitions (have -part* in name)
            if [[ "$id_path" == *-part* ]]; then continue; fi

            id_name=$(basename "$id_path")
            # Match if disk ID substring found
            if [[ "$id_name" == *"$disk_id"* ]]; then
                real_dev=$(readlink -f "$id_path")
                echo "$real_dev"
                return 0
            fi
        done
    fi
    return 1
}
```

**Disk ID Sources:**
- Superblock configuration stores disk IDs
- `/dev/disk/by-id/` symlinks provide persistent identification
- Serial numbers combined with model info

#### Device Path Types

**Three levels of device identification:**

1. **Physical Device:** `/dev/sda` - actual hardware
2. **Persistent ID Path:** `/dev/disk/by-id/ata-WDC_WD10EZEX-08M_WD-12345ABC` - hardware signature
3. **Partition:** `/dev/sda1` - logical partition on device

### Partition Discovery

**Function:** `find_partition()` (line 2141-2164)

```bash
# List partitions, sort by size, take largest unmounted
partition=$(lsblk -bpno kname,size,mountpoint "$dev" | \
    grep -v -w "$dev" | grep "$dev" | \
    sort -k2 -n | tail -1 | \
    awk '{ if($3 == "") print $1}')

# Validate partition not parent of LVM/ZFS
if ! lsblk -pno pkname "$partition" | grep -q -w "$partition"; then
    # Check exclusive lock availability (Python)
    if command -v python3 >/dev/null 2>&1; then
        if ! python3 -c "import os; os.open('$partition', os.O_WRONLY|os.O_EXCL)" 2>/dev/null; then
            dev_locked=1  # Device in use (ZFS, LVM, etc.)
        fi
    fi
    if [ "$dev_locked" -eq 0 ]; then
        echo "$partition"
    fi
fi
```

**Partition Selection Criteria:**
1. Find all partitions on device
2. Sort by size (ascending)
3. Take largest
4. Must not be mounted
5. Must not be parent of LVM/ZFS
6. Must have exclusive lock available

### Disk Size Handling

**Function:** `get_disk_size_kb()` (line 2121-2140)

```bash
# Get partition size in KB via blockdev
blockdev --getsize64 "$1" | awk '{print int($1 / 1024)}'
```

**Size Validation (line 2268-2293):**

When importing disk:
1. Get actual partition size: `disk_size=$(get_disk_size_kb "$partition")`
2. Get configured size from superblock: `configured_size=$(get_nmdstat_value "diskSize.$slot")`
3. Compare:
   - If sizes differ and unattended mode: Error
   - If sizes differ and interactive: Prompt user
   - Use configured size if confirmed, else use actual size

---

## Superblock Management

### Superblock Structure

**Location:** Configurable, default `/nonraid.dat`

**Format:** Binary format read/written by kernel module

**Contains:**
- Array configuration and metadata
- Slot-to-disk-ID mappings
- Disk size information
- Previous array state
- Parity configuration (single or dual parity)

### Superblock Operations

#### Creation (line 2802-2806)

```bash
if [ ! -f "$superblock" ]; then
    is_new_superblock=1
    echo "Creating a new array with new superblock at: $superblock"
    echo "Note: The superblock file will be created when the array is started"
fi
```

- Superblock file created on disk when array first started
- Kernel module writes superblock after initial configuration

#### Reloading (line 4071-4168)

```bash
reload_module() {
    # Stop running array
    run_nmd_command "stop"

    # Unload module
    modprobe -r nonraid

    # Load with new superblock
    modprobe nonraid super="$superblock"
}
```

Use case: Switching between different array configurations

#### Backup (line 3028-3032)

During array recreation:
```bash
mv "$superblock" "${superblock}.bak"
reload_module  # Load with fresh superblock
```

Old superblock backed up as `.bak` file

---

## State Consistency and Locking

### Consistency Mechanisms

#### Kernel Module as Source of Truth

- `/proc/nmdstat` reflects kernel module state
- nmdctl reads state before operations
- All writes go through `/proc/nmdcmd` interface

#### Disk Lock Prevention

**Exclusive Device Lock Check (line 2151-2153):**

```bash
if command -v python3 >/dev/null 2>&1; then
    if ! python3 -c "import os; os.open('$partition', os.O_WRONLY|os.O_EXCL)" 2>/dev/null; then
        dev_locked=1  # Device is locked
    fi
fi
```

Prevents using disks currently in use by other systems (ZFS, LVM, etc.)

#### Driver State Inconsistency Detection

**Function:** `check_driver_inconsistent_state()` (line 348-405)

Checks counter values that indicate inconsistency:
- `mdNumMissing` - Missing disks
- `mdNumInvalid` - Invalid disks
- `mdNumWrong` - Wrong disks (misplaced)
- `mdNumDisabled` - Disabled disks
- `mdNumReplaced` - Replaced disks
- `mdNumNew` - New disks

If any counter > 0 while array STARTED, indicates operational state requiring attention.

### Error Handling

#### Unattended Mode Safety (line 22-23)

Flag: `UNATTENDED=0`

Prevents destructive operations without confirmation:
- Cannot start array with missing disks (line 2417-2419)
- Cannot stop array with mounted filesystems (line 2533-2535)
- Cannot unassign disks if parity insufficient (line 4045-4050)
- Must explicitly specify array state for abnormal conditions (line 2468-2470)
- Cannot handle paused operations (line 2628-2630)

---

## Status Reporting and Monitoring

### Status Collection Functions

**Array Summary (line 463-471):**
- mdState, mdHealth, mdNumDisks

**Array Health (line 472-628):**
- Detailed health status based on disk states
- Counts of each disk status type

**Array Size and Parity (line 629-685):**
- Data array sizes
- Parity disk sizes
- Total capacity calculations

**Resync Status (line 686-771):**
- Current resync operation
- Progress percentage
- Estimated time remaining
- Byte rates

**Disk Status (line 772-863):**
- Per-disk status
- Device names and sizes
- Health indicators

### Output Formats

**Human-Readable (line 864-1092):**
- Colored output for terminal display
- Formatted tables and sections

**JSON (line 1305-1400):**
- Machine-parseable output
- All status data in JSON structure

**Prometheus (line 1138-1304):**
- Metrics in Prometheus format
- For monitoring integration

**Terse (line 1401-1412):**
- Minimal output
- One-line status

### Monitor Mode (line 1749-1825)

Real-time array monitoring:
```bash
nmdctl status -m [INTERVAL]  # Default: 2 second refresh
```

Features:
- Live status updates
- Progress bars for resync operations
- Real-time I/O rate calculations
- Keyboard control (q to quit)

---

## Filesystem Operations

### Disk Mounting

**Function:** `mount_array_disks()` (line 4171-4347)

For each data disk (slots 1-28):
1. Get disk name: `diskName.$slot`
2. Detect filesystem type: `blkid -s TYPE`
3. Determine mount point: `/mnt/disk<N>` or from fstab
4. Mount with appropriate options

### Disk Unmounting

**Function:** `unmount_array_disks()` (line 4349-4483)

1. Iterate all disk slots
2. Get mount point for each disk
3. Unmount with `umount` command
4. Force unmount if needed (`umount -l`)

---

## Workflow Integration

### Typical Array Lifecycle

```
1. CREATE ARRAY
   nmdctl create                          # Interactive or
   nmdctl create P:/dev/sdb1 1:/dev/sdc1  # Non-interactive

2. IMPORT DISKS (automatic during start, or manual)
   nmdctl import                          # Manual import

3. START ARRAY
   nmdctl start                           # Loads disks, starts array

4. BUILD PARITY (automatic or manual)
   nmdctl check CORRECT                   # Builds initial parity

5. MONITOR STATUS
   nmdctl status -m                       # Monitor in real-time

6. ADD/REPLACE DISKS (while array running or stopped)
   nmdctl add 1:/dev/sdd1                 # Add new disk to slot 1
   nmdctl check CORRECT                   # Clear/reconstruct new disk

7. MOUNT FILESYSTEMS (optional)
   nmdctl mount                           # Mount all data disks

8. OPERATIONAL
   nmdctl check NOCORRECT                 # Periodic parity checks
   nmdctl status                          # Status monitoring

9. MAINTENANCE
   nmdctl unmount                         # Unmount filesystems
   nmdctl stop                            # Stop array
   nmdctl unassign 5                      # Unassign disk
   nmdctl start                           # Restart array
```

---

## Implementation Notes

### Bash Shell Requirements

- Bash 4.0+ required for associative arrays (line 6-9)
- Uses `declare -g -A` for global arrays
- Supports parameter expansion and regex

### External Dependencies

- `lsmod` - Check loaded modules
- `modprobe` - Load/unload kernel modules
- `blockdev` - Get block device info
- `lsblk` - List block devices
- `blkid` - Identify filesystem types
- `mount`/`umount` - Filesystem mounting
- `python3` - Exclusive lock checking (optional)
- `grep`, `awk`, `sed` - Text processing
- `cat` - File reading

### Signal Flow

```
User Input (nmdctl)
    ↓
Bash Script (validation, formatting)
    ↓
/proc/nmdcmd (kernel command interface)
    ↓
Kernel Module (nonraid)
    ↓
/proc/nmdstat (kernel status interface)
    ↓
Bash Script (read state, format output)
    ↓
User Output (display results)
```

---

## Summary

The nmdctl tool implements a comprehensive array management interface with:

1. **State Machine Management:** Tracks array through multiple states (NEW, STOPPED, STARTED, RECON, etc.)
2. **Disk Identification:** Persistent disk IDs prevent configuration errors
3. **Safety Mechanisms:** User confirmations, unattended mode restrictions, parity preservation rules
4. **Disk Management:** Add, replace, and unassign operations with flexible slot allocation
5. **Parity Operations:** Check/reconstruction with corrective and non-corrective modes
6. **Superblock Configuration:** Persistent array configuration via kernel module parameter
7. **Status Monitoring:** Multiple output formats for human and machine consumption
8. **Error Handling:** Comprehensive validation and conflict detection

All operations are mediated through the kernel module via `/proc/nmdstat` interface, ensuring consistency and preventing configuration corruption.
