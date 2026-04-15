# Phase 7: Copy & Clipboard - Research

**Researched:** 2026-04-15
**Domain:** TUI File Copy with Clipboard State (Go/tview/tcell, Clean Architecture)
**Confidence:** HIGH

## Summary

Phase 7 在现有文件浏览器中添加面板内文件复制功能。用户按 `c` 标记当前文件为复制源，导航到目标目录后按 `p` 粘贴。剪贴板跨目录导航保持（CLP-02），通过 Esc 或新 c/x 操作清除（CLP-03）。

实现分为三个层次：(1) **端口层** -- FileService 新增 `Copy`/`CopyDir` 方法，TransferService 新增 `CopyRemoteFile`/`CopyRemoteDir` 方法；(2) **适配器层** -- LocalFS 使用 `os.Open`+`os.Create`+`io.Copy`+`os.Chtimes`+`os.Chmod`，TransferService 复用现有 download+re-upload 基础设施；(3) **UI 层** -- FileBrowser 添加 clipboard 字段，handleGlobalKeys 添加 c/p 按键，LocalPane/RemotePane 的 populateTable 添加 [C] 前缀渲染，TransferModal 新增 modeCopy 模式。

**零新外部依赖。** 所有原语已在项目中（Go stdlib `io`/`os` 包、现有 TransferService 的 DownloadFile/UploadFile）。

**Primary recommendation:** 遵循 CONTEXT.md 锁定决策，剪贴板存储在 FileBrowser struct 上，本地复制使用 stdlib，远程复制复用 TransferService download+re-upload。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** 远程复制通过 TransferService 新增 `CopyRemoteFile`/`CopyRemoteDir` 方法实现，内部复用现有 download+re-upload 基础设施（32KB buffer、onProgress 回调、conflict handler、ctx 取消传播）。不新建 CopyService port。
- **D-02:** 本地复制通过 FileService 新增 `Copy`/`CopyDir` 方法实现，底层使用 `io.Copy` + `os.Chtimes` + `os.Chmod`。不调用外部 cp 命令。
- **D-03:** 剪贴板为单文件模式 -- `c` 只标记当前光标所在文件，不支持 Space 多选批量标记。
- **D-04:** 剪贴板数据结构存储来源面板索引（0=local, 1=remote）、FileInfo、源目录路径。粘贴时验证目标面板与来源面板一致（防止跨面板粘贴）。
- **D-05:** 剪贴板清除时机：Esc 清除、新 c/x 操作清除、粘贴成功后自动清除。粘贴失败不清除（允许重试）。
- **D-06:** 同目录粘贴（复制到源目录）时自动重命名为 `file.1.txt` 格式，复用现有 `nextAvailableName` 逻辑。
- **D-07:** 本地复制保留源文件权限（os.Chmod）和修改时间（os.Chtimes）。远程复制由 download+re-upload 过程自然保留。
- **D-08:** 远程复制进度复用 TransferModal，新增 `modeCopy` 模式。下载阶段显示 "Downloading: filename"，上传阶段显示 "Uploading: filename"。复用现有进度条、取消流程、完成摘要。本地复制不显示进度（同步操作，瞬间完成）。

### Claude's Discretion

- 剪贴板状态在 FileBrowser 上的具体字段命名和位置
- [C] 前缀在 Name 列中的具体渲染方式（颜色、位置）
- TransferModal modeCopy 的具体 UI 布局（标题文本、进度格式）
- 状态栏提示文本（"1 file copied"、"Clipboard: file.txt" 等）
- 远程复制失败时的错误处理细节（下载成功但上传失败时的清理策略）

### Deferred Ideas (OUT OF SCOPE)

None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CPY-01 | 用户选中文件/目录后按 c 键，文件被标记为复制源，列表中显示 [C] 前缀 | populateTable 中添加 clipboard 前缀检查，参考现有 `selected` map 的 `* ` 前缀模式 |
| CPY-02 | 用户导航到目标目录后按 p 键，系统将标记的文件/目录复制到当前目录 | FileService.Copy/CopyDir (本地) + TransferService.CopyRemoteFile/CopyRemoteDir (远程) |
| CPY-03 | 复制目录时递归复制所有内容，远程端通过 download+re-upload 实现 | D-01 锁定：复用 DownloadDir+UploadDir；D-02 锁定：本地 filepath.WalkDir |
| CLP-01 | 剪贴板有标记时，被标记文件显示 [C] 前缀 | populateTable 中检查 FileBrowser.clipboard 是否匹配当前文件 |
| CLP-02 | 导航到其他目录后剪贴板标记仍然保留 | clipboard 存储在 FileBrowser struct 上（非 per-pane），跨导航自然保持 |
| CLP-03 | 按 Esc 或新 c/x 操作时清除之前剪贴板标记 | handleGlobalKeys Esc 分支 + handleCopy handler 开头清除逻辑 |
| RCP-01 | 远程复制显示统一进度视图，包含已复制文件数和总大小 | TransferModal 新增 modeCopy，复用 Show/Update/ShowSummary 流程 |
</phase_requirements>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `io` | builtin | `io.Copy` for local file copy | 标准库，零依赖，性能优秀 |
| Go stdlib `os` | builtin | `os.Open`, `os.Create`, `os.Chtimes`, `os.Chmod` | 标准库，本地文件操作基础 |
| Go stdlib `path/filepath` | builtin | `filepath.WalkDir`, `filepath.Join` | 标准库，递归目录遍历 |
| Go stdlib `context` | builtin | `context.Context` 取消传播 | 已在 TransferService 中使用 |
| tview/tcell | existing | TUI 框架 | 项目约束：不可引入其他 UI 框架 |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| TransferService | existing | DownloadFile/UploadFile/DownloadDir/UploadDir | 远程面板内复制的基础设施 |
| TransferModal | existing | modeCopy 进度显示 | 远程复制操作的可视化 |
| nextAvailableName | existing | 冲突重命名 `file.1.txt` | 同目录粘贴时的目标路径生成 |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| FileService.Copy/CopyDir | 独立 CopyService 接口 | CONTEXT.md D-02 已锁定：使用 FileService，不新建接口 |
| TransferService.CopyRemote* | 直接在 UI 层编排 download+upload | CONTEXT.md D-01 已锁定：通过 TransferService 方法封装 |
| io.Copy | 手动 32KB buffer copy | D-07 要求保留 Chtimes/Chmod，io.Copy 更简洁但需要额外调用来保留元数据 |

**Installation:** 无需安装新依赖。

**Version verification:** 所有使用的库为 Go 标准库或已存在于项目中的代码，无需版本验证。

## Architecture Patterns

### Recommended Project Structure

```
internal/
├── core/
│   ├── domain/
│   │   └── transfer.go          # 已有: TransferProgress, ConflictHandler (不变)
│   └── ports/
│       ├── file_service.go       # 修改: 新增 Copy, CopyDir 方法
│       └── transfer.go           # 修改: 新增 CopyRemoteFile, CopyRemoteDir 方法
├── adapters/
│   ├── data/
│   │   ├── local_fs/
│   │   │   └── local_fs.go       # 修改: 新增 Copy, CopyDir 实现
│   │   └── transfer/
│   │       └── transfer_service.go # 修改: 新增 CopyRemoteFile, CopyRemoteDir 实现
│   └── ui/
│       └── file_browser/
│           ├── file_browser.go       # 修改: 添加 clipboard 字段, [C] 渲染
│           ├── file_browser_handlers.go # 修改: 添加 c/p 按键处理
│           ├── local_pane.go         # 修改: populateTable 添加 clipboard 前缀
│           ├── remote_pane.go        # 修改: populateTable 添加 clipboard 前缀
│           └── transfer_modal.go     # 修改: 新增 modeCopy 模式
```

### Pattern 1: Clipboard State Management

**What:** 剪贴板数据结构存储在 FileBrowser struct 上，跨目录导航保持。
**When to use:** 所有需要读取剪贴板状态的场景（handleCopy、handlePaste、populateTable）。
**Example:**

```go
// Clipboard holds the state for copy/move operations.
type Clipboard struct {
    Active    bool           // whether clipboard has content
    SourcePane int            // 0 = local, 1 = remote
    FileInfo  domain.FileInfo // the marked file/directory
    SourceDir string          // directory path where the file was marked
    Operation ClipboardOp    // OpCopy (Phase 7) or OpMove (Phase 8)
}

type ClipboardOp int
const (
    OpCopy ClipboardOp = iota
    OpMove // Phase 8
)

// In FileBrowser struct:
clipboard Clipboard
```

**Key insight:** 剪贴板存储在 FileBrowser 上而非 per-pane，因为 CLP-02 要求跨目录导航保持。`selected` map（Space 多选）是 per-pane 的，与剪贴板是独立概念。

### Pattern 2: [C] Prefix Rendering in populateTable

**What:** 在 LocalPane/RemotePane 的 populateTable 中，检查 FileBrowser.clipboard 是否匹配当前文件行，匹配时添加 [C] 前缀。
**When to use:** 每次 Refresh() 重新渲染表格时。
**Example:**

```go
// In populateTable, after the existing selected[* ] check:
if clipboard != nil && clipboard.Active {
    if fi.Name == clipboard.FileInfo.Name && currentPath == clipboard.SourceDir {
        nameText = "[C] " + nameText
        nameColor = tcell.GetColor("#00FF7F") // green for clipboard marker
    }
}
```

**关键设计问题:** populateTable 在 LocalPane/RemotePane 中，但 clipboard 状态在 FileBrowser 上。需要通过回调或参数传递 clipboard 信息。推荐方案：给 populateTable 传递 `clipboard *Clipboard` 参数，或使用 `OnClipboardCheck func(fi domain.FileInfo, currentPath string) bool` 回调。

### Pattern 3: Local File Copy (FileService.Copy/CopyDir)

**What:** 使用 Go stdlib 实现，保留权限和修改时间。
**When to use:** 本地面板内复制操作。
**Example:**

```go
// Copy copies a single file from src to dst, preserving permissions and modification time.
func (l *LocalFS) Copy(src, dst string) error {
    srcFile, err := os.Open(src)
    if err != nil {
        return fmt.Errorf("open source: %w", err)
    }
    defer srcFile.Close()

    srcInfo, err := srcFile.Stat()
    if err != nil {
        return fmt.Errorf("stat source: %w", err)
    }

    dstFile, err := os.Create(dst)
    if err != nil {
        return fmt.Errorf("create destination: %w", err)
    }
    defer dstFile.Close()

    if _, err := io.Copy(dstFile, srcFile); err != nil {
        return fmt.Errorf("copy data: %w", err)
    }

    // Preserve permissions and modification time (D-07)
    if err := os.Chmod(dst, srcInfo.Mode()); err != nil {
        return fmt.Errorf("chmod: %w", err)
    }
    if err := os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime()); err != nil {
        return fmt.Errorf("chtimes: %w", err)
    }

    return nil
}
```

```go
// CopyDir recursively copies a directory from src to dst.
func (l *LocalFS) CopyDir(src, dst string) error {
    srcInfo, err := os.Stat(src)
    if err != nil {
        return err
    }
    if err := os.Mkdir(dst, srcInfo.Mode()); err != nil {
        return err
    }

    entries, err := os.ReadDir(src)
    if err != nil {
        return err
    }

    for _, entry := range entries {
        srcPath := filepath.Join(src, entry.Name())
        dstPath := filepath.Join(dst, entry.Name())

        if entry.IsDir() {
            if err := l.CopyDir(srcPath, dstPath); err != nil {
                return err
            }
        } else {
            if err := l.Copy(srcPath, dstPath); err != nil {
                return err
            }
        }
    }
    return nil
}
```

### Pattern 4: Remote Copy via Download+Re-upload (TransferService)

**What:** 远程复制复用现有 TransferService 基础设施，通过临时目录中转。
**When to use:** 远程面板内复制操作。
**Example:**

```go
// CopyRemoteFile copies a file within the remote filesystem by downloading to a
// temporary local file and re-uploading to the destination path.
func (ts *transferService) CopyRemoteFile(
    ctx context.Context,
    remoteSrc, remoteDst string,
    onProgress func(domain.TransferProgress),
    onConflict domain.ConflictHandler,
) error {
    // Create temp file for intermediate storage
    tmpFile, err := os.CreateTemp("", "lazyssh-copy-*")
    if err != nil {
        return fmt.Errorf("create temp file: %w", err)
    }
    tmpPath := tmpFile.Name()
    tmpFile.Close() // DownloadFile will create its own handle
    defer os.Remove(tmpPath)

    // Phase 1: Download remote source to temp
    downloadProgress := func(p domain.TransferProgress) {
        if onProgress != nil {
            p.FileName = "DL: " + p.FileName // distinguish phase in progress
            onProgress(p)
        }
    }
    if err := ts.DownloadFile(ctx, remoteSrc, tmpPath, downloadProgress, nil); err != nil {
        return fmt.Errorf("download for copy: %w", err)
    }

    // Phase 2: Upload temp to remote destination
    uploadProgress := func(p domain.TransferProgress) {
        if onProgress != nil {
            p.FileName = "UL: " + p.FileName
            onProgress(p)
        }
    }
    if err := ts.UploadFile(ctx, tmpPath, remoteDst, uploadProgress, onConflict); err != nil {
        return fmt.Errorf("upload for copy: %w", err)
    }

    return nil
}
```

**关键设计问题:** CopyRemoteFile 内部调用 DownloadFile 和 UploadFile，每个都有各自的 onConflict 回调。对于 CopyRemoteFile，只有上传阶段需要冲突检测（下载到临时文件不会冲突）。

### Pattern 5: TransferModal modeCopy

**What:** 复用 TransferModal 多模式状态机，新增 modeCopy 用于远程复制进度显示。
**When to use:** 远程面板内复制操作（下载+上传两阶段）。
**Example:**

```go
const (
    modeProgress       modalMode = iota
    modeCancelConfirm
    modeConflictDialog
    modeSummary
    modeCopy           // Phase 7: remote copy progress
)

// ShowCopy displays the modal in copy mode.
func (tm *TransferModal) ShowCopy(filename string) {
    tm.visible = true
    tm.mode = modeCopy
    tm.cancelConfirmed = false
    tm.SetTitle(fmt.Sprintf(" Copying %s ", filename))
    tm.bar = NewProgressBar()
    tm.speedSamples = tm.speedSamples[:0]
    tm.fileLabel = fmt.Sprintf("Copying: %s", filename)
    tm.infoLine = ""
    tm.etaLine = ""
}
```

**Draw 分支:** modeCopy 的 Draw 与 modeProgress 完全相同（进度条+速度+ETA），标题和 fileLabel 文本不同（"Copying" vs "Uploading"/"Downloading"）。可以直接复用 drawProgress，或在 Draw switch 中让 modeCopy 也调用 drawProgress。

### Pattern 6: handleCopy and handlePaste Handler Pattern

**What:** 遵循现有 handleDelete/handleRename/handleMkdir 的 goroutine+QueueUpdateDraw 模式。
**When to use:** 所有文件操作 handler。
**Example:**

```go
// handleCopy handles 'c' key: mark current file for copy.
func (fb *FileBrowser) handleCopy() {
    row, _ := fb.getActiveSelection()
    cell := fb.getActiveCell(row, 0)
    if cell == nil {
        return
    }
    fi, ok := cell.GetReference().(domain.FileInfo)
    if !ok {
        return
    }

    currentPath := fb.getCurrentPanePath()
    fb.clipboard = Clipboard{
        Active:    true,
        SourcePane: fb.activePane,
        FileInfo:  fi,
        SourceDir: currentPath,
        Operation: OpCopy,
    }

    // Refresh current pane to show [C] prefix
    fb.refreshPane(fb.activePane)
    fb.updateStatusBarTemp(fmt.Sprintf("[#00FF7F]Clipboard: %s[-]", fi.Name))
}

// handlePaste handles 'p' key: paste copied file to current directory.
func (fb *FileBrowser) handlePaste() {
    if !fb.clipboard.Active {
        return
    }
    if fb.clipboard.SourcePane != fb.activePane {
        fb.showStatusError("Cross-pane paste not supported (v1.3+)")
        return
    }

    // Same-directory check: source == target
    currentPath := fb.getCurrentPanePath()
    if currentPath == fb.clipboard.SourceDir {
        // Auto-rename using nextAvailableName (D-06)
        // ...
    }

    // Execute copy in goroutine
    go func() {
        // ... copy logic ...
        fb.app.QueueUpdateDraw(func() {
            if err != nil {
                fb.showStatusError(...)
                return // Don't clear clipboard on failure (D-05)
            }
            fb.clipboard = Clipboard{} // Clear on success (D-05)
            fb.refreshPane(fb.activePane)
        })
    }()
}
```

### Anti-Patterns to Avoid

- **使用 `selected` map 代替 clipboard:** `selected` 是 per-pane Space 多选状态，导航时自动清除。clipboard 需要跨导航保持，必须独立存储。
- **在 LocalPane/RemotePane 内部存储 clipboard:** 会导致跨 pane 引用问题，且违反 D-04 的来源面板验证需求。
- **远程复制时直接通过 SFTP 读取+写入:** SFTP 协议确实支持 ReadFile+CreateFile，但这样无法复用 TransferService 的进度跟踪、冲突处理、取消传播基础设施。D-01 明确锁定使用 download+re-upload。
- **在 handlePaste 中同步执行复制:** 本地大文件复制可能阻塞 UI。应使用 goroutine + QueueUpdateDraw 模式。
- **在 Esc 处理中清除 clipboard 后调用 close():** Esc 在 FileBrowser 中的默认行为是关闭整个浏览器。剪贴板清除应该在 close 之前执行，但由于剪贴板状态随 FileBrowser 销毁而丢失，无需显式清除。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 进度条 UI | 自定义进度条渲染 | TransferModal (已有) | 已有完整的进度/取消/冲突/摘要状态机，modeCopy 只需新增一个模式值 |
| 冲突重命名 `file.1.txt` | 自实现递增后缀 | `nextAvailableName()` (file_browser.go:587) | 已实现且经过测试，D-06 明确要求复用 |
| 文件复制 I/O | 手动 buffer 循环 | `io.Copy` (本地) / DownloadFile+UploadFile (远程) | io.Copy 使用内部优化的 buffer，远程复制 D-01 要求复用现有基础设施 |
| 状态栏错误闪烁 | 自定义定时器 | `showStatusError()` (file_browser.go:608) | 已实现 3 秒自动清除，所有文件操作失败提示都使用此方法 |
| goroutine 安全 UI 更新 | 手动 channel | `app.QueueUpdateDraw()` | tview 标准模式，所有文件操作都使用此模式 |
| overlay 组件生命周期 | 自定义显示/隐藏 | 现有 overlay pattern (visible + Draw + HandleKey) | TransferModal/ConfirmDialog/InputDialog 已验证的模式 |

**Key insight:** 本阶段的大部分 UI 工作是在现有模式上扩展，而非创建新模式。唯一的新 UI 概念是 [C] 前缀渲染，它遵循与 `* ` (Space 多选) 前缀完全相同的渲染模式。

## Common Pitfalls

### Pitfall 1: populateTable 无法访问 FileBrowser.clipboard

**What goes wrong:** populateTable 在 LocalPane/RemotePane 中，clipboard 在 FileBrowser 上。直接引用会造成循环依赖。
**Why it happens:** Go 不允许循环 import。LocalPane/RemotePane 在 file_browser 包内，FileBrowser 也在同一包内，所以不会循环。但 populateTable 方法没有接收 clipboard 参数。
**How to avoid:** 方案一：给 populateTable 传递 clipboard 参数。方案二：给 LocalPane/RemotePane 添加 `clipboardProvider func() *Clipboard` 回调字段，在 build() 时注入。推荐方案二，与现有 `onPathChange`/`onFileAction` 回调模式一致。
**Warning signs:** 编译错误 "import cycle not allowed"。

### Pitfall 2: [C] 前缀与 `* ` 前缀冲突

**What goes wrong:** 文件同时被 Space 选中（`* ` 前缀）和 clipboard 标记（`[C]` 前缀），两者都修改 nameText 和 nameColor。
**Why it happens:** D-03 明确剪贴板为单文件模式，但用户仍可先 Space 选中文件再按 c 标记。
**How to avoid:** 确定优先级。建议 clipboard 优先于 selected：如果文件同时被标记，显示 [C] 而非 `* `。或者在 handleCopy 时清除当前 pane 的 selected map。
**Warning signs:** 视觉上看到 `* [C] filename` 双重前缀。

### Pitfall 3: 远程复制临时文件泄漏

**What goes wrong:** CopyRemoteFile 创建临时文件后，如果上传阶段失败，临时文件未清理。
**Why it happens:** defer os.Remove(tmpPath) 只在函数退出时执行，但如果 DownloadFile 失败提前返回，临时文件可能残留。
**How to avoid:** 使用 defer os.Remove(tmpPath) 确保无论如何都清理。这是标准 Go 模式，defer 在函数返回时总是执行。
**Warning signs:** 系统临时目录中积累 lazyssh-copy-* 文件。

### Pitfall 4: 远程复制下载成功但上传失败时的清理策略

**What goes wrong:** DownloadFile 成功创建了目标临时文件，UploadFile 失败，但源文件未被删除（因为是 copy，不是 move）。这是正确行为 -- 不需要清理源文件。
**Why it happens:** 这是 Claude's Discretion 中明确提到的问题。
**How to avoid:** D-05 规定粘贴失败不清除剪贴板（允许重试）。临时文件通过 defer 清理。不需要额外逻辑。
**Warning signs:** 磁盘空间被临时文件占满（不太可能，因为 defer 会清理）。

### Pitfall 5: 同目录粘贴时的 nextAvailableName 路径构建

**What goes wrong:** nextAvailableName 接受 statFunc 参数（用于检查文件是否存在），本地使用 os.Stat，远程使用 sftpService.Stat。需要根据面板类型传入正确的 statFunc。
**Why it happens:** 现有 nextAvailableName (file_browser.go:587) 已处理此差异，接受 `func(string) (os.FileInfo, error)` 参数。
**How to avoid:** 复用现有调用模式，参考 buildConflictHandler 中的用法。
**Warning signs:** 同目录粘贴时总是使用原始文件名（没有 .1 后缀）。

### Pitfall 6: Esc 键在 clipboard 有内容时仍然关闭 FileBrowser

**What goes wrong:** 用户按 Esc 期望只清除剪贴板，但实际上关闭了整个文件浏览器。
**Why it happens:** 当前 Esc 的默认行为是 `fb.close()`。D-05 要求 Esc 清除剪贴板。
**How to avoid:** 在 handleGlobalKeys 的 Esc 分支中，先检查 clipboard 是否有内容，如果有则清除并返回 nil（不调用 close）。如果没有 clipboard 内容，才调用 close。
**Warning signs:** 用户按 Esc 后文件浏览器意外关闭。

### Pitfall 7: TransferModal modeCopy 与 modeProgress 的 HandleKey 行为差异

**What goes wrong:** modeCopy 应该支持 Esc 取消（与 modeProgress 一致），但如果直接复用 modeProgress 的 HandleKey 分支，可能触发错误的 onDismiss 回调。
**Why it happens:** TransferModal 的 HandleKey 按 mode switch 分发，modeCopy 需要自己的 case 或与 modeProgress 共享。
**How to avoid:** 在 HandleKey 的 switch 中，让 modeCopy 和 modeProgress 共享同一个 case 分支（`case modeProgress, modeCopy:`），因为取消/进度行为完全一致。
**Warning signs:** 按 Esc 时 modeCopy 不响应，或触发了错误的 dismiss 回调。

## Code Examples

Verified patterns from existing source code:

### Clipboard 状态在 Esc 中的处理

```go
// In handleGlobalKeys, Esc branch:
case tcell.KeyESC:
    // D-05: clear clipboard before closing browser
    if fb.clipboard.Active {
        fb.clipboard = Clipboard{}
        fb.refreshPane(fb.activePane)
        fb.updateStatusBarTemp("[#00FF7F]Clipboard cleared[-]")
        return nil
    }
    if fb.transferModal != nil && fb.transferModal.IsVisible() {
        fb.transferModal.HandleKey(event)
        return nil
    }
    fb.close()
    return nil
```

### 状态栏快捷键提示更新

```go
// setStatusBarDefault 需要添加 c 和 p 提示:
func (fb *FileBrowser) setStatusBarDefault() {
    fb.statusBar.SetText("[white]Tab[-] Switch  [white]c[-] Copy  [white]p[-] Paste  [white]d[-] Delete  [white]R[-] Rename  [white]m[-] Mkdir  [white]s[-] Sort  [white]F5[-] Transfer  [white]Esc[-] Back")
}

// updateStatusBarConnection 也需要同步更新
func (fb *FileBrowser) updateStatusBarConnection(msg string) {
    fb.statusBar.SetText(msg + "  [white]Tab[-] Switch  [white]c[-] Copy  [white]p[-] Paste  [white]d[-] Delete  [white]R[-] Rename  [white]m[-] Mkdir  [white]s[-] Sort  [white]F5[-] Transfer  [white]Esc[-] Back")
}

// updateStatusBarTemp 也需要同步更新
func (fb *FileBrowser) updateStatusBarTemp(msg string) {
    fb.statusBar.SetText(msg + "  [white]Tab[-] Switch  [white]c[-] Copy  [white]p[-] Paste  [white]d[-] Delete  [white]R[-] Rename  [white]m[-] Mkdir  [white]s[-] Sort  [white]F5[-] Transfer  [white]Esc[-] Back")
}
```

### FileService.Copy/CopyDir 接口签名

```go
// In internal/core/ports/file_service.go:
type FileService interface {
    // ... existing methods ...
    // Copy copies a single file from src to dst on the same filesystem.
    // Preserves file permissions and modification time.
    Copy(src, dst string) error
    // CopyDir recursively copies a directory from src to dst on the same filesystem.
    // Preserves directory structure, file permissions, and modification times.
    CopyDir(src, dst string) error
}
```

### TransferService.CopyRemoteFile/CopyRemoteDir 接口签名

```go
// In internal/core/ports/transfer.go:
type TransferService interface {
    // ... existing methods ...
    // CopyRemoteFile copies a file within the remote filesystem via download+re-upload.
    CopyRemoteFile(ctx context.Context, remoteSrc, remoteDst string, onProgress func(domain.TransferProgress), onConflict domain.ConflictHandler) error
    // CopyRemoteDir copies a directory within the remote filesystem via download+re-upload.
    CopyRemoteDir(ctx context.Context, remoteSrc, remoteDst string, onProgress func(domain.TransferProgress), onConflict domain.ConflictHandler) ([]string, error)
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| 独立 CopyService 接口 | FileService.Copy/CopyDir (本地) + TransferService.CopyRemote* (远程) | Phase 7 CONTEXT.md D-01/D-02 | 减少接口数量，复用现有基础设施 |
| os.Rename 移动文件 | Copy + Delete (Phase 8) | Phase 8 | 当前阶段不涉及 |
| 状态栏无 clipboard 提示 | 状态栏显示 "Clipboard: filename" | Phase 7 | 用户可感知剪贴板状态 |

**Deprecated/outdated:**
- STACK.md 中建议的独立 CopyService 接口已被 CONTEXT.md D-01/D-02 否决

## Open Questions

1. **远程复制 CopyRemoteDir 的 onProgress 策略**
   - What we know: D-08 要求下载阶段显示 "Downloading: filename"，上传阶段显示 "Uploading: filename"
   - What's unclear: 两阶段的 fileIndex/fileTotal 如何计算？是各自独立计数（下载 N 个文件 + 上传 N 个文件）还是统一计数（总共 2N 步）？
   - Recommendation: 各自独立计数更合理。下载阶段 fileTotal=N，上传阶段 fileTotal=N。TransferModal 的 title 可以显示 "Copying (1/2)" 表示阶段而非文件。

2. **剪贴板标记的持久化**
   - What we know: clipboard 存储在 FileBrowser struct 内存中，FileBrowser 关闭即丢失。
   - What's unclear: 是否需要持久化到 ~/.lazyssh/clipboard.json？
   - Recommendation: 不需要。剪贴板是临时操作状态，会话级存储足够。CONTEXT.md 未提及持久化需求。

3. **[C] 前缀与 `* ` 前缀的优先级**
   - What we know: D-03 限定剪贴板为单文件，selected map 是多文件。
   - What's unclear: 同一文件同时被 Space 选中且被 c 标记时，显示哪个前缀？
   - Recommendation: handleCopy 时清除当前 pane 的 selected map（Space 选中状态），避免视觉冲突。或者 [C] 优先于 `* `（因为 c 操作是最近的用户意图）。

## Environment Availability

Step 2.6: SKIPPED (no external dependencies identified -- all changes are code-level modifications using Go stdlib and existing project infrastructure)

## Validation Architecture

> Skipped per .planning/config.json: `workflow.nyquist_validation` is explicitly set to `false`.

## Sources

### Primary (HIGH confidence)
- 项目源码直接阅读:
  - `internal/core/ports/file_service.go` -- FileService 接口（需扩展）
  - `internal/core/ports/transfer.go` -- TransferService 接口（需扩展）
  - `internal/adapters/data/local_fs/local_fs.go` -- LocalFS 实现（需新增 Copy/CopyDir）
  - `internal/adapters/data/transfer/transfer_service.go` -- TransferService 实现（需新增 CopyRemote*）
  - `internal/adapters/ui/file_browser/file_browser.go` -- FileBrowser orchestrator（需添加 clipboard）
  - `internal/adapters/ui/file_browser/file_browser_handlers.go` -- handleGlobalKeys（需添加 c/p）
  - `internal/adapters/ui/file_browser/transfer_modal.go` -- TransferModal 状态机（需新增 modeCopy）
  - `internal/adapters/ui/file_browser/local_pane.go` -- LocalPane populateTable（需添加 [C] 前缀）
  - `internal/adapters/ui/file_browser/remote_pane.go` -- RemotePane populateTable（需添加 [C] 前缀）
  - `internal/core/domain/file_info.go` -- FileInfo 域模型
  - `internal/core/domain/transfer.go` -- TransferProgress, ConflictHandler
- CONTEXT.md 锁定决策 D-01 到 D-08

### Secondary (MEDIUM confidence)
- `.planning/research/STACK.md` -- SFTP 原语和 Go stdlib 能力分析
- Go 标准库文档 -- `io.Copy`, `os.Chtimes`, `os.Chmod`, `filepath.WalkDir`

### Tertiary (LOW confidence)
- 无需外部搜索验证，所有实现细节均从项目源码和标准库推导

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - 全部使用 Go stdlib 和现有项目代码，无新依赖
- Architecture: HIGH - 完全遵循现有 Clean Architecture 模式，扩展而非新建
- Pitfalls: HIGH - 所有 pitfalls 基于项目源码分析，有具体代码行引用

**Research date:** 2026-04-15
**Valid until:** 30 days (稳定领域 -- 文件复制 API 和 UI 模式不会变化)
