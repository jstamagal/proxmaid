# Proxmaid v2 — Testing Guide

## Overview

The test suite covers the Go API backend. Tests use Go's standard `testing` package with `httptest` for HTTP endpoint testing. All tests run in **mock mode** — no real hardware, kernel modules, or system commands are needed.

## Running Tests

```bash
# Run all tests
cd /home/admin/proxmaid-v2/api && go test ./... -v

# Run tests for a specific package
cd /home/admin/proxmaid-v2/api && go test ./internal/array/ -v
cd /home/admin/proxmaid-v2/api && go test ./internal/cache/ -v
cd /home/admin/proxmaid-v2/api && go test ./internal/api/ -v
cd /home/admin/proxmaid-v2/api && go test ./internal/disk/ -v
cd /home/admin/proxmaid-v2/api && go test ./internal/system/ -v
```

## Mock Mode Architecture

Mock mode is the foundation of the test system. It allows full testing without hardware:

```
NewManager() → checks /proc/nmdstat exists?
    ├── YES → MockMode = false (real system calls)
    └── NO  → MockMode = true  (simulated data)
```

Each manager checks `MockMode` before every operation:
- **SystemManager**: Returns hardcoded nmdstat output, prints `[MOCK] nmdctl ...` instead of running commands
- **ArrayManager**: Delegates to SystemManager (inherits mock behavior)
- **DiskManager**: Returns simulated disk inventory and SMART data
- **CacheManager**: Creates mock pools ("nvme-fast", "ssd-warm"), simulates mover runs

## Test Files

### `internal/array/manager_test.go` — 6 tests

| Test | What it verifies |
|------|------------------|
| `TestParseNmdstat_Started` | Parses a full 4-disk array: state, disk count, roles (parity/data), device names, disk IDs, I/O stats, sizes, synced state |
| `TestParseNmdstat_Stopped` | Parses minimal stopped-state output: state=STOPPED, 0 disks |
| `TestParseNmdstat_Degraded` | Parses degraded array with 1 disabled disk (DISK_DSBL), verifies numInvalid and disabled disk detection |
| `TestParseNmdstat_ResyncActive` | Parses active resync: mdResync=1, calculates ~25% progress from pos/size |
| `TestParseNmdstat_EmptyInput` | Handles completely empty string without error |
| `TestParseNmdstat_MalformedLines` | Handles lines without `=` separator gracefully |

**Key test patterns**:
- Tests use inline nmdstat strings (not files) for clarity
- Each test constructs a raw string matching real `/proc/nmdstat` format
- Parser is tested in isolation via the unexported `parseNmdstat()` function (possible because tests are in the same package)

### `internal/cache/manager_test.go` — 14 tests

| Test | What it verifies |
|------|------------------|
| `TestNewManagerMockMode` | Manager initializes in mock mode with default mover config |
| `TestMockPoolsLoaded` | Mock mode auto-creates "nvme-fast" and "ssd-warm" pools |
| `TestGetPools` | Returns all pools as a slice |
| `TestGetPoolByName` | Retrieves a specific pool by name |
| `TestGetPoolNotFound` | Returns error for nonexistent pool |
| `TestCreatePool` | Creates a new pool with name, devices, fs_type, mount point |
| `TestCreatePoolDuplicate` | Rejects pool creation with existing name |
| `TestCreatePoolNoDevices` | Rejects pool creation with empty device list |
| `TestCreatePoolNoName` | Rejects pool creation with empty name |
| `TestDeletePool` | Deletes an existing pool |
| `TestDeletePoolNotFound` | Returns error for nonexistent pool deletion |
| `TestMoverStatus` | Returns mover status with running state and config |
| `TestUpdateMoverConfig` | Updates schedule and age_threshold |
| `TestRunMover` | Triggers mover run, verifies running state |

### `internal/api/router_test.go` — 7 tests

| Test | What it verifies |
|------|------------------|
| `TestHealthEndpoint` | `GET /api/health` returns 200 with `ok: true` |
| `TestArrayStatusEndpoint` | `GET /api/array/status` returns 200 with array data |
| `TestArrayStartEndpoint` | `POST /api/array/start` returns 200 |
| `TestArrayStopEndpoint` | Start then stop returns 200 |
| `TestArrayDoubleStartReturnsError` | Double start returns 400 (idempotency guard) |
| `TestModuleStatusEndpoint` | `GET /api/system/module` returns 200 |
| `TestCorsHeaders` | OPTIONS request returns CORS headers |

**Setup pattern**: `setupTestRouter()` creates all managers in mock mode and wires them into the router, matching production initialization in `main.go`:

```go
func setupTestRouter() http.Handler {
    sysMgr := &system.Manager{MockMode: true}
    arrayMgr := array.NewManager(sysMgr)
    diskMgr := disk.NewManager(true)
    cacheMgr := cache.NewManager(true)
    return NewRouter(arrayMgr, sysMgr, diskMgr, cacheMgr)
}
```

## Mock Data

### Mock nmdstat (`system/manager.go:mockNmdstat`)

Represents a 4-disk array:
- Slot 0: NVMe parity (SK Hynix P41, ~465 GB)
- Slot 1: HDD data (WDC WD20EFRX, ~1.8 TB) with I/O: 163 reads, 42 writes
- Slot 2: HDD data (WDC WD20EFRX, ~1.8 TB) with I/O: 163 reads, 38 writes
- Slot 3: HDD data (Seagate IronWolf, ~3.6 TB) with I/O: 245 reads, 120 writes
- Slot 4: Empty (DISK_NP)

### Mock disks (`disk/manager.go:mockDisks`)

5 simulated drives:
- `sda` — Samsung SSD 870 (238.5 GB, SSD)
- `sdb` — WDC WD20EFRX (1.8 TB, HDD)
- `sdc` — WDC WD20EFRX (1.8 TB, HDD)
- `sdd` — Seagate IronWolf (3.6 TB, HDD)
- `sde` — Seagate IronWolf (3.6 TB, HDD)

### Mock cache pools (`cache/manager.go:loadMockPools`)

2 simulated pools:
- `nvme-fast` — 3× SK Hynix P41 NVMe (5.4 TB total, 30% used, XFS)
- `ssd-warm` — 2× Samsung 870 EVO (1.8 TB total, 60% used, XFS)

## Adding New Tests

1. **New manager**: Create `internal/<name>/manager_test.go`
2. **New endpoint**: Add to `internal/api/router_test.go` using `httptest`
3. **Mock data**: Add mock functions in the manager file, gated by `MockMode`
4. **Pattern**: Keep tests as table-driven when testing multiple scenarios

## Future Test Plans

- [ ] Share manager tests (CRUD, ACL validation)
- [ ] App manager tests (Docker API mocking)
- [ ] Integration tests (full API flow: create pool → assign shares → run mover)
- [ ] Frontend component tests (React Testing Library)
- [ ] E2E tests (Playwright: start API + UI, navigate pages, interact)
