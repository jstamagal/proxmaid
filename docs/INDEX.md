# Proxmaid v2 — LLM Knowledgebase Index

> **Point any LLM agent here.** This file is the entry point to understand the entire Proxmaid project.
> All documentation lives in `docs/` at the project root. The TODO lives at `TODO.md` in the project root.

---

## 📍 Where Everything Is

```
proxmaid-v2/
├── TODO.md                           ← MASTER TODO (everything that needs doing)
├── README.md                         ← Project overview + quick start
├── docs/
│   ├── INDEX.md                      ← YOU ARE HERE
│   ├── PRD.md                        ← Product Requirements Document
│   ├── ARCHITECTURE.md               ← System architecture + data flow
│   ├── API_REFERENCE.md              ← Complete REST API docs + Go structs
│   ├── TESTING.md                    ← Test system, mock mode, how to add tests
│   ├── UNRAID_FEATURE_PARITY.md      ← Unraid vs Proxmaid feature comparison
│   └── nonraid/                      ← NonRAID kernel driver deep-dive
│       ├── KNOWLEDGEBASE.md          ← Full NonRAID reference (30KB)
│       ├── _research_cli.md          ← nmdctl CLI internals
│       ├── _research_array_mgmt.md   ← Array operations + state machine
│       ├── _research_parity.md       ← Parity check/sync engine
│       ├── _research_helpers.md      ← Utility functions + test suite
│       ├── _research_kernel.md       ← Kernel module internals + I/O paths
│       └── _research_infra.md        ← Systemd, udev, packaging, CI
```

---

## 📄 Document Descriptions

### Root Files
| File | What | When to Read |
|------|------|--------------|
| [TODO.md](../TODO.md) | Complete checklist of all features, grouped by component. ~150 items, ~44 done, ~114 remaining. | **Always** — this is the master task list |
| [README.md](../README.md) | Quick start, project structure, API endpoint table | Quick orientation |

### `docs/` — Core Documentation
| File | Size | What | When to Read |
|------|------|------|--------------|
| [PRD.md](PRD.md) | 5KB | Vision, problem statement, sidecar architecture, tech stack, non-goals | Understanding the "why" |
| [ARCHITECTURE.md](ARCHITECTURE.md) | 5KB | System diagram, boot sequence, component structure, data flow, state machine, config | Understanding the "how" |
| [API_REFERENCE.md](API_REFERENCE.md) | 10KB | Every REST endpoint with request/response schemas, Go struct definitions, curl examples | Building API consumers or new endpoints |
| [TESTING.md](TESTING.md) | 6KB | Mock mode architecture, every test documented, mock data catalog, how to add tests | Writing or debugging tests |
| [UNRAID_FEATURE_PARITY.md](UNRAID_FEATURE_PARITY.md) | 9KB | Feature-by-feature comparison: what NonRAID handles, what's implemented, what's remaining | Prioritizing work, understanding scope |

### `docs/nonraid/` — NonRAID Kernel Driver Reference
| File | Size | What |
|------|------|------|
| [KNOWLEDGEBASE.md](nonraid/KNOWLEDGEBASE.md) | 30KB | Complete NonRAID reference — architecture, nmdctl commands, state machine, config, data structures |
| [_research_cli.md](nonraid/_research_cli.md) | 24KB | Deep dive into nmdctl CLI (~4800 lines Bash) — argument parsing, dispatch, every command |
| [_research_array_mgmt.md](nonraid/_research_array_mgmt.md) | 32KB | Array operations: start/stop, import, new, add/replace/unassign, disk management |
| [_research_parity.md](nonraid/_research_parity.md) | 28KB | Parity check/sync engine: progress tracking, pause/resume, unattended mode |
| [_research_helpers.md](nonraid/_research_helpers.md) | 33KB | Utility functions, BATS test suite, output formats, monitor TUI |
| [_research_kernel.md](nonraid/_research_kernel.md) | 34KB | Kernel module: md personality, stripe cache, threads, RAID6 engine, I/O paths |
| [_research_infra.md](nonraid/_research_infra.md) | 30KB | Systemd services, udev rules, DKMS packaging, CI pipelines |

---

## 🏗️ Source Code Layout

```
proxmaid-v2/
├── api/                              ← Go backend (REST API)
│   ├── cmd/proxmaid/main.go          ← Entry point, wires managers
│   ├── go.mod
│   └── internal/
│       ├── api/
│       │   ├── router.go             ← HTTP router, all endpoints (207 lines)
│       │   └── router_test.go        ← 7 endpoint tests
│       ├── array/
│       │   ├── manager.go            ← Array state machine, nmdstat parser (244 lines)
│       │   └── manager_test.go       ← 6 parser tests
│       ├── cache/
│       │   ├── manager.go            ← Cache pools, mergerfs, mover daemon (354 lines)
│       │   └── manager_test.go       ← 14 pool/mover tests
│       ├── disk/
│       │   ├── manager.go            ← Disk discovery (lsblk), SMART, wipe/format (265 lines)
│       │   └── manager_test.go
│       └── system/
│           ├── manager.go            ← Kernel module, proc interface, mock data (207 lines)
│           └── manager_test.go
│
├── ui/                               ← Next.js frontend
│   └── src/
│       ├── app/
│       │   ├── page.tsx              ← Dashboard
│       │   ├── array/page.tsx        ← Array Manager (slot grid, drag-drop)
│       │   └── cache/page.tsx        ← Cache & Tiering
│       ├── components/
│       │   ├── ArrayStatusCard.tsx
│       │   ├── DiskListCard.tsx
│       │   ├── Sidebar.tsx
│       │   └── StatsBar.tsx
│       └── lib/
│           └── api.ts                ← Typed API client (144 lines)
│
├── pve-plugin/                       ← Proxmox storage plugin (Perl) — NOT YET IMPLEMENTED
│
└── nonraid/                          ← NonRAID kernel driver source (git submodule)
    ├── tools/nmdctl                  ← Main CLI (~4800 lines Bash)
    ├── md_nonraid/6.12/              ← Kernel module source (current)
    ├── raid6/                        ← RAID6 parity library
    └── debian/                       ← DKMS packaging
```

---

## 🚀 Quick Start for Agents

### Run the API (mock mode auto-activates on dev machines)
```bash
cd /home/admin/proxmaid-v2/api && go run ./cmd/proxmaid/
```

### Run the UI
```bash
cd /home/admin/proxmaid-v2/ui && npm run dev
```

### Run all tests
```bash
cd /home/admin/proxmaid-v2/api && go test ./... -v
```

### Key patterns to follow
1. **Manager pattern**: Each subsystem gets `internal/<name>/manager.go` with a `Manager` struct
2. **Mock mode**: Check `MockMode` flag before system calls, return realistic simulated data
3. **Router**: Add endpoints in `internal/api/router.go`, inject manager via function params
4. **UI**: Pages in `ui/src/app/<name>/page.tsx`, shared components in `ui/src/components/`
5. **API client**: Add types and functions to `ui/src/lib/api.ts`

---

## 📋 Suggested Reading Order for New Agents

1. **This file** (INDEX.md) — orientation
2. **TODO.md** — understand what's done and what needs doing
3. **PRD.md** — understand the vision and constraints
4. **ARCHITECTURE.md** — understand the system design
5. **API_REFERENCE.md** — understand the existing API surface
6. **UNRAID_FEATURE_PARITY.md** — understand scope vs Unraid
7. **TESTING.md** — understand how to test changes
8. **nonraid/KNOWLEDGEBASE.md** — only if working on array/parity features
