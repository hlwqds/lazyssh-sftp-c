# Phase 7: Copy & Clipboard - Context

**Gathered:** 2026-04-15
**Status:** Ready for planning

<domain>
## Phase Boundary

面板内文件复制功能：用户按 `c` 标记当前文件为复制源（[C] 前缀），导航到目标目录后按 `p` 粘贴。剪贴板跨目录导航保持。远程复制通过 download+re-upload 实现，显示统一进度视图。不包含移动功能（Phase 8）和跨面板复制（v1.3+）。
</domain>

<decisions>
## Implementation Decisions

### 远程复制策略
- **D-01:** 远程复制通过 TransferService 新增 `CopyRemoteFile`/`CopyRemoteDir` 方法实现，内部复用现有 download+re-upload 基础设施（32KB buffer、onProgress 回调、conflict handler、ctx 取消传播）。不新建 CopyService port。
- **D-02:** 本地复制通过 FileService 新增 `Copy`/`CopyDir` 方法实现，底层使用 `io.Copy` + `os.Chtimes` + `os.Chmod`。不调用外部 cp 命令。

### 剪贴板设计
- **D-03:** 剪贴板为单文件模式——`c` 只标记当前光标所在文件，不支持 Space 多选批量标记。与 CLP-01/03 单数描述一致。
- **D-04:** 剪贴板数据结构存储：来源面板索引（0=local, 1=remote）、FileInfo、源目录路径。粘贴时验证目标面板与来源面板一致（防止跨面板粘贴，v1.3+ 功能）。
- **D-05:** 剪贴板清除时机：Esc 清除、新 c/x 操作清除、粘贴成功后自动清除。粘贴失败不清除（允许重试）。

### 粘贴行为
- **D-06:** 同目录粘贴（复制到源目录）时自动重命名为 `file.1.txt` 格式，复用现有 `nextAvailableName` 逻辑。无需冲突对话框。
- **D-07:** 本地复制保留源文件权限（os.Chmod）和修改时间（os.Chtimes）。远程复制由 download+re-upload 过程自然保留（SFTP 协议传输元数据）。

### 进度显示
- **D-08:** 远程复制进度复用 TransferModal，新增 `modeCopy` 模式。下载阶段显示 "Downloading: filename"，上传阶段显示 "Uploading: filename"。复用现有进度条、取消流程、完成摘要。本地复制不显示进度（同步操作，瞬间完成）。

### Claude's Discretion
- 剪贴板状态在 FileBrowser 上的具体字段命名和位置
- [C] 前缀在 Name 列中的具体渲染方式（颜色、位置）
- TransferModal modeCopy 的具体 UI 布局（标题文本、进度格式）
- 状态栏提示文本（"1 file copied"、"Clipboard: file.txt" 等）
- 远程复制失败时的错误处理细节（下载成功但上传失败时的清理策略）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Port Interfaces
- `internal/core/ports/file_service.go` — FileService 接口（需新增 Copy/CopyDir）
- `internal/core/ports/transfer.go` — TransferService 接口（需新增 CopyRemoteFile/CopyRemoteDir）

### UI Components
- `internal/adapters/ui/file_browser/file_browser.go` — FileBrowser orchestrator，剪贴板状态存储位置
- `internal/adapters/ui/file_browser/file_browser_handlers.go` — handleGlobalKeys 按键路由（需添加 c/p）
- `internal/adapters/ui/file_browser/transfer_modal.go` — TransferModal 多模式状态机（需新增 modeCopy）
- `internal/adapters/ui/file_browser/local_pane.go` — LocalPane，selected map 参考
- `internal/adapters/ui/file_browser/remote_pane.go` — RemotePane，selected map 参考

### Requirements
- `.planning/REQUIREMENTS.md` — CPY-01/02/03, CLP-01/02/03, RCP-01 需求定义

### Research
- `.planning/research/STACK.md` — SFTP 原语和 Go stdlib 能力

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **TransferService**: 已有 UploadFile/DownloadFile/UploadDir/DownloadDir + onProgress + onConflict + ctx cancel。CopyRemoteFile/CopyRemoteDir 可复用此基础设施。
- **nextAvailableName()**: file_browser.go:587 — 已有冲突重命名逻辑（file.1.txt 格式），同目录粘贴可直接复用。
- **TransferModal**: 已有 progress/cancelConfirm/conflictDialog/summary 四种模式。新增 modeCopy 遵循相同状态机。
- **LocalPane/RemotePane selected map**: 已有 `selected map[string]bool` 多选状态。剪贴板是独立状态（单文件），不复用此 map。
- **showStatusError()**: file_browser.go:608 — 状态栏错误闪烁 3 秒，可用于粘贴失败提示。
- **goroutine + QueueUpdateDraw**: 所有文件操作的标准异步模式。

### Established Patterns
- **Overlay 生命周期**: Show() → HandleKey() → Hide()，visible 标志控制
- **按键路由**: handleGlobalKeys 中 overlay 优先拦截，然后 switch 分发
- **状态栏模板**: `updateStatusBarConnection` 和 `setStatusBarDefault` 已有快捷键提示文本模板
- **文件操作 handler**: handleDelete/handleRename/handleMkdir 遵循相同模式：获取选中文件 → 弹出对话框 → goroutine 执行 → QueueUpdateDraw 刷新

### Integration Points
- **FileBrowser.handleGlobalKeys**: 添加 `c` 和 `p` 按键处理
- **FileBrowser struct**: 添加 clipboard 字段（Clipboard struct 或简单字段）
- **FileBrowser.Draw()**: [C] 前缀渲染（在 Name 列 cell 文本前添加标记）
- **FileService interface**: 新增 Copy/CopyDir 方法
- **TransferService interface**: 新增 CopyRemoteFile/CopyRemoteDir 方法
- **TransferModal**: 新增 modeCopy 模式 + ShowCopy/Update 方法

</code_context>

<specifics>
## Specific Ideas

- 剪贴板存储在 FileBrowser struct 上（不是 per-pane），因为标记需要跨目录导航保持
- [C] 前缀渲染：在 LocalPane/RemotePane 的 Draw 或 Refresh 中检查 FileBrowser.clipboard 是否匹配当前文件，匹配则在 Name 列添加 [C] 前缀
- 本地 Copy 实现：`io.Copy(dst, src)` + `os.Chtimes` + `os.Chmod`，不引入外部依赖
- 远程 CopyRemoteFile：先 `DownloadFile` 到临时目录，再 `UploadFile` 到目标路径，完成后清理临时文件
- TransferModal modeCopy 的方向标签可用 "Copying" 而非 "Uploading"/"Downloading"，或分阶段显示

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 07-copy-clipboard*
*Context gathered: 2026-04-15*
