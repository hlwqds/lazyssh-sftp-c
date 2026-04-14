---
phase: 05-recent-directories-popup
plan: 01
subsystem: ui
tags: [tview, tcell, overlay, popup, mru, keyboard-navigation]

# Dependency graph
requires:
  - phase: 04-directory-history-core
    provides: "RecentDirs data structure with Record/GetPaths/Show/Hide/IsVisible, NavigateTo on RemotePane"
provides:
  - "RecentDirs popup UI with Draw() rendering and HandleKey() navigation"
  - "r key routing in handleGlobalKeys with activePane==1 guard"
  - "Overlay draw chain in FileBrowser.Draw() (also fixes TransferModal rendering bug)"
  - "onSelect callback wiring: Hide -> NavigateTo -> Record -> SetFocus"
affects: [future-popup-features, file-browser-key-routing]

# Tech tracking
tech-stack:
  added: []
  patterns: ["overlay Draw in FileBrowser.Draw() after Flex.Draw()", "full key interception in handleGlobalKeys with overlay-first check"]

key-files:
  created: []
  modified:
    - internal/adapters/ui/file_browser/recent_dirs.go
    - internal/adapters/ui/file_browser/recent_dirs_test.go
    - internal/adapters/ui/file_browser/file_browser_handlers.go
    - internal/adapters/ui/file_browser/file_browser.go

key-decisions:
  - "RecentDirs kept decoupled from RemotePane: currentPath passed via SetCurrentPath() string, no direct import"
  - "Full key interception (D-08): HandleKey returns nil for ALL keys when visible, not just handled ones"
  - "TransferModal.Draw() bug fix included: added overlay draw call in FileBrowser.Draw() (Pitfall 1)"

patterns-established:
  - "Overlay-first key routing: check recentDirs.IsVisible() at top of handleGlobalKeys before any switch"
  - "Rendering order: SetRect -> DrawForSubclass -> fill row background -> tview.Print text (Pitfall 3)"

requirements-completed: [POPUP-01, POPUP-02, POPUP-03, POPUP-04, POPUP-05, AUX-01]

# Metrics
duration: 6min
completed: 2026-04-14
---

# Phase 5 Plan 1: Recent Directories Popup Summary

**Centered popup overlay with j/k navigation, current-path yellow highlighting, and TransferModal.Draw() rendering bug fix**

## Performance

- **Duration:** 6 min
- **Started:** 2026-04-14T08:50:35Z
- **Completed:** 2026-04-14T08:56:48Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- RecentDirs Draw() renders centered popup (60% width max 80, adaptive height) with path list, selection highlighting (Color236 bg), current-path yellow marking (AUX-01), and "暂无最近目录" empty state
- HandleKey() processes j/k/Down/Up/Enter/Esc with full key interception when visible (D-08)
- r key wired in handleGlobalKeys with activePane==1 && IsConnected() guard (POPUP-01)
- FileBrowser.Draw() now calls overlay Draw methods (transferModal + recentDirs), fixing pre-existing TransferModal rendering bug (Pitfall 1)
- 18 unit tests passing (8 existing + 10 new)

## Task Commits

Each task was committed atomically:

1. **Task 1 (TDD RED): Add failing tests for HandleKey, Draw, and selection** - `6dea760` (test)
2. **Task 1 (TDD GREEN): Implement RecentDirs Draw() and HandleKey()** - `79b35df` (feat)
3. **Task 2: Wire RecentDirs into FileBrowser key routing and overlay draw chain** - `38aaa32` (feat)

**Plan metadata:** (pending)

## Files Created/Modified
- `internal/adapters/ui/file_browser/recent_dirs.go` - Added Draw() rendering with popup dimensions, selection highlighting, current-path yellow marking, empty state; HandleKey() with j/k/arrows/Enter/Esc; SetOnSelect, SetCurrentPath, GetSelectedIndex, GetCurrentPath methods; Show() now resets selectedIndex
- `internal/adapters/ui/file_browser/recent_dirs_test.go` - Added 10 new tests: HandleKey visibility guard, Esc hide, Enter select, empty list Enter, j/k navigation, arrow navigation, full key interception, Show resets selectedIndex, SetCurrentPath, SetOnSelect
- `internal/adapters/ui/file_browser/file_browser_handlers.go` - Added RecentDirs overlay visibility check at top of handleGlobalKeys (D-08); added case 'r' with activePane==1 && IsConnected() guard and SetCurrentPath before Show
- `internal/adapters/ui/file_browser/file_browser.go` - Added overlay Draw calls after Flex.Draw() for both transferModal and recentDirs (Pitfall 1 fix); wired onSelect callback with Hide -> NavigateTo -> Record -> SetFlow sequence

## Decisions Made
- RecentDirs kept decoupled from RemotePane: currentPath is passed via SetCurrentPath(string) rather than importing RemotePane directly, maintaining clean separation
- Full key interception (D-08): HandleKey returns nil for ALL keys when visible, not just recognized ones -- prevents any key from leaking through to underlying components
- TransferModal.Draw() bug fix bundled: since FileBrowser.Draw() needed overlay calls anyway, fixing the pre-existing missing TransferModal.Draw() call was included (Pitfall 1)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Phase 5 is the final phase for v1.1 milestone
- All POPUP-01 through POPUP-05 and AUX-01 requirements are complete
- Milestone v1.1 "Recent Remote Directories" is ready for completion via `/gsd:complete-milestone`

---
*Phase: 05-recent-directories-popup*
*Completed: 2026-04-14*
