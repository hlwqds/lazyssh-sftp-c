---
phase: 08-move-integration
plan: 01
subsystem: ui
tags: [tview, tcell, clipboard, move, file-browser]

# Dependency graph
requires:
  - phase: 07-copy-clipboard
    provides: "clipboardProvider callback, OpCopy constant, [C] prefix rendering, handleCopy pattern"
provides:
  - OpMove constant for move clipboard operation
  - modeMove in TransferModal with ShowMove() for remote move progress
  - [M] prefix rendering in red (#FF6B6B) via clipboardProvider 4-tuple
  - handleMove() method and x key routing
  - Status bar x Move hint in all three hint functions
affects: [08-move-integration-02, file-browser, transfer-modal]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "clipboardProvider 4-tuple (bool, string, string, ClipboardOp) for operation-aware rendering"

key-files:
  created: []
  modified:
    - internal/adapters/ui/file_browser/file_browser.go
    - internal/adapters/ui/file_browser/file_browser_handlers.go
    - internal/adapters/ui/file_browser/local_pane.go
    - internal/adapters/ui/file_browser/remote_pane.go
    - internal/adapters/ui/file_browser/transfer_modal.go

key-decisions:
  - "clipboardProvider extended to 4-tuple returning ClipboardOp for [M]/[C] prefix distinction"
  - "modeMove reuses drawProgress render path identically to modeCopy (no new draw code)"
  - "[M] uses red (#FF6B6B) to visually distinguish from [C] green (#00FF7F)"

patterns-established:
  - "Move marking mirrors copy marking pattern: handleMove() mirrors handleCopy() with OpMove + red color"

requirements-completed: [MOV-01, PRG-01]

# Metrics
duration: 8min
completed: 2026-04-15
---

# Phase 8 Plan 1: Move Marking Side Summary

**Move marking UI with x key, [M] red prefix rendering, modeMove TransferModal, and extended clipboardProvider 4-tuple for operation-aware prefix display**

## Performance

- **Duration:** 8 min
- **Started:** 2026-04-15T07:50:14Z
- **Completed:** 2026-04-15T07:58:07Z
- **Tasks:** 1
- **Files modified:** 5

## Accomplishments
- OpMove constant enabled (replaced reserved comment) for move clipboard operation
- TransferModal modeMove added with ShowMove() method, integrated into Draw/HandleKey/Update
- clipboardProvider extended from 3-tuple to 4-tuple returning ClipboardOp for [M]/[C] distinction
- [M] prefix renders in red (#FF6B6B) when OpMove, [C] renders in green (#00FF7F) when OpCopy
- handleMove() method added mirroring handleCopy() with OpMove and red status feedback
- x key routed in handleGlobalKeys between c (Copy) and p (Paste)
- All three status bar hint functions updated with `[white]x[-] Move` between Copy and Paste

## Task Commits

Each task was committed atomically:

1. **Task 1: Enable OpMove, add modeMove/ShowMove, extend clipboardProvider, add [M] prefix rendering, add handleMove + x key, update status bar hints** - `349711b` (feat)

## Files Created/Modified
- `internal/adapters/ui/file_browser/file_browser.go` - OpMove constant, clipboardProvider 4-tuple wiring, handleMove(), status bar hints with x Move
- `internal/adapters/ui/file_browser/file_browser_handlers.go` - x key routing in handleGlobalKeys
- `internal/adapters/ui/file_browser/local_pane.go` - clipboardProvider 4-tuple type, [M]/[C] prefix rendering
- `internal/adapters/ui/file_browser/remote_pane.go` - clipboardProvider 4-tuple type, [M]/[C] prefix rendering
- `internal/adapters/ui/file_browser/transfer_modal.go` - modeMove constant, ShowMove() method, modeMove in Draw/HandleKey/Update

## Decisions Made
- Extended clipboardProvider from 3-tuple to 4-tuple `(bool, string, string, ClipboardOp)` -- this is the minimal change to make both panes aware of the clipboard operation type without coupling them to FileBrowser internals
- modeMove reuses the existing drawProgress path identically to modeCopy -- no new rendering code needed, just added modeMove to the existing switch cases
- Esc clears [M] clipboard mark identically to [C] -- Esc handler checks `fb.clipboard.Active` without caring about Operation, so no change needed

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Worktree was on an older commit without file_browser directory; worked directly in main repo since branching_strategy is "none"
- sed-based editing required for tab-indented code blocks that the Edit tool couldn't match; go fmt resolved all indentation inconsistencies

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Move marking side complete; ready for Plan 02 (paste/dispatch side)
- handlePaste() will need to branch on `fb.clipboard.Operation == OpMove` to call move-specific logic
- TransferModal.ShowMove() is wired but not yet called from any handler -- Plan 02 will call it from the move paste flow

---
*Phase: 08-move-integration*
*Completed: 2026-04-15*

## Self-Check: PASSED

- All 5 modified source files exist
- 08-01-SUMMARY.md exists
- Commit 349711b found in git log
- `go build ./...` passes
- `go vet ./...` passes
