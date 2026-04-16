# Phase 13: Cross-Remote Transfer - Context

**Gathered:** 2026-04-16
**Status:** Ready for planning

<domain>
## Phase Boundary

在 DualRemoteFileBrowser 中实现跨远端文件复制/移动功能。用户可通过两种交互路径在两台远程服务器之间传输文件和目录：
1. **F5 快捷传输**：选中文件后按 F5 直接传输到对面面板当前目录
2. **c/x + p 剪贴板**：标记文件 → 切换面板 → 导航 → 粘贴（更灵活）

传输机制为 download from sourceSFTP → local temp → upload to targetSFTP，复用 TransferService 基础设施。包含两阶段进度显示（下载→上传）、取消清理、冲突处理、移动失败回滚。

</domain>

<decisions>
## Implementation Decisions

### 传输后端
- **D-01:** TransferService 新增 `CrossRemoteCopyFile(ctx, srcSFTP, dstSFTP, srcPath, dstPath, onProgress, onConflict)` 和 `CrossRemoteCopyDir` 方法，接收两个独立 SFTPService 参数。内部复用 32KB buffer + onProgress + cleanup 逻辑，但支持跨连接下载和上传。不修改现有 CopyRemoteFile/CopyRemoteDir（单连接内复制）。

### 进度显示
- **D-02:** 扩展现有 TransferModal 新增 `modeCrossRemote` 模式，复用 progress/cancelConfirm/conflictDialog/summary 状态机。进度标题区分两阶段："Downloading from {sourceAlias}: filename" → "Uploading to {targetAlias}: filename"。
- **D-03:** 两阶段切换时进度条重置为 0%（下载完成 → 上传开始）。视觉分离清晰，用户可明确感知阶段切换。

### F5 快捷传输
- **D-04:** F5 键直接将当前选中文件/目录传输到对面面板的当前目录，无需剪贴板状态。Enter 键保持现有行为（进入目录）。
- **D-05:** F5 传输前：文件直接传输（无确认），目录弹出 ConfirmDialog 确认（递归传输可能很大）。

### 剪贴板适配
- **D-06:** 跨远端 c/x + p 保持单文件剪贴板模式，与 Phase 7/8 一致。Space 多选仅用于批量删除（Phase 12 handleBatchDelete），不用于跨远端粘贴。DualRemoteFileBrowser 中的 Clipboard.SourcePane 改为 0=source, 1=target（而非 FileBrowser 的 0=local, 1=remote）。

### 取消与回滚
- **D-07:** 取消时清理本地 temp 文件 + 目标端已上传的部分文件。如果正在上传阶段取消，停止上传并删除目标端不完整文件。ctx 取消传播到两个阶段。
- **D-08:** 跨远端移动（x+p）复制阶段成功但删除源文件失败时，尝试清理目标副本恢复原状。清理失败则状态栏提示用户手动清理。与 Phase 8 D-04 策略一致。

### Claude's Discretion
- CrossRemoteCopyFile/CrossRemoteDir 的具体方法签名（参数顺序、回调类型）
- TransferModal modeCrossRemote 的具体 UI 布局细节（标题颜色、服务器别名显示格式）
- temp 目录位置（os.TempDir() 或项目自定义路径）
- [C]/[M] 前缀在 RemotePane 中的具体渲染颜色
- F5 传输目录时 ConfirmDialog 的具体提示文本
- 状态栏操作提示文本

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` — XFR-01~XFR-07 需求定义

### Phase 12 Foundation
- `.planning/phases/12-dual-remote-browser/12-CONTEXT.md` — DualRemoteFileBrowser 设计决策，D-06 明确延迟剪贴板到 Phase 13
- `internal/adapters/ui/file_browser/dual_remote_browser.go` — DualRemoteFileBrowser 组件（结构体、布局、SFTP 连接管理）
- `internal/adapters/ui/file_browser/dual_remote_browser_handlers.go` — handleGlobalKeys 按键路由（需添加 c/x/p/F5）

### Clipboard & Transfer Patterns
- `.planning/phases/07-copy-clipboard/07-CONTEXT.md` — 剪贴板设计（单文件模式、[C] 前缀、生命周期）
- `.planning/phases/08-move-integration/08-CONTEXT.md` — 移动实现（move=copy+delete、冲突对话框、modeMove）
- `internal/adapters/ui/file_browser/file_browser.go` — Clipboard struct 定义、handleCopy/handlePaste/handleMove 实现参考
- `internal/adapters/ui/file_browser/transfer_modal.go` — TransferModal 多模式状态机（需新增 modeCrossRemote）

### Port Interfaces
- `internal/core/ports/transfer.go` — TransferService 接口（需新增 CrossRemoteCopyFile/CrossRemoteDir）
- `internal/core/ports/file_service.go` — SFTPService 接口（Remove/RemoveAll 用于移动删除源）
- `internal/core/domain/` — TransferProgress、ConflictHandler、FileInfo 等领域类型

### Data Adapters
- `internal/adapters/data/transfer/transfer_service.go` — 现有 CopyRemoteFile/CopyRemoteDir 实现（参考 download→temp→upload 模式）
- `internal/adapters/data/sftp_client/sftp_client.go` — SFTPClient 实现（Connect/ListDir/Remove/Rename/Mkdir）

### Reusable Components
- `internal/adapters/ui/file_browser/remote_pane.go` — RemotePane API（SelectedFiles、GetCurrentPath、GetSelection、Refresh、clipboardProvider）
- `internal/adapters/ui/file_browser/confirm_dialog.go` — ConfirmDialog overlay（F5 目录确认、冲突处理）
- `internal/adapters/ui/file_browser/input_dialog.go` — InputDialog overlay

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **TransferModal**: 已有 modeCopy/conflictDialog/cancelConfirm/summary 四种模式 + actionCh 同步机制 + progress bar 渲染。新增 modeCrossRemote 可复用全部基础设施。
- **Clipboard struct**: 已有 Active/SourcePane/FileInfo/SourceDir/Operation 字段。DualRemoteFileBrowser 可定义独立的 clipboard（SourcePane 语义改为 0=source, 1=target）。
- **CopyRemoteFile/CopyRemoteDir**: TransferService 现有方法实现了 download→temp→upload 模式（32KB buffer, onProgress, onConflict, defer cleanup）。CrossRemoteCopy 需要类似逻辑但跨两个 SFTPService。
- **handleBatchDelete**: Phase 12 已实现 Space 多选批量删除，SelectedFiles() API 可用。
- **ConfirmDialog/InputDialog**: DualRemoteFileBrowser 已有独立实例（D-05 from Phase 12）。
- **RemotePane.clipboardProvider**: 已有 func() (bool, string, string, ClipboardOp) 回调，可复用于 [C]/[M] 前缀渲染。

### Established Patterns
- **剪贴板生命周期**: handleCopy/handleMove 设置 → handlePaste 消费 → 成功清除/失败保留
- **Overlay 按键拦截**: handleGlobalKeys 中 overlay 优先拦截（InputDialog > ConfirmDialog > TransferModal > keys）
- **异步操作**: goroutine 执行 + QueueUpdateDraw 回调 UI + ctx 取消传播
- **TransferModal 模式切换**: Show/ShowCopy/ShowConflict → mode 字段 → Draw/HandleKey 按 mode 分发

### Integration Points
- **DualRemoteFileBrowser.handleGlobalKeys**: 添加 c/x/p/F5 按键处理
- **DualRemoteFileBrowser struct**: 添加 clipboard 字段 + transferModal 字段
- **DualRemoteFileBrowser.Draw()**: overlay chain 添加 TransferModal
- **TransferService interface**: 新增 CrossRemoteCopyFile/CrossRemoteDir 方法
- **RemotePane.clipboardProvider**: 设置回调以渲染 [C]/[M] 前缀

</code_context>

<specifics>
## Specific Ideas

- CrossRemoteCopyFile 内部流程：DownloadFile(srcSFTP, srcPath, tempPath) → UploadFile(dstSFTP, tempPath, dstPath) → os.Remove(tempPath)。两阶段各自有独立 onProgress 回调。
- TransferModal modeCrossRemote 的 ShowCrossRemote() 接收 sourceAlias + targetAlias 用于标题显示。
- F5 传输的内部实现可复用 handlePaste 逻辑，只是跳过剪贴板检查直接从当前选中文件获取 FileInfo。
- clipboardProvider 在 DualRemoteFileBrowser 中需要适配：两个面板都是 RemotePane，SourcePane 改为 0=source, 1=target。
- 移动操作的 cleanup：CrossRemoteCopy 成功 → sourceSFTP.Remove(srcPath) 失败 → targetSFTP.Remove(dstPath) 清理目标副本 → 仍失败则 showStatusError。

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---
*Phase: 13-cross-remote-transfer*
*Context gathered: 2026-04-16*
