# nmdctl Parity Operations Research

## Overview

This document provides comprehensive documentation of nmdctl's parity-related operations, including parity checks, sync/rebuild operations, progress tracking, and error handling. The nmdctl utility communicates with the NonRAID kernel module through `/proc/nmdcmd` (write) and `/proc/nmdstat` (read).

---

## 1. Parity Check Operations

### 1.1 The `check` Command

**Function:** `handle_check()` (lines 2572-2683)

The `check` command manages parity check operations with the following options:

- **CORRECT** (default): Run parity check and correct errors found
- **NOCORRECT**: Run parity check in read-only mode (verify without fixing)
- **PAUSE**: Pause an active parity check operation
- **RESUME**: Resume a paused parity check operation
- **CANCEL**: Stop and cancel an active parity check operation

**Usage:**
```bash
nmdctl check [OPTION]              # Interactive mode
nmdctl check CORRECT               # Start corrective parity check
nmdctl check NOCORRECT             # Start verification-only check
nmdctl check PAUSE                 # Pause active operation
nmdctl check RESUME                # Resume paused operation
nmdctl check CANCEL                # Cancel active operation
```

### 1.2 Prerequisites for Parity Check

Before starting a parity check (line 2591-2594):
- Array must be in **STARTED** state
- If array is not started: returns error "Array must be started to run parity check"

### 1.3 Check Logic Flow

The function implements the following state machine:

#### Step 1: Pause/Cancel Handling (lines 2599-2612)
```
IF option is PAUSE or CANCEL:
  IF no resync operation is running (mdResync == 0):
    Return warning: "No resync operation is currently running"
  ELSE:
    Execute: run_nmd_command "nocheck {PAUSE|CANCEL}"
    Print status message
```

#### Step 2: Conflict Detection (lines 2614-2637)
```
IF resync operation is already active (mdResync > 0 AND mdResyncPos > 0):
  Return error: "Resync operation is already in progress"

IF trying to RESUME but no paused operation (mdResync == 0 AND mdResyncPos == 0):
  Return warning: "No paused resync operation to resume"

IF previous operation is paused (mdResync == 0 AND mdResyncPos > 0) AND not RESUME:
  IF UNATTENDED mode:
    Return error: "Cannot start new operation with a paused operation pending"
  ELSE:
    Prompt user to confirm or resume
```

#### Step 3: Other Operation Detection (lines 2639-2673)
```
IF pending operation is NOT a parity check (mdResyncAction != check*):
  IF explicit option matches operation type:
    Proceed with that operation
  ELSE IF UNATTENDED mode:
    Return error: "Cannot start parity check with another sync operation pending"
  ELSE:
    Prompt user: "Do you want to proceed with {operation}?"
```

#### Step 4: Default Option Selection (lines 2658-2673)
```
IF no option provided:
  IF UNATTENDED mode:
    Use NOCORRECT mode by default
    Print warning
  ELSE:
    Prompt user for confirmation
    Default to CORRECT mode
```

#### Step 5: Execute Operation (lines 2675-2682)
```
Run kernel command: run_nmd_command "check {OPTION}"
If successful:
  Print: "{operation} started/resumed"
If failed:
  Return error
```

### 1.4 Unattended Mode Behavior

When `nmdctl -u` or `--unattended` is set (line 2628-2630, 2646-2648, 2658-2660):

1. **Paused operations block new operations**: Cannot start new operation with pending paused operation
2. **Other sync operations block new checks**: Cannot start parity check if other sync operation is pending
3. **Default to NOCORRECT**: If no option specified, use NOCORRECT mode
4. **No prompts**: All user confirmations are skipped, errors are returned instead

---

## 2. Parity Status and Progress Tracking

### 2.1 Parity Status Variables

The kernel module exports parity-related statistics through `/proc/nmdstat`:

**Core parity state variables:**
- `mdResync`: Boolean flag (0/1) - whether resync is actively running
- `mdResyncAction`: String - type of operation (check, recon D#, clear, etc.)
- `mdResyncCorr`: Boolean (0/1) - whether operation is in correcting mode
- `mdResyncPos`: Integer - current position in blocks
- `mdResyncSize`: Integer - total size in blocks
- `mdResyncDt`: Integer - time delta for rate calculation
- `mdResyncDb`: Integer - blocks processed in time delta

**Sync result variables:**
- `sbSynced`: Timestamp - when sync started (0 if not started)
- `sbSynced2`: Timestamp - when last sync completed
- `sbSyncErrs`: Integer - number of errors found during sync
- `sbSyncExit`: Integer - sync exit status code (0=success, >0=errors)

**Disk error counting:**
- `rdevNumErrors.SLOT`: Integer - read/write errors for disk in slot

### 2.2 Resync Status Collection

**Function:** `collect_resync_status()` (lines 686-769)

This function interprets kernel statistics and populates `RESYNC_STATUS_DATA` associative array:

#### Operation States Detected:

**1. Paused Operation** (lines 716-726)
```
Condition: mdResync == 0 AND mdResyncPos > 0
Indicates: Operation started but is currently paused
Data computed:
  - progress_percent = (mdResyncPos * 100) / mdResyncSize
  - position_kb = mdResyncPos
  - position_gb = converted from KB
  - size_kb, size_gb = total operation size
```

**2. Pending Operation** (lines 728-729)
```
Condition: mdResync == 0 AND action is "clear" or "recon*"
Indicates: Operation configured but not yet started
Action needed: Run "nmdctl check" to start the operation
```

**3. Active Operation** (lines 731-767)
```
Condition: mdResync != 0 (operation is running)
Data computed:
  - progress_percent = (mdResyncPos * 100) / mdResyncSize
  - rate_kb_s = mdResyncDb / mdResyncDt  (blocks/sec to KB/s)
  - eta_seconds = (remaining_blocks) / rate_kb_s
  - elapsed_seconds = current_time - sbSynced
```

#### Detailed Calculations:

**Rate Calculation** (lines 742-744):
```bash
# mdResyncDb: blocks processed in the time window
# mdResyncDt: time delta in seconds (or time units)
rate_kb_s = mdResyncDb / mdResyncDt
```

**ETA Calculation** (lines 747-751):
```bash
IF rate_kb_s > 0 AND remaining_blocks > 0:
  remaining_blocks = mdResyncSize - mdResyncPos
  eta_seconds = remaining_blocks / rate_kb_s
ELSE:
  eta_seconds = 0 (unknown)
```

**Elapsed Time** (lines 754-758):
```bash
# sbSynced is the timestamp when sync started
IF sbSynced != 0:
  elapsed_seconds = current_time - sbSynced
ELSE:
  elapsed_seconds = 0
```

### 2.3 Parity Health Status

**Function:** `collect_array_health()` (lines 472-626)

Health status is determined by parity-related counters:

**Health Status Levels:**
- `HEALTHY` (code 0): All disks present, parity consistent, no errors
- `WARNING` (code 1): Some I/O errors but all disks present
- `DEGRADED` (code 1): Missing disks or sync errors detected
- `OFFLINE` (code 2): No disks imported
- `ERROR` (code 2): Array in ERROR state
- `NEW_DISK` (code 1): New disk added, needs parity sync

**Parity-related Health Indicators** (lines 517-548):

```bash
# Sync error tracking
sbsyncerrs     # Number of errors found during sync
sbsyncexit     # Exit status from last sync operation (>0 = had errors)

# Last sync timing
sbsynced       # Timestamp when last sync started
sbsynced2      # Timestamp when last sync completed

# Parity checking
If sbsynced2 != 0:
  Health status includes: "Sync {status} {time_ago}"
  Status = "completed" | "errors encountered"
  Time = formatted duration (e.g., "2 hours ago")

If last_sync_ago > 8640000 seconds (100 days):
  Mark as warning: "more than 100 days"
```

### 2.4 Parity Disk Status

**Function:** `collect_array_size_and_parity()` (lines 629-683)

Tracks parity disk presence and capacity:

**Parity Disk Locations:**
- Slot 0: Primary parity (P)
- Slot 29: Secondary parity (Q)

**Status Checking** (lines 637-659):
```bash
FOR slot in [0, 29]:
  IF diskSize.{slot} > 0:
    has_parity = true
  IF rdevStatus.{slot} matches [DISK_NP*, DISK_DSBL_NEW, DISK_INVALID]:
    has_parity = false  # Parity is not functional
```

---

## 3. Parity Sync/Build Operations

### 3.1 Operation Types

**Function:** `format_resync_action()` (lines 1919-1958)

The `mdResyncAction` field encodes different operation types:

**Parity Check Operation:**
```
Action: "check" [args]
Formatted: "Parity-Check" with mode:
  - mdResyncCorr == 1: "Parity-Check (correcting)"
  - mdResyncCorr == 0: "Parity-Check (check only)"
```

**Parity Sync/Rebuild Operation:**
```
Action: "recon" [args]
Formatted as:
  - "recon D#": "Data-Rebuild Disk #" (disk reconstruction)
  - "recon ..." : "Parity-Sync ..." (parity rebuild)
```

**Disk Clearing:**
```
Action: "clear"
Formatted: "Disk clearing"
```

### 3.2 Parity Build on Array Start

When array transitions from NEW_ARRAY to STARTED state:
- Kernel module automatically initiates parity build
- This is tracked as a "recon" operation (reconstruction)
- Requires all disks to be spinning
- Progress can be monitored via status command
- Can be paused/resumed with `nmdctl check` commands

### 3.3 Parity Sync Rate Limiting

**Function:** `set_array_setting()` (lines 4485-4691)

Sync operations are rate-limited by the `md_sync_limit` setting:

**Setting:** `md_sync_limit`
- Range: 0-100 (percentage)
- Default: 5%
- Purpose: Throttle parity operations to reduce impact on normal I/O
- Description: "Sync queue limit as percentage (0-100, throttles parity operations)"

**Usage:**
```bash
nmdctl set md_sync_limit 10    # Allow 10% I/O for sync operations
nmdctl set md_sync_limit 0     # Disable rate limiting (full speed)
```

---

## 4. Rebuild Operations (Disk Replacement)

### 4.1 Disk Replacement Scenarios

**Function:** `add_disk()` (lines 3314-3909)

The add_disk function handles multiple disk replacement/rebuild scenarios:

#### Scenario 1: Replace Missing Data Disk
```
Condition: Slot 1-28 has diskSize > 0 but no diskId (unassigned)
Action:
  - User selects new disk
  - Kernel initiates "recon D#" operation (data rebuild)
  - Array starts reconstruction when array is started
```

#### Scenario 2: Replace Missing Parity Disk
```
Condition: Slot 0 or 29 has diskSize > 0 but no diskId
Action:
  - Similar to data disk replacement
  - Kernel initiates parity rebuild (recon operation)
```

#### Scenario 3: Parity Swap (Slot 0/29 Already Assigned)
```
Condition:
  - Parity slot (0 or 29) already has diskId assigned
  - An unassigned data slot exists
Action:
  - Alert: "This will start a parity swap operation"
  - User must manually copy parity to new disk BEFORE array start
  - Replace_slot is marked for parity swap
```

#### Scenario 4: Add New Data Disk
```
Condition: Adding to previously empty slot
Action:
  - No rebuild needed (no data to reconstruct)
  - Disk imported when array started
```

### 4.2 Disk Replacement Flow

**State Requirements** (lines 3413-3426):
- Array must be in STOPPED state
- Cannot add to NEW_ARRAY state (must start first to build parity)
- Cannot add to STARTED state (must stop first)

**Disk Selection** (lines 3536-3600):
```
1. Scan available devices with unused partitions
2. Filter out disks already in array (by disk ID match)
3. Show compatible disks to user
4. User selects disk device
```

**Disk Import** (lines 3866):
```bash
run_nmd_command "import {slot} {partition_name} 0 {size_kb} {preclear_done} {disk_id}"
```

---

## 5. Pause/Resume Functionality

### 5.1 Pause Mechanism

**Command:** `nmdctl check PAUSE` (line 2605)

**Execution:**
```bash
run_nmd_command "nocheck PAUSE"
```

**Kernel Behavior:**
- Sets `mdResync` to 0 (stops active processing)
- Preserves `mdResyncPos` (saves current position)
- Preserves `mdResyncSize` (operation scope unchanged)

**State After Pause:**
- `mdResync == 0`
- `mdResyncPos > 0` (position preserved)
- `mdResyncSize == original` (size unchanged)

### 5.2 Resume Mechanism

**Command:** `nmdctl check RESUME` (line 2619-2622)

**Validation:**
```
IF mdResync == 0 AND mdResyncPos == 0:
  Error: "No paused resync operation to resume"
ELSE:
  Proceed with resume
```

**Execution:**
```bash
run_nmd_command "check RESUME"
```

**Kernel Behavior:**
- Restarts operation from saved position
- Continues with original operation type
- Continues with original correcting mode

### 5.3 Pause State Detection

**Display Function:** `format_human_resync_status()` (lines 943-953)

When paused (lines 943-953):
```
Display:
  Operation: {friendly_action} (PAUSED)
  Progress: {percent}% ({current_pos} / {total_size})
  Hint: Resume with 'nmdctl check resume' command
```

---

## 6. Error Handling and Recovery

### 6.1 Parity Error Detection

**Sync Error Counting** (lines 517-518, 605-606):

During parity operations, kernel tracks:
- `sbSyncErrs`: Total errors found during sync
- `sbSyncExit`: Exit status code (0 = success, >0 = errors occurred)

**Health Impact** (lines 593-608):
```
IF sbSyncErrs > 0 OR sbSyncExit > 0:
  Health Status = DEGRADED
  Details = "Sync Errors: {count}"
  Health Code = 1 (warning)
```

**Display Format** (lines 539-542):
```
IF sbSyncExit > 0:
  Last sync: "{time_ago} ago (errors encountered)"
ELSE:
  Last sync: "{time_ago} ago"
```

### 6.2 Disk Error Tracking

**Per-Disk Errors** (lines 504-510):

Each disk slot tracks I/O errors:
- `rdevNumErrors.SLOT`: Read/write error count
- Summed across all slots: `disks_num_errors`

**Health Impact** (lines 609-611):
```
IF disks_num_errors > 0:
  Health Status = WARNING
  Details = "All disks present, but some have I/O errors ({count} total)"
  Health Code = 1
```

### 6.3 Operation Error Recovery

**Cancel Operation** (line 2605):
```bash
nmdctl check CANCEL
```

Execution:
```bash
run_nmd_command "nocheck CANCEL"
```

This:
- Stops the active operation completely
- Clears the operation state
- Allows starting a new operation

**Resume After Pause** (line 2619):

Users can:
1. Pause operation: `nmdctl check PAUSE`
2. Fix issues (e.g., replace failed disk)
3. Resume operation: `nmdctl check RESUME`

---

## 7. Kernel Module Interface

### 7.1 Command Interface: /proc/nmdcmd

**Function:** `run_nmd_command()` (lines 117-124)

Commands are sent to kernel via:
```bash
echo -n "COMMAND" > /proc/nmdcmd
```

**Parity-related Commands:**

```
check CORRECT          # Start parity check with correction
check NOCORRECT        # Start parity check verification-only
check PAUSE            # Pause active operation
check RESUME           # Resume paused operation
nocheck PAUSE          # Pause active operation (alias)
nocheck CANCEL         # Cancel active operation
```

**Error Handling** (lines 119-122):
```bash
IF write to /proc/nmdcmd fails:
  Print: "Error: Failed to run command '{cmd}'"
  Return 1 (error)
ELSE:
  Return 0 (success)
```

### 7.2 Status Interface: /proc/nmdstat

**Function:** `get_all_nmdstat_values()` (lines 214-233)

Status is read from `/proc/nmdstat` (configurable via `PROC_NMDSTAT`):

**Parsing:**
```bash
# Read /proc/nmdstat line by line
# Format: key = value
# Parse into associative array: NMDSTAT_VALUES["key"] = value
```

**Key Parity Variables:**
```
mdResync              # Active resync flag
mdResyncAction        # Operation type
mdResyncCorr          # Correction mode flag
mdResyncPos           # Current position
mdResyncSize          # Total size
mdResyncDt            # Time delta
mdResyncDb            # Blocks processed
sbSynced              # Sync start timestamp
sbSynced2             # Sync end timestamp
sbSyncErrs            # Error count
sbSyncExit            # Exit status
rdevNumErrors.*       # Per-disk error count
```

### 7.3 Module Loading

**Function:** `check_module_loaded()` (lines 127-187)

Module is checked/loaded with superblock path:

```bash
# Check if already loaded
IF lsmod | grep nonraid:
  # Module is loaded, verify correct superblock if creating array
  ...
ELSE:
  # Load module with superblock
  modprobe nonraid sbpath={SUPERBLOCK_PATH}
```

---

## 8. Unattended Mode Specifics

### 8.1 Unattended Mode Flag

**Global Variable:** `UNATTENDED` (line 36)

**Set by:**
```bash
nmdctl -u             # Short form
nmdctl --unattended   # Long form
```

### 8.2 Parity Check Behavior in Unattended Mode

**Default Mode Selection** (lines 2658-2660):
```
IF no option provided to 'nmdctl check':
  Use NOCORRECT mode by default
  Print warning message
```

**Paused Operation Blocking** (lines 2628-2630):
```
IF paused operation exists AND trying to start new operation:
  Return error: "Cannot start new operation with a paused operation pending"
  Do NOT prompt user, simply fail
```

**Other Sync Operation Blocking** (lines 2646-2648):
```
IF other sync operation pending AND trying to start parity check:
  Return error: "Cannot start parity check with another sync operation pending"
  Do NOT prompt user, simply fail
```

### 8.3 Comparison: Interactive vs Unattended

| Scenario | Interactive | Unattended |
|----------|------------|-----------|
| No check option | Prompt user, default CORRECT | Use NOCORRECT, no prompt |
| Paused operation exists | Prompt user | Return error |
| Other sync pending | Prompt user | Return error |
| Disk replacement | Interactive menus | Parameter mode only |

---

## 9. Status Display and Monitoring

### 9.1 Resync Status Formatting

**Function:** `format_human_resync_status()` (lines 936-984)

Formats parity status for human-readable output:

**Active Operation Display** (lines 963-983):
```
Operation     : {friendly_action}
Progress      : {percent}% ({current_pos} / {total_size})
Speed         : {rate} (KB/s or MB/s)
Elapsed Time  : {duration}
ETA           : {remaining_time}
```

**Paused Operation Display** (lines 943-953):
```
Operation     : {friendly_action} (PAUSED)
Progress      : {percent}% ({current_pos} / {total_size})
Resume with   : 'nmdctl check resume' command
```

**Pending Operation Display** (lines 954-962):
```
IF clearing:
  Disk clearing pending: New disk needs to be cleared
ELSE:
  Disk reconstruction pending: {friendly_action}
  Hint: Use 'nmdctl check' command to start the operation
```

### 9.2 Monitor Mode

**Command:** `nmdctl status -m [INTERVAL]` (line 67)

**Function:** `monitor_status_loop()` (lines 1749-1825)

Continuously refreshes parity status:
- Default interval: 2 seconds
- Updates parity progress in real-time
- Allows user to watch operation completion
- Can show rate, ETA, and elapsed time

### 9.3 JSON Output

**Function:** `format_json_output()` (lines 1305-1400)

Resync status exported as JSON:
```json
{
  "resync": {
    "active": true/false,
    "paused": true/false,
    "pending": true/false,
    "action": "check|recon|clear",
    "progress_percent": 0-100,
    "position_kb": 0,
    "position_gb": 0.0,
    "size_kb": 0,
    "size_gb": 0.0,
    "rate_kb_s": 0,
    "elapsed_seconds": 0,
    "eta_seconds": 0,
    "friendly_action": "Parity-Check (correcting)"
  }
}
```

### 9.4 Prometheus Metrics

**Function:** `format_prometheus_output()` (lines 1138-1304)

Exports parity metrics:
```
nonraid_resync_active{action="..."} 0|1
nonraid_resync_progress_percent{action="..."} 0-100
nonraid_resync_position_kb{action="..."} 0
nonraid_resync_size_kb{action="..."} 0
nonraid_resync_speed_kb_s{action="..."} 0
nonraid_resync_elapsed_seconds{action="..."} 0
nonraid_resync_eta_seconds{action="..."} 0
nonraid_sync_errors_total{action="..."} 0
nonraid_sync_exit_status{action="..."} 0
```

---

## 10. Array Settings Affecting Parity Operations

### 10.1 Write Method

**Setting:** `md_write_method`
- **Range:** 0 (default) or 1
- **Mode 0:** READ_MODIFY_WRITE (standard mode)
  - Only requires parity disk and target disk spinning
  - Suitable for large arrays
- **Mode 1:** RECONSTRUCT_WRITE (turbo mode)
  - Requires ALL disks spinning
  - Faster writes when all disks available
  - Not suitable during disk rebuilds

**Command:**
```bash
nmdctl set md_write_method 1      # Enable turbo mode
nmdctl set md_write_method 0      # Use standard mode
nmdctl set md_write_method        # Reset to default (0)
```

### 10.2 Sync Rate Limiting

**Setting:** `md_sync_limit`
- **Range:** 0-100 (percentage)
- **Default:** 5%
- **Purpose:** Throttle parity operations
- **Impact:** Higher = faster sync, more I/O impact on normal operations

**Command:**
```bash
nmdctl set md_sync_limit 10       # 10% of I/O for sync
nmdctl set md_sync_limit 0        # No rate limiting
nmdctl set md_sync_limit          # Reset to default (5%)
```

### 10.3 Normal I/O Queue Limit

**Setting:** `md_queue_limit`
- **Range:** 1-100 (percentage)
- **Default:** 80%
- **Purpose:** Reserve bandwidth for normal I/O
- **Impact:** Works together with `md_sync_limit`

**Command:**
```bash
nmdctl set md_queue_limit 80      # Reserve 80% for normal I/O
nmdctl set md_queue_limit         # Reset to default (80%)
```

### 10.4 Invalid Slots

**Setting:** `invalidslot`
- **Default:** "0 29" (parity slots)
- **Purpose:** Specify which slots are for parity (not data)
- **Usage:** Advanced configuration, rarely changed

**Command:**
```bash
nmdctl set invalidslot "0 29"     # Standard: P and Q slots
nmdctl set invalidslot            # Reset to default
```

### 10.5 Resync Position Control

**Setting:** `resync_start` and `resync_end`
- **Purpose:** Control which portion of array to check
- **Units:** Sectors
- **Default:** 0 (full range)
- **Usage:** Debugging and testing

**Commands:**
```bash
nmdctl set resync_start 0         # Start from beginning
nmdctl set resync_end 0           # Check to end (full array)
```

---

## 11. Notifications and Event Triggers

### 11.1 Status-Based Notifications

The nmdctl utility provides notifications through console output based on parity states:

**Operation Started:**
```
Starting {friendly_action}...
{friendly_action} started
```

**Operation Resumed:**
```
Resuming {friendly_action}...
{friendly_action} resumed
```

**Operation Paused:**
```
Pausing {friendly_action}...
{friendly_action} paused
```

**Operation Stopped:**
```
Stopping {friendly_action}...
{friendly_action} stopped
```

### 11.2 Warning Conditions

**Paused Operation Warning** (lines 2625-2627):
```
Warning: Previous resync operation {action} is currently paused at {percent}%
You can resume it with nmdctl check RESUME
```

**Other Sync Pending Warning** (line 2645):
```
Warning: A sync operation other than parity check is pending: {action}
```

**No Option Warning** (line 2662):
```
Warning: No option provided, defaulting to corrective parity check
```

**Unattended Mode Warning** (line 2659):
```
Warning: Using NOCORRECT mode by default in unattended mode
```

---

## 12. Error Conditions and Edge Cases

### 12.1 Array State Errors

**Not Started** (lines 2591-2594):
```
Error: Array must be started to run parity check
Return: error code 1
```

**Already Operating** (lines 2614-2617):
```
Error: Resync operation {action} is already in progress
Return: error code 1
```

### 12.2 Paused State Conflicts

**No Paused Operation** (lines 2619-2622):
```
Warning: No paused resync operation to resume
Return: error code 1
```

**Paused + New Operation (unattended)** (lines 2628-2631):
```
Error: Cannot start new operation with a paused operation pending (unattended mode)
Return: error code 1
```

### 12.3 Competing Sync Operations

**Different Sync Pending (unattended)** (lines 2646-2649):
```
Error: Cannot start parity check with another sync operation pending (unattended mode)
Return: error code 1
```

### 12.4 Module/Stat Interface Errors

**Missing /proc/nmdstat** (lines 2576-2578):
```
check_nmdstat_exists returns error
handle_check returns error code 1
```

**Command Execution Failure** (lines 2676-2679):
```
IF run_nmd_command fails:
  Error: Failed to start {operation}
  Return: error code 1
```

---

## 13. Implementation Details

### 13.1 Key Data Structures

**NMDSTAT_VALUES** - Associative array (line 43)
```bash
declare -g -A NMDSTAT_VALUES
```
Stores all `/proc/nmdstat` key-value pairs

**RESYNC_STATUS_DATA** - Associative array (line 48)
```bash
declare -g -A RESYNC_STATUS_DATA
```
Stores computed/formatted parity operation data:
- active, paused, pending (boolean strings)
- action (operation type)
- progress_percent
- position_kb, position_gb
- size_kb, size_gb
- rate_kb_s
- elapsed_seconds
- eta_seconds
- friendly_action

**ARRAY_STATUS_DATA** - Associative array (line 46)
```bash
declare -g -A ARRAY_STATUS_DATA
```
Stores health status and parity disk info:
- health_status
- health_code
- has_parity, has_second_parity
- sbsyncerrs, sbsyncexit
- last_sync_human, last_sync_status
- last_sync_timestamp, last_sync_ago

### 13.2 Global Configuration

**Module Communication** (lines 50-51):
```bash
PROC_NMDSTAT="${PROC_NMDSTAT:-/proc/nmdstat}"
```
Allows overriding nmdstat path for testing

**Superblock Path** (lines 28-29):
```bash
DEFAULT_SUPERBLOCK="/nonraid.dat"
SUPERBLOCK_PATH=""
```

**Parity-related Settings** (lines 34-40):
```bash
VERBOSE=0          # Verbose output flag
UNATTENDED=0       # Unattended mode flag
MONITOR_INTERVAL=2 # Status refresh interval (seconds)
```

### 13.3 Function Call Chain

For a typical parity check operation:

1. **User Command:** `nmdctl check CORRECT`
2. **Handler:** `handle_check()` (line 4802)
3. **Validation:** `check_root()`, `check_module_loaded()`, `check_nmdstat_exists()`
4. **State Check:** `get_all_nmdstat_values()` reads kernel state
5. **Logic:** Implements conflict detection, prompts, etc.
6. **Execute:** `run_nmd_command("check CORRECT")`
7. **Kernel:** Processes command, updates `/proc/nmdstat`
8. **Feedback:** Print status message to user

---

## 14. Summary: Parity Operation Workflow

### Typical Parity Check Workflow:

```
1. Start array: nmdctl start
   └─ Kernel detects new array, initiates parity build

2. Monitor progress: nmdctl status -m
   └─ Shows progress, speed, ETA

3. If issues arise, pause: nmdctl check PAUSE
   └─ Suspends operation at current position

4. Fix issues (replace disk, etc.)

5. Resume: nmdctl check RESUME
   └─ Continues from saved position

6. Monitor completion: nmdctl status
   └─ Shows final results, errors (if any)

7. Check health: nmdctl status -v
   └─ Confirms parity sync completed successfully
```

### Disk Replacement Workflow:

```
1. Stop array: nmdctl stop

2. Replace disk: nmdctl add SLOT:/dev/dev:ID
   └─ Kernel marks slot for reconstruction

3. Start array: nmdctl start
   └─ Kernel initiates data rebuild (recon D# operation)

4. Monitor: nmdctl status -m
   └─ Watch reconstruction progress

5. Verify: nmdctl status
   └─ Confirm rebuild complete, all disks healthy
```

### Unattended Mode Workflow:

```
1. Configure sync limits: nmdctl set md_sync_limit 5

2. Run check in unattended mode: nmdctl -u check
   └─ Uses NOCORRECT mode by default
   └─ Errors block operation (no prompts)

3. Automate monitoring: while true; do
     nmdctl -u status -o json | check_progress
     sleep 60
   done
```

---

## References

**File:** `/home/admin/proxmaid-v2/nonraid/tools/nmdctl`

**Key Functions:**
- `handle_check()` - lines 2572-2683
- `collect_resync_status()` - lines 686-769
- `collect_array_health()` - lines 472-626
- `collect_array_size_and_parity()` - lines 629-683
- `format_resync_action()` - lines 1919-1958
- `format_human_resync_status()` - lines 936-984
- `add_disk()` - lines 3314-3909
- `set_array_setting()` - lines 4485-4691
- `run_nmd_command()` - lines 117-124
- `get_all_nmdstat_values()` - lines 214-233

**Related Documentation:**
- Kernel module interface: `/proc/nmdcmd`, `/proc/nmdstat`
- Array state management: STARTED, STOPPED, NEW_ARRAY, ERROR states
- Disk slot mapping: 0=Primary Parity (P), 1-28=Data, 29=Secondary Parity (Q)
