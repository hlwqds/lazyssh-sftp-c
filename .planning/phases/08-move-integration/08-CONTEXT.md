# Phase 8: Move & Integration - Context

**Gathered:** 2026-04-15
**Status:** Ready for planning

<domain>
## Phase Boundary

面板内文件移动功能 + 复制/移动冲突对话框 + 进度显示完善。用户按 `x` 标记当前文件为移动源（[M] 前缀），导航到目标目录后按 `p` 粘贴（复制+删除源文件）。移动失败时保留源文件并尝试清理目标副本。所有粘贴操作（复制+移动）在目标文件已存在时弹出冲突对话框。远程移动通过 TransferService 新增 modeMove 模式显示进度。本地复制/移动保持同步无进度。不包含跨面板复制/移动（v1.3+）和多文件剪贴板。
</domain>

<decisions>
## Implementation Decisions

### 冲突对话框
- **D-01:** 所有粘贴操作（复制和移动）在目标文件已存在时弹出冲突对话框（覆盖/跳过/重命名），替代 Phase 7 D-06 的同目录自动重命名。TransferModal 已有 conflictDialog 模式，可复用其布局和 actionCh 机制。
- **D-02:** 冲突对话框选择「重命名」时使用现有 `nextAvailableName()` 逻辑生成目标名称（file.1.txt 格式）。

### 移动实现策略
- **D-03:** 移动 = 复制 + 删除源文件。本地移动：FileService.Copy/CopyDir + FileService.Remove/RemoveAll。远程移动：TransferService.CopyRemoteFile/CopyRemoteDir + SFTPService.Remove/RemoveAll。
- **D-04:** 移动操作失败时（MOV-03），保留源文件不变。如果复制阶段成功但删除源文件失败，尝试清理目标目录的副本以恢复原始状态。清理失败则在状态栏提示用户手动清理。
- **D-05:** 远程移动的 cleanup 策略：CopyRemoteFile/CopyRemoteDir 产生的临时文件在 copy 阶段已由 defer 清理。如果后续 delete 源文件失败，需要额外删除已上传到目标路径的副本。

### 剪贴板扩展
- **D-06:** `x` 键标记移动源，Clipboard.Operation 设为 OpMove，文件列表显示 `[M]` 前缀（类似 Phase 7 的 `[C]` 前缀）。`x` 和 `c` 共享同一剪贴板状态（单文件模式，D-03 from Phase 7），新操作替换旧标记。
- **D-07:** `p` 键粘贴时根据 Clipboard.Operation 判断执行复制还是移动。粘贴成功后清除剪贴板（D-05 from Phase 7）。粘贴失败不清除（允许重试）。

### 进度显示
- **D-08:** 远程移动新增 TransferModal `modeMove` 模式。复制阶段显示 "Moving: filename"（复用 progress bar），删除源阶段显示 "Deleting source..."（简单状态文本，无需进度条）。
- **D-09:** 本地复制和移动保持同步执行，不显示进度（本地磁盘操作通常很快）。仅远程操作通过 TransferModal 显示进度。

### Claude's Discretion
- [M] 前缀在 Name 列中的具体渲染颜色（建议与 [C] 区分，如黄色/红色）
- TransferModal modeMove 的具体 UI 布局细节
- 删除源阶段 "Deleting source..." 的显示位置和样式
- 状态栏移动操作提示文本
- 冲突对话框的默认选中项（建议 Skip 或 Rename，不默认 Overwrite）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Port Interfaces
- `internal/core/ports/file_service.go` — FileService 接口（Copy/CopyDir/Remove/RemoveAll 已存在）
- `internal/core/ports/transfer.go` — TransferService 接口（CopyRemoteFile/CopyRemoteDir 已存在）

### UI Components
- `internal/adapters/ui/file_browser/file_browser.go` — FileBrowser orchestrator, clipboard 状态, handleCopy/handlePaste, nextAvailableName(), buildConflictHandler()
- `internal/adapters/ui/file_browser/file_browser_handlers.go` — handleGlobalKeys 按键路由（需添加 x 键 + TransferModal 全按键拦截已修复）
- `internal/adapters/ui/file_browser/transfer_modal.go` — TransferModal 多模式状态机（modeCopy/conflictDialog 已存在，需新增 modeMove）
- `internal/adapters/ui/file_browser/local_pane.go` — LocalPane, clipboardProvider 回调
- `internal/adapters/ui/file_browser/remote_pane.go` — RemotePane, clipboardProvider 回调

### Data Adapters
- `internal/adapters/data/transfer/transfer_service.go` — CopyRemoteFile/CopyRemoteDir 实现（download+re-upload + defer cleanup）
- `internal/adapters/data/local_fs/local_fs.go` — Copy/CopyDir 实现

### Requirements
- `.planning/REQUIREMENTS.md` — MOV-01/02/03, PRG-01, CNF-01/02 需求定义

### Research
- `.planning/research/STACK.md` — SFTP 原语和 Go stdlib 能力

### Prior Phase Context
- `.planning/phases/07-copy-clipboard/07-CONTEXT.md` — Phase 7 剪贴板设计和复制实现决策
- `.planning/phases/06-basic-file-operations/06-CONTEXT.md` — Phase 6 overlay 模式和错误处理决策

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **TransferModal conflictDialog mode**: 已有三行布局（标题 + 信息 + 选项：[o] Overwrite [s] Skip [r] Rename）+ actionCh 同步机制。粘贴冲突可直接复用。
- **TransferModal modeCopy**: 已有远程复制进度渲染逻辑（progress bar + cancel + 文件名）。modeMove 可复用 progress bar 渲染，仅修改标题文本。
- **buildConflictHandler(ctx)**: 已支持 ctx 取消（最新未提交修复）。粘贴冲突可直接调用。
- **handleCopy()/handlePaste()**: 已实现剪贴板标记和粘贴逻辑。handlePaste() 可扩展为根据 Operation 类型分发 copy/move。
- **nextAvailableName()**: file_browser.go — 已有冲突重命名逻辑。冲突对话框选「重命名」时复用。
- **Clipboard struct**: 已有 Active/FileInfo/SourceDir/SourcePane/Operation 字段。OpMove 已定义但未使用。
- **clipboardProvider callback**: panes 通过 func() (bool, string, string) 查询剪贴板状态渲染 [C] 前缀。需扩展支持 [M] 前缀。
- **showStatusError()**: 状态栏错误闪烁 3 秒。
- **TransferService.CopyRemoteFile/CopyRemoteDir**: 已有 download+re-upload + temp file cleanup + onProgress + onConflict。
- **FileService.Copy/CopyDir/Remove/RemoveAll**: 本地操作已就绪。
- **SFTPService.Remove/RemoveAll**: 远程删除已就绪（Phase 6 实现）。

### Established Patterns
- **剪贴板生命周期**: handleCopy/handleMove 设置 → handlePaste 消费 → 成功清除/失败保留
- **Overlay 按键拦截**: handleGlobalKeys 中 overlay 优先拦截（TransferModal > ConfirmDialog > RecentDirs）
- **异步操作**: goroutine 执行 + QueueUpdateDraw 回调 UI + ctx 取消传播
- **TransferModal 模式切换**: Show/ShowCopy/ShowConflict → mode 字段 → Draw/HandleKey 按 mode 分发

### Integration Points
- **FileBrowser.handleGlobalKeys**: 添加 `x` 按键处理
- **handlePaste()**: 扩展为根据 clipboard.Operation 分发 copy/move 逻辑
- **Clipboard.clipboardProvider**: 扩展返回 Operation 类型以区分 [C]/[M] 前缀
- **LocalPane/RemotePane Draw**: [M] 前缀渲染（类似 [C]）
- **TransferModal**: 新增 modeMove + ShowMove() + "Deleting source..." 阶段
- **FileService**: Move 不需要新接口方法（复用 Copy + Remove）

</code_context>

<specifics>
## Specific Ideas

- handlePaste() 内部：if clipboard.Operation == OpCopy → 现有复制逻辑; if OpMove → 复制 + 删除源
- 远程移动 cleanup 流程：CopyRemoteFile 成功 → 调用 Remove 删除源 → 失败则调用 Remove 删除目标副本 → 仍失败则状态栏提示
- 同目录移动（移动到当前目录）= 重命名，可直接调用 FileService.Rename 而非 Copy+Delete
- 冲突对话框复用 buildConflictHandler()，粘贴时先检查目标是否存在（Stat），存在则弹出

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 08-move-integration*
*Context gathered: 2026-04-15*
