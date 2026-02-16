# nmdctl Helper Functions and Utilities - Comprehensive Research

This document provides detailed analysis of nmdctl's internal helper functions, utilities, status output formatting, and test suite. It documents the infrastructure that enables nmdctl to function as the primary NonRAID array management utility.

---

## 1. Global State Management

### Global Variables and Arrays

**Location:** Lines 16-51

The script uses global bash variables and associative arrays to maintain state:

```bash
VERSION=1.22.0

# Color codes for terminal output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Monitor mode status bars
HEADER_BG='\033[44;37m'   # Blue background, white text
FOOTER_BG='\033[100;97m'  # Dark gray background, bright white text

# Default superblock path
DEFAULT_SUPERBLOCK="/nonraid.dat"
SUPERBLOCK_PATH=""

# LUKS keyfile path
LUKS_KEYFILE="/etc/nonraid/luks-keyfile"

# Operational flags
VERBOSE=0              # Verbose output enabled
UNATTENDED=0          # Unattended mode (no prompts)
NO_FS=0               # Skip filesystem information collection
MONITOR_MODE=0        # Monitor mode enabled
MONITOR_INTERVAL=2    # Default monitor refresh interval in seconds

# Global associative arrays for caching nmdstat values
declare -g -A NMDSTAT_VALUES          # Raw values from /proc/nmdstat
declare -g -A ARRAY_STATUS_DATA       # Processed array status information
declare -g -A DISK_STATUS_DATA        # Per-disk status and metrics
declare -g -A RESYNC_STATUS_DATA      # Resync operation status

# Driver stats interface (overridable for testing)
PROC_NMDSTAT="${PROC_NMDSTAT:-/proc/nmdstat}"
```

### Design Pattern: Lazy-Loaded Caches

The NMDSTAT_VALUES, ARRAY_STATUS_DATA, DISK_STATUS_DATA, and RESYNC_STATUS_DATA arrays use a lazy-loading pattern:
- Data is populated on-demand via collection functions
- The MONITOR_MODE uses cached values to reduce filesystem I/O
- Arrays can be safely cleared between operations

---

## 2. Core Helper Functions

### 2.1 Permission and Module Management

#### `check_root()` (Line 109)
**Purpose:** Verify script is running as root
**Returns:** Exits with code 1 if not root
**Implementation:** Checks `$EUID` against 0
**Error Message:** "Error: This command must be run as root"

#### `run_nmd_command()` (Line 117)
**Purpose:** Execute commands via /proc/nmdcmd interface
**Parameters:**
- `$1`: Command string to execute
**Returns:** 0 on success, 1 on failure
**Implementation:**
```bash
if ! echo -n "$1" > /proc/nmdcmd; then
    echo -e "${RED}Error: Failed to run command '$1'${NC}"
    return 1
fi
```

#### `check_module_loaded()` (Line 127)
**Purpose:** Verify nonraid module is loaded and using correct superblock
**Behavior:**
- Returns 0 if module is loaded with correct superblock
- Attempts to load module if not loaded and superblock exists
- Handles create_array special case (allows non-existent superblock)
- Provides detailed error messages about superblock mismatches

**Key Logic:**
- Detects create_array context via call stack inspection: `[[ "$call_stack" == *"create_array"* ]]`
- For create_array: allows non-existent superblocks, indicates new file will be created
- For other commands: requires existing superblock file
- Attempts `modprobe nonraid super="$superblock"` to load module

#### `check_nmdstat_exists()` (Line 189)
**Purpose:** Verify /proc/nmdstat is available
**Behavior:**
- Clears NMDSTAT_VALUES array if file doesn't exist
- Attempts to load module if not loaded
- Returns 1 if still unavailable after attempts
- Provides helpful error message

---

### 2.2 Data Retrieval Functions

#### `get_all_nmdstat_values()` (Line 214)
**Purpose:** Read all values from /proc/nmdstat into associative array
**Parameters:**
- `$1`: Name of array variable to populate (passed by reference)
**Implementation:**
```bash
local -n array_ref=$1
# Clear existing data
for key in "${!array_ref[@]}"; do
    unset 'array_ref["$key"]'
done
# Read key=value pairs from /proc/nmdstat
while IFS='=' read -r key value; do
    array_ref["$key"]="$value"
done < <(cat "$PROC_NMDSTAT" 2>/dev/null)
# Add timestamp for rate calculations
array_ref["timestamp"]=$(date +%s)
```
**Returns:** 0 if data read successfully, 1 if empty

#### `get_nmdstat_value()` (Line 235)
**Purpose:** Get single value from /proc/nmdstat (backward compatibility)
**Parameters:**
- `$1`: Key name to lookup
**Implementation:** `grep -E "^$1=" "$PROC_NMDSTAT" | cut -d= -f2`
**Note:** Less efficient than get_all_nmdstat_values; use only for single lookups

#### `get_defined_slots_count()` (Line 406)
**Purpose:** Count array slots that have disks defined
**Implementation:**
- Iterates through NMDSTAT_VALUES
- Matches keys: `diskSize.X` where X is 0-29
- Counts only slots with diskSize > 0
- Returns space-separated list of slot numbers

#### `get_defined_slots()` (Line 425)
**Purpose:** Get list of defined slot numbers
**Returns:** Space-separated sorted list of slot indices (e.g., "0 1 2 29")
**Implementation:**
```bash
for key in "${!NMDSTAT_VALUES[@]}"; do
    if [[ $key =~ ^diskSize\.([0-9]+)$ ]] && [ "${NMDSTAT_VALUES[$key]}" -gt 0 ]; then
        slots+=("${BASH_REMATCH[1]}")
    fi
done
printf "%s\n" "${slots[@]}" | sort -n | tr '\n' ' '
```

---

## 3. Status Data Collection Functions

These functions populate the global status arrays by processing raw nmdstat values. They're called before output formatting.

### 3.1 Array Summary Data

#### `collect_array_summary()` (Line 463)
**Purpose:** Extract basic array identification data
**Data Populated:**
- `mdstate`: Current array state (STARTED, STOPPED, NEW_ARRAY, ERROR, etc.)
- `sblabel`: User-assigned array label (empty string if not set)
- `sbname`: Superblock file path
- `mdnumdisks`: Total number of disk slots defined
- `total_slots`: Count of defined slots

### 3.2 Array Health Analysis

#### `collect_array_health()` (Line 472)
**Purpose:** Comprehensive health status determination
**Data Populated:**
- `health_status`: Overall health classification (HEALTHY, WARNING, DEGRADED, OFFLINE, ERROR, NEW, NEW_DISK, READY, PARTIAL)
- `health_code`: Numeric code (0=healthy, 1=warning/degraded, 2=offline/error)
- `health_details`: Human-readable explanation of health status
- `last_sync_human`: Time-formatted last sync completion
- `last_sync_status`: Status category (completed, in_progress, errors, never)
- `last_sync_timestamp`: Unix timestamp of last sync completion
- `last_sync_ago`: Seconds since last sync
- `last_sync_elapsed`: Duration of last sync operation
- Disk error counters and status summaries

**Health Status Decision Logic:**
1. **ERROR** (code 2): Array state begins with "ERROR:"
2. **NEW** (code 1): State is NEW_ARRAY
3. **NEW_DISK** (code 1): New disks added (mdNumNew > 0)
4. **OFFLINE** (code 2): No disks imported
5. **PARTIAL** (code 1): Not all disks imported while array is stopped
6. **DEGRADED** (code 1): Missing/invalid/wrong/disabled/replaced disks OR sync errors OR sync exit code > 0
7. **WARNING** (code 1): All disks present but some have I/O errors
8. **READY** (code 1): All disks imported but array stopped
9. **HEALTHY** (code 0): No issues detected

**Key Variables Examined:**
- `mdNumMissing`, `mdNumInvalid`, `mdNumWrong`, `mdNumDisabled`, `mdNumReplaced`, `mdNumNew`
- `sbSynced`, `sbSynced2`, `sbSyncErrs`, `sbSyncExit`
- Per-slot `rdevNumErrors` counters

### 3.3 Array Size and Parity Information

#### `collect_array_size_and_parity()` (Line 629)
**Purpose:** Determine array capacity and parity configuration
**Data Populated:**
- `has_parity`: Boolean (true/false)
- `parity_size_kb`, `parity_size_gb`: Size of P disk (slot 0)
- `has_second_parity`: Boolean (true/false)
- `second_parity_size_kb`, `second_parity_size_gb`: Size of Q disk (slot 29)
- `data_disk_count`: Number of data disks (slots 1-28)
- `data_size_kb`, `data_size_gb`: Capacity (minimum data disk size)

**Parity Configuration:**
- **Single Parity (P):** Only slot 0 has disk
- **Dual Parity (P+Q):** Both slots 0 and 29 have disks
- **No Parity:** Neither slot has disk

**Capacity Calculation:**
- Data capacity is limited by smallest data disk
- Formula: `min(diskSize.1..28) * count(data_disks)`

### 3.4 Resync Operation Status

#### `collect_resync_status()` (Line 686)
**Purpose:** Track active, paused, and pending resync operations
**Data Populated:**
- `active`: Boolean (true if resync operation running)
- `paused`: Boolean (true if operation paused)
- `pending`: Boolean (true if operation scheduled but not started)
- `action`: Raw action string (check P, recon P, check Q, recon Q, etc.)
- `friendly_action`: Human-readable action description
- `progress_percent`: Completion percentage (0-100)
- `position_kb`, `position_gb`: Current progress position
- `size_kb`, `size_gb`: Total operation size
- `rate_kb_s`: Current operation rate
- `elapsed_seconds`: Time elapsed since start
- `eta_seconds`: Estimated seconds remaining

**Operation State Detection:**
1. **Paused:** `mdResync=0` AND `mdResyncPos!=0` (position saved but not running)
2. **Pending:** `mdResync=0` AND action is "clear" or starts with "recon"
3. **Active:** `mdResync!=0` (operation running)

**Rate Calculation:**
- Formula: `mdResyncDb / mdResyncDt` (blocks per delta-time)
- Driver uses 8-sector blocks (4KB each)

**ETA Calculation:**
- Formula: `(mdResyncSize - mdResyncPos) / rate_kb_s`
- Only computed if rate > 0 and position < size

### 3.5 Per-Disk Status

#### `collect_disk_status()` (Line 772)
**Purpose:** Gather comprehensive per-disk information
**Data Populated Per Slot (0-29):**
- `slot_X_present`: Boolean (true if slot defined)
- `slot_X_type`: Parity type (P, Q, or data)
- `slot_X_size_kb`, `slot_X_size_gb`: Disk capacity
- `slot_X_device`: Device name (e.g., nmd1p1, or "none")
- `slot_X_status`: Disk status (DISK_OK, DISK_INVALID, DISK_NEW, etc.)
- `slot_X_errors`: I/O error count
- `slot_X_disk_id`: Unique disk identifier
- `slot_X_disk_name`: Human-assigned disk name
- `slot_X_reads`: Total read operations (4KB blocks)
- `slot_X_writes`: Total write operations (4KB blocks)

**Filesystem Data (if array STARTED):**
- `slot_X_fs_type`: Filesystem type (ext4, btrfs, etc.) or "unknown"
- `slot_X_mountpoint`: Mount path or "unmounted"
- `slot_X_usage`: Formatted usage (e.g., "15.2 GB / 100 GB")

**Optimization:**
- In MONITOR_MODE, filesystem information is cached (line 841-843)
- This avoids repeated stat() calls which are expensive
- FS data refreshed only on first run or when NO_FS=0

**Slot Processing Rules:**
- Slot 0 (P): Always displayed
- Slot 29 (Q): Displayed if diskId set OR status is DISK_*_NEW
- Slots 1-28 (data): Displayed if diskName set OR status is DISK_NEW

---

## 4. Output Formatting Functions

### 4.1 Color Formatting Helpers

#### `format_array_state()` (Line 268)
**Purpose:** Apply color codes to array state strings
**Outputs:**
- STARTED → `${GREEN}STARTED${NC}`
- STOPPED → `${YELLOW}STOPPED${NC}`
- NEW_ARRAY → `${BLUE}NEW_ARRAY${NC}`
- RECON_DISK, DISABLE_DISK, SWAP_DSBL → Yellow
- ERROR:* → `${RED}ERROR...${NC}`
- Other → Uncolored

#### `format_disk_status()` (Line 240)
**Purpose:** Apply color codes to disk status strings
**Outputs:**
- DISK_OK → Green OK
- DISK_INVALID → Red INVALID
- DISK_NP_MISSING → Yellow MISSING
- DISK_WRONG → Red WRONG
- DISK_DSBL, DISK_NP_DSBL → Yellow DISABLED
- DISK_NEW, DISK_DSBL_NEW → Blue NEW
- Other → Yellow (unknown status)

#### `format_health_status()` (Line 326)
**Purpose:** Apply color codes to health status
**Outputs:**
- ERROR → Red
- NEW, NEW_DISK → Blue
- OFFLINE → Red
- PARTIAL, DEGRADED, WARNING, READY → Yellow
- HEALTHY → Green

#### `format_time_duration()` (Line 444)
**Purpose:** Convert seconds to human-readable time spans
**Examples:**
- 45 → "45 sec"
- 150 → "2 minutes, 30 seconds"
- 7200 → "2 hours, 00 minutes"
- 90000 → "1 days, 1 hours"
**Logic:**
```bash
if [ "$days" -gt 0 ]; then format as days, hours
elif [ "$hours" -gt 0 ]; then format as hours, minutes
elif [ "$minutes" -gt 0 ]; then format as minutes, seconds
else format as seconds
```

---

### 4.2 Size Formatting

#### `format_kbytes()` (Line 1657)
**Purpose:** Format kilobytes as human-readable storage sizes
**Parameters:**
- `$1`: Size in kilobytes
- `$2`: Add unit suffix (default: 1)
- `$3`: Decimal places (default: 1)
- `$4`: Force specific unit (kb, mb, gb, tb, b)

**Intelligent Decimal Display:**
- Small remainders (< 10% of unit) display as integers
- Significant remainders display with requested decimals
- Example: 1.09 MB shows as "1 MB", but 1.5 MB shows decimals

**Unit Selection (auto):**
- Bytes (B) if < 1024 bytes
- Kilobytes (kB) if < 1024 KB
- Megabytes (MB) if < 1024 MB
- Gigabytes (GB) if < 1024 GB
- Terabytes (TB) otherwise

**Examples:**
```bash
format_kbytes 1048576 0 0 "gb"    # → "1"
format_kbytes 1536 1 1            # → "1.5 MB"
format_kbytes 1026 1 1            # → "1 MB" (small remainder)
format_kbytes 1048576 1 2         # → "1 GB" (exact match, no decimals)
```

#### `format_io_rate()` (Line 1632)
**Purpose:** Format disk I/O rates in human-readable speed units
**Parameters:**
- `$1`: Rate in KB/s
- `$2`: Colorize output (default: 1)
**Output:** Formatted size/s with color (if active)
**Returns:** "-" if rate is 0

#### `format_io_count()` (Line 1648)
**Purpose:** Convert 4KB block counts to human-readable sizes
**Implementation:**
- Multiplies count by 4 (blocks are 8-sector = 4KB units)
- Calls format_kbytes for human output

---

### 4.3 Disk Entry Formatting

#### `format_disk_entry_line()` (Line 1093)
**Purpose:** Format a single disk line in the status table
**Parameters:** Uses global DISK_STATUS_DATA and VERBOSE flag
**Output:** Single table row with proper alignment
**Columns:**
1. Slot (P/Q/number)
2. Status (colored)
3. Device (diskId in verbose mode)
4. Size (formatted)
5. DiskName (if verbose)
6. Filesystem Type (if started)
7. Mount Point (if started)
8. Usage (if mounted)
9. Read Rate/Count (if started)
10. Write Rate/Count (if started)

**Width Calculation:**
- Dynamically calculates column widths based on content
- Uses `get_visible_length()` to account for ANSI color codes
- Handles colored output with proper padding

#### `get_visible_length()` (Line 1413)
**Purpose:** Calculate visible string length excluding ANSI escape sequences
**Implementation:**
```bash
shopt -s extglob
str="${str//$'\e'\[*([0-9;])m/}"  # Remove all ANSI codes
echo "${#str}"
```
**Used By:** Column width calculations to align colored output

#### `format_disk_entry_data()` (Line 1434)
**Purpose:** Format disk entry as pipe-separated data string
**Returns:** Pipe-delimited string with all disk information
**Used By:** Table formatting to handle column layout
**Format:** `idx|slot_display|status|device|size|diskname|fs|mountpoint|usage|read|write`

---

### 4.4 Output Format Functions

#### `format_human_output()` (Line 864)
**Purpose:** Generate human-readable array status report
**Output Structure:**
1. Array State (with color)
2. Array Label (if set)
3. Superblock path (with validation)
4. Disks Present count
5. Driver inconsistency warning (if applicable)
6. Array Health status
7. Last check/sync information
8. Resync operation status (if active)
9. Disk inventory table

**Key Features:**
- Detects and warns about driver inconsistent state
- Shows resync progress and ETA if operation active
- Includes detailed verbose information if `-v` flag used

#### `format_terse_output()` (Line 1401)
**Purpose:** Single-line status summary
**Output:** "NonRAID Array State: STARTED, Health: HEALTHY"
**Use Case:** Scripting and monitoring integration

#### `format_prometheus_output()` (Line 1138)
**Purpose:** Generate Prometheus metrics format
**Metrics Exported:**
- `nonraid_array_state` - State code (0=STOPPED, 1=STARTED, 2=NEW, 3=ERROR, 4=UNKNOWN)
- `nonraid_array_health` - Health code (0=HEALTHY, 1=WARNING, 2=OFFLINE)
- `nonraid_disks_present` - Total disk count
- `nonraid_disks_imported` - Imported disk count
- `nonraid_disks_unassigned` - Unassigned disk count
- `nonraid_array_size_gb` - Data capacity in GB
- `nonraid_data_disks_count` - Number of data disks
- `nonraid_parity_disks_count` - Parity disk count (0/1/2)
- `nonraid_disk_errors_total` - Total I/O error count
- `nonraid_nummissing_count` - Missing disk count
- `nonraid_numinvalid_count` - Invalid disk count
- `nonraid_numwrong_count` - Wrong disk count
- `nonraid_numdisabled_count` - Disabled disk count
- `nonraid_numreplaced_count` - Replaced disk count
- `nonraid_numnew_count` - New disk count
- `nonraid_last_sync_timestamp` - Unix timestamp
- `nonraid_last_sync_age_seconds` - Age of last sync
- `nonraid_last_sync_elapsed_seconds` - Duration of last sync
- `nonraid_resync_active` - Boolean (0/1)
- `nonraid_resync_progress_percent` - Progress percentage
- `nonraid_resync_rate_mb_per_sec` - Current rate (if active)

**Label:** All metrics tagged with `label="$sblabel"`

#### `format_json_output()` (Line 1305)
**Purpose:** Generate JSON-formatted status
**Top-level Fields:**
- `timestamp`: ISO8601 timestamp
- `code`: Exit code (0=healthy, 1=warning, 2=error)
- `state`: Array state string
- `status`: Health status string
- `label`: Array label
- `superblock`: Superblock path
- `array`: Object with array-specific data
- `disks`: Array of disk objects
- `resync`: Resync operation status (if active)
- `messages`: Array of warning/info messages

**Disk Object Fields:**
- `slot`: Slot number
- `type`: P/Q/data
- `device`: Device name
- `size_gb`: Capacity
- `status`: Disk health status
- `errors`: Error count
- `diskid`: Unique identifier
- `filesystem`: FS type
- `mountpoint`: Mount path
- `usage_percent`: Disk usage percentage

---

## 5. Monitor Mode Functions

Monitor mode provides real-time array status with keyboard controls.

#### `setup_monitor_terminal()` (Line 1515)
**Purpose:** Initialize terminal for TUI mode
**Terminal Escapes:**
- `\e[?1049h` - Use alternative screen buffer
- `\e[?7l` - Disable line wrapping
- `\e[?25l` - Hide cursor
- `\e[2J` - Clear screen
**Side Effects:** Disables input echo with `stty -echo`

#### `reset_monitor_terminal()` (Line 1527)
**Purpose:** Restore terminal to normal state
**Terminal Escapes:**
- `\e[?7h` - Re-enable line wrapping
- `\e[?25h` - Show cursor
- `\e[2J` - Clear screen
- `\e[?1049l` - Restore main screen buffer
**Side Effects:** Re-enables input echo with `stty echo`

#### `update_monitor_line()` (Line 1539)
**Purpose:** Update terminal line with proper clearing
**Escapes:** `\r` (start of line) + `\e[K` (clear to end)

#### `update_status_bar()` (Line 1546)
**Purpose:** Create full-width colored status bar
**Parameters:**
- `$1`: Text content
- `$2`: Background color code
**Implementation:**
- Calculates visible text length excluding ANSI codes
- Pads with spaces to fill terminal width
- Applies background color to padding

#### `calculate_disk_io_rates()` (Line 1564)
**Purpose:** Calculate I/O rates from counter deltas
**Algorithm:**
1. First loop: Store initial counters, set rates to 0
2. Subsequent loops:
   - Calculate delta since last reading
   - Divide by time difference
   - Handle counter wraparound (reset delta to 0 if negative)
   - Formula: `(reads_delta * 4) / time_diff` for KB/s

**Notes:**
- Driver blocks are 8-sector = 4KB units
- Stores previous counters for next iteration
- Caches timestamp of last measurement

#### `monitor_status_loop()` (Line 1749)
**Purpose:** Main monitor mode event loop
**Controls:**
- 'q': Quit monitor
- 'r': Refresh immediately
- '?': Show help

**Behavior:**
- Calls `redraw_monitor_status()` every MONITOR_INTERVAL seconds
- Processes keyboard input with timeout
- Updates NMDSTAT_VALUES and recalculates metrics on each refresh

#### `redraw_monitor_status()` (Line 1713)
**Purpose:** Redraw entire monitor display
**Output:**
- Header bar with timestamp
- Refresh interval indicator
- Full status output (from format_human_output)
- Footer bar with help text
- Blank lines from ESC[J (clear to end of screen)

#### `read_key_with_timeout()` (Line 1737)
**Purpose:** Non-blocking keyboard input
**Parameters:** `$1` - Timeout in seconds (default: 0.1)
**Implementation:** `read -rt "$timeout" -n 1 -s key`
**Returns:** 0 if key pressed, 1 if timeout

---

## 6. Validation and Error Handling

### 6.1 Device Validation

#### `validate_device_path()` (Line 2686)
**Purpose:** Verify device path is valid and safe
**Checks:**
- Path exists and is a block device
- Path does not start with "/" if not force flag
- Path syntax is valid
**Parameters:** Device path, optional force flag
**Returns:** 0 if valid, 1 if invalid

#### `list_available_devices()` (Line 2725)
**Purpose:** Enumerate available block devices
**Returns:** Array of device info strings
**Format:** `device|partition|size_kb|size_gb|disk_id`

#### `format_available_devices()` (Line 292)
**Purpose:** Pretty-print device listing
**Output Format:**
```
Available devices:

  #   Device      Size(GB)  ID
  --  ----------  --------  ---------------------------------
  1   sdb1        1000      DISK_ID_1234567890ABCDEF
  2   sdc1        2000      DISK_ID_FEDCBA0987654321
```

**Features:**
- Truncates long disk IDs with "..."
- Shows error message for missing disk IDs
- Aligns columns for readability

---

### 6.2 Filesystem Detection

#### `get_fs_type()` (Line 1960)
**Purpose:** Detect filesystem type on device
**Parameters:** Device path
**Implementation:** Uses blkid or fs detection
**Returns:** Filesystem type (ext4, btrfs, xfs, etc.) or "unknown"

#### `get_mountpoint()` (Line 1998)
**Purpose:** Find mount point for device
**Parameters:**
- Device name (e.g., nmd1p1)
- Filesystem type
**Implementation:**
- Checks /etc/fstab for static mounts
- Checks /proc/mounts for active mounts
- Handles LUKS-encrypted mounts
**Returns:** Mount path or "unmounted"

#### `get_fs_usage()` (Line 2071)
**Purpose:** Get filesystem usage statistics
**Parameters:**
- Mount point
- Filesystem type
**Returns:** Formatted string like "15.2 GB / 100 GB (15%)"

#### `get_disk_size_kb()` (Line 2121)
**Purpose:** Get disk capacity in kilobytes
**Parameters:** Device path
**Implementation:** Uses sfdisk or blockdev
**Returns:** Size in KB

---

### 6.3 Disk Discovery

#### `find_partition()` (Line 2141)
**Purpose:** Locate partition for disk
**Parameters:** Device path
**Returns:** Full partition path

#### `find_matching_disk()` (Line 2167)
**Purpose:** Find disk by identifier or path pattern
**Parameters:** Disk identifier or path
**Returns:** Matched disk device

---

## 7. Check/Parity Operation Support

#### `check_driver_inconsistent_state()` (Line 348)
**Purpose:** Detect driver state inconsistencies
**Problem Scenario:** mdNum* counters show non-zero values but all disk statuses are DISK_OK
**Returns:** 0 if inconsistent, 1 if consistent
**Conditions Checked:**
- Array must be STARTED
- At least one mdNum* counter > 0
- All disks must have status DISK_OK
**When Occurs:** Often after initial array creation before normal operation

#### `format_resync_action()` (Line 1919)
**Purpose:** Convert raw action code to human-readable description
**Inputs:** Action string (e.g., "check P", "recon P Q")
**Outputs:** Human description (e.g., "Parity-Check P", "Parity-Sync P+Q")

---

## 8. Test Suite (`test_nmdctl_basic.bats`)

### 8.1 Test Infrastructure

**Setup (Lines 4-18):**
```bash
setup() {
    export PATH="$BATS_TEST_DIRNAME/..:$PATH"
    source "$BATS_TEST_DIRNAME/../nmdctl"

    # Mock out external dependencies
    eval 'check_root() { return 0; }'
    eval 'check_module_loaded() { return 0; }'
    eval 'run_nmd_command() { return 1; }'
    eval 'check_nmdstat_exists() { return 0; }'
    eval 'get_nmdstat_value() { echo "STOPPED"; }'
    eval 'validate_device_path() { return 0; }'
    eval 'get_disk_size_kb() { echo "1000000"; }'
}
```

**Mock nmdstat Creation (Lines 26-78):**
```bash
create_mock_nmdstat() {
    local state=${1:-STOPPED}
    local missing=${2:-0}
    local invalid=${3:-0}
    local resync=${4:-0}
    local resync_action=${5:-check P}
    # ... generates /proc/nmdstat format
}
```

### 8.2 Helper Function Tests

#### Version and Help (Lines 80-96)
- Verify version string format
- Verify help output completeness

#### Parameter Validation (Lines 98-113)
- Test missing parameters
- Test invalid parameter types
- Test out-of-range values

#### `format_kbytes` Tests (Lines 115-194)
**Coverage:**
- Basic conversions (KB, MB, GB, TB)
- Decimal place handling (0, 1, 2)
- Smart decimal display (small remainders as integers)
- Forced unit output
- Edge cases (exact 1.0 values)
- Rounding behavior

#### `format_time_duration` Tests (Lines 196-212)
**Coverage:**
- Seconds only
- Minutes + seconds
- Hours + minutes
- Days + hours

#### `get_visible_length` Tests (Lines 214-225)
**Coverage:**
- Strings with color codes
- Multiple color codes in one string
- Proper length calculation excluding ANSI codes

---

### 8.3 Status Parsing Tests

#### State Detection (Lines 228-265)
- HEALTHY state (STARTED, no issues)
- STOPPED state
- DEGRADED state (missing disk)
- Invalid disks detection

#### Array Calculations (Lines 266-276)
- Array size calculation
- Data disk count
- Parity detection

#### Operation Status (Lines 291-335)
- Parity check in progress (progress percentage)
- Parity sync in progress
- Errors found during checks
- Last sync tracking

#### Disk Error Tracking (Lines 337-353)
- Per-disk error count
- Total error warning

#### Parity Detection (Lines 355-373)
- Single parity (P)
- Dual parity (P+Q)
- Q disk detection

---

### 8.4 Output Format Tests

#### Default Format (Lines 399-425)
- Verify human-readable output structure
- Explicit format specification (`-o default`)

#### Prometheus Format (Lines 427-484)
- Verify metric names and format
- Health status metric values
- Degraded state metrics
- Tag labels

#### JSON Format (Lines 442-497)
- Valid JSON output validation with `jq`
- Required fields
- Degraded state encoding

#### Invalid Format (Lines 461-471)
- Error handling for unknown formats

---

### 8.5 Data Collection Tests

#### Integration Test (Lines 499-550)
**Validates:**
- `get_all_nmdstat_values` populates array
- `collect_array_summary` extracts basics
- `collect_array_health` determines health
- `collect_array_size_and_parity` calculates capacity
- `collect_resync_status` tracks operations
- `collect_disk_status` gathers per-disk data

**Assertions:**
- ARRAY_STATUS_DATA populated correctly
- RESYNC_STATUS_DATA initialized
- DISK_STATUS_DATA has per-slot data

---

### 8.6 Array Creation Tests

#### Parameter Parsing (Lines 562-610)
- P notation (P=slot0)
- Numeric notation (0=slot0)
- Q notation (Q=slot29)
- Invalid format detection
- Duplicate slot detection

#### Creation Flow (Lines 649-668)
- Full creation workflow
- Array layout validation
- Disk import success
- Missing device detection (without force flag)

---

### 8.7 Test Patterns

**Mock Substitution:**
```bash
eval 'function_name() { ... }'  # Override for testing
```

**Mock File Creation:**
```bash
create_mock_nmdstat > "$BATS_TMPDIR/mock_nmdstat_$testname"
export PROC_NMDSTAT="$BATS_TMPDIR/mock_nmdstat_$testname"
```

**ANSI Code Stripping:**
```bash
clean_output=$(echo "$output" | sed 's/\x1b\[[0-9;]*m//g')
```

**Status Testing:**
```bash
run command_being_tested
[ "$status" -eq 0 ]  # Exit code
[[ "$output" =~ pattern ]]  # Output matching
```

---

## 9. Exit Codes and Status Conventions

### Status Return Codes

| Code | Meaning | Examples |
|------|---------|----------|
| 0 | Success / HEALTHY | Array healthy, operation completed |
| 1 | Warning / Recoverable issue | DEGRADED, PARTIAL, READY states, validation errors |
| 2 | Error / Critical issue | OFFLINE, ERROR states, command failure |

### Health Code Mapping

```bash
0 = HEALTHY              # All systems operational
1 = WARNING/DEGRADED     # Issue detected, but array functional
2 = OFFLINE/ERROR        # Critical state, array non-functional
```

---

## 10. Configuration and State Files

### Runtime State

**Superblock File:**
- Path: Specified via `-s`/`--super` flag (default: `/nonraid.dat`)
- Role: Persistent array metadata storage
- Module Parameter: `modprobe nonraid super="/path/to.dat"`

**LUKS Keyfile:**
- Path: `/etc/nonraid/luks-keyfile` (default, overridable with `-k`)
- Role: Encryption key for mounted array disks
- Usage: Automatic mounting of encrypted partitions

**Proc Interface:**
- `/proc/nmdstat` - Read-only array status interface
- `/proc/nmdcmd` - Write-only command interface
- Format: Key=value pairs, one per line

### Module Loading

**Conditions:**
- Loads automatically if superblock file exists and module not loaded
- Uses `modprobe nonraid super="$SUPERBLOCK_PATH"`
- Can be explicitly reloaded with `nmdctl reload`

---

## 11. Terminal/TUI Capabilities

### ANSI Escape Sequences Used

**Cursor Control:**
- `\e[H` - Move to home position (top-left)
- `\e[J` - Clear from cursor to end of screen
- `\r` - Carriage return (start of line)

**Screen Buffers:**
- `\e[?1049h` - Switch to alternative screen buffer
- `\e[?1049l` - Switch to primary screen buffer

**Display Attributes:**
- `\e[2J` - Clear entire screen
- `\e[?7l` - Disable line wrapping
- `\e[?7h` - Enable line wrapping
- `\e[?25l` - Hide cursor
- `\e[?25h` - Show cursor
- `\e[K` - Clear line from cursor right

**Input Control:**
- `stty -echo` / `stty echo` - Toggle input echo
- `read -rt TIMEOUT` - Non-blocking input read

### Color Palette

```bash
RED='\033[0;31m'     # Errors, critical issues
GREEN='\033[0;32m'   # Success, healthy status
YELLOW='\033[0;33m'  # Warnings, suspended states
BLUE='\033[0;34m'    # New states, informational
NC='\033[0m'         # No color (reset)

HEADER_BG='\033[44;37m'   # Status bar header (blue bg, white text)
FOOTER_BG='\033[100;97m'  # Footer bar (dark gray bg, bright white text)
```

---

## 12. Key Design Patterns

### Lazy-Loading Pattern
Status data is calculated only when needed via collection functions. This reduces unnecessary I/O and allows efficient caching in monitor mode.

### Color Code Abstraction
All color codes stored as global variables. Setting `RED=""` etc. disables colors via `--no-color` flag.

### Associative Array Data Transfer
Functions use bash name references (`local -n array_ref=$1`) to populate caller's arrays, avoiding subshell overhead and enabling data persistence.

### Monitor Mode Optimization
- Filesystem data cached when in MONITOR_MODE
- Disk I/O rates calculated from counter deltas
- Terminal redrawn efficiently using escape sequences

### Slot-Based Architecture
All disk operations use 0-29 slot model:
- Slot 0: P (primary parity)
- Slots 1-28: Data disks
- Slot 29: Q (secondary parity)

---

## 13. Error Handling Strategy

### Validation Levels

1. **Input Validation:** Parameter type and range checks
2. **Precondition Checks:** Module loaded, superblock exists, permissions
3. **Operation Validation:** Device exists, size reasonable, no conflicts
4. **Execution Validation:** Command succeeded via /proc/nmdcmd

### Error Messaging

**Format:** `${RED}Error: <message>${NC}`
**Provides:** Clear problem description + remediation hint when applicable
**Example:**
```bash
Error: nonraid module is not loaded
Superblock file not found: $superblock
To create an array with this new superblock, run: nmdctl --super $superblock create
```

### Silent Failures

Some operations fail gracefully:
- `stty` commands suppressed with `2>/dev/null || true`
- Alternative screen buffer cleanup ignored if already reset
- Mock function failures in tests expected

---

## 14. Performance Considerations

### Array Size Limits

- Max 30 disk slots (0-29)
- Single parity: max 29 data disks
- Dual parity: max 28 data disks

### Time Complexity

| Operation | Complexity | Notes |
|-----------|-----------|-------|
| get_all_nmdstat_values | O(1) | Single file read |
| collect_disk_status | O(30) | Fixed slot count |
| format_human_output | O(30) | Iterates all slots |
| monitor_status_loop | O(30) | Per refresh interval |

### Caching Strategy

**MONITOR_MODE caching (line 841-843):**
```bash
if [ "$MONITOR_MODE" -eq 1 ] && [ -n "${DISK_STATUS_DATA[slot_${idx}_fs_type]}" ]; then
    # Use cached value, skip expensive stat()
else
    # Perform fresh filesystem detection
fi
```

---

## Summary

nmdctl's helper infrastructure provides:

1. **Data Access Layer:** Reading and caching /proc/nmdstat values
2. **Processing Layer:** Interpreting raw values into health status, size calculations, operation states
3. **Presentation Layer:** Multiple output formats (human, JSON, Prometheus, terse) with optional color coding
4. **UI Layer:** TUI monitor mode with real-time updates and keyboard controls
5. **Validation Layer:** Comprehensive input and precondition checking
6. **Error Layer:** Clear error messages with remediation guidance

The design emphasizes efficiency (caching, lazy-loading), clarity (color-coded output, detailed validation), and flexibility (multiple output formats, overridable mock points for testing).

