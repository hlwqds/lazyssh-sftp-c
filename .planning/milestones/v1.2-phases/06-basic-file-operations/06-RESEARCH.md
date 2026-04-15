# Phase 6: Basic File Operations - Research

**Researched:** 2026-04-15
**Domain:** TUI overlay components + port interface expansion for delete/rename/mkdir
**Confidence:** HIGH

## Summary

Phase 6 在现有双栏文件浏览器上添加三项基本文件操作：删除（单文件/多选/递归目录）、重命名、新建目录。技术实现分三层：(1) 扩展 `FileService` port 接口添加 `Remove`/`RemoveAll`/`Rename`/`Mkdir`/`Stat` 方法；(2) 在 `SFTPClient` 和 `LocalFS` adapter 中实现这些方法（全部是 `pkg/sftp` 或 `os` 标准库的一行代理）；(3) 创建两个新的 overlay 组件 `ConfirmDialog` 和 `InputDialog`，遵循 `TransferModal`/`RecentDirs` 的已确立 overlay 模式。

零新外部依赖。所有底层原语已存在于 `pkg/sftp v1.13.10` 和 Go `os` 标准库中。主要工作集中在 UI 层：overlay 组件的 Draw/HandleKey 实现、按键路由链集成、操作后刷新和光标定位逻辑。

**Primary recommendation:** 严格遵循 TransferModal/RecentDirs overlay 模式构建 ConfirmDialog 和 InputDialog，将 FileService 接口扩展作为第一个 plan 的任务以建立编译时约束。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** ConfirmDialog 和 InputDialog 作为独立 overlay 组件（confirm_dialog.go, input_dialog.go），不扩展 TransferModal。遵循 TransferModal/RecentDirs 的 overlay 模式：嵌入 `*tview.Box`，`visible` 标志，`Draw()` + `HandleKey()` 手动管理。
- **D-02:** InputDialog 组件被重命名和新建目录共用，通过标题和预填内容区分用途。内部使用 `tview.InputField` 处理文本编辑，通过 `doneFunc` 回调处理 Enter/Esc。
- **D-03:** 单文件删除确认显示详细信息：文件名、大小、文件类型（目录/文件）、修改时间。格式类似 TransferModal 的 conflictDialog 模式。
- **D-04:** 多选批量删除显示"删除 N 个文件？共 X.XMB"的汇总确认，一个确认操作删除全部。
- **D-05:** 删除非空目录时额外显示递归警告："目录非空，将递归删除所有内容"。
- **D-06:** 重命名通过居中弹出 InputDialog 触发（R 键），预填当前文件名，光标位于文件名部分末尾（不含扩展名）。按 Enter 确认，Esc 取消。
- **D-07:** 新建目录通过居中弹出 InputDialog 触发（m 键），空输入框，按 Enter 创建，Esc 取消。创建后自动刷新列表并定位到新目录。
- **D-08:** 重命名目标名称已存在时，提示名称冲突，用户可选择覆盖或取消。
- **D-09:** 文件操作失败时（权限不足、文件不存在等），在状态栏显示红色错误信息，几秒后自动恢复默认文本。不打断用户操作流程。
- **D-10:** 删除/重命名/新建目录方法提升到 FileService 接口（共享），而非仅添加到 SFTPService。这确保本地和远程面板使用统一接口。
- **D-11:** SFTPService 已有的 Remove（仅文件/空目录）保留，新增 RemoveAll（递归删除）、Rename（重命名）、Mkdir（创建单个目录）。pkg/sftp 原生支持这些操作。

### Claude's Discretion
- ConfirmDialog 的具体布局细节（颜色、间距、边距）由 Claude 基于 TransferModal 的 cancelConfirm/conflictDialog 模式决定
- InputDialog 中 tview.InputField 的焦点管理方式（手动调用 InputHandler 而非依赖 tview focus 系统）
- 状态栏错误信息闪烁的具体持续时间（建议 3 秒）
- 递归删除是否显示进度（小目录同步执行，大目录可考虑状态栏提示"删除中..."）

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DEL-01 | 用户选中文件后按 d 键弹出确认对话框，确认后执行删除 | FileService.Remove 一行代理；ConfirmDialog overlay 遵循 RecentDirs 模式 |
| DEL-02 | 删除目录时递归删除所有内容，显示进度 | FileService.RemoveAll 代理 pkg/sftp.RemoveAll 或 os.RemoveAll；goroutine + 状态栏提示 |
| DEL-03 | Space 多选后按 d 键显示待删文件数量和总大小，批量删除 | Pane.SelectedFiles() 已有实现；计算总大小后汇总显示 |
| DEL-04 | 删除完成后自动刷新列表，光标定位到合理位置 | pane.Refresh() + Select(row) 定位到删除项的相邻行 |
| REN-01 | 选中文件/目录后按 R 键弹出输入框，预填当前文件名，Enter 确认 | InputDialog overlay + tview.InputField + SetText/setCursorEnd |
| REN-02 | 重命名目标名称已存在时提示冲突 | FileService.Stat 检查 + 第二次 ConfirmDialog 提示覆盖/取消 |
| MKD-01 | 按 m 键弹出输入框输入目录名，Enter 创建 | InputDialog overlay + FileService.Mkdir |
| MKD-02 | 新建目录完成后刷新列表，光标定位到新目录 | pane.Refresh() + 遍历查找新目录名并 Select |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.24.6 | Runtime | Project standard |
| pkg/sftp | v1.13.10 | SFTP file operations | Already in go.mod (indirect), provides RemoveAll/Rename/Mkdir |
| tview | latest | TUI framework | Project UI framework, provides InputField for text editing |
| tcell/v2 | v2.9.0 | Terminal cell manipulation | Project rendering layer |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| Go os package | stdlib | Local filesystem ops | Remove, RemoveAll, Rename, Mkdir, Stat |
| zap | v1.27.0 | Structured logging | Error logging for file operations |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `tview.InputField` in overlay | Manual text buffer + Draw | 重新实现光标移动、文本选择、Unicode 处理 -- 工作量大无收益 |
| `FileService.RemoveAll` | 自实现递归删除 | pkg/sftp.RemoveAll 已处理符号链接、权限错误等边界情况 |
| `os.RemoveAll` | 自实现递归删除 | 标准库已优化，无需自行实现 |

**Installation:**
```bash
# No new dependencies needed
go mod tidy  # Will promote pkg/sftp from indirect to direct
```

**Version verification:** `pkg/sftp v1.13.10` 已确认包含 `RemoveAll`、`Rename`、`Mkdir` 方法（已在 `~/go/pkg/mod/github.com/pkg/sftp@v1.13.10/client.go` 中验证行号 804/892/971/1038）。

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── core/
│   ├── domain/
│   │   └── file_info.go          # 不变 -- FileInfo 结构体
│   └── ports/
│       └── file_service.go        # 扩展: 添加 Remove/RemoveAll/Rename/Mkdir/Stat 到 FileService
├── adapters/
│   ├── data/
│   │   ├── local_fs/
│   │   │   └── local_fs.go        # 扩展: 实现 FileService 新增方法
│   │   └── sftp_client/
│   │       └── sftp_client.go     # 扩展: 实现 FileService 新增方法
│   └── ui/
│       └── file_browser/
│           ├── confirm_dialog.go   # 新增: ConfirmDialog overlay
│           ├── input_dialog.go     # 新增: InputDialog overlay
│           ├── file_browser.go     # 扩展: 添加 overlay 字段、handler 方法、Draw chain
│           ├── file_browser_handlers.go  # 扩展: overlay 拦截链
│           ├── local_pane.go       # 扩展: d/R/m 按键 + callbacks
│           └── remote_pane.go      # 扩展: d/R/m 按键 + callbacks
```

### Pattern 1: Overlay Component (ConfirmDialog/InputDialog)
**What:** 嵌入 `*tview.Box`，使用 `visible` 标志控制显示，手动 `Draw()` 和 `HandleKey()` 管理。
**When to use:** 所有需要拦截按键并显示自定义 UI 的弹出层。
**Example:**
```go
// Source: 项目现有 TransferModal/RecentDirs 模式
type ConfirmDialog struct {
    *tview.Box
    app        *tview.Application
    visible    bool
    message    string
    onConfirm  func()
    onCancel   func()
}

func (cd *ConfirmDialog) Draw(screen tcell.Screen) {
    if !cd.visible {
        return
    }
    cd.Box.DrawForSubclass(screen, cd)
    // ... render centered content using tview.Print
}

func (cd *ConfirmDialog) HandleKey(event *tcell.EventKey) *tcell.EventKey {
    if !cd.visible {
        return event
    }
    switch event.Rune() {
    case 'y':
        cd.Hide()
        if cd.onConfirm != nil { cd.onConfirm() }
        return nil
    case 'n':
        cd.Hide()
        if cd.onCancel != nil { cd.onCancel() }
        return nil
    }
    // Esc 也确认取消
    if event.Key() == tcell.KeyEscape {
        cd.Hide()
        if cd.onCancel != nil { cd.onCancel() }
        return nil
    }
    return nil // 消费所有按键
}
```

### Pattern 2: InputDialog with tview.InputField
**What:** overlay 中嵌入 `tview.InputField`，通过 `inputField.InputHandler()` 手动路由按键，绕过 tview focus 系统。
**When to use:** 需要用户文本输入的场景（重命名、新建目录）。
**Example:**
```go
// Source: ARCHITECTURE.md 研究推荐方案
func NewInputDialog(app *tview.Application) *InputDialog {
    id := &InputDialog{Box: tview.NewBox(), app: app}
    id.inputField = tview.NewInputField()
    id.inputField.SetDoneFunc(func(key tcell.Key) {
        if key == tcell.KeyEnter {
            if id.onSubmit != nil { id.onSubmit(id.inputField.GetText()) }
            id.Hide()
        } else if key == tcell.KeyEscape {
            if id.onCancel != nil { id.onCancel() }
            id.Hide()
        }
    })
    return id
}

func (id *InputDialog) HandleKey(event *tcell.EventKey) *tcell.EventKey {
    if !id.visible { return event }
    id.inputField.InputHandler(event, func(tview.Primitive) {})
    return nil // 消费所有按键
}
```

### Pattern 3: Key Routing Chain with Overlay Interception
**What:** `handleGlobalKeys` 顶部检查所有 overlay 的 `IsVisible()`，可见时将所有按键传递给对应 overlay 的 `HandleKey()`。
**When to use:** 新增任何 overlay 组件时。
**Example:**
```go
// Source: file_browser_handlers.go:32-36 (现有模式)
func (fb *FileBrowser) handleGlobalKeys(event *tcell.EventKey) *tcell.EventKey {
    // Overlay key interception chain (order matters)
    if fb.inputDialog != nil && fb.inputDialog.IsVisible() {
        return fb.inputDialog.HandleKey(event)
    }
    if fb.confirmDialog != nil && fb.confirmDialog.IsVisible() {
        return fb.confirmDialog.HandleKey(event)
    }
    if fb.recentDirs != nil && fb.recentDirs.IsVisible() {
        return fb.recentDirs.HandleKey(event)
    }
    if fb.transferModal != nil && fb.transferModal.IsVisible() {
        fb.transferModal.HandleKey(event)
        return nil
    }
    // ... global keys ...
}
```

### Pattern 4: Draw Chain Update
**What:** `FileBrowser.Draw()` 在 `Flex.Draw(screen)` 后依次调用各 overlay 的 `Draw()`。
**When to use:** 新增 overlay 组件时。
**Example:**
```go
// Source: file_browser.go:222-231
func (fb *FileBrowser) Draw(screen tcell.Screen) {
    fb.Flex.Draw(screen)
    if fb.transferModal != nil && fb.transferModal.IsVisible() {
        fb.transferModal.Draw(screen)
    }
    if fb.recentDirs != nil && fb.recentDirs.IsVisible() {
        fb.recentDirs.Draw(screen)
    }
    // 新增:
    if fb.confirmDialog != nil && fb.confirmDialog.IsVisible() {
        fb.confirmDialog.Draw(screen)
    }
    if fb.inputDialog != nil && fb.inputDialog.IsVisible() {
        fb.inputDialog.Draw(screen)
    }
}
```

### Pattern 5: Status Bar Error Display with Auto-Recovery
**What:** 使用 `updateStatusBarTemp()` 显示红色错误消息，通过 goroutine + `time.After` + `QueueUpdateDraw` 恢复默认文本。
**When to use:** 所有需要临时反馈的操作错误。
**Example:**
```go
// 基于 file_browser.go:508-511 的 updateStatusBarTemp 模式
func (fb *FileBrowser) showStatusError(msg string) {
    fb.updateStatusBarTemp(fmt.Sprintf("[#FF6B6B]%s[-]", msg))
    go func() {
        <-time.After(3 * time.Second)
        fb.app.QueueUpdateDraw(func() {
            fb.setStatusBarDefault()
        })
    }()
}
```

### Anti-Patterns to Avoid
- **使用 `tview.Modal` 做确认/输入对话框:** `tview.Modal` 使用 `app.SetRoot()` 替换整个视图，破坏 overlay draw chain，导致视觉残影。必须用 `*tview.Box` + 手动 `Draw()` 模式。
- **在 UI 线程中同步执行递归删除:** `RemoveAll` 对大目录可能耗时数分钟。必须在 goroutine 中执行，通过 `QueueUpdateDraw` 更新 UI。
- **在 pane 的 `SetSelectedFunc` 中处理删除/重命名:** `SetSelectedFunc` 已被 Enter 键占用（目录导航/文件传输），会冲突。
- **在 UI 层做 `if local then os.Remove else sftp.Remove`:** 违反 Clean Architecture，应该在 port 接口层统一。
- **ConfirmDialog/InputDialog 双重可见:** 必须确保同时只有一个 overlay 可见，避免按键路由歧义。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 递归删除目录 | 自实现递归遍历+逐文件 Remove | `pkg/sftp.RemoveAll` / `os.RemoveAll` | 已处理符号链接、权限错误、部分失败等边界情况 |
| 文本输入框 | 手动实现光标、文本编辑、Unicode | `tview.InputField` | 已处理光标移动、文本选择、Unicode 宽度计算 |
| 文件大小格式化 | 手动实现 B/KB/MB/GB 转换 | `formatSize()` (local_pane.go:333) | 已有实现，两个 pane 共用 |
| 路径拼接 | 手动字符串拼接 | `joinPath()` (remote_pane.go:441) / `filepath.Join()` | 远程用 joinPath，本地用 filepath.Join |
| 状态栏消息 | 手动 SetText + 计算布局 | `updateStatusBarTemp()` (file_browser.go:509) | 已有模式，包含键盘提示后缀 |

**Key insight:** Phase 6 的所有底层操作（Remove/RemoveAll/Rename/Mkdir/Stat）都是标准库或 pkg/sftp 的一行代理。复杂度集中在 UI 层的 overlay 交互和按键路由。不要在 adapter 层过度工程化。

## Common Pitfalls

### Pitfall 1: SFTP Remove 不能删除非空目录
**What goes wrong:** 调用 `FileService.Remove()` 删除非空目录，返回 `SSH_FX_FAILURE`。
**Why it happens:** `pkg/sftp.Client.Remove` 只能删除空目录或文件。递归删除需要 `RemoveAll`。
**How to avoid:** 删除操作根据 `IsDir` 选择 `Remove`（文件）或 `RemoveAll`（目录）。`RemoveAll` 对文件也是安全的（等价于 `Remove`），所以可以统一使用 `RemoveAll` 简化逻辑。
**Warning signs:** 删除目录时返回 "directory not empty" 或 "SSH_FX_FAILURE" 错误。
**Confidence:** HIGH -- 已在 pkg/sftp 源码和项目代码中验证。

### Pitfall 2: 删除后光标定位到错误位置
**What goes wrong:** 删除文件后刷新列表，光标跳到第一行或最后一行，而不是保持用户期望的位置。
**Why it happens:** `pane.Refresh()` 调用 `populateTable()` 后会 `Select(1, 0)`（选中第一行）。
**How to avoid:** 记住删除前的选中行号，刷新后定位到 `min(deletedRow, totalRows)`。如果删除的是最后一行，定位到新的最后一行。
**Warning signs:** 删除后用户需要重新导航到之前浏览的位置。
**Confidence:** HIGH -- 基于代码审查 local_pane.go:209-211 的 `Select(1, 0)` 硬编码。

### Pitfall 3: InputDialog 的 Enter/Esc 双重触发
**What goes wrong:** 在 HandleKey 中手动检查 `KeyEnter`，同时 InputField 的 `doneFunc` 也处理 Enter，导致 `onSubmit` 被调用两次。
**Why it happens:** `tview.InputField.InputHandler()` 会内部处理 Enter 键并触发 `doneFunc`。如果 HandleKey 也检查 Enter，会双重触发。
**How to avoid:** 只通过 `doneFunc` 处理 Enter/Esc，不在 HandleKey 中重复检查。HandleKey 只负责将按键路由到 `inputField.InputHandler()`。
**Warning signs:** 重命名操作执行两次，或创建两个同名目录。
**Confidence:** HIGH -- ARCHITECTURE.md 已详细分析此问题并给出正确方案。

### Pitfall 4: ConfirmDialog 中按键泄漏到背景 Table
**What goes wrong:** 用户在确认对话框中按 `y` 以外的字母键，这些键被传递到背景的 Table 组件，触发意外行为（如跳转到以该字母开头的文件）。
**Why it happens:** 如果 ConfirmDialog 的 `HandleKey` 返回 event 而非 nil，tview 会将 event 传递给下一个 InputCapture handler。
**How to avoid:** ConfirmDialog 可见时，HandleKey 必须对所有按键返回 nil（消费所有按键），仅对 `y`/`n`/`Esc` 执行对应操作。
**Warning signs:** 在确认对话框中按字母键后，背景文件列表的光标移动。
**Confidence:** HIGH -- 现有 RecentDirs 已正确实现此模式（recent_dirs.go:308: `return nil`）。

### Pitfall 5: 多选删除时 FileInfo 大小信息不可用
**What goes wrong:** 用户 Space 多选了多个文件后按 `d`，确认对话框需要显示总大小，但 `Pane.SelectedFiles()` 返回的 `FileInfo` 可能为零值。
**Why it happens:** `SelectedFiles()` 通过文件名在当前目录重新 ListDir 并匹配。如果 ListDir 使用缓存或文件被并发修改，Size 可能为 0。
**How to avoid:** 直接从 Table 单元格的 Reference 获取 FileInfo，而不是重新 ListDir。或在确认前调用 `FileService.Stat` 获取最新大小。
**Warning signs:** 确认对话框显示 "0B" 总大小。
**Confidence:** MEDIUM -- 需要在实现时验证 SelectedFiles() 返回的 Size 是否准确。

### Pitfall 6: 重命名时光标定位到扩展名前需要特殊处理
**What goes wrong:** 重命名 `config.yaml` 时，InputField 光标应该在 `config` 和 `.yaml` 之间，但 `tview.InputField` 的 `SetCursorEnd()` 将光标放到字符串末尾。
**Why it happens:** `tview.InputField` 没有原生的 "set cursor before extension" 方法。
**How to avoid:** 预填文件名后，计算最后一个 `.` 的位置，使用 `inputField.SetCursorOffset(extIndex)` 设置光标位置。需要验证 tview.InputField 是否暴露此 API。
**Warning signs:** 重命名时光标在 `.yaml` 后面，用户需要手动左移光标。
**Confidence:** MEDIUM -- 需要检查 tview.InputField 的 API 文档确认光标控制方法。

### Pitfall 7: 状态栏错误恢复 goroutine 泄漏
**What goes wrong:** 用户快速连续触发多个错误（如多次重命名失败），每次都启动一个 3 秒的恢复 goroutine。之前的 goroutine 在 3 秒后覆盖后续操作的状态栏内容。
**Why it happens:** `time.After` 不会取消，多个 goroutine 独立运行。
**How to avoid:** 使用一个 `statusErrorTimer` 字段，每次新错误触发时 `Stop` 之前的 timer，再启动新 timer。或使用 `context.Context` 取消机制。
**Warning signs:** 错误消息在 3 秒后突然恢复默认，覆盖了用户正在进行的操作反馈。
**Confidence:** HIGH -- 这是 Go timer 管理的经典问题。

### Pitfall 8: 重命名与已有文件冲突的二次确认
**What goes wrong:** 用户重命名文件名为已存在的文件名，需要弹出第二次 ConfirmDialog 提示覆盖。但如果 ConfirmDialog 已经在显示中（第一次确认），不能嵌套弹出。
**Why it happens:** 两个 ConfirmDialog 实例或同一个实例的两次 Show 调用可能冲突。
**How to avoid:** 重命名流程分两步：(1) InputDialog 输入新名称 -> onSubmit 回调中检查冲突；(2) 如果冲突，显示 ConfirmDialog 询问覆盖 -> onConfirm 执行 Rename。这两步是顺序的，不会同时显示两个 overlay。
**Warning signs:** 重命名冲突时 UI 行为异常或按键路由混乱。
**Confidence:** HIGH -- 基于架构设计，两步流程是顺序的。

## Code Examples

### FileService 接口扩展
```go
// Source: internal/core/ports/file_service.go (现有 + 新增方法)
type FileService interface {
    // --- Existing ---
    ListDir(path string, showHidden bool, sortField domain.FileSortField, sortAsc bool) ([]domain.FileInfo, error)

    // --- Phase 6 Additions ---
    // Remove deletes a single file or empty directory.
    Remove(path string) error

    // RemoveAll recursively deletes a directory and all its contents.
    RemoveAll(path string) error

    // Rename renames or moves a file/directory.
    Rename(oldPath, newPath string) error

    // Mkdir creates a single directory. Returns error if parent doesn't exist.
    Mkdir(path string) error

    // Stat returns file info for the given path.
    Stat(path string) (os.FileInfo, error)
}
```

### SFTPClient 新增方法
```go
// Source: internal/adapters/data/sftp_client/sftp_client.go
// 遵循现有 mutex 模式

func (c *SFTPClient) RemoveAll(path string) error {
    c.mu.Lock()
    client := c.client
    c.mu.Unlock()
    if client == nil {
        return fmt.Errorf("not connected: call Connect first")
    }
    return client.RemoveAll(path)
}

func (c *SFTPClient) Rename(oldPath, newPath string) error {
    c.mu.Lock()
    client := c.client
    c.mu.Unlock()
    if client == nil {
        return fmt.Errorf("not connected: call Connect first")
    }
    return client.Rename(oldPath, newPath)
}

func (c *SFTPClient) Mkdir(path string) error {
    c.mu.Lock()
    client := c.client
    c.mu.Unlock()
    if client == nil {
        return fmt.Errorf("not connected: call Connect first")
    }
    return client.Mkdir(path)
}
```

### LocalFS 新增方法
```go
// Source: internal/adapters/data/local_fs/local_fs.go
// 一行代理到 os 包

func (l *LocalFS) Remove(path string) error {
    return os.Remove(path)
}

func (l *LocalFS) RemoveAll(path string) error {
    return os.RemoveAll(path)
}

func (l *LocalFS) Rename(oldPath, newPath string) error {
    return os.Rename(oldPath, newPath)
}

func (l *LocalFS) Mkdir(path string) error {
    return os.Mkdir(path, 0o750)
}

func (l *LocalFS) Stat(path string) (os.FileInfo, error) {
    return os.Stat(path)
}
```

### 删除后光标定位逻辑
```go
// 删除后保持光标位置的推荐模式
func (fb *FileBrowser) deleteAndRefresh(pane FilePane, row int) {
    // row 是被删除项的行号（1-indexed，因为第0行是表头）
    // 记住当前位置用于恢复
    totalRows := pane.GetRowCount() // 包含表头

    // 执行删除...

    // 刷新列表
    pane.Refresh()

    // 重新计算光标位置
    newTotalRows := pane.GetRowCount()
    targetRow := row
    if targetRow >= newTotalRows {
        targetRow = newTotalRows - 1 // 超出范围则定位到最后一行
    }
    if targetRow < 1 {
        targetRow = 1 // 至少选中第一行数据
    }
    pane.Select(targetRow, 0)
}
```

### 新建目录后定位到新目录
```go
// 创建目录后在列表中找到并选中
func (fb *FileBrowser) mkdirAndFocus(pane FilePane, dirName string) {
    fullPath := pane.GetCurrentPath()
    dirPath := fullPath + "/" + dirName // 或使用 joinPath/filepath.Join

    // 执行 Mkdir...

    // 刷新列表
    pane.Refresh()

    // 遍历表格查找新目录并选中
    for row := 1; row < pane.GetRowCount(); row++ {
        cell := pane.GetCell(row, 0)
        if cell == nil { continue }
        ref := cell.GetReference()
        if ref == nil { continue }
        fi, ok := ref.(domain.FileInfo)
        if ok && fi.Name == dirName {
            pane.Select(row, 0)
            return
        }
    }
}
```

### Pane callback 注册模式
```go
// Source: local_pane.go:311 (OnFileAction 模式扩展)
// 在 FileBrowser.build() 中注册新回调

lp.onDelete = func(fi domain.FileInfo) {
    fb.handleDelete(fi, 0) // 0 = local pane
}
lp.onRename = func(fi domain.FileInfo) {
    fb.handleRename(fi, 0)
}
lp.onMkdir = func() {
    fb.handleMkdir(0)
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| 删除/重命名/新建在 port 接口外处理 | 提升到 FileService 统一接口 | Phase 6 | 本地和远程面板共享同一操作接口，UI 层无需类型判断 |
| 确认对话框用 tview.Modal | 独立 overlay 组件 (*tview.Box) | Phase 5 (RecentDirs) | 不替换 root view，overlay draw chain 保持一致 |
| 按键路由单一 overlay 检查 | 链式 overlay 拦截 | Phase 5 | 多个 overlay 互不干扰，按键不会泄漏 |

**Deprecated/outdated:**
- `SFTPService.Remove`/`Stat` 作为独立方法: Phase 6 将它们下沉到 `FileService`，`SFTPService` 通过嵌入继承。行为不变，仅接口层级调整。

## Open Questions

1. **tview.InputField 的光标控制 API**
   - What we know: `tview.InputField` 有 `SetText()` 方法设置文本。D-06 要求光标位于扩展名前。
   - What's unclear: tview.InputField 是否暴露 `SetCursorOffset(int)` 或类似方法来程序化设置光标位置。
   - Recommendation: 如果 tview 不支持光标偏移设置，使用 `SetText(nameWithoutExt)` + `SetText(fullName)` 技巧（先设短文本再设长文本），或接受光标在末尾作为 fallback。

2. **递归删除是否需要进度显示**
   - What we know: CONTEXT.md 将此列为 Claude's Discretion。小目录 RemoveAll 同步即可。大目录可能需要状态栏提示。
   - What's unclear: "大目录"的阈值是多少？500 文件？1000 文件？
   - Recommendation: Phase 6 对所有删除操作统一使用 goroutine + 状态栏 "Deleting..." 提示。不使用 TransferModal 显示详细进度（DEL-02 的 "显示进度" 可以是简单的状态栏文字，不是进度条）。如果用户反馈需要详细进度，在后续 phase 增强。

3. **多选删除的总大小计算**
   - What we know: `Pane.SelectedFiles()` 通过 ListDir 重新查询获取 FileInfo，其中包含 Size 字段。
   - What's unclear: 是否需要处理符号链接的大小（符号链接本身大小通常很小，但指向的文件可能很大）。
   - Recommendation: 统计时跳过目录（目录大小在文件系统中通常为 4096，无实际意义），只统计文件大小。符号链接按链接本身大小统计（这是 os.Stat/lstat 的默认行为）。

## Environment Availability

> Step 2.6: SKIPPED (no external dependencies identified -- Phase 6 only uses existing Go stdlib + pkg/sftp, both already in the project)

## Sources

### Primary (HIGH confidence)
- 项目源码分析:
  - `internal/core/ports/file_service.go` -- FileService + SFTPService 接口定义
  - `internal/adapters/data/sftp_client/sftp_client.go` -- SFTPClient 实现（Remove 已有，mutex 模式参考）
  - `internal/adapters/data/local_fs/local_fs.go` -- LocalFS 实现（ListDir 模式参考）
  - `internal/adapters/ui/file_browser/transfer_modal.go` -- Overlay 模式参考（Box + visible + Draw + HandleKey）
  - `internal/adapters/ui/file_browser/recent_dirs.go` -- Overlay 模式参考（简单 popup）
  - `internal/adapters/ui/file_browser/file_browser.go` -- FileBrowser 编排、Draw chain、overlay 集成
  - `internal/adapters/ui/file_browser/file_browser_handlers.go` -- 按键路由链
  - `internal/adapters/ui/file_browser/local_pane.go` -- Pane callback 模式、SelectedFiles
  - `internal/adapters/ui/file_browser/remote_pane.go` -- 同上 + 连接状态管理
  - `internal/core/domain/file_info.go` -- FileInfo 结构体
- pkg/sftp v1.13.10 源码验证:
  - `client.go:804` -- Remove
  - `client.go:892` -- Rename
  - `client.go:971` -- Mkdir
  - `client.go:1038` -- RemoveAll
- `.planning/research/ARCHITECTURE.md` -- 接口扩展方案、overlay 集成、按键路由
- `.planning/research/STACK.md` -- SFTP 原语和 Go stdlib 能力
- `.planning/research/PITFALLS.md` -- TOCTOU、递归操作、符号链接等 pitfall

### Secondary (MEDIUM confidence)
- `.planning/REQUIREMENTS.md` -- DEL/REN/MKD 需求定义

### Tertiary (LOW confidence)
- 无 -- 所有发现均基于项目源码和库源码的直接验证。

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - 所有依赖已在 go.mod 中，API 已在库源码中验证
- Architecture: HIGH - 基于项目现有代码的直接分析，overlay 模式已验证
- Pitfalls: HIGH - 基于 pkg/sftp 文档、项目代码审查和 Go stdlib 行为

**Research date:** 2026-04-15
**Valid until:** 30 days (stable domain -- Go stdlib 和 pkg/sftp API 不会频繁变更)
