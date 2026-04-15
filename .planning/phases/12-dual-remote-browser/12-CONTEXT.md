# Phase 12: Dual Remote Browser - Context

**Gathered:** 2026-04-16
**Status:** Ready for planning

<domain>
## Phase Boundary

创建独立的 DualRemoteFileBrowser 组件，左栏为远端源端（RemotePane A），右栏为远端目标端（RemotePane B），各自持有独立的 SFTPClient 实例。支持键盘导航（Tab/方向键/Enter/h）、同面板内文件操作（删除/重命名/新建目录），以及并行 SFTP 连接。不包含跨远端传输（Phase 13）。

</domain>

<decisions>
## Implementation Decisions

### 布局设计
- **D-01:** 50:50 Flex 布局，与现有 FileBrowser 一致。上方添加 header bar 显示两个服务器的别名和 IP 地址，格式如 "Source: myserver (1.2.3.4) | Target: otherserver (5.6.7.8)"。
- **D-02:** 面板内文件列表复用 FileBrowser 的 4 列格式（Name, Size, Modified, Permissions），保持一致性。
- **D-03:** 活跃面板通过高亮边框或不同背景色标识，与 FileBrowser 的 Tab 切换体验一致。

### 面板标签
- **D-04:** 每个面板顶部显示服务器别名 + IP，如 "Source: myserver (1.2.3.4)"。与 FileBrowser 的路径显示风格一致。目标端面板显示 "Target: otherserver (5.6.7.8)"。

### 同面板内文件操作
- **D-05:** Phase 12 包含每个远程面板内的删除（d）、重命名（R）、新建目录（m）操作。直接复用 Phase 6 的 ConfirmDialog 和 InputDialog overlay 组件。
- **D-06:** 不包含同服务器内的复制/移动（c/x + p），这些操作在 Phase 13 与跨远端传输一起实现。

### 连接管理
- **D-07:** 两个 SFTP 连接并行建立（goroutine 并发），用户体验更快。连接状态在每个面板内显示（Connecting/Connected/Error）。
- **D-08:** 一个连接失败时，失败面板显示错误信息，另一个正常面板可继续浏览。用户可手动按 Esc 退出。不自动关闭整个浏览器。

### 状态栏与快捷键
- **D-09:** 底部状态栏显示：两个服务器别名、两个连接状态（Connected/Error）、活跃面板指示、快捷键提示。格式与 FileBrowser 状态栏一致（• 分隔）。
- **D-10:** 快捷键方案与 FileBrowser 完全一致：Tab 切换面板、Esc 退出、d 删除、R 重命名、m 新建目录、/ 搜索、. 隐藏文件、Enter 进入目录、h 返回上级、Space 多选。
- **D-11:** Esc 关闭 DualRemoteFileBrowser 并清理两个 SFTP 连接（DRB-04 已锁定）。

### Claude's Discretion
- Header bar 的具体颜色和样式（基于 tview/tcell 现有颜色方案）
- 活跃面板高亮的具体实现方式（边框颜色 vs 背景色 vs 两者）
- 状态栏的具体文本格式和布局
- 连接失败时的具体错误信息措辞
- ConfirmDialog/InputDialog 在 DualRemoteFileBrowser 中的集成方式（作为独立字段还是共享引用）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` — DRB-01~DRB-04 需求定义

### Entry point
- `internal/adapters/ui/handlers.go:189` — `handleDualRemoteBrowser(source, target domain.Server)` 占位函数（Phase 11 创建，Phase 12 填充实现）

### TUI integration
- `internal/adapters/ui/tui.go` — tui struct，handleDualRemoteBrowser 将在此创建 DualRemoteFileBrowser 并显示
- `internal/adapters/ui/handlers.go:151` — `handleServerMark()` 方法（调用 handleDualRemoteBrowser 的入口）

### Reusable components
- `internal/adapters/ui/file_browser/remote_pane.go` — RemotePane 组件（DRB-02 明确复用），包含 SFTP 连接管理、文件列表渲染、键盘导航
- `internal/adapters/ui/file_browser/file_browser.go` — FileBrowser 组件（参考布局、overlay chain、状态栏模式）
- `internal/adapters/ui/file_browser/file_browser_handlers.go` — handleGlobalKeys 按键路由（参考快捷键方案）
- `internal/adapters/ui/file_browser/confirm_dialog.go` — ConfirmDialog overlay（D-05 复用）
- `internal/adapters/ui/file_browser/input_dialog.go` — InputDialog overlay（D-05 复用）

### Port interfaces
- `internal/core/ports/sftp_service.go` — SFTPService 接口（Connect, ListDir, Remove, Rename, Mkdir 等）
- `internal/adapters/data/sftp_client/sftp_client.go` — SFTPClient 实现（基于 pkg/sftp，通过系统 SSH 二进制建立连接）

### Prior phase context
- `.planning/phases/11-t-key-marking/11-CONTEXT.md` — T 键标记上下文，handleDualRemoteBrowser 入口设计
- `.planning/phases/06-basic-file-operations/06-CONTEXT.md` — Overlay 组件设计模式

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **RemotePane**: 完整的远程文件浏览组件，包含 SFTP 连接管理、文件列表渲染（4 列表格）、键盘导航（Tab/Enter/h/Space）、连接状态显示（Connecting/Connected/Error）。DualRemoteFileBrowser 需要两个独立实例。
- **ConfirmDialog/InputDialog**: 独立 overlay 组件，遵循 TransferModal 的 overlay 模式（嵌入 *tview.Box, visible 标志, Draw/HandleKey）。可直接在 DualRemoteFileBrowser 中复用。
- **FileBrowser 布局模式**: 50:50 Flex 布局 + 状态栏 + overlay chain。DualRemoteFileBrowser 遵循相同模式。
- **SFTPService**: Connect(server) 建立连接，ListDir() 列出文件，Remove/Rename/Mkdir 文件操作。每个 RemotePane 需要独立实例。

### Established Patterns
- **Overlay 生命周期**: Show() → HandleKey() → Hide()，visible 标志控制
- **按键路由**: handleGlobalKeys 中 overlay 优先拦截，然后 switch 分发
- **异步操作**: goroutine 执行 + app.QueueUpdateDraw() 回调 UI
- **连接状态**: RemotePane.ShowConnecting() → 连接成功后 ShowConnected() → 失败则显示错误

### Integration Points
- `handleDualRemoteBrowser()` (handlers.go:189) — 填充实现，创建 DualRemoteFileBrowser
- tui struct — 需要持有 DualRemoteFileBrowser 引用或通过 Modal 方式显示
- SFTPService 实例化 — 需要为源端和目标端各创建一个独立实例

</code_context>

<specifics>
## Specific Ideas

- DualRemoteFileBrowser 作为独立组件（DRB-01），不复用 FileBrowser，但内部复用 RemotePane
- 两个 RemotePane 实例各自持有独立的 SFTPClient，并行连接
- Header bar 使用 tview.TextView 显示服务器信息，位于两个面板上方
- 状态栏复用 FileBrowser 的模式：tview.TextView + SetDynamicColors + SetTextAlign
- ConfirmDialog/InputDialog 作为 DualRemoteFileBrowser 的字段，Draw() 中添加到 overlay chain
- Esc 先检查 overlay 可见性（清除 overlay），再关闭整个浏览器

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 12-dual-remote-browser*
*Context gathered: 2026-04-16*
