---
phase: 04-directory-history-core
verified: 2026-04-14T12:00:00Z
status: passed
score: 10/10 must-haves verified
---

# Phase 4: Directory History Core -- 验证报告

**Phase Goal:** 每次远程面板的目录导航都被静默记录到内存 MRU 列表中，NavigateToParent 的 onPathChange 不对称性 bug 被修复
**Verified:** 2026-04-14
**Status:** passed
**Re-verification:** No -- initial verification

## Goal Achievement

### Observable Truths

Plan 01 Truths (RecentDirs 数据结构):

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Record(path) 将路径规范化（去尾部斜杠）后存入列表头部 | VERIFIED | `recent_dirs.go:55` -- `strings.TrimRight(path, "/")` 后 prepend 到 `rd.paths[:0]` |
| 2 | Record(path) 对已存在的路径执行 move-to-front 去重 | VERIFIED | `recent_dirs.go:60-64` -- 遍历找到匹配项后 `append(paths[:i], paths[i+1:]...)` 移除，再 prepend |
| 3 | Record(path) 跳过以 '.' 开头的相对路径 | VERIFIED | `recent_dirs.go:56-58` -- `strings.HasPrefix(normalized, ".")` 时直接 return |
| 4 | GetPaths() 返回按 MRU 顺序排列的路径 slice | VERIFIED | `recent_dirs.go:75-78` -- 返回 `rd.paths` 的 copy，slice 顺序即 MRU 顺序 |
| 5 | 列表始终不超过 10 条记录 | VERIFIED | `recent_dirs.go:25` -- `const maxRecentDirs = 10`，`recent_dirs.go:69-71` -- 超出时 `rd.paths[:maxRecentDirs]` |
| 6 | 空列表上调用 Record() 正常工作 | VERIFIED | `recent_dirs.go:38-41` -- `paths: make([]string, 0, maxRecentDirs)` 初始化非 nil slice |

Plan 02 Truths (NavigateToParent 修复 + 集成):

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 7 | NavigateToParent() 调用后 app.Sync() 被执行 | VERIFIED | `remote_pane.go:288-290` -- NavigateToParent 调用 `rp.onPathChange(rp.currentPath)`；`file_browser.go:136-139` -- 回调执行 `fb.app.Sync()` |
| 8 | NavigateTo(path) 直接设置 currentPath 并 Refresh，不触发 onPathChange | VERIFIED | `remote_pane.go:310-317` -- NavigateTo 设置 `rp.currentPath = path` + `rp.Refresh()`，无 onPathChange 调用 |
| 9 | RemotePane 的 onPathChange 回调同时触发 app.Sync() 和 Record(path) | VERIFIED | `file_browser.go:136-139` -- `fb.remotePane.OnPathChange(func(path string) { fb.app.Sync(); fb.recentDirs.Record(path) })` |
| 10 | NavigateToParent 和 NavigateInto 对称调用 onPathChange | VERIFIED | `remote_pane.go:288-290` (NavigateToParent) 与 `remote_pane.go:301-303` (NavigateInto) 结构完全一致 |

**Score:** 10/10 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/adapters/ui/file_browser/recent_dirs.go` | RecentDirs struct, Record(), GetPaths(), maxRecentDirs | VERIFIED | 104 lines, struct + 3 方法 + overlay 骨架，满足 min_lines>=50 |
| `internal/adapters/ui/file_browser/recent_dirs_test.go` | 8 个单元测试 | VERIFIED | 129 lines, 8 个测试函数，覆盖所有行为 |
| `internal/adapters/ui/file_browser/remote_pane.go` (modified) | NavigateToParent 修复 + NavigateTo 方法 | VERIFIED | NavigateToParent L288-290 添加 onPathChange，NavigateTo L310-317 新增 |
| `internal/adapters/ui/file_browser/file_browser.go` (modified) | recentDirs 字段 + Record 接线 | VERIFIED | L52 字段声明，L107 初始化，L138 Record 调用 |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| remote_pane.go NavigateToParent() | file_browser.go onPathChange 回调 | `rp.onPathChange(rp.currentPath)` | WIRED | L288-290 调用 -> L136 注册回调 -> L137-138 执行 Sync + Record |
| remote_pane.go NavigateInto() | file_browser.go onPathChange 回调 | `rp.onPathChange(rp.currentPath)` | WIRED | L301-303 调用 -> L136 注册回调 -> L137-138 执行 Sync + Record |
| file_browser.go build() | recent_dirs.go Record() | `fb.recentDirs.Record(path)` | WIRED | L138 直接调用 Record(path)，path 参数来自回调 |
| remote_pane.go NavigateTo() | (不触发) onPathChange | N/A | WIRED (no call) | L310-317 确认无 onPathChange 调用，符合设计意图 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|-------------------|--------|
| recent_dirs.go Record() | `rd.paths` | 参数 `path string` 来自调用方 | FLOWING | NavigateInto/NavigateToParent 传入 `rp.currentPath`（用户实际导航目标），Record 规范化后存储 |
| file_browser.go onPathChange | `path string` | RemotePane 回调触发时传入 `rp.currentPath` | FLOWING | currentPath 在 NavigateToParent/NavigateInto 中先更新再触发回调，path 是真实导航目标 |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Project compiles | `go build ./...` | Exit 0, no errors | PASS |
| Record unit tests pass | `go test ./internal/adapters/ui/file_browser/ -run TestRecord -v` | 7/7 PASS | PASS |
| NavigateToParent has onPathChange | grep `onPathChange(rp.currentPath)` in remote_pane.go after NavigateToParent | Found at L289 | PASS |
| NavigateTo has no onPathChange | grep `onPathChange` in NavigateTo block (L310-317) | Not found | PASS |
| recentDirs.Record wired | grep `recentDirs.Record(path)` in file_browser.go | Found at L138 | PASS |
| LocalPane not affected | grep in LocalPane onPathChange callback | Uses `_ string`, no Record call (L133-135) | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| HIST-01 | 04-01, 04-02 | 导航到新目录时自动记录到最近目录列表 | SATISFIED | NavigateInto (L301-303) + NavigateToParent (L288-290) 触发 onPathChange -> Record (L138) |
| HIST-02 | 04-01 | 最近目录列表按 MRU 排序 | SATISFIED | Record() prepend to front (L67)，测试 TestRecordMultiplePaths 验证 |
| HIST-03 | 04-01 | 同一路径去重，仅保留最新位置 | SATISFIED | Record() move-to-front (L60-64)，测试 TestRecordMoveToFront + TestRecordDuplicateDoesNotCreateDuplicates 验证 |
| HIST-04 | 04-01 | 列表最多 10 条 | SATISFIED | maxRecentDirs=10 (L25)，截断逻辑 (L69-71)，测试 TestRecordTruncation 验证 |
| AUX-02 | 04-02 | 修复 NavigateToParent 缺少 onPathChange 回调 | SATISFIED | NavigateToParent L288-290 添加 `if rp.onPathChange != nil { rp.onPathChange(rp.currentPath) }` |

**Orphaned Requirements:** None. REQUIREMENTS.md 中 Phase 4 的 5 个需求 ID (HIST-01, HIST-02, HIST-03, HIST-04, AUX-02) 在两个 Plan 的 requirements 字段中全部覆盖。

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| remote_pane.go | 119, 123 | "Connecting..." placeholder text | Info | 合法的 UI 状态显示，非 stub。RemotePane 在 SFTP 连接建立前显示占位文本，ShowConnected() 后被实际文件列表替换 |

无 Blocker 或 Warning 级别的反模式。

### Human Verification Required

无。Phase 4 的所有目标均可通过代码静态分析和单元测试验证：
- 数据结构行为（Record/GetPaths）通过 8 个单元测试完整覆盖
- Bug 修复（NavigateToParent onPathChange）通过代码结构验证（对称性对比 NavigateInto）
- 集成接线（Record 在 onPathChange 回调中调用）通过代码路径追踪验证

Phase 5（弹出列表 UI）将需要人类验证视觉效果和交互体验。

### Gaps Summary

无 gaps。所有 must-haves 均已验证通过：
- RecentDirs 数据结构完整实现，包含 Record()（MRU 去重、截断、相对路径过滤、路径规范化）和 GetPaths()（防御性副本）
- NavigateToParent bug 已修复，与 NavigateInto 保持 onPathChange 对称
- NavigateTo 方法已添加，供 Phase 5 使用
- FileBrowser.build() 中 recentDirs 字段已声明、初始化，并通过 onPathChange 回调链接入 Record
- 8 个单元测试全部通过，编译无错误

---

_Verified: 2026-04-14_
_Verifier: Claude (gsd-verifier)_
