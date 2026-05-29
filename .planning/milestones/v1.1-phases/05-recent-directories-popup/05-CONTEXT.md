# Phase 5: Recent Directories Popup - Context

**Gathered:** 2026-04-14
**Status:** Ready for planning

<domain>
## Phase Boundary

为 Phase 4 创建的 RecentDirs 数据结构添加完整的 UI 弹出层。用户在远程面板按 `r` 键弹出居中目录列表，通过 j/k 导航、Enter 跳转、Esc 关闭。包含空状态处理和当前路径高亮（AUX-01）。本阶段是纯 UI 层，不涉及数据结构变更。

</domain>

<decisions>
## Implementation Decisions

### 列表布局和尺寸
- **D-01:** 弹窗居中显示，宽度 = 终端宽度的 60%（最大 80 列），高度 = min(路径数量 + 2, 15) 行。+2 为标题行 + 内边距。
- **D-02:** 列表每行显示完整绝对路径（如 `/home/user/projects/my-app`），不缩写。
- **D-03:** 列表最多 10 行（与 `maxRecentDirs` 一致），不滚动。路径少于 10 条时弹窗高度自适应缩小。

### 颜色和视觉风格
- **D-04:** 选中项使用 `tcell.Color236` 深灰背景 + `tcell.Color250` 白色文字。未选中行为默认黑底白字。
- **D-05:** 当前路径条目（AUX-01）使用 `tcell.ColorYellow` 黄色文字显示，与其他条目的白色形成对比。如果当前路径恰好是选中项，同时应用选中背景色 + 黄色文字。
- **D-06:** 空状态时居中显示灰色文字「暂无最近目录」，使用 `tcell.Color240` 灰色。

### 快捷键和交互行为
- **D-07:** `r` 键仅在远程面板获得焦点（`fb.activePane == 1`）时触发弹窗。本地面板按 `r` 无效。
- **D-08:** 弹窗可见时，完全拦截所有按键——j/k/上下方向键/Enter/Esc 全部由弹窗的 HandleKey 处理，不传递到下层组件。
- **D-09:** 按 Esc 关闭弹窗，焦点恢复到远程面板。不额外调用 app.Sync()（关闭弹窗后 FileBrowser.Draw 会自然重绘）。
- **D-10:** 按 Enter 选中路径后，调用 `RemotePane.NavigateTo(path)`（Phase 4 已添加，不触发 onPathChange），然后关闭弹窗。选中路径同时调用 `Record(path)` 将其提升到 MRU 列表头部（D-07 from Phase 4 CONTEXT.md）。

### Draw() 渲染实现
- **D-11:** 使用手动渲染方式——在 Draw() 中用 `tview.Print()` 逐行渲染文本，手动计算位置和颜色。与 TransferModal 的渲染模式一致，完全控制每个像素。
- **D-12:** 选中项仅通过背景色区分，不额外显示 `>` 或 `▸` 符号。
- **D-13:** 弹窗顶部显示「 Recent Directories 」标题，使用 `rd.SetTitle(" Recent Directories ")`，与 TransferModal 的标题风格一致。

### Claude's Discretion
- Draw() 中边框内边距（上下左右 padding）的具体像素值
- HandleKey 中 j/k 与上下方向键的具体实现细节（直接修改 selectedIndex 还是用 tview 内置机制）
- Show() 时是否需要调用 app.ForceDraw() 确保立即渲染

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### File Browser Core
- `internal/adapters/ui/file_browser/file_browser.go` — FileBrowser struct, build(), handleGlobalKeys, Draw(), recentDirs field
- `internal/adapters/ui/file_browser/file_browser_handlers.go` — handleGlobalKeys key routing chain (lines 22-26 comments), switchFocus, close
- `internal/adapters/ui/file_browser/remote_pane.go` — RemotePane struct, NavigateTo() method (Phase 4), NavigateInto, GetCurrentPath(), onPathChange
- `internal/adapters/ui/file_browser/recent_dirs.go` — RecentDirs struct, Record(), GetPaths(), Show/Hide/IsVisible, Draw() skeleton
- `internal/adapters/ui/file_browser/transfer_modal.go` — TransferModal overlay pattern reference (struct, Draw, HandleKey, Show/Hide, visible flag, multi-mode state machine)

### Research Artifacts
- `.planning/research/ARCHITECTURE.md` — Complete integration analysis, data flow, overlay rendering approach
- `.planning/research/PITFALLS.md` — 11 pitfalls with prevention strategies (Phase 5 addresses P2, P3, P4, P7, P8, P10, P11)
- `.planning/research/FEATURES.md` — Feature table stakes, MVP definition, competitor analysis
- `.planning/research/STACK.md` — Zero new dependencies confirmed

### Phase 4 Context (Locked Decisions)
- `.planning/phases/04-directory-history-core/04-CONTEXT.md` — D-03 (overlay pattern), D-07 (re-record on popup select), D-09 (NavigateTo method)

### Requirements & Planning
- `.planning/REQUIREMENTS.md` — POPUP-01 through POPUP-05, AUX-01
- `.planning/ROADMAP.md` — Phase 5 goal, success criteria

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `RecentDirs` struct: 已有 `*tview.Box` embed, `visible` flag, `Show()`/`Hide()`/`IsVisible()`, `Draw()` skeleton, `Record()`, `GetPaths()` — Phase 5 补全 Draw() 和添加 HandleKey
- `TransferModal` overlay pattern: `*tview.Box` embed, `visible` flag, `Show()`/`Hide()`/`IsVisible()`, `HandleKey()` dispatch, manual `Draw()` — RecentDirs 应遵循相同模式
- `handleGlobalKeys` key routing: 已有 Esc 拦截 TransferModal 的模式，`r` 键拦截可复用相同结构
- `NavigateTo(path)`: Phase 4 已添加到 RemotePane，不触发 onPathChange，供弹出列表跳转使用
- `GetPaths()`: 返回 MRU 列表的防御性拷贝
- `GetCurrentPath()`: RemotePane 已有，用于判断当前路径条目高亮

### Established Patterns
- Overlay rendering: custom Draw() 在 FileBrowser.Draw() 中 `fb.Flex.Draw(screen)` 之后调用
- Key routing: global keys in FileBrowser.handleGlobalKeys, pane-specific keys in Pane.SetInputCapture
- Modal event interception: TransferModal.HandleKey 返回 nil 拦截事件，return event 传递事件
- Focus management: `fb.app.SetFocus(component)` 切换焦点

### Integration Points
- `FileBrowser.handleGlobalKeys()` (file_browser_handlers.go): 添加 `case 'r':` 拦截，检查 `fb.activePane == 1` 后调用 `fb.recentDirs.Show()`
- `RecentDirs.Draw()` (recent_dirs.go): 补全渲染逻辑——标题、路径列表、选中项高亮、当前路径标记、空状态
- `RecentDirs.HandleKey()` (recent_dirs.go): 新增方法，处理 j/k/上/下/Enter/Esc
- `FileBrowser.Draw()` (file_browser.go line 224-233): 在 `fb.Flex.Draw(screen)` 之后添加 `fb.recentDirs.Draw(screen)` overlay 调用
- `RemotePane.NavigateTo(path)` (remote_pane.go line 310): Enter 选中时调用
- `RemotePane.GetCurrentPath()` (remote_pane.go): 用于 AUX-01 当前路径比较

</code_context>

<specifics>
## Specific Ideas

- 弹窗尺寸计算伪代码:
  ```go
  width := termWidth * 60 / 100
  if width > 80 { width = 80 }
  height := len(paths) + 2 // +2 for title + padding
  if height > 15 { height = 15 }
  if height < 3 { height = 3 } // minimum: title + 1 empty line + border
  x := (termWidth - width) / 2
  y := (termHeight - height) / 2
  ```
- 当前路径比较逻辑:
  ```go
  currentPath := strings.TrimRight(fb.remotePane.GetCurrentPath(), "/")
  if path == currentPath { /* yellow text */ }
  ```
- Enter 选中后的完整流程: `fb.recentDirs.Hide()` → `fb.remotePane.NavigateTo(selectedPath)` → `fb.recentDirs.Record(selectedPath)` → `fb.app.SetFocus(fb.remotePane)`
- 空状态渲染: 当 `len(rd.paths) == 0` 时，Draw() 在弹窗中央渲染灰色「暂无最近目录」文本

</specifics>

<deferred>
## Deferred Ideas

- 路径缩写显示（过长路径缩写中间部分）— v1.x
- 数字键快速选择（按 1-9 直接跳转）— v1.x
- 持久化书签/收藏夹 — v2+

</deferred>

---

*Phase: 05-recent-directories-popup*
*Context gathered: 2026-04-14*
