# Phase 4: Directory History Core - Context

**Gathered:** 2026-04-14
**Status:** Ready for planning

<domain>
## Phase Boundary

构建内存 MRU 目录列表数据结构（RecentDirs），自动记录远程面板的每次目录导航（NavigateInto 和 NavigateToParent），修复 NavigateToParent 缺少 onPathChange 回调的预存 bug。本阶段不含 UI 弹出功能——仅数据层 + bug 修复。

</domain>

<decisions>
## Implementation Decisions

### Data Structure
- **D-01:** 使用 `[]string` 有序 slice（非 map）维护 MRU 列表，最多 10 条。10 条上限下 O(n) 去重完全足够，无需 container/list。
- **D-02:** Record 方法实现 move-to-front 去重：先移除已有条目，再 prepend 到头部，最后截断到 10 条。
- **D-03:** RecentDirs 作为独立 struct，嵌入 `*tview.Box`（与 TransferModal 一致），存放在 `file_browser/recent_dirs.go`。数据存储在 RecentDirs 内部，由 FileBrowser 持有。

### Path Recording
- **D-04:** 路径记录通过现有 `onPathChange` 回调实现——在 FileBrowser.build() 中为 RemotePane 的 onPathChange 添加 `fb.recentDirs.Record(path)` 调用。
- **D-05:** 不记录相对路径。如果路径以 `"."` 开头（如 `"."`, `"./docs"`），跳过记录。只有在 NavigateToParent 返回到绝对路径后才开始记录。
- **D-06:** 路径规范化仅去尾部斜杠（`strings.TrimRight(path, "/")`），不做完整路径解析（SFTP 远程路径通常是规范绝对路径）。
- **D-07:** Phase 5 中用户从最近列表选择路径跳转后，该路径重新提升到列表头部（调用 Record）。

### Bug Fix
- **D-08:** 修复 `RemotePane.NavigateToParent()` 缺少 `onPathChange` 回调——在方法末尾添加 `if rp.onPathChange != nil { rp.onPathChange(rp.currentPath) }`。这同时修复了返回上级时 `app.Sync()` 未调用的问题。

### NavigateTo Method
- **D-09:** 在 RemotePane 上添加 `NavigateTo(path string)` 方法——直接设置 currentPath 并 Refresh，不触发 onPathChange 回调。用于 Phase 5 的弹出列表选择跳转（避免通过 NavigateInto 间接触发）。

### Claude's Discretion
- RecentDirs 的 Draw() 方法实现细节（颜色、宽度、边距等）留给 Phase 5 规划
- NavigateTo 方法是否需要调用 UpdateTitle() —— 当前设计不调用，因为 Refresh() 已内部调用

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### File Browser Core
- `internal/adapters/ui/file_browser/file_browser.go` — FileBrowser struct, build() method, OnPathChange wiring, handleGlobalKeys
- `internal/adapters/ui/file_browser/remote_pane.go` — RemotePane struct, NavigateInto, NavigateToParent (bug location), onPathChange callback
- `internal/adapters/ui/file_browser/file_browser_handlers.go` — Key routing chain documentation (lines 22-26 comments)
- `internal/adapters/ui/file_browser/transfer_modal.go` — TransferModal overlay pattern reference (struct, Draw, HandleKey, Show/Hide, visible flag)

### Research Artifacts
- `.planning/research/ARCHITECTURE.md` — Complete integration analysis, data flow, build order, NavigateToParent asymmetry finding
- `.planning/research/PITFALLS.md` — 11 pitfalls with prevention strategies (Phase 4 addresses P1, P5, P6, P9)
- `.planning/research/FEATURES.md` — Feature table stakes, MVP definition, competitor analysis
- `.planning/research/STACK.md` — Zero new dependencies confirmed

### Requirements & Planning
- `.planning/REQUIREMENTS.md` — HIST-01 through HIST-04, AUX-02
- `.planning/ROADMAP.md` — Phase 4 goal, success criteria

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `TransferModal` pattern: `*tview.Box` embed, `visible` flag, `Show()`/`Hide()`/`IsVisible()`, `HandleKey()` dispatch, manual `Draw()` — RecentDirs 应遵循相同模式
- `onPathChange` callback chain: FileBrowser.build() 已为两个 pane 注册 onPathChange，只需在 RemotePane 的回调中追加 Record 调用
- `joinPath()` helper: `file_browser/remote_pane.go` 中的 Unix-style path join 函数

### Established Patterns
- Callback pattern for pane events: RemotePane 定义回调槽（onPathChange, onFileAction），FileBrowser 在 build() 中注册
- Key routing: pane-specific keys in Pane.SetInputCapture, global keys in FileBrowser.handleGlobalKeys
- Overlay rendering: custom Draw() in FileBrowser.Draw() after fb.Flex.Draw(screen)

### Integration Points
- `RemotePane.NavigateToParent()` (line 276-288): 需要在 line 287 之后添加 onPathChange 调用
- `FileBrowser.build()` (line 128-133): RemotePane 的 onPathChange 回调中添加 Record 调用
- `RemotePane` struct (line 30-42): 添加 `onShowRecentDirs` 回调字段（Phase 5 使用，Phase 4 可预留）
- `FileBrowser.Draw()` (line 209-218): Phase 5 在此处添加 overlay Draw 调用

</code_context>

<specifics>
## Specific Ideas

- Record 方法伪代码参考（来自 PITFALLS.md P9）:
  ```go
  func (rd *RecentDirs) Record(path string) {
      normalized := strings.TrimRight(path, "/")
      if strings.HasPrefix(normalized, ".") { return } // skip relative paths
      for i, p := range rd.paths {
          if p == normalized {
              rd.paths = append(rd.paths[:i], rd.paths[i+1:]...)
              break
          }
      }
      rd.paths = append([]string{normalized}, rd.paths...)
      if len(rd.paths) > maxRecentDirs { rd.paths = rd.paths[:maxRecentDirs] }
  }
  ```
- NavigateToParent fix: 一行改动 + 触发现有 app.Sync() 的副作用修复
- NavigateTo 方法: NavigateInto 的简化版，不触发 onPathChange，Phase 5 使用

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 04-directory-history-core*
*Context gathered: 2026-04-14*
