---
name: nonraid
description: "Load NonRAID architecture context and knowledgebase. Use when working on the nonraid codebase, nmdctl, kernel module, or any related component. Provides full system understanding."
argument-hint: "[topic or question]"
---

# NonRAID Architecture Reference

Load the knowledgebase for context on the NonRAID codebase. This skill provides comprehensive understanding of the system architecture, nmdctl CLI, kernel module, RAID6 engine, systemd integration, and packaging.

## Quick Reference

Read `nonraid/agent_docs/KNOWLEDGEBASE.md` for the full reference. Key sections:

1. **Project Overview** -- What NonRAID is, design principles
2. **Architecture** -- Communication model (proc interface), slot model
3. **nmdctl Commands** -- All 13 commands with flags and examples
4. **Array Lifecycle** -- State machine, start/stop, import, disk management
5. **Kernel Module** -- md personality, stripe cache, threads, superblock
6. **RAID6 Engine** -- P/Q parity, algorithm selection, recovery functions
7. **I/O Paths** -- Read/write strategies (RMW vs reconstruct)
8. **Status & Monitoring** -- Health levels, output formats, monitor mode
9. **Systemd Integration** -- Services, timers, unclean shutdown detection
10. **Packaging** -- DKMS + tools packages, CI/CD
11. **Configuration** -- /etc/default/nonraid, module params, file paths
12. **Data Structures** -- Bash arrays, kernel structs (stripe_head, column_t, unraid_conf)
13. **Error Handling** -- Disk failures, recovery, parity preservation
14. **Function Reference** -- All key functions with line numbers

## Detailed Research Documents

For deep dives, consult these research files in `nonraid/agent_docs/`:

| File | Topic |
|------|-------|
| `_research_cli.md` | Complete CLI interface, argument parsing, dispatch logic |
| `_research_array_mgmt.md` | Array operations, state machine, disk import, add/replace/unassign |
| `_research_parity.md` | Parity check/sync, pause/resume, progress tracking, unattended mode |
| `_research_helpers.md` | Utility functions, test suite, output formats, monitor TUI |
| `_research_kernel.md` | Kernel module internals, RAID6, data layout, I/O paths, locking |
| `_research_infra.md` | Systemd services, udev, packaging, CI pipelines, installation |

## Key File Locations

```
nonraid/tools/nmdctl                    -- Main CLI (~4800 lines Bash)
nonraid/tools/tests/test_nmdctl_basic.bats -- BATS unit tests
nonraid/tools/systemd/                  -- Service/timer files
nonraid/tools/systemd/nonraid.default   -- Configuration template
nonraid/tools/udev/nonraid.udev         -- udev rules
nonraid/md_nonraid/6.12/               -- Kernel module (current)
nonraid/md_nonraid/6.6/                -- Kernel module (6.5-6.8)
nonraid/md_nonraid/6.1/                -- Kernel module (legacy)
nonraid/raid6/                          -- RAID6 parity library
nonraid/debian/                         -- DKMS package metadata
nonraid/tools/debian/                   -- Tools package metadata
nonraid/dkms.conf                       -- DKMS build config
nonraid/Makefile                        -- Top-level build
```

## Task: $ARGUMENTS

Read the knowledgebase and relevant source files, then address the topic or question above. If no specific topic was given, provide a summary of the architecture and offer to help with implementation tasks.
