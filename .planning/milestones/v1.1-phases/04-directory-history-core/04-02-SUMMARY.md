---
phase: 04-directory-history-core
plan: 02
subsystem: ui
tags: [tview, tcell, callback, bugfix, go]

# Dependency graph
requires:
  - phase: 04-01
    provides: "RecentDirs struct with Record() MRU logic, GetPaths(), Show/Hide/IsVisible overlay skeleton"
provides:
  - "NavigateToParent onPathChange callback fix (AUX-02)"
  - "NavigateTo(path) method for direct navigation without recording (D-09)"
  - "FileBrowser.recentDirs field + OnPathChange callback wiring (D-04)"
  - "OnPathChange callback registration for both panes in build()"
affects: [05-popup-list-ui]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "NavigateToParent/NavigateInto symmetric onPathChange notification"
    - "NavigateTo silent navigation pattern (no observer notification)"

key-files:
  created: []
  modified:
    - internal/adapters/ui/file_browser/remote_pane.go
    - internal/adapters/ui/file_browser/file_browser.go

key-decisions:
  - "OnPathChange callbacks were missing from build() entirely -- added registration for both panes (Rule 2 deviation)"
  - "NavigateTo does not trigger onPathChange to prevent re-recording when Phase 5 popup selects a path"

patterns-established:
  - "All navigation methods that modify currentPath must notify observers via onPathChange (NavigateInto, NavigateToParent)"
  - "NavigateTo is the exception: direct path set without observer notification, used for programmatic navigation"

requirements-completed: [HIST-01, AUX-02]

# Metrics
duration: 3min
completed: 2026-04-14
---

# Phase 04 Plan 02: NavigateToParent Fix + NavigateTo + Record Wiring Summary

**NavigateToParent onPathChange callback fix, NavigateTo silent navigation method, and RecentDirs.Record() wiring through OnPathChange callback chain**

## Performance

- **Duration:** 3 min
- **Started:** 2026-04-14T07:07:41Z
- **Completed:** 2026-04-14T07:10:46Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Fixed NavigateToParent missing onPathChange callback (AUX-02) -- now symmetric with NavigateInto
- Added NavigateTo(path) method for direct navigation without triggering onPathChange (D-09, Phase 5 consumer)
- Wired RecentDirs.Record() into RemotePane's onPathChange callback in FileBrowser.build() (D-04, HIST-01 complete)
- Added missing OnPathChange callback registration for both panes in build() (Rule 2 auto-fix)

## Task Commits

Each task was committed atomically:

1. **Task 1: Fix NavigateToParent bug and add NavigateTo method** - `af8344f` (fix)
2. **Task 2: Wire RecentDirs into FileBrowser with onPathChange callback registration** - `38e9c1c` (feat)

## Files Created/Modified
- `internal/adapters/ui/file_browser/remote_pane.go` - NavigateToParent onPathChange fix + NavigateTo method
- `internal/adapters/ui/file_browser/file_browser.go` - recentDirs field, OnPathChange registration, Record wiring

## Decisions Made
- Followed D-08 (NavigateToParent fix) and D-09 (NavigateTo method) from CONTEXT.md exactly as specified
- OnPathChange callback registration was entirely missing from build() -- added for both panes to ensure app.Sync() is called on all navigation events

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical Functionality] Added missing OnPathChange callback registration in build()**
- **Found during:** Task 2 (Wire RecentDirs into FileBrowser)
- **Issue:** Plan assumed OnPathChange callbacks were already registered in FileBrowser.build() (referenced as lines 126-133), but no such registration existed. Both panes define OnPathChange setters and call onPathChange internally, but FileBrowser never registered any callbacks. This meant app.Sync() was never called on navigation, and Record() could not be wired.
- **Fix:** Added OnPathChange callback registration for both LocalPane (app.Sync() only) and RemotePane (app.Sync() + recentDirs.Record(path)) in build() after the OnFileAction callbacks.
- **Files modified:** internal/adapters/ui/file_browser/file_browser.go
- **Verification:** go build passes, all 7 existing Record tests pass
- **Committed in:** `38e9c1c` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 missing critical functionality)
**Impact on plan:** The missing callback registration was a prerequisite for the planned Record() wiring. Adding it completes the integration that was assumed to already exist. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 4 fully complete: RecentDirs data layer (Plan 01) + integration wiring + bug fix (Plan 02)
- NavigateTo(path) method ready for Phase 5 popup list selection navigation
- recentDirs field accessible on FileBrowser for Phase 5 to add overlay rendering
- All navigation events (NavigateInto, NavigateToParent) now properly trigger onPathChange -> Record()

## Self-Check: PASSED

- remote_pane.go NavigateToParent contains `rp.onPathChange(rp.currentPath)`: FOUND (line 288)
- remote_pane.go NavigateTo method exists: FOUND (line 309)
- remote_pane.go NavigateTo does NOT contain onPathChange: CONFIRMED
- file_browser.go contains `recentDirs *RecentDirs` field: FOUND (line 52)
- file_browser.go contains `fb.recentDirs = NewRecentDirs()`: FOUND (line 100)
- file_browser.go RemotePane onPathChange contains `fb.recentDirs.Record(path)`: FOUND (line 130)
- file_browser.go LocalPane onPathChange uses `_ string` (no Record): CONFIRMED (line 125)
- Commit af8344f: FOUND
- Commit 38e9c1c: FOUND
- go build ./...: PASS
- go test TestRecord: 7/7 PASS

---
*Phase: 04-directory-history-core*
*Completed: 2026-04-14*
