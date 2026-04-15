---
phase: 09-dup-ssh-connection
plan: 01
subsystem: ui
tags: [tview, keyboard-handler, server-duplication]

# Dependency graph
requires:
  - phase: 01-08 (all prior phases)
    provides: "server list, ServerForm, handleServerSave, ServerService"
provides:
  - "D key (Shift+d) duplicate server handler with deep copy and unique alias"
  - "generateUniqueAlias() helper with -copy, -copy-2, ... suffix logic"
  - "dupPendingAlias tracking for post-save list selection"
  - "Status bar and server details D key hints"
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "dupPendingAlias pattern: track pending alias across form open/save for post-save UI selection"

key-files:
  created: []
  modified:
    - internal/adapters/ui/handlers.go
    - internal/adapters/ui/tui.go
    - internal/adapters/ui/status_bar.go
    - internal/adapters/ui/server_details.go

key-decisions:
  - "dupPendingAlias field on tui struct to track the expected alias for post-save list selection, cleared on edit mode and after dup save"
  - "Deep copy all slice fields (Aliases, IdentityFiles, Tags, LocalForward, RemoteForward, DynamicForward, SendEnv, SetEnv) to avoid shared references between original and duplicate"
  - "generateUniqueAlias uses ListServers to build alias set for uniqueness check, falls back to base-copy if service errors"

patterns-established:
  - "Server duplication via pre-filled ServerForm in Add mode with unique alias"

requirements-completed: [DUP-01, DUP-02, DUP-03, DUP-04]

# Metrics
duration: 2min
completed: 2026-04-15
---

# Phase 9 Plan 1: Duplicate Server Entry Summary

**D key (Shift+d) server duplication with deep copy, unique -copy alias suffix, runtime metadata clearing, and post-save list auto-scroll**

## Performance

- **Duration:** 2 min
- **Started:** 2026-04-15T12:54:13Z
- **Completed:** 2026-04-15T12:56:30Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- `handleServerDup()` method creates a deep copy of the selected server, clears runtime metadata (PinnedAt, SSHCount, LastSeen), generates a unique alias with -copy suffix, and opens ServerForm in Add mode
- `generateUniqueAlias()` helper checks existing server aliases and appends -copy, -copy-2, -copy-3, ... to avoid conflicts
- Post-save list auto-scroll selects the newly duplicated entry by alias matching
- Status bar shows `D Dup` hint; server details Commands section shows `D: Duplicate entry`

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement handleServerDup() and D key routing** - `c533c65` (feat)
2. **Task 2: Update status bar and server details hints for D key** - `e100fef` (feat)

## Files Created/Modified
- `internal/adapters/ui/handlers.go` - Added handleServerDup(), generateUniqueAlias(), case 'D' routing, dupPendingAlias logic in handleServerSave
- `internal/adapters/ui/tui.go` - Added dupPendingAlias field to tui struct
- `internal/adapters/ui/status_bar.go` - Added D Dup hint to status bar
- `internal/adapters/ui/server_details.go` - Added D: Duplicate entry to Commands section

## Decisions Made
- Used `dupPendingAlias` field on tui struct to bridge the gap between form open and form save for auto-scrolling to the new entry
- Deep copy all slice fields to prevent shared references between original and duplicate server structs
- generateUniqueAlias queries ListServers("") to build a set of existing aliases for uniqueness, graceful fallback on error

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 9 is the only plan in this phase, milestone v1.3 is complete
- All requirements (DUP-01 through DUP-04) fulfilled

---
*Phase: 09-dup-ssh-connection*
*Completed: 2026-04-15*

## Self-Check: PASSED

- FOUND: 09-01-SUMMARY.md
- FOUND: c533c65 (Task 1 commit)
- FOUND: e100fef (Task 2 commit)
- FOUND: 70d4b83 (docs commit)
- `go build ./...` passes with no errors
- All 8 verification checks from plan confirmed
