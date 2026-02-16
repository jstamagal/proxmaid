# Kernel Module and RAID6 Subsystem Research

## Executive Summary

This document provides comprehensive technical documentation of the nonRAID kernel module architecture, its integration with the Linux md (multiple devices) subsystem, and the embedded RAID6 parity calculation engine. The nonRAID personality implements a novel RAID variant designed for unRAID-compatible operation on Linux.

## 1. Module Purpose and Design Philosophy

### 1.1 Overview

The `md-nonraid` kernel module is a Linux md subsystem personality that implements the **nonRAID/unRAID** array personality. It enables Linux systems to mount and manage disk arrays created by Unraid, providing full read/write support with dual-parity protection using P and Q disks.

**Key Design Goals:**
- Compatibility with Unraid array formats and superblock layout
- Dual-parity fault tolerance (can survive loss of any 2 disks)
- Efficient stripe-based I/O processing
- Support for dynamic array reconfiguration (disk replacement, expansion)
- Integrated parity calculation using optimized RAID6 algorithms

### 1.2 Module Architecture

```
┌─────────────────────────────────────────────────┐
│         Linux Block Device Layer                │
├─────────────────────────────────────────────────┤
│              nonRAID md Personality             │
│  ┌──────────────────────────────────────────┐  │
│  │  md_unraid.c (md.c adapted for nonRAID) │  │
│  │  - Superblock handling                   │  │
│  │  - Array state management                │  │
│  │  - Thread management                     │  │
│  │  - Proc/sysfs interface                  │  │
│  └──────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────┐  │
│  │  unraid.c (Core stripe engine)           │  │
│  │  - Stripe cache management               │  │
│  │  - I/O path processing                   │  │
│  │  - Parity calculation & recovery         │  │
│  │  - Sync operations                       │  │
│  └──────────────────────────────────────────┘  │
├─────────────────────────────────────────────────┤
│           Embedded RAID6 Library                │
│  ┌──────────────────────────────────────────┐  │
│  │  algos.c - Algorithm selection           │  │
│  │  recov.c - Recovery algorithms           │  │
│  │  tables.c - Galois field lookup tables   │  │
│  │  int/sse2/avx2/avx512 - Optimized impls │  │
│  └──────────────────────────────────────────┘  │
└─────────────────────────────────────────────────┘
```

### 1.3 Kernel Version Support

The module is built for three kernel version ranges via DKMS:

| Kernel Version | Source Files | Status |
|---|---|---|
| >= 6.9 | `6.12/md_unraid.c`, `6.12/unraid.c` | Current |
| 6.5-6.8 | `6.6/md_unraid.c`, `6.6/unraid.c` | Stable |
| < 6.5 | `6.1/md_unraid.c`, `6.1/unraid.c` | Legacy |

The build system automatically selects the appropriate version based on kernel version at compile time (see `md_nonraid/Makefile`).

## 2. The Nonraid/Unraid Personality

### 2.1 What Makes nonRAID Different?

Unlike standard RAID levels (RAID0, RAID5, RAID6), nonRAID is designed specifically for the Unraid use case:

**Standard RAID6:**
- Stripes data across all disks in fixed patterns
- Full capacity = (N-2) * disk_size where N = number of disks
- Equal role for all disks

**nonRAID/Unraid:**
- Data and parity distributed asymmetrically across disk slots
- P disk (slot 0): Always stores P parity
- Q disk (slot 29): Always stores Q parity
- Data disks (slots 1-28): Store data blocks
- Supports up to 28 data disks + 2 parity disks
- Each disk can have different size (limited by smallest parity disk)
- Can lose ANY 2 disks and still recover (as long as at least one parity disk survives)

### 2.2 Array Architecture

**Superblock Layout (4096 bytes):**
```c
typedef struct mdp_superblock_s {
    // Common info (32 4-byte words)
    __u32 md_magic;           // 0xb92b4efc
    __u32 major_version;      // 2
    __u32 minor_version;      // 9
    __u32 patch_version;      // 35
    __u32 sb_csum;            // Checksum of entire superblock
    __u32 ctime;              // Creation time
    __u32 utime;              // Last update time
    __u32 events;             // Update counter
    __u32 md_minor;           // Preferred md device number
    __u32 state;              // Array state flags
    __u32 num_disks;          // Number of active disk slots (N)
    __u32 stime;              // Last sync start time
    __u32 sync_errs;          // Sync errors during rebuild
    __u32 stime2;             // Last sync end time
    __u32 sync_exit;          // Sync exit code
    __u8  label[32];          // Human-readable label
    // ... reserved fields

    // Disk descriptors (30 x 128 bytes each)
    mdp_disk_t disks[MD_SB_DISKS];
    // disks[0] = P parity disk descriptor
    // disks[1..28] = Data disk descriptors
    // disks[29] = Q parity disk descriptor
} mdp_super_t;

typedef struct mdp_device_descriptor_s {
    __u32 major, minor;       // Device numbers (unused)
    __u32 number;             // Slot number
    __u32 state;              // Disk operational state bits
    __u64 size;               // Size in 1024-byte blocks
    __u8  id[MD_ID_SIZE];     // Udev-style ID string (model_serial)
    __u32 reserved[6];
} mdp_disk_t;
```

**Disk State Bits:**
```c
#define MD_DISK_VALID       0  // Disk is valid/correct
#define MD_DISK_ENABLED     1  // Disk is enabled in array
#define MD_DISK_ACTIVE      2  // Disk is actively in use
```

Disk states tracked in driver:
- `DISK_OK`: Enabled, present, correct
- `DISK_NP_MISSING`: Enabled but missing from system
- `DISK_INVALID`: Present but not valid (needs rebuild)
- `DISK_WRONG`: Present but wrong disk for this slot
- `DISK_DSBL`: Disabled, old disk present
- `DISK_NEW`: New uninitialized disk

### 2.3 Virtual Device Exposure

When an array is started, the driver creates virtual block devices:
- Major device: 127 (MAJOR_NR)
- Minor device: 1-28 (data disk slots)
- Device name: `nmd1p1`, `nmd2p1`, ..., `nmd28p1`
- Each device has capacity = disk_size in 1024-byte blocks

The parity disks (P and Q) are NOT exposed as block devices.

## 3. Data Layout and Block Addressing

### 3.1 Stripe Definition

A **stripe** is the fundamental unit of parity protection:

```
Stripe size = PAGE_SIZE (4096 bytes) = BUFFER_SIZE
Stripe = 8 consecutive 512-byte sectors (BUFFER_SECT)
Stripe offset = First sector address AND ~(BUFFER_SECT-1)

Stripe numbering: sector_address / BUFFER_SECT = stripe_number
```

### 3.2 Column Arrangement in Stripe

For an array with N active data disks (num_disks = N+2 for P and Q):

```c
// In superblock (mdp_disk_t array):
disks[0]   = P (parity) descriptor
disks[1]   = Data disk 1 descriptor
...
disks[N]   = Data disk N descriptor
disks[29]  = Q (parity) descriptor

// In stripe cache (column_t array, arranged for algorithm):
col[0]    = Data disk 1
col[1]    = Data disk 2
...
col[N-1]  = Data disk N
col[N]    = P parity
col[N+1]  = Q parity

// Mapping:
int idx = (i < pd_idx) ? i+1 : ((i == pd_idx) ? 0 : MD_SB_DISKS-1)
```

**Example: 5-disk array (3 data + P + Q)**
```
Stripe layout in col[] array:
col[0] = data from disks[1]  (Data D1)
col[1] = data from disks[2]  (Data D2)
col[2] = data from disks[3]  (Data D3)
col[3] = parity from disks[0] (Parity P)
col[4] = parity from disks[29] (Parity Q)

pd_idx = 3 (P index)
qd_idx = 4 (Q index)
disks = 5 (num_disks = num_data + 2)
```

### 3.3 Stripe Sector Layout on Physical Disks

For a given stripe (starting sector S):
```
Data Disk i layout:
  Physical sector: disk_offset + stripe_sector
  Logical stripe: S to S + BUFFER_SECT-1

P Disk layout:
  Physical sector: disk_offset + stripe_sector
  Contains: XOR of all data blocks in this stripe

Q Disk layout:
  Physical sector: disk_offset + stripe_sector
  Contains: Reed-Solomon Q syndrome for this stripe
```

Where `disk_offset` is the configured partition offset in sectors.

### 3.4 Handling Different Disk Sizes

The array can support disks of different sizes with constraints:

```c
// Size validation:
smallest_parity_size = min(P_size, Q_size)
largest_data_size = max(all_data_disk_sizes)

// Valid configuration requires:
smallest_parity_size >= largest_data_size

// This ensures:
// - All data can be stored
// - All disks can store their stripe data
// - No disk is undersized
```

## 4. I/O Path: Read and Write Operations

### 4.1 Request Handling Flow

```
Block device write/read request (sector, size, flags)
    ↓
md_submit_bio() [md_unraid.c:931]
    ↓
    - Verify disk is active
    - Split to page boundaries
    - Forward to unraid_make_request()
    ↓
unraid_make_request() [unraid.c:1714]
    ↓
    - Loop through each stripe affected by request
    - Get or create active stripe
    - Attach bio to stripe
    - Mark STRIPE_HANDLE bit
    - Release stripe reference
    ↓
unraidd thread (one per disk slot)
    ↓
handle_stripe() [unraid.c:1054]
    ↓
    - Analyze stripe state
    - Schedule reads for missing data
    - Compute parities if needed
    - Schedule writes
    - Return completed requests
```

### 4.2 Stripe Cache Management

The stripe cache is the core data structure:

```c
struct stripe_head {
    struct hlist_node hash;        // Hash table entry
    struct list_head lru;          // Inactive or handle list

    sector_t sector;               // Stripe start sector
    atomic_t count;                // Reference counter
    unsigned long state;           // STRIPE_* flags

    void *srcs[MD_SB_DISKS];       // Buffer pointers (for parity calc)
    column_t col[0];               // Per-column data (variable length)
};

struct unraid_conf {
    struct hlist_head *stripe_hashtbl;  // Stripe lookup table

    int disks;                     // Number of columns (data+P+Q)
    mdp_disk_t *disk[MD_SB_DISKS];
    mdk_rdev_t *rdev[MD_SB_DISKS];

    struct list_head handle_list[MD_SB_DISKS-1];  // Per-thread work
    struct list_head inactive_list;               // Idle stripes

    atomic_t active_stripes[MD_SB_DISKS-1];  // Per-thread stripe count

    spinlock_t device_lock;        // Protects lists and counts
};
```

### 4.3 Read Path Details

**Case 1: Simple Read (All disks valid)**

```
1. Check if stripe data is already cached
2. If in cache:
   - Copy data to user bio from cache buffer
   - Complete request
3. If not in cache:
   - Schedule read of data disk
   - When complete, copy to user buffer
   - Complete request
```

**Case 2: Read with Failures (1-2 disks failed)**

```
1. Detect failed disk (invalid state)
2. Schedule reads of all valid disks
3. When reads complete:
   - Reconstruct failed disk data:
     * 1 failure: XOR or Q recover
     * 2 failures: RAID6 recovery
   - Copy requested data to user buffer
   - Complete request
4. Optionally schedule write of reconstructed data
```

**Case 3: Read-Ahead Restriction**

```
if (req->flags & REQ_RAHEAD && md_restrict & 2):
    - Return error (not enough stripe cache)
    - Client falls back to regular read
```

### 4.4 Write Path Details

The write method is selectable: `READ_MODIFY_WRITE` (RMW) or `RECONSTRUCT_WRITE`.

**RMW Strategy (default for most disks):**
```
For write to data disk D in stripe S:

1. Read current values of all columns
2. XOR out old D value, XOR in new D value
3. Recalculate P and Q incrementally:
   P_new = P_old XOR D_old XOR D_new
   Q_new = (Q_old XOR syndrome(D_old)) XOR syndrome(D_new)
4. Write D, P, Q back to disk
```

**RECONSTRUCT_WRITE Strategy (full stripe writes):**
```
For write to one or more data disks:

1. Read all other data disks
2. Copy new data into place
3. Calculate P = XOR of all data
4. Calculate Q = RS syndrome of all data
5. Write all modified columns to disk
```

The driver automatically selects RMW for small writes, reconstruct for full stripes.

### 4.5 Flush/Barrier Handling

```c
// For writes with REQ_PREFLUSH:
1. Issue flush commands to:
   - Target data disk
   - P disk (if valid)
   - Q disk (if valid)
2. Wait for all flushes to complete
3. Then process the actual data write
4. Issue another flush after write if REQ_FUA
```

## 5. Parity Calculation and RAID6 Integration

### 5.1 RAID6 Concepts

The nonRAID module implements **RAID6** parity, which uses two independent parity blocks:

**P Parity (XOR-based):**
```
P = D1 XOR D2 XOR D3 ... XOR Dn

Properties:
- Can recover any single failed disk from P and other data
- Compute: XOR all data together
- Same as RAID5 parity
```

**Q Parity (Reed-Solomon based):**
```
Q = (D1⊗g^0) XOR (D2⊗g^1) XOR (D3⊗g^2) ... (Dn⊗g^(n-1))

Where:
- ⊗ is Galois Field multiplication
- g is a primitive generator (0x02 in GF(2^8))
- g^n are successive powers in GF(256)

Properties:
- Can recover any single failed disk if P is good
- Can recover two failed disks using both P and Q
- Used for RAID6 recovery algorithms
```

### 5.2 Embedded RAID6 Library

The module includes a complete RAID6 parity library in `raid6/`:

**Components:**

```
raid6/
├── algos.c         - Algorithm selection and dispatcher
├── recov.c         - Recovery algorithms (2-data, data+P)
├── tables.c        - Galois field lookup tables (generated)
├── int1-8.o        - Pure-C implementations
├── sse2x1-4.o      - SSE2 SIMD versions
├── avx2x1-4.o      - AVX2 SIMD versions
├── avx512x1-4.o    - AVX-512 SIMD versions
├── recov_*.o       - Optimized recovery for each arch
└── nonraid_pq.h    - Symbol mappings
```

**Algorithm Selection:**

The module automatically selects the fastest available algorithm at module load time:

```c
raid6_select_algo() {
    for each available algorithm {
        if algorithm.valid():
            nonraid_gen_syndrome = algorithm.gen_syndrome
            nonraid_xor_syndrome = algorithm.xor_syndrome
            nonraid_2data_recov = recovery.data2
            nonraid_datap_recov = recovery.datap
            break
    }
}
```

Priority order (from fast to slow):
1. AVX-512 (if CPU supports it)
2. AVX2 (if CPU supports it)
3. SSE2 (if CPU supports it)
4. Pure C integer implementation (fallback, always available)

### 5.3 Parity Computation Functions

**Generate P and Q Together:**
```c
raid6_gen_syndrome(int disks, size_t bytes, void **ptrs)
// Compute both P and Q from data blocks
// ptrs[0..disks-3] = data blocks
// ptrs[disks-2] = P output
// ptrs[disks-1] = Q output
// Optimized path in stripe: raid6_generate_pq()
```

**Syndrome XOR (Incremental update):**
```c
raid6_xor_syndrome(int disks, int datefail, int faila, size_t bytes, void **ptrs)
// XOR a single data block into P and Q
// Used during read-modify-write:
//   - First XOR to subtract old data
//   - Then XOR again to add new data
// Optimized for RMW path: rmw6_write_data()
```

**Recover Two Data Blocks:**
```c
raid6_2data_recov(int disks, size_t bytes, int faila, int failb, void **ptrs)
// Recover two failed data disks from P and Q
// Uses matrix inversion in Galois Field
// Generates temp P/Q from remaining data
// Solves linear system for failed blocks
// Optimized path in stripe: raid6_generate_dd()
```

**Recover Data+P:**
```c
raid6_datap_recov(int disks, size_t bytes, int faila, void **ptrs)
// Recover data disk when P is missing
// Uses only Q and other data blocks
// Simpler than 2-data recovery
// Optimized path in stripe: raid6_generate_dp()
```

**Recover Single Disk:**
```c
raid5_generate_d(struct stripe_head *sh, int dd_idx)
// Single-disk recovery (XOR all others)
// P = D1 XOR D2 XOR ... Dn
// Can recover any one disk using all others
// Works for data, P, or Q individually
```

### 5.4 Parity Check Operation

During array check/sync, parity is verified:

```c
check_parity(struct stripe_head *sh, int fail_idx, int recover) {
    if (fail_idx == Q_idx) {
        // Check P only (no failed disks)
        correct = check_parity5(sh);
        if (!correct && recover)
            raid5_generate_p(sh);
    } else {
        // General case: Check both P and Q
        // Generate P/Q in temp buffers
        p_temp = conf->p_scribble;
        q_temp = conf->q_scribble;
        raid6_gen_syndrome(disks, BUFFER_SIZE, ptrs);

        // Compare with on-disk values
        if (P != P_temp) {
            if (recover) memcpy(P, P_temp);
        }
        if (Q != Q_temp) {
            if (recover) memcpy(Q, Q_temp);
        }
    }
}
```

Errors are reported to `/proc/nmdstat` and can be corrected if `recovery_option` is set.

## 6. Kernel Thread Architecture

### 6.1 Thread Organization

The module spawns N kernel threads where N = num_data_disks + 1:

```
Thread 0: nmdrecoveryd
  - Parity check/reconstruction
  - Full stripe generation
  - Array check operations

Thread 1: nonraidd1
  - Handles I/O for data disk 1
  - Process stripes with changes to disk 1

Thread 2: nonraidd2
  - Handles I/O for data disk 2

...

Thread N: nonraidd N
  - Handles I/O for data disk N
```

Each thread has:
```c
mdk_thread_t {
    void (*run)(mddev_t *mddev, unsigned long arg);
    mddev_t *mddev;
    wait_queue_head_t wqueue;      // Wait for work
    struct task_struct *tsk;        // Linux task
    atomic_t flags;                 // THREAD_WAKEUP bit
};
```

### 6.2 Work Distribution

Each thread processes its per-unit handle_list:

```c
void unraidd(mddev_t *mddev, unsigned long unit) {
    while (!list_empty(conf->handle_list[unit])) {
        // Get first stripe from this unit's handle list
        sh = list_entry(handle_list[unit].next, stripe_head, lru);

        // Call main stripe handler
        handle_stripe(sh);

        // Release and requeue if needed
        release_stripe(sh);
    }
}
```

**Stripe Queuing:**

Stripes are queued to threads based on which disk they're destined for:
```c
// In _release_stripe():
if (STRIPE_HANDLE bit is set) {
    list_add_tail(&sh->lru, &conf->handle_list[sh->unit]);
    md_wakeup_thread(conf->thread[sh->unit]);
} else {
    list_add_tail(&sh->lru, &conf->inactive_list);
}
```

### 6.3 Synchronization

**device_lock (spinlock):**
- Protects: stripe hash table, active/inactive lists, stripe counters
- Held during stripe acquisition/release
- Also held when checking for config changes

**Recovery thread (nmdrecoveryd):**
- Serialized by recovery_sem
- Only one check/resync can run at a time
- Marked with STRIPE_SYNCING bit to coordinate with normal I/O

## 7. Synchronization and Locking

### 7.1 Lock Hierarchy

```
device_lock (spinlock, irqsave)
  └─ stripe lock (implicit via atomic count)
      └─ I/O completion (doesn't require locks)
```

**device_lock scope:**
- Stripe hash table lookups
- Active/inactive list modifications
- Stripe counter updates
- Queue limit checks

**Stripe-level locking:**
- Stripe head reference counting (atomic)
- Per-stripe I/O state tracking
- Implicit: if count > 0, stripe cannot be freed or reused

### 7.2 Stripe Life Cycle

```
1. CREATION:
   - Allocate in stripe cache or get from inactive_list
   - Initialize sector, columns, state
   - Insert in hash table
   - Increment reference count

2. ACTIVE:
   - Incoming I/O requests attached to stripe
   - Reference count tracks all holders
   - In hash table (findable)

3. HANDLING:
   - Thread removes from list, increments count
   - Calls handle_stripe()
   - On completion, decrements count

4. COMPLETION:
   - When count reaches 0:
     If STRIPE_HANDLE bit set: move to handle_list
     Else: move to inactive_list

5. CLEANUP:
   - Eventually removed from inactive_list
   - Cache buffers freed
   - Memory returned to pool
```

### 7.3 Write Operation Serialization

For RMW writes:

```
1. stripe_lock (implicit via atomic_inc)
   - Read old data
   - Determine RMW vs reconstruct

2. device_lock (briefly)
   - Update counters

3. I/O operations (no lock held)
   - Perform computations
   - Schedule disk I/O

4. Release (device_lock required)
   - Decrement counters
   - Check if stripe complete
```

## 8. Sysfs/Proc Interface

### 8.1 Proc Commands

The `/proc/nmdcmd` file is used for array management:

```bash
# Import a disk slot
echo "import 0 sda1 0 1000000 0 WD10EZEX_12345" > /proc/nmdcmd

# Start the array
echo "start STOPPED" > /proc/nmdcmd

# Stop the array
echo "stop" > /proc/nmdcmd

# Check/rebuild array
echo "check CORRECT" > /proc/nmdcmd

# Pause rebuild
echo "nocheck PAUSE" > /proc/nmdcmd

# Get array status
cat /proc/nmdstat
```

Command format: `command [subcommand] [arg1] [arg2] ...`

### 8.2 Proc Status

The `/proc/nmdstat` file outputs array status in `key=value` format:

**Superblock Info:**
```
sbName=/path/to/superblock
sbVersion=2.9.35
sbCreated=1234567890
sbUpdated=1234567999
sbEvents=42
sbState=0
sbNumDisks=5
sbLabel=MyArray
sbSynced=1234567800
sbSyncErrs=0
sbSyncExit=0
```

**Array Status:**
```
mdVersion=2.9.35
mdState=STARTED
mdNumDisks=5
mdNumDisabled=0
mdNumInvalid=0
mdNumNew=0
mdResyncAction=check P Q
mdResyncSize=1000000
mdResync=0
mdResyncPos=0
```

**Per-Disk Info (for each slot 0-29):**
```
diskNumber.1=1
diskName.1=nmd1p1
diskSize.1=1000000
diskState.1=7
diskId.1=WD10EZEX_12345

rdevNumber.1=1
rdevStatus.1=DISK_OK
rdevName.1=sda1
rdevSize.1=1000000
rdevReads.1=12345
rdevWrites.1=6789
rdevNumErrors.1=0
```

### 8.3 Module Parameters

Tunable parameters in `/proc/sys/kernel/` (set via module params):

```
md_num_stripes      - Number of stripe cache slots (default: 1280)
md_queue_limit      - I/O queue depth % (1-100, default: 80)
md_sync_limit       - Sync/rebuild queue depth % (1-100, default: 5)
md_write_method     - 0=RMW, 1=Reconstruct (default: 0)
md_restrict         - I/O restrictions (default: 1)
                      bit 0: Limit sector count per request
                      bit 1: Fail read-ahead when stripe cache full
```

## 9. Error Handling and Degraded Mode

### 9.1 Error Detection

**Read Errors:**
```c
void end_request(struct bio *bi) {
    if (bio_data_dir(bi) == READ) {
        if (uptodate) {
            set_buff_uptodate(col);
        } else {
            md_read_error(mddev, disk_number, sector);
            mark_disk_invalid(col);  // Mark disk as bad
        }
    }
}
```

**Write Errors:**
```c
int md_write_error(mddev_t *mddev, int disk_number, sector_t sector) {
    if (disk_enabled(disk) && num_disabled < 2) {
        if (!disk_valid(disk)) {
            // Replacement disk failed - disable again
            mark_disk_disabled(disk);
        } else if (num_invalid < 2) {
            // First failure - mark invalid
            mark_disk_invalid(disk);
            mark_disk_disabled(disk);
        }
    }
    return 1;  // Need superblock update
}
```

### 9.2 Handling Disk Failures

**1 Disk Failed (Valid Parity):**
```
- Read from valid disks
- Reconstruct failed disk from P or Q
- Can continue normal operations
```

**2 Disks Failed (Both P and Q Valid):**
```
- Read from valid disks
- Use RAID6 recovery to reconstruct both
- Can continue normal operations
```

**> 2 Disks Failed:**
```
- Any read to failed disk: return error
- Array is unrecoverable
- No writes allowed
```

**P Disk Failed (Q valid):**
```
- Q still provides single protection
- Can reconstruct P if needed
- Array continues degraded
```

### 9.3 Degraded Mode I/O

In `handle_stripe()`, the failure count is checked:

```c
if (failed > 2) {
    // Too many failures
    // Fail any pending I/O requests
    if (col->read_bi) {
        col->read_bi->bi_status = BLK_STS_IOERR;
        return_bi = col->read_bi;
    }
} else if (failed == 2) {
    // Use RAID6 recovery
    raid6_generate_dd(sh, faila, failb);
} else if (failed == 1) {
    // Use RAID5 or Q recovery
    raid5_generate_d(sh, faila);
}
```

## 10. Key Data Structures

### 10.1 Column Structure

Per-disk data within a stripe:

```c
typedef struct column_s {
    unsigned long state;              // Disk state bits + buffer flags

    struct bio *read_bi;              // Pending read request
    struct bio *write_bi;             // Pending write request
    struct bio *written_bi;           // Write awaiting completion

    struct bio bio;                   // Bio for disk I/O
    struct bio_vec vec;               // Single page vector
    struct page *page;                // Page buffer for this column
} column_t;
```

**State Flags:**
```c
#define MD_BUFF_UPTODATE  8   // Buffer has valid data
#define MD_BUFF_LOCKED    9   // Buffer scheduled for I/O
#define MD_BUFF_READ      10  // Scheduled for read
#define MD_BUFF_WRITE     11  // Scheduled for write
#define MD_UPDATE_SB      12  // Config change, need superblock write
```

### 10.2 Stripe Head Structure

```c
struct stripe_head {
    struct hlist_node hash;           // Hash table for fast lookup
    struct list_head lru;             // List: inactive or handle

    sector_t sector;                  // Starting sector of stripe
    unsigned long state;              // Stripe flags
    atomic_t count;                   // Reference count

    int unit;                         // Which disk this goes to
    int write_method;                 // RMW or RECONSTRUCT

    void *srcs[MD_SB_DISKS];          // Pointers for parity calc
    column_t col[0];                  // Flex array of columns
};
```

**Stripe State Flags:**
```c
#define STRIPE_HANDLE     0           // Needs processing
#define STRIPE_SYNCING    1           // Parity check in progress
#define STRIPE_CLEARING   2           // Clearing new disks
#define STRIPE_INSYNC     3           // Sync write complete
```

### 10.3 Rdev Structure (Device)

```c
typedef struct mdk_rdev_s {
    struct block_device *bdev;        // Open block device

    unsigned long offset;             // Partition offset (sectors)
    unsigned long long size;          // Disk size (1024-byte blocks)
    unsigned char id[MD_ID_SIZE];     // Disk ID string
    unsigned char name[BDEVNAME_SIZE];// Device name
    int erased;                       // Factory-erased flag

    unsigned long reads;              // Read counter
    unsigned long writes;             // Write counter
    unsigned long errors;             // Error counter

    unsigned long simulate_rderror;   // Test: force read error
    unsigned long simulate_wrerror;   // Test: force write error
} mdk_rdev_t;
```

### 10.4 Configuration Structure

```c
typedef struct unraid_conf {
    struct hlist_head *stripe_hashtbl;    // Stripe hash table

    mddev_t *mddev;                       // Parent mddev
    int disks;                            // Num columns (data+P+Q)
    mdp_disk_t *disk[MD_SB_DISKS];        // Superblock disk descriptors
    mdk_rdev_t *rdev[MD_SB_DISKS];        // Runtime device state

    void *p_scribble;                     // Temp P buffer for checks
    void *q_scribble;                     // Temp Q buffer for checks

    struct kmem_cache *slab_cache;        // Stripe object cache
    int num_stripes;                      // Total allocated stripes

    mdk_thread_t *thread[MD_SB_DISKS-1];  // Worker threads
    struct list_head handle_list[MD_SB_DISKS-1];  // Work per thread

    struct list_head inactive_list;       // Idle stripes
    wait_queue_head_t wait_for_stripe;    // Waiters for free stripe

    atomic_t active_flushes;              // Pending flush operations
    atomic_t active_stripes[MD_SB_DISKS-1];  // Per-thread stripe count

    spinlock_t device_lock;               // Protects all above
} unraid_conf_t;
```

## 11. Kernel Version Differences (6.1 vs 6.6 vs 6.12)

### 11.1 API Changes

The three versions track kernel API changes for:

**Block Device Operations:**
- 6.1: Uses older bio handling, different struct layouts
- 6.6: Updated bio APIs, better integration with modern kernel
- 6.12: Latest APIs (e.g., `bio_split_to_limits`, `file_bdev`, `bdev_file_open_by_path`)

**Thread Management:**
- 6.1: Older kthread APIs
- 6.6: Updated kthread parking mechanism
- 6.12: Latest kthread infrastructure

**Proc FS:**
- All versions support `/proc/nmdcmd` and `/proc/nmdstat`
- Proc ops structure may differ in field names

### 11.2 Automatic Version Selection

The Makefile automatically selects the right version:

```makefile
K_MAJOR := $(shell echo $(KVERSION) | cut -f1 -d.)
K_MINOR := $(shell echo $(KVERSION) | cut -f2 -d.)
K_GT_6_8 := $(shell [ $(K_MAJOR) -gt 6 -o \( $(K_MAJOR) -eq 6 -a $(K_MINOR) -gt 8 \) ] && echo true)
K_GE_6_5 := $(shell [ $(K_MAJOR) -gt 6 -o \( $(K_MAJOR) -eq 6 -a $(K_MINOR) -ge 5 \) ] && echo true)

ifeq ($(K_GT_6_8),true)
    md-nonraid-m += 6.12/md_unraid.o 6.12/unraid.o
else ifeq ($(K_GE_6_5),true)
    md-nonraid-m += 6.6/md_unraid.o 6.6/unraid.o
else
    md-nonraid-m += 6.1/md_unraid.o 6.1/unraid.o
endif
```

### 11.3 Key API Differences

**File Opening (6.1 vs 6.6+ vs 6.12):**

6.1:
```c
struct file *fp = filp_open(filename, O_RDONLY, 0);
```

6.6:
```c
struct file *fp = filp_open(filename, O_RDONLY, 0);
```

6.12:
```c
struct file *bdev_file = bdev_file_open_by_path(path, FMODE_READ|FMODE_WRITE, NULL, NULL);
struct block_device *bdev = file_bdev(bdev_file);
```

**Block Device Operations:**

6.1/6.6: Uses direct `submit_bio()` and request handling

6.12: Modernized to use latest block layer APIs

**Memory Allocation:**

All versions: `kzalloc()`, `kmem_cache_alloc()` (unchanged)

## 12. Configuration and Module Loading

### 12.1 DKMS Configuration

File: `/home/admin/proxmaid-v2/nonraid/dkms.conf`

```
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

This results in two modules:
1. `md-nonraid.ko` - Main personality module
2. `nonraid6_pq.ko` - RAID6 library module

### 12.2 Module Loading

```bash
# Load dependencies
modprobe md_mod

# Load RAID6 library
insmod raid6/nonraid6_pq.ko

# Load main module
insmod md_nonraid/md-nonraid.ko super=/path/to/superblock.dat
```

**Module Parameters:**

```bash
# Super block file path (no default, must be specified)
super=/path/to/superblock.dat

# Number of stripe cache entries (default: 1280)
md_num_stripes=1280

# I/O queue limit as % (1-100, default: 80)
md_queue_limit=80

# Sync queue limit as % (1-100, default: 5)
md_sync_limit=5

# Write method: 0=RMW, 1=Reconstruct (default: 0)
md_write_method=0

# Restrictions: bit0=sector limit, bit1=read-ahead fail (default: 1)
md_restrict=1
```

## 13. Performance Considerations

### 13.1 Stripe Cache Sizing

The stripe cache is the core resource:

```
Memory per stripe = sizeof(stripe_head) + N*sizeof(column_t) + N*PAGE_SIZE
                  = 200 bytes + 100*20 bytes + 100*4096 bytes
                  = ~410KB per stripe (for large N)

Default cache = 1280 stripes = ~512 MB

Tuning:
- More stripes = higher throughput, higher memory
- Fewer stripes = lower latency, less memory
- md_queue_limit controls per-unit stripe allocation
```

### 13.2 Write Method Selection

**RMW (Read-Modify-Write):**
```
Pros:
- Better for small random writes
- Reads less data from disk
- Lower latency per write

Cons:
- Requires read of old data
- More I/Os
- Susceptible to read failures

Used when: (active_streams == 1)
```

**RECONSTRUCT:**
```
Pros:
- Better for sequential writes
- Can compute from existing data
- More parallelizable

Cons:
- Reads more data
- Higher latency
- More complex computation

Used when: (active_streams > 1)
```

### 13.3 Algorithm Selection

At module load, the fastest RAID6 algorithm is selected:

```
Test each algorithm:
1. AVX-512 (4 stripes/pass) - Intel/AMD recent
2. AVX2 (4 stripes/pass) - Intel/AMD Sandy Bridge+
3. SSE2 (4 stripes/pass) - Intel/AMD P4+
4. Integer C (1 stripe/pass) - Universal fallback
```

Speed improvement (typical): AVX2 ~3-5x faster than integer C

## 14. Summary: Key Takeaways

1. **Architecture**: nonRAID is a specialized RAID6 variant for Unraid compatibility on Linux
2. **Stripe-based**: All I/O organized around fixed 4KB stripes with per-stripe parity
3. **Dual parity**: P (XOR) and Q (RS) provide protection against 2 concurrent disk failures
4. **Stripe cache**: In-memory stripe buffers manage all I/O and parity operations
5. **Kernel threads**: Per-disk worker threads process I/O and parity computations
6. **Optimized**: RAID6 algorithms auto-selected for CPU (AVX512/AVX2/SSE2 with fallback)
7. **Version flexible**: Automatically builds for kernel 6.1, 6.6, or 6.12+ APIs
8. **Tunable**: Queue limits, stripe count, write method configurable at runtime
9. **Synchronized**: Careful locking ensures correctness under concurrent I/O
10. **Fault-tolerant**: Graceful degradation with 1-2 disk failures, recoverable parity

