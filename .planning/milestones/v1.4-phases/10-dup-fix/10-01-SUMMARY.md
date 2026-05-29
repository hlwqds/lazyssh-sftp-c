---
phase: 10-dup-fix
plan: 01
subsystem: ui
tags: [tview, dup, handleServerDup, AddServer]

# Dependency graph
requires:
  - phase: 09-dup-ssh-connection
    provides: handleServerDup with ServerForm intermediate step
provides:
  - Direct save dup workflow (D key -> done, no form)
  - Search-aware positioning (clears filter before refresh)
  - Dead code removal (dupPendingAlias field and all references)
affects: [11-dual-remote-mark]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Direct save pattern: bypass form for simple operations"

key-files:
  created: []
  modified:
    - internal/adapters/ui/handlers.go
    - internal/adapters/ui/tui.go

key-decisions:
  - "Clear search bar text before refreshServerList to avoid index mismatch"
  - "Iterate t.serverList.servers (filtered+sorted) instead of ListServers for index alignment"
  - "Synchronous AddServer call (fast file I/O, no goroutine needed)"

patterns-established:
  - "Search-aware mutation: clear search filter before operations that add entries"

requirements-completed: [DUP-FIX-01, DUP-FIX-02]

# Metrics
duration: 2min
completed: 2026-04-15
---

# Phase 10 Plan 1: Dup Fix Summary

**D key duplication simplified from 3-step (D -> form -> save) to 1-step (D -> done) with search-aware positioning and dead code removal**

## Performance

- **Duration:** 2 min
- **Started:** 2026-04-15T15:06:55Z
- **Completed:** 2026-04-15T15:09:13Z
- **Tasks:** 1
- **Files modified:** 2

## Accomplishments
- handleServerDup() now saves directly via AddServer() without opening ServerForm
- Search bar text cleared before refresh so new entry is always visible in unfiltered list
- New entry auto-selected after duplication by iterating serverList.servers
- Green/red status bar feedback for success/failure
- dupPendingAlias field and all references fully removed (dead code cleanup)

## Task Commits

Each task was committed atomically:

1. **Task 1: Rewrite handleServerDup() for direct save with search-aware positioning** - `6935c4a` (feat)

## Files Created/Modified
- `internal/adapters/ui/handlers.go` - Replaced ServerForm creation with direct save, removed dupPendingAlias references from handleServerSave
- `internal/adapters/ui/tui.go` - Removed dupPendingAlias field from tui struct

## Decisions Made
- Clear search bar via `SetText("")` before `refreshServerList()` to ensure the new entry appears in the unfiltered list (per RESEARCH.md Pitfall 3 resolution)
- Use `t.serverList.servers` (private field, same package access) for index lookup after refresh -- this is the filtered+sorted list that `SetCurrentItem` operates on
- Synchronous execution -- `AddServer()` is fast file I/O, no goroutine needed

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- **Worktree sync issue**: The worktree was behind the main repo (07cb2ab vs 9af8d7b). Resolved by merging the main repo's HEAD into the worktree before starting execution. This was a blocking issue (Rule 3) -- the source files to modify did not exist in the worktree.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Dup fix complete, ready for Phase 11 (dual remote mark)
- No blockers or concerns

## Self-Check: PASSED

- FOUND: 10-01-SUMMARY.md
- FOUND: 6935c4a (task commit)
- PASS: no dupPendingAlias references in codebase
- PASS: go build ./... compiles cleanly

---
*Phase: 10-dup-fix*
*Completed: 2026-04-15*
