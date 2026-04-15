# Phase 8: Move & Integration - Research

**Researched:** 2026-04-15
**Domain:** TUI file operations -- move within pane + conflict dialogs + progress display
**Confidence:** HIGH

## Summary

Phase 8 在 Phase 7 复制/剪贴板基础上扩展三个核心能力：(1) `x` 键标记移动源并显示 `[M]` 前缀，(2) `p` 键根据剪贴板 Operation 类型分发复制或移动逻辑，(3) 所有粘贴操作在目标文件已存在时弹出冲突对话框。移动 = 复制 + 删除源文件，失败时保留源文件不变。

**关键技术发现:** 移动操作不需要新增任何 Port 接口方法。`FileService` 已有 `Copy`/`CopyDir`/`Remove`/`RemoveAll`/`Rename`，`TransferService` 已有 `CopyRemoteFile`/`CopyRemoteDir`，`SFTPService` 已有 `Remove`/`RemoveAll`。全部改动集中在 UI 层：剪贴板扩展（`OpMove`）、`handlePaste()` 分发逻辑、`TransferModal` 新增 `modeMove`、以及面板前缀渲染扩展。

**主要风险点:** 远程移动的非原子性 -- copy 成功但 delete 源文件失败时需要 cleanup 目标副本。这是 STATE.md 中已标记的 blocker/concern，CONTEXT.md D-04/D-05 已定义 cleanup 策略。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** 所有粘贴操作（复制和移动）在目标文件已存在时弹出冲突对话框（覆盖/跳过/重命名），替代 Phase 7 D-06 的同目录自动重命名。TransferModal 已有 conflictDialog 模式，可复用其布局和 actionCh 机制。
- **D-02:** 冲突对话框选择「重命名」时使用现有 `nextAvailableName()` 逻辑生成目标名称（file.1.txt 格式）。
- **D-03:** 移动 = 复制 + 删除源文件。本地移动：FileService.Copy/CopyDir + FileService.Remove/RemoveAll。远程移动：TransferService.CopyRemoteFile/CopyRemoteDir + SFTPService.Remove/RemoveAll。
- **D-04:** 移动操作失败时（MOV-03），保留源文件不变。如果复制阶段成功但删除源文件失败，尝试清理目标目录的副本以恢复原始状态。清理失败则在状态栏提示用户手动清理。
- **D-05:** 远程移动的 cleanup 策略：CopyRemoteFile/CopyRemoteDir 产生的临时文件在 copy 阶段已由 defer 清理。如果后续 delete 源文件失败，需要额外删除已上传到目标路径的副本。
- **D-06:** `x` 键标记移动源，Clipboard.Operation 设为 OpMove，文件列表显示 `[M]` 前缀。`x` 和 `c` 共享同一剪贴板状态（单文件模式），新操作替换旧标记。
- **D-07:** `p` 键粘贴时根据 Clipboard.Operation 判断执行复制还是移动。粘贴成功后清除剪贴板。粘贴失败不清除（允许重试）。
- **D-08:** 远程移动新增 TransferModal `modeMove` 模式。复制阶段显示 "Moving: filename"（复用 progress bar），删除源阶段显示 "Deleting source..."（简单状态文本，无需进度条）。
- **D-09:** 本地复制和移动保持同步执行，不显示进度（本地磁盘操作通常很快）。仅远程操作通过 TransferModal 显示进度。

### Claude's Discretion
- [M] 前缀在 Name 列中的具体渲染颜色（建议与 [C] 区分，如黄色/红色）
- TransferModal modeMove 的具体 UI 布局细节
- 删除源阶段 "Deleting source..." 的显示位置和样式
- 状态栏移动操作提示文本
- 冲突对话框的默认选中项（建议 Skip 或 Rename，不默认 Overwrite）

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| MOV-01 | x 键标记移动源，显示 [M] 前缀 | OpMove 已定义但未启用；clipboardProvider 回调模式已建立（Phase 7）；[C] 前缀渲染可平行扩展为 [M] |
| MOV-02 | p 键执行移动（复制+删除源文件） | handlePaste() 已有分发入口；本地 FileService.Copy/Remove 和远程 CopyRemoteFile/SFTPService.Remove 均已实现 |
| MOV-03 | 移动失败时保留源文件 | D-04 定义 cleanup 策略；FileService.Remove/RemoveAll 已可用；本地 Rename 可用于同目录移动优化 |
| PRG-01 | 复制/移动大文件时显示进度条 | TransferModal modeCopy 已有完整实现（progress bar + cancel + speed/ETA）；modeMove 可复用 progress bar 渲染 |
| CNF-01 | 目标文件存在时弹出冲突对话框 | buildConflictHandler() 已实现（channel 同步 + Stat 检查）；TransferModal conflictDialog 模式已有三行布局 |
| CNF-02 | 多文件冲突逐个询问 | 当前单文件剪贴板模式下不存在多文件场景；此需求在未来多文件剪贴板（v1.3+）时才需要 |
</phase_requirements>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.24.6 | Runtime | 项目标准 |
| tview | v0.0.0 | TUI framework | 项目约束：不可引入其他 UI 框架 |
| tcell/v2 | v2.9.0 | Terminal cell rendering | 与 tview 配套 |
| pkg/sftp | v1.13.10 | Remote file operations (Remove/RemoveAll) | 已有依赖，提供远程删除 |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| zap | 1.27.0 | Structured logging | 移动失败的错误日志 |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Copy+Delete 移动 | os.Rename / SFTP.Rename | 同文件系统内 Rename 更高效，但跨文件系统会失败；Copy+Delete 更安全可靠 |

**Installation:** 无需安装新依赖。所有操作使用已有接口方法。

## Architecture Patterns

### Pattern 1: Clipboard 生命周期管理

**What:** 剪贴板通过 `Clipboard` struct 管理状态，支持 `OpCopy` 和 `OpMove` 两种操作类型。

**When to use:** `x`/`c` 键设置剪贴板，`p` 键消费剪贴板，`Esc` 清除剪贴板。

**Current code (file_browser.go:38-55):**
```go
type ClipboardOp int

const (
    OpCopy ClipboardOp = iota
    // OpMove is reserved for Phase 8.
)

type Clipboard struct {
    Active     bool
    SourcePane int
    FileInfo   domain.FileInfo
    SourceDir  string
    Operation  ClipboardOp
}
```

**Phase 8 扩展:** 启用 `OpMove` 常量，`handleCopy()` 已设 `OpCopy`，新增 `handleMove()` 设 `OpMove`。

### Pattern 2: handlePaste() 分发逻辑

**What:** `handlePaste()` 根据 `clipboard.Operation` 类型分发到复制或移动处理函数。

**Current code (file_browser.go:928-969):** handlePaste() 已有完整的入口逻辑：
1. 检查剪贴板是否 Active
2. 检查是否跨面板粘贴
3. 检查远程连接
4. 计算目标路径（同目录自动重命名）
5. 分发到 handleLocalPaste() 或 handleRemotePaste()

**Phase 8 扩展:** 在步骤 5 之前增加 Operation 类型判断：
```go
// Pseudocode for handlePaste extension
if fb.clipboard.Operation == OpMove {
    fb.handleLocalMove(sourcePath, targetPath, targetName)
    // or
    fb.handleRemoteMove(sourcePath, targetPath, targetName)
} else {
    fb.handleLocalPaste(sourcePath, targetPath, targetName)
    // or
    fb.handleRemotePaste(sourcePath, targetPath, targetName)
}
```

### Pattern 3: TransferModal 多模式状态机

**What:** TransferModal 通过 `modalMode` 枚举在不同显示模式间切换。每个模式有独立的 Draw 和 HandleKey 行为。

**Current modes (transfer_modal.go:51-59):**
```go
const (
    modeProgress       modalMode = iota
    modeCancelConfirm
    modeConflictDialog
    modeSummary
    modeCopy                            // Phase 7
)
```

**Phase 8 扩展:** 新增 `modeMove`。modeMove 在复制阶段复用 drawProgress（与 modeCopy 共享 progress bar 渲染），在删除源阶段可切换为简单的文本显示（如直接修改 fileLabel 和 infoLine 而非新增 mode）。

**关键决策:** D-08 说"复制阶段显示 Moving: filename（复用 progress bar），删除源阶段显示 Deleting source..."。删除源阶段是瞬时的（SFTP Remove 是单次 RPC），不需要进度条。建议在删除源前通过 QueueUpdateDraw 修改 fileLabel = "Deleting source..." 即可，不需要新增 mode。

### Pattern 4: clipboardProvider 回调

**What:** 面板通过 `func() (bool, string, string)` 回调查询剪贴板状态，渲染 [C] 前缀。

**Current code (file_browser.go:127-132):**
```go
fb.localPane.SetClipboardProvider(func() (bool, string, string) {
    return fb.clipboard.Active, fb.clipboard.FileInfo.Name, fb.clipboard.SourceDir
})
```

**Current rendering (local_pane.go:171-186):** 只检查 `active && clipName == fi.Name && clipDir == currentPath`，硬编码 `[C]` 前缀。

**Phase 8 扩展选项:**
- 选项 A: 扩展 clipboardProvider 返回值签名，增加 Operation 类型 -> `func() (bool, string, string, ClipboardOp)`
- 选项 B: 在面板渲染中直接访问 FileBrowser.clipboard（需传入 fb 引用或 Operation 字段）
- 选项 C（推荐）: 保持签名不变，但面板在渲染时根据前缀文本区分。FileBrowser 设置剪贴板时将 "[C]" 或 "[M]" 编码到返回的 string 中。

**推荐选项 A** -- 改动最小，类型安全。clipboardProvider 签名变为 `func() (bool, string, string, ClipboardOp)`，面板根据 Operation 决定前缀文本和颜色。

### Pattern 5: 冲突对话框复用

**What:** buildConflictHandler() 已实现完整的冲突处理流程：Stat 检查 -> ShowConflict -> actionCh 阻塞 -> 处理用户选择。

**Current 限制 (D-06 from Phase 7):** 同目录粘贴时自动重命名，不弹冲突对话框。

**Phase 8 改变 (D-01):** 所有粘贴操作（复制和移动）在目标文件已存在时弹出冲突对话框，替代同目录自动重命名。

**实现要点:**
- handlePaste() 中的同目录自动重命名逻辑需要改为冲突检查 + 弹窗
- buildConflictHandler() 已支持 ctx 取消，可直接复用
- 冲突对话框选择「重命名」时调用 nextAvailableName() 生成新名称（D-02）

### Anti-Patterns to Avoid

- **在 TransferService 中新增 Move 方法:** 移动 = Copy + Delete 是 UI 层编排逻辑，不应下沉到 service 层。这保持 service 层职责单一。
- **同目录移动使用 Copy+Delete:** 同目录移动应直接调用 FileService.Rename（原子操作），而非先复制再删除。更高效且原子性更好。
- **在 buildConflictHandler 中区分复制和移动:** 冲突对话框的行为（Overwrite/Skip/Rename）不因操作类型而异。buildConflictHandler 无需改动。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 远程文件删除 | 自实现递归删除 | `SFTPService.RemoveAll()` | pkg/sftp 已处理边界情况（权限、符号链接） |
| 本地文件删除 | 自实现 | `FileService.Remove()`/`FileService.RemoveAll()` | os.Remove/os.RemoveAll 已足够 |
| 冲突重命名 | 自实现文件名递增 | `nextAvailableName()` | 已有 file.1.txt 格式实现 |
| 进度显示 | 自实现 progress bar | `TransferModal` modeCopy 渲染逻辑 | 已有完整 progress bar + speed + ETA |
| 冲突对话框 | 自实现弹窗 | `TransferModal.ShowConflict()` + `buildConflictHandler()` | 已有 channel 同步机制 |
| 状态栏错误提示 | 自实现 | `showStatusError()` | 已有 3 秒自动清除 |

**Key insight:** Phase 8 的所有操作都是对已有组件的编排，不需要新建任何核心组件。移动逻辑 = handleCopy + handleDelete 的组合，冲突对话框 = buildConflictHandler 的直接复用。

## Common Pitfalls

### Pitfall 1: 远程移动的 cleanup 失败导致数据不一致

**What goes wrong:** CopyRemoteFile 成功（文件已到目标路径），但 SFTPService.Remove(源路径) 失败（权限不足），此时用户看到"源文件仍在、目标文件也在"，造成数据重复。如果再尝试 cleanup 目标文件也失败，用户得到两个副本且无法自动恢复。

**Why it happens:** SFTP Remove 失败可能因为源目录权限、文件被锁定、或 SSH 连接中断。远程文件系统状态不可控。

**How to avoid (D-04):**
1. Copy 阶段成功 -> 尝试 Remove 源文件
2. Remove 失败 -> 尝试 Remove 目标副本（cleanup）
3. Cleanup 失败 -> 状态栏显示警告"Move partially failed: source kept, target copy may need manual cleanup"
4. 剪贴板不清除（允许重试）

**Warning signs:** 删除源文件时的 error 应被 log.Errorw 记录，且在 UI 上给用户明确反馈。

### Pitfall 2: 同目录移动使用 Copy+Delete 而非 Rename

**What goes wrong:** 用户在同一目录内"移动"文件（实质是改名），如果使用 Copy+Delete，会先复制一份再删除源文件。这比直接 Rename 慢得多（尤其大文件），且如果 Copy 成功但 Delete 失败，留下重复文件。

**Why it happens:** handlePaste() 的统一逻辑可能不区分同目录和跨目录场景。

**How to avoid:** 在 handlePaste() 中检查 `currentPath == clipboard.SourceDir`，如果是同目录移动，直接调用 `FileService.Rename()`，绕过 Copy+Delete 流程。Ren-01/REN-02 已验证 Rename 在两个面板上都可用。

### Pitfall 3: [M] 前缀渲染与 [C] 前缀冲突

**What goes wrong:** clipboardProvider 回调只返回 `(bool, string, string)`，面板渲染时无法区分 [C] 和 [M]。如果只扩展渲染逻辑而不改回调签名，面板会在 OpMove 时仍显示 [C]。

**Why it happens:** Phase 7 的 clipboardProvider 设计没有预留 Operation 类型参数。

**How to avoid:** 扩展 clipboardProvider 签名为 `func() (bool, string, string, ClipboardOp)`，两个面板同步修改。这是纯 UI 层改动，不影响其他组件。

### Pitfall 4: buildConflictHandler 中的 activePane 依赖

**What goes wrong:** buildConflictHandler() 内部使用 `fb.activePane` 判断冲突检查方向（检查远程还是本地文件）。如果粘贴操作发生在与当前 activePane 不同的面板（虽然当前单面板粘贴不会发生），冲突检查会指向错误的目标。

**Why it happens:** buildConflictHandler() 闭包捕获 `fb.activePane`，这在当前单面板粘贴流程中是正确的。

**How to avoid:** 当前实现是安全的——handlePaste() 已验证 `clipboard.SourcePane == fb.activePane`。但如果未来扩展多文件剪贴板，需要将 pane 索引作为参数传入。Phase 8 不需要改动。

### Pitfall 5: 状态栏快捷键提示未更新

**What goes wrong:** setStatusBarDefault() 和 updateStatusBarTemp() 中的快捷键提示文本不包含 `x` 键提示，用户不知道可以用 `x` 标记移动。

**Why it happens:** Phase 7 添加 `c` 时已更新提示文本，但 Phase 8 的 `x` 是新增快捷键。

**How to avoid:** 在 setStatusBarDefault()、updateStatusBarConnection()、updateStatusBarTemp() 中添加 `[white]x[-] Move` 提示。

## Code Examples

### 移动标记 -- handleMove()

```go
// handleMove handles the 'x' key: mark current file as move source (MOV-01).
// Pattern mirrors handleCopy() with OpMove instead of OpCopy.
func (fb *FileBrowser) handleMove() {
    row, _ := fb.getActiveSelection()
    cell := fb.getActiveCell(row, 0)
    if cell == nil {
        return
    }
    fi, ok := cell.GetReference().(domain.FileInfo)
    if !ok {
        return
    }

    if fb.activePane == 1 && !fb.remotePane.IsConnected() {
        fb.showStatusError("Not connected to remote")
        return
    }

    fb.clipboard = Clipboard{
        Active:     true,
        SourcePane: fb.activePane,
        FileInfo:   fi,
        SourceDir:  fb.getCurrentPanePath(),
        Operation:  OpMove, // D-06
    }

    fb.refreshPane(fb.activePane)
    fb.focusOnItem(fb.activePane, fi.Name)
    fb.updateStatusBarTemp(fmt.Sprintf("[#FF6B6B]Move: %s[-]", fi.Name))
}
```

### handlePaste() 扩展 -- 同目录移动优化

```go
// In handlePaste(), after targetPath calculation:
// Same-directory move optimization (D-03 specific idea): use Rename instead of Copy+Delete
if fb.clipboard.Operation == OpMove && currentPath == fb.clipboard.SourceDir {
    // Rename to targetName (or original name if no conflict)
    // This is atomic and avoids Copy+Delete overhead
    go func() {
        err := fs.Rename(sourcePath, targetPath)
        fb.app.QueueUpdateDraw(func() {
            if err != nil {
                fb.showStatusError(fmt.Sprintf("Move failed: %s", trimError(err.Error(), 50)))
                return
            }
            fb.clipboard = Clipboard{}
            fb.refreshPane(fb.activePane)
            fb.focusOnItem(fb.activePane, targetName)
        })
    }()
    return
}
```

### 远程移动 -- cleanup 策略

```go
// handleRemoteMove performs a remote file move via CopyRemoteFile + Remove source (D-03, D-04).
func (fb *FileBrowser) handleRemoteMove(sourcePath, targetPath, targetName string) {
    fb.transferring = true
    ctx, cancel := context.WithCancel(context.Background())
    fb.transferCancel = cancel

    // Show TransferModal in modeMove (D-08)
    fb.transferModal.ShowMove(fb.clipboard.FileInfo.Name)
    fb.app.QueueUpdateDraw(func() {})

    go func() {
        // Phase 1: Copy remote source to target
        onConflict := fb.buildConflictHandler(ctx)
        var copyErr error
        if fb.clipboard.FileInfo.IsDir {
            _, copyErr = fb.transferSvc.CopyRemoteDir(ctx, sourcePath, targetPath, fb.moveProgressCallback(), onConflict)
        } else {
            copyErr = fb.transferSvc.CopyRemoteFile(ctx, sourcePath, targetPath, fb.moveProgressCallback(), onConflict)
        }

        if copyErr != nil {
            // Copy failed -- source preserved (D-04: MOV-03 satisfied)
            fb.app.QueueUpdateDraw(func() {
                fb.showStatusError(fmt.Sprintf("Move failed: %s", trimError(copyErr.Error(), 50)))
                fb.transferring = false
                fb.transferModal.Hide()
            })
            return
        }

        // Phase 2: Delete source (D-08: show "Deleting source...")
        fb.app.QueueUpdateDraw(func() {
            fb.transferModal.fileLabel = "Deleting source..."
            fb.transferModal.infoLine = ""
            fb.transferModal.etaLine = ""
        })

        var removeErr error
        if fb.clipboard.FileInfo.IsDir {
            removeErr = fb.sftpService.RemoveAll(sourcePath)
        } else {
            removeErr = fb.sftpService.Remove(sourcePath)
        }

        fb.app.QueueUpdateDraw(func() {
            fb.transferCancel = nil
            if removeErr != nil {
                // D-04: Copy succeeded but delete failed -- try cleanup target
                fb.log.Errorw("move: failed to delete source, attempting cleanup", "source", sourcePath, "error", removeErr)
                if cleanupErr := fb.cleanupMoveTarget(targetPath, fb.clipboard.FileInfo.IsDir); cleanupErr != nil {
                    fb.log.Warnw("move: cleanup also failed", "target", targetPath, "error", cleanupErr)
                    fb.showStatusError("Move failed: source kept, target copy may need manual cleanup")
                } else {
                    fb.showStatusError("Move failed: source kept, target cleaned up")
                }
                fb.transferring = false
                fb.transferModal.Hide()
                return // D-07: do NOT clear clipboard on failure
            }

            // Success
            fb.transferModal.Hide()
            fb.transferring = false
            fb.clipboard = Clipboard{} // D-07: clear on success
            fb.refreshPane(fb.activePane)
            fb.focusOnItem(fb.activePane, targetName)
        })
    }()
}
```

### 粘贴冲突检查（替换同目录自动重命名）

```go
// In handlePaste(), replace the same-directory auto-rename logic (D-01):
// OLD (Phase 7):
//   if currentPath == fb.clipboard.SourceDir {
//       targetPath = nextAvailableName(targetPath, statFunc)
//   }
//
// NEW (Phase 8): Check for conflict and show dialog
if _, err := statFunc(targetPath); err == nil {
    // Target exists -- show conflict dialog via buildConflictHandler
    // This replaces the auto-rename for ALL paste operations (copy AND move)
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    onConflict := fb.buildConflictHandler(ctx)
    action, newPath := onConflict(targetName)
    switch action {
    case domain.ConflictSkip:
        return // user chose to skip
    case domain.ConflictRename:
        targetName = filepath.Base(newPath)
        targetPath = fb.buildPath(fb.activePane, currentPath, targetName)
    case domain.ConflictOverwrite:
        // continue with original path
    }
}
```

### clipboardProvider 签名扩展

```go
// file_browser.go -- updated clipboardProvider wiring
fb.localPane.SetClipboardProvider(func() (bool, string, string, ClipboardOp) {
    return fb.clipboard.Active, fb.clipboard.FileInfo.Name, fb.clipboard.SourceDir, fb.clipboard.Operation
})
fb.remotePane.SetClipboardProvider(func() (bool, string, string, ClipboardOp) {
    return fb.clipboard.Active, fb.clipboard.FileInfo.Name, fb.clipboard.SourceDir, fb.clipboard.Operation
})
```

```go
// local_pane.go / remote_pane.go -- updated rendering in populateTable
if lp.clipboardProvider != nil {
    if active, clipName, clipDir, op := lp.clipboardProvider(); active && clipName == fi.Name && clipDir == lp.currentPath {
        if op == OpMove {
            nameText = "[M] " + nameText
            nameColor = tcell.GetColor("#FF6B6B") // red for move marker
        } else {
            nameText = "[C] " + nameText
            nameColor = tcell.GetColor("#00FF7F") // green for copy marker
        }
        nameBg = tcell.Color236
        nameAttrs = tcell.AttrBold
    } else if lp.selected[fi.Name] {
        // ... existing Space selection logic
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| 同目录粘贴自动重命名 | 弹出冲突对话框让用户选择 | Phase 8 (D-01) | handlePaste() 中替换 nextAvailableName 为 buildConflictHandler |
| 只有 OpCopy 剪贴板 | OpCopy + OpMove 双模式 | Phase 8 (D-06) | 启用已定义的 OpMove 常量 |
| 粘贴只执行复制 | 粘贴根据 Operation 分发 | Phase 8 (D-07) | handlePaste() 内部分支 |
| modeCopy 用于远程复制 | modeMove 用于远程移动 | Phase 8 (D-08) | TransferModal 新增 mode 或复用 modeCopy |

**Deprecated/outdated:**
- Phase 7 D-06（同目录自动重命名）被 D-01 替代。handlePaste() 中的 `nextAvailableName` 调用需改为冲突检查流程。

## Open Questions

1. **CNF-02 在单文件剪贴板模式下是否需要处理？**
   - What we know: CNF-02 要求"多文件复制/移动时，每个冲突文件单独询问"。当前剪贴板是单文件模式（D-03 from Phase 7），不存在多文件场景。
   - What's unclear: 是否应在 Phase 8 实现 CNF-02 的数据结构/接口，即使功能不启用。
   - Recommendation: Phase 8 不实现 CNF-02（单文件模式下无意义）。在 REQUIREMENTS.md traceability 中标记 CNF-02 为 "Partially -- single file only"。多文件支持在 v1.3+ 实现。

2. **TransferModal modeMove 是否需要独立的 mode 枚举值？**
   - What we know: modeMove 在复制阶段与 modeCopy 行为完全一致（progress bar + speed + ETA），仅在标题文本和删除源阶段有区别。
   - What's unclear: 是否值得新增 mode 值，还是直接复用 modeCopy 并修改标题。
   - Recommendation: 新增 `modeMove` 枚举值，即使 Draw 行为与 modeCopy 共享。理由：(1) HandleKey 中 Esc 取消可能需要区分行为；(2) 未来如果移动需要额外的 UI 反馈（如删除源阶段），有独立 mode 更灵活；(3) 代码可读性更好。

3. **本地移动是否需要冲突对话框？**
   - What we know: D-01 说"所有粘贴操作（复制和移动）在目标文件已存在时弹出冲突对话框"。本地粘贴当前是同步操作，没有 goroutine，buildConflictHandler 需要 UI 线程同步。
   - What's unclear: 同步操作中弹出异步冲突对话框（需要 goroutine + channel 同步）是否过于复杂。
   - Recommendation: 是的，本地粘贴也需要冲突对话框。将 handleLocalPaste() 改为 goroutine 执行（与 handleRemotePaste 一致），在 goroutine 中调用 buildConflictHandler。这统一了本地和远程的粘贴流程。

## Environment Availability

Step 2.6: SKIPPED (no external dependencies identified -- all changes are code-only within existing project dependencies)

## Sources

### Primary (HIGH confidence)
- 项目源码 -- 所有 canonical_refs 中列出的文件已逐一阅读
  - `internal/core/ports/file_service.go` -- FileService 接口（Copy/CopyDir/Remove/RemoveAll/Rename 已存在）
  - `internal/core/ports/transfer.go` -- TransferService 接口（CopyRemoteFile/CopyRemoteDir 已存在）
  - `internal/adapters/ui/file_browser/file_browser.go` -- Clipboard struct, handleCopy/handlePaste, buildConflictHandler
  - `internal/adapters/ui/file_browser/transfer_modal.go` -- modalMode 枚举, modeCopy 实现
  - `internal/adapters/ui/file_browser/file_browser_handlers.go` -- handleGlobalKeys 按键路由
  - `internal/adapters/ui/file_browser/local_pane.go` -- clipboardProvider 回调和 [C] 前缀渲染
  - `internal/adapters/ui/file_browser/remote_pane.go` -- clipboardProvider 回调和 [C] 前缀渲染
  - `internal/adapters/data/transfer/transfer_service.go` -- CopyRemoteFile/CopyRemoteDir 实现
  - `internal/adapters/data/local_fs/local_fs.go` -- Copy/CopyDir/Remove/RemoveAll/Rename 实现
  - `internal/adapters/data/sftp_client/sftp_client.go` -- Remove/RemoveAll 实现
- `.planning/research/STACK.md` -- SFTP 原语和 Go stdlib 能力验证
- `.planning/phases/07-copy-clipboard/07-CONTEXT.md` -- Phase 7 剪贴板设计决策

### Secondary (MEDIUM confidence)
- `.planning/STATE.md` -- Blocker/concern 记录（"移动操作非原子性"）
- `.planning/REQUIREMENTS.md` -- MOV/PRG/CNF 需求定义

### Tertiary (LOW confidence)
- 无 -- 所有研究基于项目源码直接验证

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- 零新依赖，全部使用已有接口
- Architecture: HIGH -- 所有模式已在 Phase 6/7 中建立并验证
- Pitfalls: HIGH -- 基于 Phase 6/7 实际踩过的坑总结
- Cleanup strategy: MEDIUM -- D-04/D-05 定义了策略但未在生产环境验证

**Research date:** 2026-04-15
**Valid until:** 30 days (stable domain -- all interfaces and patterns are established)
