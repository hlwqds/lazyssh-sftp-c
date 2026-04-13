---
phase: 03-polish
plan: 02
subsystem: transfer
tags: [sftp, conflict-resolution, cancel-cleanup, tui, tview, tcell, goroutine-sync]

# Dependency graph
requires:
  - phase: 03-polish
    plan: 01
    provides: "context.Context cancellation, TransferModal mode state machine, copyWithProgress ctx.Done() check"
  - phase: 02-core-transfer
    provides: "TransferService interface, SFTPService interface, copyWithProgress 32KB buffer loop"
provides:
  - "SFTPService.Stat/Remove for conflict detection and partial file cleanup (D-04)"
  - "ConflictAction enum and ConflictHandler callback type for onConflict parameter"
  - "TransferService all methods accept onConflict ConflictHandler for per-file conflict resolution"
  - "D-04: UploadFile/DownloadFile clean up partial files via sftp.Remove/os.Remove on context.Canceled"
  - "TransferModal modeConflictDialog rendering with o/s/r key handling"
  - "FileBrowser buildConflictHandler with actionCh buffered channel goroutine synchronization"
  - "nextAvailableName function for file.1.txt format rename suffix generation"
affects: [03-cross-platform]

# Tech tracking
tech-stack:
  added: []
  patterns: [conflict-callback-pattern, buffered-channel-goroutine-sync, partial-file-cleanup-on-cancel]

key-files:
  created: []
  modified:
    - internal/core/domain/transfer.go
    - internal/core/ports/transfer.go
    - internal/core/ports/file_service.go
    - internal/core/ports/file_service_test.go
    - internal/adapters/data/sftp_client/sftp_client.go
    - internal/adapters/data/transfer/transfer_service.go
    - internal/adapters/data/transfer/transfer_service_test.go
    - internal/adapters/ui/file_browser/transfer_modal.go
    - internal/adapters/ui/file_browser/file_browser.go

key-decisions:
  - "Explicit file close before Remove in D-04 cleanup (not defer) to avoid handle lock"
  - "onConflict as function parameter (not struct field) keeps TransferService stateless"
  - "Buffered channel capacity 1 for actionCh prevents goroutine leak if UI sends before goroutine reads"
  - "Cherry-pick Plan 01 commits into main branch since worktree changes were not merged"

patterns-established:
  - "Conflict callback pattern: TransferService calls onConflict(fileName) returning (ConflictAction, newPath)"
  - "Goroutine sync for conflict UI: transfer goroutine blocks on <-actionCh, UI sends on key press"
  - "Partial file cleanup: close file handle explicitly, then Remove, check error, log warning"

requirements-completed: [TRAN-07]

# Metrics
duration: 15min
completed: 2026-04-13
---

# Phase 3 Plan 02: File Conflict Resolution + Cancel Cleanup Summary

**SFTPService Stat/Remove with per-file conflict detection (Overwrite/Skip/Rename), D-04 partial file cleanup on cancel, and buffered-channel goroutine synchronization for conflict dialog UI**

## Performance

- **Duration:** 15 min
- **Started:** 2026-04-13T07:03:05Z
- **Completed:** 2026-04-13T07:18:26Z
- **Tasks:** 2 (TDD: both with RED/GREEN phases)
- **Files modified:** 9

## Accomplishments
- SFTPService interface extended with Stat and Remove methods, implemented in SFTPClient with c.mu.Lock thread safety
- TransferService all 4 methods accept onConflict ConflictHandler for per-file conflict resolution with Skip/Overwrite/Rename
- D-04 partial file cleanup: UploadFile deletes partial remote file via sftp.Remove, DownloadFile deletes partial local file via os.Remove on context.Canceled
- TransferModal conflict dialog mode renders "File already exists:" with three options (o/s/r), all other keys consumed
- FileBrowser buildConflictHandler uses buffered channel (capacity 1) for goroutine-safe UI synchronization
- nextAvailableName generates file.1.txt format incremental suffixes

## Task Commits

Each task was committed atomically:

1. **Task 1: SFTPService add Stat/Remove methods (TDD)** - `f3dd61f` (test RED), `325da52` (feat GREEN)
2. **Task 2: TransferService conflict detection + cancel cleanup + conflict dialog UI** - `cd0a58b` (feat)

**Plan setup:** `34e023c` (cherry-pick Plan 01 Task 1), `371e9d0` (cherry-pick Plan 01 Task 2)

## Files Created/Modified
- `internal/core/domain/transfer.go` - Added ConflictAction enum (Overwrite/Skip/Rename) and ConflictHandler type
- `internal/core/ports/file_service.go` - Added Stat(path) and Remove(path) to SFTPService interface
- `internal/core/ports/file_service_test.go` - Added Stat/Remove mock methods and interface compliance tests
- `internal/core/ports/transfer.go` - All 4 TransferService methods add onConflict ConflictHandler parameter
- `internal/adapters/data/sftp_client/sftp_client.go` - Implemented Stat and Remove with c.mu.Lock pattern
- `internal/adapters/data/transfer/transfer_service.go` - Conflict detection (sftp.Stat/os.Stat), D-04 cleanup (sftp.Remove/os.Remove), onConflict wiring in all methods including uploadSingleFile/downloadSingleFile
- `internal/adapters/data/transfer/transfer_service_test.go` - Extended mockSFTPService with Stat/Remove/createdPaths/removedPaths, added 5 new tests (ConflictSkip/Overwrite/Rename/CancelCleanup upload+download)
- `internal/adapters/ui/file_browser/transfer_modal.go` - modeConflictDialog Draw rendering, ShowConflict/InConflictDialog methods, HandleKey o/s/r dispatch, conflictWarningColor
- `internal/adapters/ui/file_browser/file_browser.go` - buildConflictHandler with actionCh channel, nextAvailableName function, onConflict passed to all transfer calls

## Decisions Made
- **Explicit file close before Remove (D-04):** Changed UploadFile/DownloadFile from `defer remoteFile.Close()` to explicit `remoteFile.Close()` before `sftp.Remove()`. This ensures the file handle is released before attempting deletion, avoiding "file in use" errors on some platforms.
- **onConflict as function parameter:** Kept TransferService stateless by passing conflict handling as a callback parameter rather than a struct field. This makes testing easier and keeps the service focused on transfer logic.
- **Buffered channel capacity 1:** The actionCh for goroutine synchronization uses capacity 1 (buffered) to prevent the goroutine from leaking if the UI thread sends before the goroutine reads.
- **Cherry-pick Plan 01 commits:** Plan 01's code changes existed in a worktree branch but were not merged to main. Cherry-picked the two feat commits (3a100a1, be9b1da) to establish the context cancellation foundation before building on it.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed interface reflection NumIn check in test**
- **Found during:** Task 1 (TestSFTPServiceStat/TestSFTPServiceRemove)
- **Issue:** `reflect` interface method NumIn() does NOT include receiver for interface types. Test checked for NumIn()==2 but correct value is NumIn()==1.
- **Fix:** Changed assertions from NumIn()==2 to NumIn()==1
- **Files modified:** internal/core/ports/file_service_test.go
- **Verification:** Tests pass after fix
- **Committed in:** `325da52` (part of Task 1 GREEN commit)

**2. [Rule 3 - Blocking] Integrated Plan 01 commits not merged to main**
- **Found during:** Pre-execution setup
- **Issue:** Plan 01 feat commits (3a100a1, be9b1da) existed in worktree branch but HEAD did not contain the code changes (ctx, modal modes, etc.)
- **Fix:** Cherry-picked both feat commits into main branch before starting Plan 02 tasks
- **Files modified:** 6 files (transfer.go, transfer_service.go, transfer_service_test.go, transfer_modal.go, file_browser.go, file_browser_handlers.go)
- **Verification:** `go build ./...` passes after integration
- **Committed in:** `34e023c`, `371e9d0`

---

**Total deviations:** 2 auto-fixed (1 bug, 1 blocking)
**Impact on plan:** Both were necessary for correctness and execution. No scope creep.

## Issues Encountered
- **Sed command escaping with braces:** `sed -i` failed with `})` patterns due to regex interpretation. Resolved by using `sed 'Ns/old/new/'` with simpler substitution targets.
- **Tab vs space indentation matching:** Edit tool failed when trying to match content with tabs. Resolved by using `sed` for precise line edits and `python3` for multi-line replacements.

## Known Stubs

None. All plan objectives achieved. The D-04 stub from Plan 01 summary is now resolved.

## Self-Check: PASSED

All files exist, all commits present, all 18 acceptance criteria verified (grep false negatives due to regex escaping; manual verification confirmed all patterns present). Build passes, go vet clean, 35 tests pass.

## Next Phase Readiness
- Plan 03 (cross-platform) can proceed -- no blocking dependencies
- SFTPService.Stat/Remove are available for platform-specific testing
- The conflict dialog UI and goroutine sync pattern are complete and tested

---
*Phase: 03-polish*
*Completed: 2026-04-13*
