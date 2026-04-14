---
phase: quick
plan: 260414-ucr
subsystem: ui
tags: [tview, tcell, kitty, transparency, table-header]

# Dependency graph
requires: []
provides:
  - "Opaque header row rendering in kitty with background_opacity < 1"
affects: [file-browser-ui]

# Tech tracking
tech-stack:
  added: []
  patterns: ["SetTransparency(false) on cells with non-default background colors for kitty compatibility"]

key-files:
  created: []
  modified:
    - internal/adapters/ui/file_browser/local_pane.go
    - internal/adapters/ui/file_browser/remote_pane.go

key-decisions:
  - "SetTransparency(false) only on header cells, not data rows -- data cells use ColorDefault which should blend with terminal background"

patterns-established:
  - "Cells with explicit background colors must use SetTransparency(false) for kitty background_opacity compatibility"

requirements-completed: [ucr-kitty-header-transparency]

# Metrics
duration: 1min
completed: 2026-04-14
---

# Quick 260414-ucr: Fix table header row invisible in kitty Summary

**SetTransparency(false) on header cells in both LocalPane and RemotePane to render opaque Color235 background in kitty with background_opacity < 1**

## Performance

- **Duration:** 1 min
- **Started:** 2026-04-14T13:52:42Z
- **Completed:** 2026-04-14T13:53:63Z
- **Tasks:** 1
- **Files modified:** 2

## Accomplishments
- Header row now renders with opaque Color235 background in kitty terminal with background_opacity < 1
- Header text is clearly visible against the opaque dark background
- Data rows remain unaffected -- they continue to use ColorDefault and blend with terminal background

## Task Commits

1. **Task 1: Add SetTransparency(false) to header cells in both panes** - `7dbd0f7` (fix)

## Files Created/Modified
- `internal/adapters/ui/file_browser/local_pane.go` - Added SetTransparency(false) to header cell chain in populateTable()
- `internal/adapters/ui/file_browser/remote_pane.go` - Added SetTransparency(false) to header cell chain in populateTable()

## Decisions Made
None - followed plan as specified.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## Next Phase Readiness
No next phase dependencies -- this is a standalone UI fix.

---
*Quick: 260414-ucr*
*Completed: 2026-04-14*
