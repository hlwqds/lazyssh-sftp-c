---
phase: quick
plan: 260414-oow
subsystem: file-browser
tags: [persistence, recent-dirs, transfer-only-recording]
dependency_graph:
  requires: []
  provides: [recent-dirs-persistence]
  affects: [file-browser]
tech_stack:
  added: []
  patterns: [disk-persistence-per-server, JSON-file-storage]
key_files:
  created: []
  modified:
    - internal/adapters/ui/file_browser/recent_dirs.go
    - internal/adapters/ui/file_browser/file_browser.go
    - internal/adapters/ui/file_browser/recent_dirs_test.go
decisions: []
metrics:
  duration: "10min"
  completed_date: "2026-04-14"
---

# Quick Task 260414-oow: Recent Dirs Transfer-Only Recording & Persistence Summary

RecentDirs 组件从纯内存 MRU 列表升级为磁盘持久化存储，移除导航时的噪音记录，仅在实际文件/目录传输成功后才记录远程目录路径。每个服务器的最近目录独立存储在 `~/.lazyssh/recent-dirs/{user@host}.json`。

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add disk persistence and per-server isolation to RecentDirs | 8423d88 | recent_dirs.go |
| 2 | Remove navigation recording and update FileBrowser calls | 589cea2 | file_browser.go, recent_dirs_test.go |

## Changes Made

### Task 1: 磁盘持久化 + 按服务器隔离

**recent_dirs.go:**
- `NewRecentDirs` 签名变更：`NewRecentDirs(log *zap.SugaredLogger, serverHost, serverUser string)`
- 新增字段：`log`, `serverKey` ("user@host"), `filePath` (绝对路径)
- 新增 `loadFromDisk()`: 构造时从 `~/.lazyssh/recent-dirs/{user@host}.json` 加载历史数据，文件不存在时静默返回空切片
- 新增 `saveToDisk()`: 使用 `json.MarshalIndent` + `os.WriteFile` (权限 0o600) 持久化，错误仅日志不中断 Record 调用
- `Record()` 末尾调用 `saveToDisk()`
- 目录权限 0o750、文件权限 0o600，与 `metadata_manager.go` 一致

### Task 2: 移除导航记录 + 更新调用

**file_browser.go:**
- `OnPathChange` 回调不再调用 `Record()`（参数改为 `_ string`）
- `NewRecentDirs` 调用更新为 `NewRecentDirs(fb.log, fb.server.Host, fb.server.User)`
- 传输记录保留：`initiateTransfer` 和 `initiateDirTransfer` 成功后的 `Record` 调用不变
- 弹窗选择目录后的 `Record` 调用保留（MRU 语义）

**recent_dirs_test.go:**
- 所有测试用例更新为使用 `newTestRecentDirs(t)` 辅助函数，使用临时目录避免污染真实文件系统
- 新增 `TestPersistenceSaveAndLoad`: 验证写入-读取往返
- 新增 `TestPersistenceMissingFile`: 验证文件不存在时静默处理

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Test file compilation failure after constructor signature change**
- **Found during:** Task 2 verification (`go vet`)
- **Issue:** `recent_dirs_test.go` 中 16 处 `NewRecentDirs()` 调用缺少新参数，导致编译失败
- **Fix:** 创建 `newTestRecentDirs(t)` 测试辅助函数，使用 `t.TempDir()` 创建临时目录用于持久化隔离，手动构造 RecentDirs 结构体避免写入真实 `~/.lazyssh/` 路径。同时新增 2 个持久化测试用例。
- **Files modified:** `internal/adapters/ui/file_browser/recent_dirs_test.go`
- **Commit:** 589cea2

## Verification Results

- `go build ./...`: PASS
- `go vet ./...`: PASS (0 warnings)
- `go test ./internal/adapters/ui/file_browser/`: 20/20 PASS
- OnPathChange 回调不再调用 Record: Confirmed (line 144)
- initiateTransfer Record 保留: Confirmed (line 397)
- initiateDirTransfer Record 保留: Confirmed (line 503)
- NewRecentDirs 参数正确: Confirmed (line 107)

## Known Stubs

None.
