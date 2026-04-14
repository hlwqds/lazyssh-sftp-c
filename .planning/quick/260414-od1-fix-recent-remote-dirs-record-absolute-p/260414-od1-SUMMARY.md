---
phase: quick
plan: 260414-od1
subsystem: ui
tags: [sftp, recent-dirs, bugfix, tview]

# Dependency graph
requires:
  - phase: 04-directory-history-core
    provides: "RecentDirs data layer with Record() and MRU dedup"
  - phase: 05-recent-directories-popup
    provides: "RecentDirs popup UI with Draw/HandleKey integration"
provides:
  - "SFTPService.HomeDir() interface method for resolving absolute remote paths"
  - "RemotePane absolute path initialization via HomeDir()"
  - "File/dir transfer completion triggers RecentDirs recording"
affects: [file-browser, recent-dirs]

# Tech tracking
tech-stack:
  added: []
  patterns: []

key-files:
  created: []
  modified:
    - internal/core/ports/file_service.go
    - internal/adapters/ui/file_browser/remote_pane.go
    - internal/adapters/ui/file_browser/file_browser.go
    - internal/core/ports/file_service_test.go
    - internal/adapters/data/transfer/transfer_service_test.go

key-decisions:
  - "HomeDir() added to SFTPService interface rather than passing path as parameter to ShowConnected"
  - "Record() called in both initiateTransfer and initiateDirTransfer success branches"

patterns-established: []

requirements-completed: []

# Metrics
duration: 2min
completed: 2026-04-14
---

# Quick 260414-od1: 修复 RecentDirs 远程目录记录空列表 bug

**通过 HomeDir() 接口暴露绝对路径 + ShowConnected 路径初始化 + 传输完成记录，使 RecentDirs 弹窗能正确显示已访问的远程目录**

## Performance

- **Duration:** 2 min
- **Started:** 2026-04-14T09:38:34Z
- **Completed:** 2026-04-14T09:41:16Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- SFTPService 接口新增 HomeDir() 方法，暴露 SFTPClient 已有的绝对路径获取能力
- RemotePane.ShowConnected() 将 "." 初始路径解析为绝对路径，避免 Record() 过滤
- 文件传输和目录传输成功后自动调用 Record() 记录远程目录路径

## Task Commits

1. **Task 1: SFTPService 接口添加 HomeDir() 方法并修复 RemotePane 绝对路径初始化** - `c4d8087` (fix)
2. **Task 2: 传输完成后记录远程目录路径到 RecentDirs** - `2274835` (fix)

## Files Created/Modified
- `internal/core/ports/file_service.go` - SFTPService 接口新增 HomeDir() string 方法
- `internal/adapters/ui/file_browser/remote_pane.go` - ShowConnected() 中将 "." 替换为 HomeDir() 绝对路径
- `internal/adapters/ui/file_browser/file_browser.go` - initiateTransfer/initiateDirTransfer 成功分支调用 Record()
- `internal/core/ports/file_service_test.go` - mockSFTPService 添加 HomeDir() 桩方法
- `internal/adapters/data/transfer/transfer_service_test.go` - mockSFTPService 添加 HomeDir() 桩方法

## Decisions Made
- HomeDir() 直接添加到 SFTPService 接口，而非通过 ShowConnected 参数传入路径——保持接口语义清晰，RemotePane 可自主获取远程主目录
- 仅在 currentPath == "." 时替换，避免用户后续导航产生的绝对路径被覆盖

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] 两个 mockSFTPService 缺少 HomeDir() 方法导致 go vet 失败**
- **Found during:** Task 2 (验证编译时)
- **Issue:** SFTPService 接口新增 HomeDir() 后，file_service_test.go 和 transfer_service_test.go 中的 mockSFTPService 未实现该方法，go vet 报错
- **Fix:** 在两个 mock 文件中添加 `func (m *mockSFTPService) HomeDir() string { return "/home/test" }` 桩方法
- **Files modified:** internal/core/ports/file_service_test.go, internal/adapters/data/transfer/transfer_service_test.go
- **Verification:** go build ./... && go vet ./... 均通过
- **Committed in:** `2274835` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Mock 修复是接口变更的必要配套工作，无范围蔓延。

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- RecentDirs 功能现已完整可用：路径初始化为绝对路径 + 导航记录 + 传输记录
- 无已知阻塞问题

## Self-Check: PASSED

---
*Phase: quick*
*Completed: 2026-04-14*
