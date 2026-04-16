# Phase 13: Cross-Remote Transfer - Research

**Researched:** 2026-04-16
**Domain:** Go TUI cross-remote file relay transfer (download from SFTP A -> local temp -> upload to SFTP B)
**Confidence:** HIGH

## Summary

Phase 13 在 DualRemoteFileBrowser 组件中实现跨远端文件传输功能。核心机制是 "中转传输"（relay transfer）：从源服务器 SFTP 下载到本地临时文件，再从临时文件上传到目标服务器 SFTP。这个模式已在 Phase 7/8 的 CopyRemoteFile/CopyRemoteDir 中充分验证（单 SFTP 连接内的远端复制），Phase 13 将其扩展到两个独立 SFTP 连接之间。

STATE.md 已决定使用 `RelayTransferService` 组合两个 `transfer.New()` 实例的方案，实现零代码重复。UI 层复用现有 TransferModal，新增 `modeCrossRemote` 模式。交互路径有两条：F5 快捷传输（直接将选中文件传到对面面板）和 c/x + p 剪贴板（标记后导航到目标目录再粘贴）。

**Primary recommendation:** 使用 RelayTransferService（新端口+适配器）组合两个 transfer.New() 实例，完全复用 DownloadFile/UploadFile 的现有逻辑（冲突检测、取消传播、清理），零代码重复。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** TransferService 新增 `CrossRemoteCopyFile(ctx, srcSFTP, dstSFTP, srcPath, dstPath, onProgress, onConflict)` 和 `CrossRemoteCopyDir` 方法，接收两个独立 SFTPService 参数。内部复用 32KB buffer + onProgress + cleanup 逻辑，但支持跨连接下载和上传。不修改现有 CopyRemoteFile/CopyRemoteDir（单连接内复制）。
- **D-02:** 扩展现有 TransferModal 新增 `modeCrossRemote` 模式，复用 progress/cancelConfirm/conflictDialog/summary 状态机。进度标题区分两阶段："Downloading from {sourceAlias}: filename" -> "Uploading to {targetAlias}: filename"。
- **D-03:** 两阶段切换时进度条重置为 0%（下载完成 -> 上传开始）。视觉分离清晰，用户可明确感知阶段切换。
- **D-04:** F5 键直接将当前选中文件/目录传输到对面面板的当前目录，无需剪贴板状态。Enter 键保持现有行为（进入目录）。
- **D-05:** F5 传输前：文件直接传输（无确认），目录弹出 ConfirmDialog 确认（递归传输可能很大）。
- **D-06:** 跨远端 c/x + p 保持单文件剪贴板模式，与 Phase 7/8 一致。Space 多选仅用于批量删除（Phase 12 handleBatchDelete），不用于跨远端粘贴。DualRemoteFileBrowser 中的 Clipboard.SourcePane 改为 0=source, 1=target（而非 FileBrowser 的 0=local, 1=remote）。
- **D-07:** 取消时清理本地 temp 文件 + 目标端已上传的部分文件。如果正在上传阶段取消，停止上传并删除目标端不完整文件。ctx 取消传播到两个阶段。
- **D-08:** 跨远端移动（x+p）复制阶段成功但删除源文件失败时，尝试清理目标副本恢复原状。清理失败则状态栏提示用户手动清理。与 Phase 8 D-04 策略一致。

### Claude's Discretion
- CrossRemoteCopyFile/CrossRemoteDir 的具体方法签名（参数顺序、回调类型）
- TransferModal modeCrossRemote 的具体 UI 布局细节（标题颜色、服务器别名显示格式）
- temp 目录位置（os.TempDir() 或项目自定义路径）
- [C]/[M] 前缀在 RemotePane 中的具体渲染颜色
- F5 传输目录时 ConfirmDialog 的具体提示文本
- 状态栏操作提示文本

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| XFR-01 | 用户可以在双远端浏览器中通过 Enter/F5 触发跨远端文件传输（download A -> temp -> upload B） | D-04: F5 直接触发传输；Enter 保持进入目录行为。RelayTransferService.RelayFile 实现底层中转。TransferModal.modeCrossRemote 显示进度。 |
| XFR-02 | 支持跨远端目录递归传输 | RelayTransferService.RelayDir 递归下载+上传目录。两阶段各自有进度回调。目录传输使用 ConfirmDialog 确认（D-05）。 |
| XFR-03 | 跨远端传输显示进度（复用 TransferModal，两阶段进度：下载进度 -> 上传进度） | D-02/D-03: TransferModal 新增 modeCrossRemote。两阶段进度切换时 ResetProgress 重置为 0%。进度标题区分 "Downloading from {sourceAlias}" 和 "Uploading to {targetAlias}"。 |
| XFR-04 | 跨远端传输支持取消（Esc），取消后清理本地临时文件 | D-07: ctx 取消传播。DownloadFile/UploadFile 内置取消清理（context.Canceled 时删除部分文件）。RelayTransferService 的 defer 确保 temp 清理。 |
| XFR-05 | 跨远端文件冲突处理（覆盖/跳过/重命名，复用 ConfirmDialog） | 复用 TransferModal.ShowConflict + conflictActionCh 通道同步机制。buildConflictHandler 适配目标端为 dstSFTP.Stat。 |
| XFR-06 | 支持跨远端复制（c 标记 + p 粘贴，绿色 [C] 前缀） | D-06: DualRemoteFileBrowser 添加 Clipboard struct（SourcePane: 0=source, 1=target）。handleCopy/handlePaste 参考 FileBrowser 实现。RemotePane.clipboardProvider 回调渲染 [C] 前缀。 |
| XFR-07 | 支持跨远端移动（x 标记 + p 粘贴，红色 [M] 前缀，复制+删除源文件） | D-06/D-08: handleMove 设置 OpMove。handlePaste 中移动操作 = RelayFile/RelayDir + sourceSFTP.Remove/RemoveAll。删除失败时回滚（清理目标副本）。 |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib (context, os, io, path/filepath) | 1.24.6 | ctx 取消传播、temp 文件管理、IO 操作 | 已有模式，transfer_service.go 广泛使用 |
| tview | v0.0.0 | TUI 框架，TransferModal overlay 渲染 | 项目唯一 UI 框架（CLAUDE.md 约束） |
| tcell/v2 | v2.9.0 | 终端颜色定义、键盘事件处理 | 与 tview 配套使用 |
| zap | v1.27.0 | 结构化日志记录 | 项目统一日志库 |
| domain.TransferProgress | -- | 进度回调数据结构 | 现有领域类型，无需修改 |
| domain.ConflictHandler | -- | 冲突解决回调类型 | 现有领域类型，无需修改 |

### Supporting
| Component | Location | Purpose | When to Use |
|-----------|----------|---------|-------------|
| transfer.New() | internal/adapters/data/transfer/transfer_service.go | 创建 TransferService 实例 | RelayTransferService 内部创建 dlSvc/ulSvc |
| SFTPService.OpenRemoteFile | internal/adapters/data/sftp_client/sftp_client.go | 打开远端文件进行读写 | 下载阶段读取源文件 |
| SFTPService.CreateRemoteFile | internal/adapters/data/sftp_client/sftp_client.go | 创建远端文件用于写入 | 上传阶段创建目标文件 |
| SFTPService.Remove/RemoveAll | internal/core/ports/file_service.go | 删除远端文件/目录 | 移动操作删除源文件 |
| SFTPService.Stat | internal/core/ports/file_service.go | 获取远端文件信息 | 冲突检测 |
| SFTPService.MkdirAll | internal/core/ports/file_service.go | 递归创建远端目录 | 目录传输创建目标目录结构 |
| SFTPService.WalkDir | internal/core/ports/file_service.go | 列出远端目录下所有文件 | DownloadDir 使用 |
| ConfirmDialog | internal/adapters/ui/file_browser/confirm_dialog.go | F5 目录确认、冲突处理 | 已有 overlay 组件 |
| TransferModal | internal/adapters/ui/file_browser/transfer_modal.go | 进度显示、取消确认、冲突对话框 | 新增 modeCrossRemote 模式 |
| Clipboard struct | internal/adapters/ui/file_browser/file_browser.go | 剪贴板状态管理 | DualRemoteFileBrowser 复用相同结构 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| RelayTransferService (组合两个 transfer.New()) | 单一 RelayFile 方法内重新实现 IO 逻辑 | 零代码重复 vs 完全独立（80行 copyWithProgress 重复） |
| RelayTransferService (组合两个 transfer.New()) | 提取 copyWithProgress 为包级函数共享 | 需要重构现有 transfer_service.go，影响已验证代码 |
| TransferService 新增方法 (D-01) | 独立 RelayTransferService 端口接口 | D-01 要求在 TransferService 新增方法，但 STATE.md 决定使用独立 RelayTransferService |

**关于 D-01 与 STATE.md 的协调：**

D-01 描述的是 "TransferService 新增 CrossRemoteCopyFile/CrossRemoteDir"，而 STATE.md 决定使用 "RelayTransferService 组合两个 transfer.New() 实例"。这两个决策本质上一致：
- **D-01 的核心意图**是接收两个独立 SFTPService 参数，实现跨连接下载上传
- **STATE.md 的 RelayTransferService** 是这个意图的具体实现方案
- 实际实现时，创建 `RelayTransferService` 端口接口（ports/relay_transfer.go）和适配器（adapters/data/transfer/relay_transfer_service.go），内部组合两个 `transfer.New()` 实例
- 这比直接修改 TransferService 接口更符合 Clean Architecture（单一职责），也避免 TransferService 方法签名膨胀

**推荐方案：** 遵循 STATE.md 的 RelayTransferService 方案，视为 D-01 的具体实现。

## Architecture Patterns

### Recommended Project Structure

```
internal/
├── core/
│   ├── domain/
│   │   └── transfer.go          # 已有 - TransferProgress, ConflictHandler
│   └── ports/
│       ├── transfer.go          # 已有 - TransferService 接口（不修改）
│       ├── file_service.go      # 已有 - SFTPService 接口（不修改）
│       └── relay_transfer.go    # 新增 - RelayTransferService 端口接口
├── adapters/
│   ├── data/
│   │   └── transfer/
│   │       ├── transfer_service.go           # 已有 - 不修改
│   │       └── relay_transfer_service.go     # 新增 - RelayTransferService 适配器
│   └── ui/
│       └── file_browser/
│           ├── dual_remote_browser.go        # 修改 - 添加 clipboard, transferModal 字段
│           ├── dual_remote_browser_handlers.go # 修改 - 添加 c/x/p/F5 按键处理
│           └── transfer_modal.go             # 修改 - 添加 modeCrossRemote 模式
```

### Pattern 1: RelayTransferService（中转传输服务）

**What:** 新的端口+适配器，内部持有两个 SFTPService 引用（srcSFTP + dstSFTP），通过本地临时文件中转实现跨服务器文件传输。

**When to use:** 所有跨远端文件复制/移动操作。

**核心设计：**
```go
// internal/core/ports/relay_transfer.go

// RelayTransferService transfers files between two remote servers via local relay.
type RelayTransferService interface {
    RelayFile(ctx context.Context, srcPath, dstPath string,
        onProgress func(domain.TransferProgress),
        onConflict domain.ConflictHandler) error

    RelayDir(ctx context.Context, srcPath, dstPath string,
        onProgress func(domain.TransferProgress),
        onConflict domain.ConflictHandler) ([]string, error)
}
```

**适配器实现策略 -- 组合两个 transfer.New()：**
```go
// internal/adapters/data/transfer/relay_transfer_service.go

type relayTransferService struct {
    log     *zap.SugaredLogger
    srcSFTP ports.SFTPService
    dstSFTP ports.SFTPService
}

func (rs *relayTransferService) RelayFile(ctx context.Context,
    srcPath, dstPath string,
    onProgress func(domain.TransferProgress),
    onConflict domain.ConflictHandler) error {

    tmpFile, _ := os.CreateTemp("", "lazyssh-relay-*")
    tmpPath := tmpFile.Name()
    _ = tmpFile.Close()
    defer func() { _ = os.Remove(tmpPath) }()

    // Phase 1: 从源服务器下载
    dlSvc := transfer.New(rs.log, rs.srcSFTP)
    if err := dlSvc.DownloadFile(ctx, srcPath, tmpPath, onProgress, nil); err != nil {
        return fmt.Errorf("relay download: %w", err)
    }

    // Phase 2: 上传到目标服务器
    ulSvc := transfer.New(rs.log, rs.dstSFTP)
    if err := ulSvc.UploadFile(ctx, tmpPath, dstPath, onProgress, onConflict); err != nil {
        return fmt.Errorf("relay upload: %w", err)
    }
    return nil
}
```

**优雅之处：**
- `dlSvc` 的 srcSFTP 扮演远端角色，临时文件扮演本地角色
- `ulSvc` 的 dstSFTP 扮演远端角色，临时文件扮演本地角色
- 完全复用 DownloadFile/UploadFile 的冲突检测、取消传播、错误清理逻辑
- 估计 ~120 行代码

**Confidence:** HIGH -- 基于现有 CopyRemoteFile 的同构模式（transfer_service.go:436-472），CopyRemoteFile 已经成功实现了 download->temp->upload 的两阶段模式。RelayTransferService 只是将其扩展到两个不同的 SFTPService 实例。

### Pattern 2: TransferModal.modeCrossRemote（进度显示扩展）

**What:** 在 TransferModal 的 modalMode 枚举中新增 `modeCrossRemote`，复用现有 progress/cancelConfirm/conflictDialog/summary 状态机。

**When to use:** 所有跨远端传输操作（F5 快捷传输 + c/x + p 粘贴）。

**模式切换流程：**
```
ShowCrossRemote(sourceAlias, targetAlias, filename)
  -> modeCrossRemote, title = " Transfer: filename "
  -> fileLabel = "Downloading from sourceAlias: filename"
  -> 两阶段各自有 ResetProgress()

下载完成 -> ResetProgress() + fileLabel = "Uploading to targetAlias: filename"
上传完成 -> ShowSummary(transferred, failed, failedFiles)
取消 -> ShowCanceledSummary()
冲突 -> ShowConflict() + conflictActionCh
```

**实现要点：**
- 新增 `modeCrossRemote modalMode = 6` 常量
- 新增 `ShowCrossRemote(sourceAlias, targetAlias, filename string)` 方法
- Draw() 的 modeProgress/modeCopy/modeMove case 改为包含 modeCrossRemote
- HandleKey() 的 modeProgress/modeCopy/modeMove case 改为包含 modeCrossRemote
- Update() 的 mode 检查改为包含 modeCrossRemote

**Confidence:** HIGH -- 现有 modeCopy 和 modeMove 已验证了扩展 modalMode 枚举的模式。modeCrossRemote 是完全同构的扩展。

### Pattern 3: DualRemoteFileBrowser 剪贴板（c/x + p）

**What:** 在 DualRemoteFileBrowser 中添加 Clipboard struct，SourcePane 语义改为 0=source, 1=target（与 FileBrowser 的 0=local, 1=remote 对应）。

**When to use:** c 标记复制 + p 粘贴，x 标记移动 + p 粘贴。

**生命周期：**
```
handleCopy()  -> clipboard = {Active: true, SourcePane: activePane, FileInfo, SourceDir, OpCopy}
                 -> refreshPane + focusOnItem 显示 [C] 前缀
                 -> statusTemp: "Clipboard: filename"

handleMove()  -> clipboard = {Active: true, SourcePane: activePane, FileInfo, SourceDir, OpMove}
                 -> refreshPane + focusOnItem 显示 [M] 前缀
                 -> statusTemp: "Move: filename"

handlePaste() -> 检查 clipboard.Active
                 -> 检查两个面板都 IsConnected()
                 -> goroutine: relaySvc.RelayFile/RelayDir
                 -> 成功: clipboard = {} + refreshPane + focusOnItem
                 -> 失败: 保留 clipboard + showStatusError
                 -> 移动: RelayFile 成功后 sourceSFTP.Remove(srcPath)
                   删除失败 -> targetSFTP.Remove(dstPath) 回滚
                   回滚失败 -> showStatusError("手动清理提示")
```

**clipboardProvider 回调：**
```go
// 设置两个 RemotePane 的 clipboardProvider
drb.sourcePane.SetClipboardProvider(func() (bool, string, string, ClipboardOp) {
    return drb.clipboard.Active, drb.clipboard.FileInfo.Name, drb.clipboard.SourceDir, drb.clipboard.Operation
})
drb.targetPane.SetClipboardProvider(func() (bool, string, string, ClipboardOp) {
    return drb.clipboard.Active, drb.clipboard.FileInfo.Name, drb.clipboard.SourceDir, drb.clipboard.Operation
})
```

**Confidence:** HIGH -- FileBrowser 中已完整实现相同模式（file_browser.go:908-1031）。DualRemoteFileBrowser 的适配仅需修改 SourcePane 语义和面板选择逻辑。

### Pattern 4: F5 快捷传输

**What:** F5 键直接将当前选中文件/目录传输到对面面板的当前目录，无需剪贴板状态。

**When to use:** 用户选中文件后想快速传到对面。

**实现：**
```
handleF5Transfer():
  -> 获取当前选中 FileInfo
  -> 确定对面面板（activePane ^ 1）
  -> 检查两个面板都 IsConnected()
  -> 如果是目录 -> ConfirmDialog 确认
  -> goroutine: relaySvc.RelayFile/RelayDir
  -> 刷新对面面板 + focusOnItem
```

**内部实现可复用 handlePaste 的传输逻辑**，只是跳过剪贴板检查直接从当前选中文件获取 FileInfo。

**Confidence:** HIGH -- 与 handlePaste 的传输逻辑高度重用，区别仅在数据来源（当前选中 vs 剪贴板）。

### Anti-Patterns to Avoid

- **直接修改 TransferService 接口：** 不要在 TransferService 中新增 CrossRemoteCopyFile。应创建独立的 RelayTransferService 端口，保持 TransferService 的单一职责（本地 <-> 单远端）。
- **在 handlePaste 中内联中转逻辑：** 不要在 UI handler 中直接操作 SFTP 连接进行中转传输。应通过 RelayTransferService 端口解耦。
- **共享 transferService 实例：** RelayTransferService 内部每次操作创建新的 dlSvc/ulSvc 实例（`transfer.New()`），不要缓存复用。这些实例很轻量（仅持有 log + sftpService 引用）。
- **忽略面板连接状态检查：** 跨远端传输需要两个面板都 IsConnected()。一个面板断连时应报错，不要尝试传输。
- **Space 多选用于跨远端粘贴：** D-06 明确 Space 多选仅用于批量删除，跨远端粘贴保持单文件剪贴板模式。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 文件 IO + 进度回调 | 自己实现带进度回调的文件复制 | transfer.New().DownloadFile/UploadFile | 已有 32KB buffer、ctx 取消、错误清理、冲突检测 |
| 临时文件管理 | 手动创建/清理 temp 文件 | os.CreateTemp + defer os.Remove | CopyRemoteFile 已验证此模式（Pitfall 3） |
| 冲突解决 UI | 自己实现冲突对话框 | TransferModal.ShowConflict + conflictActionCh | 已有完整的 [o]verwrite/[s]kip/[r]ename 三选项 UI |
| 取消确认 UI | 自己实现取消确认 | TransferModal.ShowCancelConfirm | 已有 [y]es/[n]o 确认 + Esc 继续 |
| 进度条渲染 | 自己实现进度条 | TransferModal.Update + ProgressBar | 已有滑动窗口速度计算、ETA、百分比 |
| 目录递归传输 | 自己实现 WalkDir + 批量传输 | DownloadDir/UploadDir | 已有 fileIndex/fileTotal 多文件进度跟踪 |
| [C]/[M] 前缀渲染 | 自己在表格中添加前缀 | RemotePane.clipboardProvider 回调 | 已有渲染逻辑（绿色 [C]、红色 [M]） |

**Key insight:** 本 Phase 的核心工作不是构建新基础设施，而是将现有已验证组件（TransferService、TransferModal、Clipboard）组合到新上下文中。RelayTransferService 是唯一的"新"基础设施，但它也完全复用现有 DownloadFile/UploadFile。

## Common Pitfalls

### Pitfall 1: RelayTransferService 的 onProgress 标签
**What goes wrong:** 下载阶段和上传阶段使用相同的 onProgress 回调，进度标签不区分阶段，用户看到 "Downloading" 一直到最后。
**Why it happens:** CopyRemoteFile 中两个阶段共用同一个 onProgress，标签通过 UI 层的 combinedProgress 回调区分。RelayTransferService 的 onProgress 是从 UI 层传入的，需要在 UI 层区分阶段。
**How to avoid:** UI 层传入的 onProgress 回调应在下载完成时（p.Done == true）切换标签并调用 ResetProgress()。参考 file_browser.go:1352-1368 的 combinedProgress 模式。
**Warning signs:** 进度条从 0% 直接跳到 100%，没有两阶段视觉分离。

### Pitfall 2: 取消时清理目标端不完整文件
**What goes wrong:** 上传阶段取消后，目标端留下部分文件（如 0 字节或半截文件）。
**Why it happens:** context.Canceled 时 UploadFile 内部会删除部分远端文件（transfer_service.go:88-93），但这只在 UploadFile 内部。如果 RelayFile 在两个阶段之间被取消（下载完成后、上传开始前），temp 文件会被 defer 清理，但目标端不会有残留。
**How to avoid:** UploadFile 已有内置清理逻辑（context.Canceled -> sftp.Remove）。RelayTransferService 只需确保 defer 清理 temp 文件。对于 RelayDir 的多文件上传，部分已上传的文件不会被自动清理 -- 这是已知的限制（与 CopyRemoteDir 行为一致）。
**Warning signs:** 取消传输后目标目录出现 0 字节文件或空目录。

### Pitfall 3: 移动操作删除源文件失败后的回滚
**What goes wrong:** 跨远端移动时，RelayFile 成功但 sourceSFTP.Remove() 失败（如权限问题），目标端副本已存在但源文件未删除，状态不一致。
**Why it happens:** 网络/权限问题导致删除操作失败。
**How to avoid:** 实现 D-08 回滚策略：RelayFile 成功 -> sourceSFTP.Remove() 失败 -> targetSFTP.Remove(dstPath) 清理目标副本 -> 仍失败则 showStatusError("手动清理: targetPath")。与 Phase 8 D-04 策略一致。
**Warning signs:** 用户报告"文件被复制但源文件未删除"。

### Pitfall 4: clipboardProvider 在两个面板间共享状态
**What goes wrong:** 在源面板标记 [C] 后，切换到目标面板粘贴时，目标面板的 [C] 前缀仍显示（因为 clipboardProvider 检查的是同一个 clipboard 状态）。
**Why it happens:** clipboardProvider 返回全局 clipboard 状态，不区分当前是哪个面板。但 [C] 前缀应该只在源面板显示（因为文件在源面板）。
**How to avoid:** clipboardProvider 回调中需要额外检查 clipboard.SourcePane == 当前面板索引。但这在 FileBrowser 中不是问题，因为 FileBrowser 的 clipboardProvider 不检查 SourcePane -- [C] 前缀在任何面板都显示。DualRemoteFileBrowser 中应该保持一致：[C]/[M] 前缀在两个面板都显示，帮助用户记住剪贴板状态。
**Warning signs:** 粘贴后 [C] 前缀仍在目标面板显示。

### Pitfall 5: TransferModal 的 overlay 链未包含新模式
**What goes wrong:** modeCrossRemote 模式下 Esc 不进入取消确认，或冲突对话框不工作。
**Why it happens:** Draw() 和 HandleKey() 的 switch/case 未包含 modeCrossRemote。
**How to avoid:** 修改 Draw() 中 `case modeProgress, modeCopy, modeMove:` 为 `case modeProgress, modeCopy, modeMove, modeCrossRemote:`。HandleKey() 同理。Update() 的 mode 检查同理。
**Warning signs:** modeCrossRemote 下按 Esc 无反应，或进度不更新。

### Pitfall 6: F5 传输目录时的确认对话框与 CancelDialog 冲突
**What goes wrong:** F5 按下后弹出 ConfirmDialog 确认目录传输，用户按 Esc 取消确认后，Esc 事件可能传播到 DualRemoteFileBrowser 导致退出。
**Why it happens:** handleGlobalKeys 的 overlay 拦截链需要正确处理 ConfirmDialog 的 Esc。
**How to avoid:** ConfirmDialog.HandleKey 中 Esc 触发 onCancel 回调并 Hide()，返回 nil 消费事件。handleGlobalKeys 中 ConfirmDialog.IsVisible() 检查优先于 Esc 处理。已有模式（Phase 12 的 delete/rename 操作）已验证此链路。
**Warning signs:** 按 Esc 取消 F5 目录确认后，整个 DualRemoteFileBrowser 也关闭了。

## Code Examples

### RelayFile 实现（参考 ARCHITECTURE.md 3.3）
```go
// Source: .planning/research/ARCHITECTURE.md:351-375
func (rs *relayTransferService) RelayFile(ctx context.Context,
    srcPath, dstPath string,
    onProgress func(domain.TransferProgress),
    onConflict domain.ConflictHandler) error {

    tmpFile, err := os.CreateTemp("", "lazyssh-relay-*")
    if err != nil {
        return fmt.Errorf("create temp file: %w", err)
    }
    tmpPath := tmpFile.Name()
    _ = tmpFile.Close()
    defer func() { _ = os.Remove(tmpPath) }()

    // Phase 1: 从源服务器下载到临时文件
    dlSvc := transfer.New(rs.log, rs.srcSFTP)
    if err := dlSvc.DownloadFile(ctx, srcPath, tmpPath, onProgress, nil); err != nil {
        return fmt.Errorf("relay download: %w", err)
    }

    // Phase 2: 从临时文件上传到目标服务器
    ulSvc := transfer.New(rs.log, rs.dstSFTP)
    if err := ulSvc.UploadFile(ctx, tmpPath, dstPath, onProgress, onConflict); err != nil {
        return fmt.Errorf("relay upload: %w", err)
    }
    return nil
}
```

### handlePaste 传输逻辑（参考 FileBrowser remotePasteFile 模式）
```go
// Source: internal/adapters/ui/file_browser/file_browser.go:1238-1288
// 适配为 DualRemoteFileBrowser 版本
func (drb *DualRemoteFileBrowser) handleCrossRemotePaste() {
    if !drb.clipboard.Active {
        return
    }
    // 确定源和目标 SFTPService
    srcPaneIdx := drb.clipboard.SourcePane  // 0=source, 1=target
    dstPaneIdx := 1 - srcPaneIdx            // 对面面板

    srcSFTP := drb.sftpForPane(srcPaneIdx)
    dstSFTP := drb.sftpForPane(dstPaneIdx)

    if !drb.paneForIdx(srcPaneIdx).IsConnected() || !drb.paneForIdx(dstPaneIdx).IsConnected() {
        drb.showStatusError("Both panels must be connected")
        return
    }

    dstPath := drb.pathForPane(dstPaneIdx)
    targetName := drb.clipboard.FileInfo.Name
    srcPath := joinPath(drb.clipboard.SourceDir, drb.clipboard.FileInfo.Name)

    // goroutine 执行传输
    go func() {
        ctx, cancel := context.WithCancel(context.Background())
        defer cancel()

        onConflict := drb.buildConflictHandler(ctx, dstSFTP, dstPath)

        if drb.clipboard.FileInfo.IsDir {
            failed, err := drb.relaySvc.RelayDir(ctx, srcPath, joinPath(dstPath, targetName), onProgress, onConflict)
            // ... handle result
        } else {
            err := drb.relaySvc.RelayFile(ctx, srcPath, joinPath(dstPath, targetName), onProgress, onConflict)
            // ... handle result
        }

        // 移动操作：删除源文件
        if drb.clipboard.Operation == OpMove && err == nil {
            if drb.clipboard.FileInfo.IsDir {
                err = srcSFTP.RemoveAll(srcPath)
            } else {
                err = srcSFTP.Remove(srcPath)
            }
            if err != nil {
                // D-08: 回滚
                dstFullPath := joinPath(dstPath, targetName)
                if rmErr := dstSFTP.Remove(dstFullPath); rmErr != nil {
                    drb.app.QueueUpdateDraw(func() {
                        drb.showStatusError("Move failed. Manual cleanup needed: " + dstFullPath)
                    })
                }
            }
        }
    }()
}
```

### 两阶段进度回调（参考 FileBrowser remotePasteFile 的 combinedProgress）
```go
// Source: internal/adapters/ui/file_browser/file_browser.go:1352-1368
var dlDone bool
combinedProgress := func(p domain.TransferProgress) {
    if p.Done && !dlDone {
        dlDone = true
        drb.app.QueueUpdateDraw(func() {
            drb.transferModal.ResetProgress()
            drb.transferModal.fileLabel = fmt.Sprintf("Uploading to %s: %s",
                drb.aliasForPane(dstPaneIdx), targetName)
        })
        return
    }
    label := fmt.Sprintf("Downloading from %s: %s",
        drb.aliasForPane(srcPaneIdx), drb.clipboard.FileInfo.Name)
    drb.app.QueueUpdateDraw(func() {
        drb.transferModal.fileLabel = label
        drb.transferModal.Update(p)
    })
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| CopyRemoteFile (单 SFTP 连接内复制) | RelayTransferService (两个独立 SFTP 连接中转) | Phase 13 | 跨远端传输能力 |
| FileBrowser 剪贴板 (local/remote) | DualRemoteFileBrowser 剪贴板 (source/target) | Phase 13 | SourcePane 语义变更 |
| modeCopy/modeMove (TransferModal) | modeCrossRemote (新增模式) | Phase 13 | 两阶段进度显示区分源/目标 |

**现有模式完全适用：**
- CopyRemoteFile 的 download->temp->upload 两阶段模式已在 v1.3 充分验证
- TransferModal 的多模式状态机（progress/cancelConfirm/conflictDialog/summary）已验证
- Clipboard 的生命周期管理（设置->消费->成功清除/失败保留）已验证
- goroutine + QueueUpdateDraw 的异步操作模式已验证
- buildConflictHandler 的 channel 同步机制已验证

## Open Questions

1. **RelayTransferService 实例的创建位置**
   - What we know: 不应在 main.go 中创建（ARCHITECTURE.md 明确指出）。应在 DualRemoteFileBrowser 内部创建，因为需要 sourceSFTP 和 targetSFTP 两个实例。
   - What's unclear: RelayTransferService 是作为 DualRemoteFileBrowser 的字段，还是作为构造函数参数注入。
   - Recommendation: 在 DualRemoteFileBrowser 内部创建（持有 srcSFTP/dstSFTP 引用），不需要外部注入。与 transfer.New() 的使用模式一致。

2. **RelayDir 的多文件取消清理**
   - What we know: DownloadDir/UploadDir 内部在 context.Canceled 时停止剩余文件传输，但不会删除已成功传输的文件。CopyRemoteDir 也有相同行为。
   - What's unclear: 用户取消目录传输后，目标端已上传的部分文件是否需要清理。
   - Recommendation: 与 CopyRemoteDir 行为一致 -- 不清理已成功传输的文件。这是已知限制，不在 v1.4 范围内解决。TransferModal.ShowCanceledSummary 可以显示 "Partial transfer" 提示。

3. **handlePaste 中的 targetName 冲突时的重命名路径**
   - What we know: buildConflictHandler 中 ConflictRename 使用 nextAvailableName() 函数。
   - What's unclear: 跨远端场景下，nextAvailableName 需要检查目标 SFTP（dstSFTP.Stat），而不是本地 os.Stat。
   - Recommendation: buildConflictHandler 的适配需要接受目标 SFTP 作为参数，用于 Stat 检查和 nextAvailableName 调用。

## Environment Availability

> Step 2.6: SKIPPED (no external dependencies identified)

本 Phase 纯代码变更，依赖的 Go 标准库、tview、tcell、zap 均已在 go.mod 中。不涉及新的外部工具或服务。

## Sources

### Primary (HIGH confidence)
- `internal/adapters/data/transfer/transfer_service.go` -- CopyRemoteFile/CopyRemoteDir 两阶段实现模式
- `internal/adapters/ui/file_browser/transfer_modal.go` -- TransferModal 多模式状态机
- `internal/adapters/ui/file_browser/file_browser.go` -- Clipboard struct、handleCopy/handlePaste/handleMove、buildConflictHandler
- `internal/adapters/ui/file_browser/dual_remote_browser.go` -- DualRemoteFileBrowser 结构和布局
- `internal/adapters/ui/file_browser/dual_remote_browser_handlers.go` -- handleGlobalKeys 按键路由
- `internal/adapters/ui/file_browser/remote_pane.go` -- RemotePane API、clipboardProvider 回调
- `internal/core/ports/transfer.go` -- TransferService 接口
- `internal/core/ports/file_service.go` -- SFTPService 接口
- `.planning/research/ARCHITECTURE.md` -- RelayTransferService 设计方案

### Secondary (MEDIUM confidence)
- `.planning/STATE.md` -- "RelayTransferService 组合两个 transfer.New() 实例" 决策
- `.planning/phases/07-copy-clipboard/07-CONTEXT.md` -- 剪贴板设计决策
- `.planning/phases/08-move-integration/08-CONTEXT.md` -- 移动实现设计决策

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - 所有依赖已在项目中使用，无新外部依赖
- Architecture: HIGH - 基于已验证的 CopyRemoteFile 两阶段模式和 RelayTransferService 设计方案
- Pitfalls: HIGH - 基于代码审查和已验证模式的已知行为

**Research date:** 2026-04-16
**Valid until:** 30 days (stable domain, all patterns already verified in codebase)
