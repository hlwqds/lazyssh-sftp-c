---
phase: 11-t-key-marking
plan: 01
subsystem: ui
tags: [tview, tcell, marking, keyboard-shortcuts, server-list]

# Dependency graph
requires:
  - phase: 10-dup-fix
    provides: handleServerDup direct-save pattern, tui struct layout
provides:
  - T key marking state machine (idle -> source -> target -> browser)
  - formatServerLine mark prefix rendering ([S] green, [T] blue)
  - handleDualRemoteBrowser placeholder for Phase 12
  - MarkStateGetter/markClearer callback pattern on ServerList
affects: [12-dual-remote-browser]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - MarkStateGetter callback: ServerList queries mark state from tui
    - markClearer callback: ServerList delegates Esc priority to tui
    - Shift+t dispatch: case 'T' for marking, case 't' for tags

key-files:
  created: []
  modified:
    - internal/adapters/ui/tui.go
    - internal/adapters/ui/server_list.go
    - internal/adapters/ui/utils.go
    - internal/adapters/ui/utils_test.go
    - internal/adapters/ui/handlers.go
    - internal/adapters/ui/status_bar.go

key-decisions:
  - "Mark state stored in tui struct (not ServerList) for cross-component access"
  - "MarkStateGetter callback pattern decouples ServerList from tui mark state"
  - "Esc priority: markClearer checked before onReturnToSearch to allow clearing marks without leaving list"
  - "Alias matching for mark identification (sufficient for uniqueness per SSH config)"

patterns-established:
  - "MarkStateGetter callback: closure-based state query from child to parent component"
  - "markClearer callback: boolean-returning Esc handler for priority chain"

requirements-completed: [MARK-01, MARK-02, MARK-03, MARK-04, MARK-05]

# Metrics
duration: 2min
completed: 2026-04-15
---

# Phase 11 Plan 1: T Key Marking Summary

**T key marking state machine with [S]/[T] visual prefix, Esc clear, same-server protection, and dual remote browser placeholder**

## Performance

- **Duration:** 2 min
- **Started:** 2026-04-15T15:50:02Z
- **Completed:** 2026-04-15T15:51:57Z
- **Tasks:** 3 (2 auto + 1 auto-approved checkpoint)
- **Files modified:** 6

## Accomplishments
- T key (Shift+t) marking state machine: idle -> source_marked -> target_marked -> open browser
- Green [S] prefix (#A0FFA0) for source, blue [T] prefix (#5FAFFF) for target in server list
- Esc clears marks before returning to search bar (priority chain)
- Same-server protection with red error message (#FF6B6B)
- handleDualRemoteBrowser placeholder wired for Phase 12 integration
- T key hint added to status bar

## Task Commits

Each task was committed atomically:

1. **Task 1: Add mark state infrastructure and formatServerLine rendering** - `0f60790` (feat)
2. **Task 2: Add handleServerMark handler, status bar hint, and handleDualRemoteBrowser placeholder** - `ab2c06a` (feat)

_Note: Task 3 was a checkpoint:human-verify auto-approved since auto_advance is true_

## Files Created/Modified
- `internal/adapters/ui/tui.go` - Added markSource/markTarget fields, domain import, markStateGetter/markClearer wiring in buildComponents, handleMarkClear method
- `internal/adapters/ui/server_list.go` - Added MarkStateGetter type, markStateGetter/markClearer fields, OnMarkState/OnMarkClear setters, Esc priority in InputCapture, mark state passing in UpdateServers
- `internal/adapters/ui/utils.go` - Changed formatServerLine signature to accept markSource/markTarget, added mark prefix rendering logic
- `internal/adapters/ui/utils_test.go` - Added TestFormatServerLine_MarkPrefix test covering no-mark, source, target, and non-matching alias cases
- `internal/adapters/ui/handlers.go` - Added case 'T' dispatch, handleServerMark state machine, handleDualRemoteBrowser placeholder
- `internal/adapters/ui/status_bar.go` - Added [white]T[-] Mark hint to DefaultStatusText

## Decisions Made
- Mark state stored in tui struct rather than ServerList, because the state needs cross-component access (handlers, list rendering, clear logic)
- MarkStateGetter callback pattern chosen to decouple ServerList from tui's internal state
- Esc priority chain: markClearer returns bool, checked before onReturnToSearch, allowing marks to be cleared without triggering search bar focus
- Alias matching used for identifying marked servers (sufficient uniqueness per SSH config)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- handleDualRemoteBrowser placeholder ready for Phase 12 to implement DualRemoteFileBrowser
- Mark state machine fully functional for integration testing
- No blockers identified

---
*Phase: 11-t-key-marking*
*Completed: 2026-04-15*
