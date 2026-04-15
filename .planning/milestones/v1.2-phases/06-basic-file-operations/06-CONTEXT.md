# Phase 6: Basic File Operations - Context

**Gathered:** 2026-04-15
**Status:** Ready for planning

<domain>
## Phase Boundary

扩展 FileService/SFTPService port 接口，新增删除/重命名/新建目录操作（本地+远程双面板），并实现对应的 overlay UI 组件（ConfirmDialog + InputDialog）。不包含复制/移动功能（Phase 7/8）。
</domain>

<decisions>
## Implementation Decisions

### Overlay 组件设计
- **D-01:** ConfirmDialog 和 InputDialog 作为独立 overlay 组件（confirm_dialog.go, input_dialog.go），不扩展 TransferModal。遵循 TransferModal/RecentDirs 的 overlay 模式：嵌入 `*tview.Box`，`visible` 标志，`Draw()` + `HandleKey()` 手动管理。
- **D-02:** InputDialog 组件被重命名和新建目录共用，通过标题和预填内容区分用途。内部使用 `tview.InputField` 处理文本编辑，通过 `doneFunc` 回调处理 Enter/Esc。

### 删除确认 UX
- **D-03:** 单文件删除确认显示详细信息：文件名、大小、文件类型（目录/文件）、修改时间。格式类似 TransferModal 的 conflictDialog 模式。
- **D-04:** 多选批量删除显示"删除 N 个文件？共 X.XMB"的汇总确认，一个确认操作删除全部。
- **D-05:** 删除非空目录时额外显示递归警告："目录非空，将递归删除所有内容"。

### 重命名/新建目录 UX
- **D-06:** 重命名通过居中弹出 InputDialog 触发（R 键），预填当前文件名，光标位于文件名部分末尾（不含扩展名）。按 Enter 确认，Esc 取消。
- **D-07:** 新建目录通过居中弹出 InputDialog 触发（m 键），空输入框，按 Enter 创建，Esc 取消。创建后自动刷新列表并定位到新目录。
- **D-08:** 重命名目标名称已存在时，提示名称冲突，用户可选择覆盖或取消。

### 错误处理
- **D-09:** 文件操作失败时（权限不足、文件不存在等），在状态栏显示红色错误信息，几秒后自动恢复默认文本。不打断用户操作流程。

### Port 接口扩展
- **D-10:** 删除/重命名/新建目录方法提升到 FileService 接口（共享），而非仅添加到 SFTPService。这确保本地和远程面板使用统一接口。
- **D-11:** SFTPService 已有的 Remove（仅文件/空目录）保留，新增 RemoveAll（递归删除）、Rename（重命名）、Mkdir（创建单个目录）。pkg/sftp 原生支持这些操作。

### Claude's Discretion
- ConfirmDialog 的具体布局细节（颜色、间距、边距）由 Claude 基于 TransferModal 的 cancelConfirm/conflictDialog 模式决定
- InputDialog 中 tview.InputField 的焦点管理方式（手动调用 InputHandler 而非依赖 tview focus 系统）
- 状态栏错误信息闪烁的具体持续时间（建议 3 秒）
- 递归删除是否显示进度（小目录同步执行，大目录可考虑状态栏提示"删除中..."）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Port Interfaces
- `internal/core/ports/sftp_service.go` — FileService 和 SFTPService 接口定义，Phase 6 需扩展
- `internal/core/ports/file_service.go` — 不存在，接口在 sftp_service.go 中定义

### SFTP Client Adapter
- `internal/adapters/data/sftp_client/sftp_client.go` — SFTPService 实现，基于 pkg/sftp
- `internal/adapters/data/sftp_client/sftp_client_test.go` — 现有测试

### Local Filesystem Adapter
- `internal/adapters/data/localfs/` — LocalFS adapter（FileService 实现）

### UI Components
- `internal/adapters/ui/file_browser/file_browser.go` — FileBrowser orchestrator, Draw() overlay chain, AfterDrawFunc
- `internal/adapters/ui/file_browser/file_browser_handlers.go` — handleGlobalKeys 按键路由链
- `internal/adapters/ui/file_browser/transfer_modal.go` — TransferModal overlay 模式参考（多模式状态机、HandleKey、Draw）
- `internal/adapters/ui/file_browser/recent_dirs.go` — RecentDirs overlay 模式参考（简单 popup）
- `internal/adapters/ui/file_browser/local_pane.go` — LocalPane（h/Space/. 按键、InputCapture）
- `internal/adapters/ui/file_browser/remote_pane.go` — RemotePane（同上 + 连接状态管理）

### Research
- `.planning/research/STACK.md` — SFTP 原语和 Go stdlib 能力
- `.planning/research/ARCHITECTURE.md` — 接口扩展和 overlay 集成方案
- `.planning/research/PITFALLS.md` — TOCTOU、递归操作、符号链接等 pitfall

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **TransferModal overlay pattern**: 嵌入 `*tview.Box`，`visible` 标志，`Draw()` 按 mode 分发，`HandleKey()` 返回 nil 拦截所有按键。ConfirmDialog/InputDialog 应遵循此模式。
- **TransferModal.cancelConfirm mode**: "Cancel transfer?" + [y] Yes [n] No 布局参考。ConfirmDialog 可复用类似布局。
- **TransferModal.conflictDialog mode**: 三行布局（标题行 + 信息行 + 选项行）。InputDialog 可参考此布局添加 InputField。
- **handleGlobalKeys overlay interception**: 第 34-36 行检查 `recentDirs.IsVisible()` 后拦截所有按键。新增 overlay 需在此处添加类似的可见性检查。
- **FileBrowser.Draw() overlay chain**: 在 Flex.Draw() 后依次绘制 transferModal 和 recentDirs。新 overlay 需添加到此链。
- **app.QueueUpdateDraw()**: goroutine 中安全更新 UI 的标准模式。

### Established Patterns
- **Overlay 生命周期**: `Show()` 设置 visible=true + 更新内容 → `Hide()` 设置 visible=false → `Draw()` 检查 visible
- **按键路由**: overlay.HandleKey() 返回 nil 表示消费，返回 event 表示传递
- **异步操作**: goroutine 执行 + QueueUpdateDraw 回调 UI
- **状态栏**: `fb.statusBar.SetText()` + `updateStatusBarConnection()` 在 AfterDrawFunc 中渲染

### Integration Points
- **FileBrowser.handleGlobalKeys**: 添加 d/m/R 按键处理，以及新 overlay 的可见性检查
- **FileBrowser.Draw()**: 添加 ConfirmDialog/InputDialog 的 Draw 调用
- **LocalPane/RemotePane InputCapture**: 可能需要在 pane 级别拦截 d/m/R（或保持在 global 级别）
- **FileService interface**: 扩展 Remove/RemoveAll/Rename/Mkdir 方法
- **SFTPClient**: 实现 FileService 新增方法，利用 pkg/sftp 原生支持

</code_context>

<specifics>
## Specific Ideas

- ConfirmDialog 布局参考 TransferModal 的 cancelConfirm 和 conflictDialog 模式
- InputDialog 使用 tview.InputField，通过 `inputField.InputHandler()` 手动路由按键，避免依赖 tview focus 系统
- 重命名时光标定位到扩展名前（如 "config|.yaml"），需要解析文件名找到最后一个 '.' 的位置
- 状态栏错误信息闪烁使用 goroutine + time.After + QueueUpdateDraw 恢复默认文本

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 06-basic-file-operations*
*Context gathered: 2026-04-15*
