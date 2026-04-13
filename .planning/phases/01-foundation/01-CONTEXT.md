# Phase 1: Foundation - Context

**Gathered:** 2026-04-13
**Status:** Ready for planning

<domain>
## Phase Boundary

构建双栏文件浏览器的基础 UI 框架和本地文件浏览能力，同时建立 SFTP 连接基础设施。此阶段交付后，用户可以打开双栏界面、浏览本地文件、建立 SFTP 连接，但远程浏览和文件传输功能在 Phase 2 实现。
</domain>

<decisions>
## Implementation Decisions

### File List Component
- **D-01:** 使用 `tview.Table` 组件显示文件列表（而非 `tview.List`）
- **D-02:** 每行显示 4 列：Name, Size, Modified date, Permissions (drwxr-xr-x)
- **D-03:** 目录用特殊标识区分（如 `/` 后缀或不同颜色）

### Dual-Pane Layout
- **D-04:** 双栏宽度比例 50:50（`tview.FlexColumn`，各占 1:1）
- **D-05:** 每个 pane 顶部显示当前路径（作为 pane 的 Title）
- **D-06:** 遵循现有 TUI 布局模式：`app.SetRoot(fileBrowser, true)` 覆盖全屏，Esc 返回主界面

### SFTP Connection Behavior
- **D-07:** 按 `F` 打开文件浏览器时立即建立 SFTP 连接（非 lazy connect）
- **D-08:** 连接失败时在右栏 pane 内显示错误信息 + 原因，左栏仍可正常浏览本地文件
- **D-09:** 使用 `pkg/sftp` 的 `NewClientPipe()` 通过系统 SSH binary 建立连接

### Navigation Behavior
- **D-10:** 初始目录：本地 `~` (home dir)，远程 `~` (SSH default home)
- **D-11:** 返回上级目录快捷键：`Backspace` + `h` 均支持
- **D-12:** `Tab` 切换左右 pane 焦点
- **D-13:** `Space` 标记/取消标记文件（多选）

### Keyboard Shortcuts
- **D-14:** 快捷键 `F` (Shift+f) 触发文件浏览器（`f` 已被端口转发占用）
- **D-15:** `Esc` 关闭文件浏览器，返回主界面（`returnToMain()`）

### Claude's Discretion
- 文件大小显示格式（human readable: 1.2K, 3.4M vs bytes）
- 目录排序规则（目录优先 vs 混合排序）
- 空目录显示文本
- 文件类型图标/颜色编码
- 表格列宽分配策略

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing Codebase
- `internal/adapters/ui/tui.go` — Main TUI struct, layout building, component initialization
- `internal/adapters/ui/handlers.go` — Global key handling, `handleGlobalKeys()`, `returnToMain()`, `showStatusTemp()`
- `internal/adapters/ui/server_list.go` — Existing tview.List usage pattern (reference for tview component style)
- `internal/adapters/ui/status_bar.go` — Status bar component pattern
- `internal/core/ports/services.go` — Service interface pattern
- `internal/core/ports/repositories.go` — Repository interface pattern
- `internal/core/services/server_service.go` — Service implementation pattern
- `cmd/main.go` — Dependency injection pattern

### Research
- `.planning/research/STACK.md` — pkg/sftp library decision, key binding conflict analysis
- `.planning/research/ARCHITECTURE.md` — New component structure, integration points, build order
- `.planning/research/PITFALLS.md` — P4 (f key conflict), P5 (UI thread blocking), P9 (connection cleanup)
- `.planning/research/FEATURES.md` — UX key binding patterns from Midnight Commander

### Requirements
- `.planning/REQUIREMENTS.md` — Phase 1 requirements: UI-01, UI-02, UI-03, UI-04, UI-05, UI-07, UI-08, BROW-01, BROW-03, BROW-04, BROW-05, BROW-06, INTG-01, INTG-02

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `tui.returnToMain()` — Returns to server list view (reuse for Esc in file browser)
- `tui.showStatusTemp()` / `showStatusTempColor()` — Temporary status messages
- `tui.app.SetRoot()` — View switching pattern (use for opening/closing file browser)
- `tui.handleGlobalKeys()` — Key binding registration pattern (add `case 'F'`)
- `tui.app.QueueUpdateDraw()` — Thread-safe UI updates (essential for async SFTP operations)
- `tui.serverList.GetSelectedServer()` — Get current server for SFTP connection

### Established Patterns
- Components initialized in `buildComponents()`, layout in `buildLayout()`, events in `bindEvents()`
- Full-screen views via `app.SetRoot(component, true)`, return via `returnToMain()`
- Background operations use goroutines + `app.QueueUpdateDraw()` for UI updates
- Error display: `showStatusTempColor(msg, "#FF6B6B")` for red error messages
- Theme: Dark theme with tcell.Color232-250 palette, defined in `initializeTheme()`

### Integration Points
- `handlers.go:83-85` — Add `case 'F':` for file transfer entry
- `tui.go:51-59` — `NewTUI()` constructor may need FileTransferService injection
- `tui.go:90-108` — `buildComponents()` may need file browser component initialization
- `tui.go:110-127` — `buildLayout()` shows Flex pattern to follow

</code_context>

<specifics>
## Specific Ideas

- Right pane 在 SFTP 连接建立前显示 "Connecting..." 占位文本
- 隐藏文件默认不显示，用快捷键切换（如 `.` 或 `Ctrl+H`）
- 排序切换快捷键参考现有服务器列表的 `s`/`S` 模式
- 遵循 Midnight Commander 的 Tab 切换面板焦点模式

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 01-foundation*
*Context gathered: 2026-04-13*
