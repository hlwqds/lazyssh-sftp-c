# Phase 1: Foundation - Research

**Researched:** 2026-04-13
**Domain:** tview TUI 双栏文件浏览器 + pkg/sftp 连接基础设施
**Confidence:** HIGH

## Summary

Phase 1 构建双栏文件浏览器的 UI 框架和本地文件浏览能力，同时建立 SFTP 连接基础设施。技术核心基于 tview（已有依赖）的 `Table` 组件和 `Flex` 布局，以及 `pkg/sftp` 的 `NewClientPipe()` 通过系统 SSH binary 建立连接。

经过深入调研，tview Table 的内置导航（j/k/h/l + 箭头键）与我们的自定义键绑定存在冲突：Table 内置 `h` 用于左移列，而我们需要 `h` 用于返回上级目录。解决方案是通过 `Box.SetInputCapture()` 在 Table 的 InputHandler 之前拦截 `h`、`Backspace`、`Tab`、`Space` 等快捷键，其他按键传递给 Table 的默认 InputHandler 处理导航。

tview 的 `TableContent` 接口支持自定义数据后端，可以在未来用于懒加载大目录列表（P8），但 Phase 1 使用默认的内存实现即可。

**Primary recommendation:** 使用 tview.Table 作为文件列表组件，通过 SetInputCapture 处理自定义快捷键，利用 pkg/sftp.NewClientPipe() 建立 SFTP 连接。所有代码遵循现有 Clean Architecture 模式。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** 使用 `tview.Table` 组件显示文件列表（而非 `tview.List`）
- **D-02:** 每行显示 4 列：Name, Size, Modified date, Permissions (drwxr-xr-x)
- **D-03:** 目录用特殊标识区分（如 `/` 后缀或不同颜色）
- **D-04:** 双栏宽度比例 50:50（`tview.Flex`，各占 1:1）
- **D-05:** 每个 pane 顶部显示当前路径（作为 pane 的 Title）
- **D-06:** 遵循现有 TUI 布局模式：`app.SetRoot(fileBrowser, true)` 覆盖全屏，Esc 返回主界面
- **D-07:** 按 `F` 打开文件浏览器时立即建立 SFTP 连接（非 lazy connect）
- **D-08:** 连接失败时在右栏 pane 内显示错误信息 + 原因，左栏仍可正常浏览本地文件
- **D-09:** 使用 `pkg/sftp` 的 `NewClientPipe()` 通过系统 SSH binary 建立连接
- **D-10:** 初始目录：本地 `~` (home dir)，远程 `~` (SSH default home)
- **D-11:** 返回上级目录快捷键：`Backspace` + `h` 均支持
- **D-12:** `Tab` 切换左右 pane 焦点
- **D-13:** `Space` 标记/取消标记文件（多选）
- **D-14:** 快捷键 `F` (Shift+f) 触发文件浏览器（`f` 已被端口转发占用）
- **D-15:** `Esc` 关闭文件浏览器，返回主界面（`returnToMain()`）

### Claude's Discretion
- 文件大小显示格式（human readable: 1.2K, 3.4M vs bytes）
- 目录排序规则（目录优先 vs 混合排序）
- 空目录显示文本
- 文件类型图标/颜色编码
- 表格列宽分配策略

### Deferred Ideas (OUT OF SCOPE)
None
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| UI-01 | User can open file browser by pressing `F` on a selected server | D-14 锁定使用 `F`，handlers.go 中 `case 'F':` 入口点，Table 内置 j/k 导航 |
| UI-02 | User sees dual-pane layout (left=local, right=remote) | D-04 锁定 FlexColumn 50:50，D-05 路径作为 Title |
| UI-03 | User can navigate files with arrow keys and j/k | tview.Table 内置 j/k/arrow 导航（rows selectable 模式） |
| UI-04 | User can select multiple files with Space key | D-13 锁定 Space 多选，TableCell.Reference 存储选中状态 |
| UI-05 | User can switch pane focus with Tab key | D-12 锁定 Tab 切换，Flex.SetInputCapture 拦截 |
| UI-07 | User sees status bar with connection info and transfer status | 复用现有 status_bar 模式，文件浏览器有自己的 status bar |
| UI-08 | User sees error messages displayed clearly in the UI | D-08 锁定错误在右栏 pane 内显示，复用 showStatusTempColor 模式 |
| BROW-01 | User can browse local directories with file list display (name, size, date, permissions) | D-01/D-02 锁定 Table 4 列，os.ReadDir() 获取本地文件信息 |
| BROW-03 | User can navigate to parent directory (../) in both panes | D-11 锁定 Backspace + h，osfilepath.Dir() 处理路径 |
| BROW-04 | User can toggle hidden file visibility in both panes | 隐藏文件过滤逻辑，`. ` 或 `Ctrl+H` 快捷键 |
| BROW-05 | User can see current path displayed for both local and remote panes | D-05 锁定路径作为 Table 的 SetTitle |
| BROW-06 | User can sort files by name, size, or date in both panes | 参考现有 sort.go 的 SortMode 模式，实现文件排序 |
| INTG-01 | File browser uses existing SSH config from selected server (zero-config) | Server domain entity 包含所有 SSH 配置，通过 BuildSSHCommand 模式构建连接 |
| INTG-02 | SFTP connection established via system SSH binary (respects ~/.ssh/config, ssh-agent) | D-09 锁定 pkg/sftp.NewClientPipe()，通过 exec.Command("ssh") 建立管道 |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **安全原则**: 不引入新的安全风险，复用系统 scp/sftp 命令，不存储/传输/修改密钥
- **跨平台**: 必须在 Linux/Windows/Darwin 上正常工作
- **架构一致**: 遵循现有 Clean Architecture 模式，通过 Port/Adapter 解耦
- **UI 框架**: 基于 tview/tcell 构建，不可引入其他 UI 框架
- **零外部依赖**: 不引入需要额外安装的依赖，sc/sftp 必须是系统自带的
- **GSD 工作流**: 所有文件变更必须通过 GSD 命令发起
- **命名**: snake_case.go 文件名，PascalCase 导出，camelCase 私有
- **错误处理**: 返回 error 作为最后一个返回值，使用 log.Errorw() 记录
- **依赖注入**: 通过构造函数注入，cmd/main.go 中组装

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| tview | v0.0.0-20250625164341 (最新 commit) | TUI 框架，提供 Table/Flex/Box 组件 | 已有依赖，Table 内置 j/k/arrow 导航 |
| tcell/v2 | v2.9.0 | 终端底层操作，颜色、事件处理 | 已有依赖，tview 基础 |
| pkg/sftp | v1.13.10 (最新稳定版) | SFTP 客户端，通过系统 SSH binary 管道连接 | v2 仍为 alpha (v2.0.0-alpha)，v1 是稳定版 |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| mattn/go-runewidth | v0.0.16 | Unicode 字符显示宽度计算 | 文件名包含 CJK/emoji 时的列对齐 |
| go.uber.org/zap | v1.27.0 | 结构化日志 | SFTP 连接/错误日志记录 |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| tview.Table | tview.List | List 不支持多列（name/size/date/permissions），无法满足 D-02 |
| tview.Table | tview.TreeView | TreeView 是树形结构，不适合平面文件列表 |
| pkg/sftp v1 | pkg/sftp v2 | v2 仍为 alpha，API 可能不稳定，v1 是成熟稳定版 |
| pkg/sftp | os/exec + sftp batch mode | 需要解析非结构化文本输出，P2 坑——文件名含空格/Unicode 会崩溃 |

**Installation:**
```bash
go get github.com/pkg/sftp@v1.13.10
```

**Version verification:**
- `pkg/sftp` v1.13.10 — 通过 `go list -m -versions github.com/pkg/sftp` 确认为最新稳定版 (2026-04-13)
- `tview` v0.0.0-20250625164341 — 已在 go.mod 中锁定
- `pkg/sftp/v2` v2.0.0-alpha — 仍为 alpha，不推荐生产使用

## Architecture Patterns

### Recommended Project Structure

```
internal/
├── core/
│   ├── domain/
│   │   └── file_info.go          # FileInfo 结构体（name, size, mode, modTime, isDir, isSymlink）
│   └── ports/
│       └── file_service.go       # FileService 接口（ListDir, Connect, Close）
├── adapters/
│   ├── data/
│   │   ├── local_fs/
│   │   │   └── local_fs.go       # 本地文件系统操作（os.ReadDir, filepath）
│   │   └── sftp_client/
│   │       └── sftp_client.go    # SFTP 连接管理（NewClientPipe, ReadDir, Close）
│   └── ui/
│       └── file_browser/
│           ├── file_browser.go   # 主双栏布局（FlexColumn + 两个 Table）
│           ├── local_pane.go     # 左栏本地文件浏览（tview.Table + 键盘处理）
│           ├── remote_pane.go    # 右栏远程文件浏览（tview.Table + SFTP）
│           └── file_browser_handlers.go  # 全局键盘处理（Tab/Esc/排序/隐藏文件）
```

### Pattern 1: 双栏文件浏览器布局

**What:** 使用 `tview.Flex` (FlexColumn) 包含两个 `tview.Table`，顶部有路径标题，底部有状态栏。

**When to use:** Phase 1 的核心 UI 结构。

**Example:**
```go
// 主布局结构
fileBrowser := tview.NewFlex().SetDirection(tview.FlexRow)
// 顶部：双栏文件列表
content := tview.NewFlex().SetDirection(tview.FlexColumn).
    AddItem(localPane, 0, 1, true).   // 50% 宽度
    AddItem(remotePane, 0, 1, true)   // 50% 宽度
// 底部：状态栏
statusBar := tview.NewTextView()
fileBrowser.
    AddItem(content, 0, 1, true).
    AddItem(statusBar, 1, 0, false)
```

**关键细节:**
- 两个 Table 都设置 `SetSelectable(true, false)` 启用行选择
- Flex 的 `SetInputCapture` 处理 Tab 切换焦点
- 每个 Table 的 `SetInputCapture` 处理 h（返回上级）、Space（多选）
- 路径显示通过 `table.SetTitle(currentPath)` 实现

### Pattern 2: Table 自定义快捷键拦截

**What:** tview.Table 内置 j/k/arrow 导航，但我们需要拦截 h（返回上级）、Space（多选）、Enter（进入目录/选中文件）。

**When to use:** 文件浏览器中所有需要覆盖 Table 默认行为的场景。

**Example:**
```go
// 在 file_browser.go 的 Flex 上设置全局拦截
fileBrowser.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
    switch event.Key() {
    case tcell.KeyTab:
        // 切换左右 pane 焦点
        fb.switchFocus()
        return nil
    case tcell.KeyESC:
        // 关闭文件浏览器，返回主界面
        fb.returnToMain()
        return nil
    }
    return event
})

// 在每个 pane 的 Table 上设置拦截
table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
    switch event.Rune() {
    case 'h':
        // 返回上级目录
        fb.navigateToParent()
        return nil
    case ' ':
        // 切换多选
        fb.toggleSelection()
        return nil
    case '.':
        // 切换隐藏文件
        fb.toggleHiddenFiles()
        return nil
    }
    if event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2 {
        fb.navigateToParent()
        return nil
    }
    return event  // j/k/arrow 传给 Table 内置处理
})
```

**关键细节:**
- `Box.SetInputCapture` 在 `InputHandler` 之前执行，返回 nil 表示事件已消费
- j/k/arrow 不拦截，直接传递给 Table 内置导航
- Enter 通过 `Table.SetSelectedFunc` 处理（进入目录/传输文件）

### Pattern 3: SFTP 连接通过系统 SSH Binary

**What:** 使用 `pkg/sftp.NewClientPipe()` 连接系统 SSH binary 的 stdin/stdout，获得完整的 Go SFTP API。

**When to use:** INTG-02 要求的 SFTP 连接建立。

**Example:**
```go
import (
    "os/exec"
    "io"
    "github.com/pkg/sftp"
)

func (c *SFTPClient) Connect(server domain.Server) error {
    // 构建与 BuildSSHCommand 相同的 SSH 参数
    args := buildSSHArgs(server)
    args = append(args, "-s", "sftp")  // 请求 SFTP 子系统

    cmd := exec.Command("ssh", args...)
    // 获取 stdin pipe（写入 SFTP 请求）
    stdin, err := cmd.StdinPipe()
    if err != nil {
        return err
    }
    // 获取 stdout pipe（读取 SFTP 响应）
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return err
    }
    cmd.Stderr = os.Stderr  // SSH 错误信息输出到 stderr

    if err := cmd.Start(); err != nil {
        return err
    }

    // 通过管道创建 SFTP 客户端
    client, err := sftp.NewClientPipe(stdout, stdin)
    if err != nil {
        cmd.Process.Kill()
        return err
    }
    c.client = client
    c.cmd = cmd
    return nil
}
```

**关键细节:**
- `ssh -s sftp` 请求远程 SFTP 子系统
- SSH 参数复用 `BuildSSHCommand` 中的 ProxyJump、ProxyCommand、IdentityFile 等逻辑
- 连接在 goroutine 中建立，通过 `app.QueueUpdateDraw()` 更新 UI
- `cmd.Process.Kill()` 用于清理失败的连接
- `defer client.Close(); cmd.Wait()` 用于正常关闭

### Pattern 4: 文件列表填充

**What:** 将 `os.FileInfo` 转换为 tview.Table 的行。

**When to use:** 本地文件浏览和远程文件浏览。

**Example:**
```go
func populateTable(table *tview.Table, entries []domain.FileInfo) {
    table.Clear()
    // 固定表头行
    table.SetFixed(1, 0)
    headerStyle := tcell.StyleDefault.Bold(true)
    table.SetCell(0, 0, tview.NewTableCell("Name").SetStyle(headerStyle).SetExpansion(1))
    table.SetCell(0, 1, tview.NewTableCell("Size").SetStyle(headerStyle).SetAlign(tview.AlignRight))
    table.SetCell(0, 2, tview.NewTableCell("Modified").SetStyle(headerStyle))
    table.SetCell(0, 3, tview.NewTableCell("Permissions").SetStyle(headerStyle))

    for i, f := range entries {
        row := i + 1  // 跳过表头
        name := f.Name
        if f.IsDir {
            name += "/"  // D-03: 目录用 / 后缀
        }
        table.SetCell(row, 0, tview.NewTableCell(name).
            SetReference(f).  // 存储 FileInfo 引用，用于后续操作
            SetExpansion(1))
        table.SetCell(row, 1, tview.NewTableCell(formatSize(f.Size)).
            SetAlign(tview.AlignRight))
        table.SetCell(row, 2, tview.NewTableCell(f.ModTime.Format("2006-01-02 15:04")))
        table.SetCell(row, 3, tview.NewTableCell(f.Mode.String()))
    }
    table.Select(1, 0)  // 选中第一行（第一个文件/目录）
}
```

**关键细节:**
- `SetFixed(1, 0)` 固定表头行，滚动时表头始终可见
- `SetExpansion(1)` 让 Name 列自动扩展填充剩余宽度
- `SetReference(f)` 将 FileInfo 存储在 TableCell 中，Enter 选中时可以取回
- 目录颜色可以用 `SetTextColor(tcell.Color)` 设置蓝色区分

### Pattern 5: 现有代码集成点

**What:** 文件浏览器集成到现有 TUI 结构中，遵循已有的视图切换模式。

**When to use:** UI-01 入口点和 UI-02 视图切换。

**Example:**
```go
// handlers.go 中添加入口
case 'F':
    t.handleFileBrowser()
    return nil

func (t *tui) handleFileBrowser() {
    server, ok := t.serverList.GetSelectedServer()
    if !ok {
        t.showStatusTempColor("No server selected", "#FF6B6B")
        return
    }
    fb := NewFileBrowser(t.app, t.logger, server, t.fileService)
    t.app.SetRoot(fb, true)  // 与 handleServerAdd/handlePortForward 相同模式
}
```

**关键细节:**
- `tui` 结构体需要新增 `fileService` 字段和 `fileBrowser` 字段
- `NewTUI` 构造函数需要新增 `fileService` 参数
- `cmd/main.go` 需要注入 `FileService` 依赖
- Esc 返回主界面通过 `t.returnToMain()` 实现（已存在）

### Anti-Patterns to Avoid

- **直接在 Table 上添加列标题为普通行:** 使用 `SetFixed(1, 0)` 固定表头，不要将标题作为可滚动数据的一部分
- **在 goroutine 中直接操作 tview 组件:** 所有 UI 更新必须通过 `app.QueueUpdateDraw()` 回到主线程
- **每次导航都重新创建 Table:** 使用 `Table.Clear()` + 重新填充，而不是销毁重建
- **在 Table.SetInputCapture 中拦截所有按键:** 只拦截需要自定义行为的按键（h/Space/Backspace），j/k/arrow 传递给内置处理
- **使用 SFTP batch mode (`sftp -b -`):** P2 坑——文件名含空格/Unicode 时解析会崩溃，使用 pkg/sftp API
- **手动构建 SSH 命令字符串:** 复用现有 `BuildSSHCommand` 中的参数构建逻辑，避免重复

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 文件列表导航 | 自定义 j/k/arrow 键处理和滚动逻辑 | tview.Table 内置导航（SetSelectable + rows=true） | Table 已实现行选择、滚动、PgUp/PgDn/g/G，内置处理比手写更可靠 |
| SFTP 协议实现 | 用 os/exec 调用 sftp 命令并解析文本输出 | pkg/sftp.NewClientPipe() | P2 坑——文本解析脆弱，P3 坑——每次调用新连接 |
| 进度条组件 | 手动在 TextView 中用字符串渲染进度 | Phase 2 再处理（自定义 tview primitive） | Phase 1 不需要传输功能 |
| 文件大小格式化 | 手写 if/else 判断 B/KB/MB/GB | humanity 包或自写简单函数 | 自写即可，逻辑简单（< 10 行），不值得引入依赖 |
| SSH 参数构建 | 重新实现 ProxyJump/IdentityFile 等参数拼接 | 复用 utils.go 中 BuildSSHCommand 的辅助函数 | 避免重复，保持与 SSH 连接行为一致 |

**Key insight:** tview.Table 是高度成熟的组件，内置了行选择、滚动、键盘导航等功能。不要"为了控制"而绕过它的内置能力——在 SetInputCapture 中只拦截需要自定义行为的按键即可。

## Common Pitfalls

### Pitfall 1: tview Table 内置 `h` 键冲突
**What goes wrong:** Table 内置 `h` 键用于左移列选择，但我们需要 `h` 返回上级目录。如果直接拦截 `h`，列选择功能丢失。
**Why it happens:** tview.Table 的 InputHandler 中 h/j/k/l 绑定为方向键。
**How to avoid:** 在 `SetInputCapture` 中拦截 `h` 返回 nil，这样 InputHandler 不会收到 `h` 事件。对于文件浏览器，列选择不需要（我们设置 `SetSelectable(true, false)` 即行选择模式），所以 h 的默认行为本来就不需要。
**Warning signs:** 按 `h` 时光标移到左边列而不是返回上级目录。

### Pitfall 2: SFTP 连接阻塞 UI 线程
**What goes wrong:** `exec.Command("ssh").Start()` + `sftp.NewClientPipe()` 在 UI goroutine 中执行，导致界面冻结直到连接成功/超时。
**Why it happens:** tview 是单线程模型，阻塞操作会冻结整个 UI。
**How to avoid:** SFTP 连接必须在 goroutine 中建立，通过 `app.QueueUpdateDraw()` 更新 UI（显示连接成功/失败）。右栏在连接中显示 "Connecting..." 占位文本。
**Warning signs:** 按 `F` 后界面卡住几秒不动。

### Pitfall 3: SFTP 连接未正确关闭（P9）
**What goes wrong:** 关闭文件浏览器时 SFTP 连接和 SSH 进程未清理，导致资源泄漏。
**Why it happens:** 忘记在 Esc 关闭处理中调用 `client.Close()` 和 `cmd.Wait()`。
**How to avoid:** 在 FileBrowser 结构体上实现清理方法，在 `returnToMain()` 之前调用。使用 `defer` 模式确保连接关闭。
**Warning signs:** 系统中 ssh 进程数持续增长。

### Pitfall 4: Unicode 文件名显示宽度不对齐（P11）
**What goes wrong:** 文件名包含 CJK 字符时，Table 列宽计算错误导致列不对齐。
**Why it happens:** CJK 字符显示宽度为 2，但字节数不同。tview 使用 `go-runewidth`（已有依赖）计算显示宽度。
**How to avoid:** tview.Table 内置使用 `go-runewidth` 计算列宽，只需确保 Name 列设置 `SetExpansion(1)` 让它自适应宽度。`MaxWidth` 可以设置一个合理上限防止过长的文件名挤压其他列。
**Warning signs:** CJK 文件名导致 Size/Date 列错位。

### Pitfall 5: SetInputCapture 事件传播
**What goes wrong:** Flex 上的 SetInputCapture 和 Table 上的 SetInputCapture 事件处理顺序混乱。
**Why it happens:** tview 的事件传播链是：Flex.SetInputCapture -> Flex.InputHandler -> 焦点组件.SetInputCapture -> 焦点组件.InputHandler。
**How to avoid:** Flex 的 SetInputCapture 只处理全局键（Tab/Esc），返回 event 传递给焦点组件。Table 的 SetInputCapture 处理 pane 特定键（h/Space/.），返回 event 传递给 Table 的内置 InputHandler。
**Warning signs:** 按 Tab 时既切换了焦点又触发了其他行为。

### Pitfall 6: 排序时目录优先
**What goes wrong:** 排序文件时目录和文件混合在一起，用户难以找到目录。
**Why it happens:** 默认排序按字符串比较，不区分文件和目录。
**How to avoid:** 排序时始终将目录放在前面（目录优先），然后在目录和文件组内分别按选择的字段排序。这是文件管理器的通用约定。
**Warning signs:** 用户在大目录中难以找到子目录。

## Code Examples

Verified patterns from official sources and existing codebase:

### 文件大小格式化（Claude's Discretion: human readable）
```go
func formatSize(bytes int64) string {
    const (
        KB = 1024
        MB = KB * 1024
        GB = MB * 1024
    )
    switch {
    case bytes >= GB:
        return fmt.Sprintf("%.1fG", float64(bytes)/float64(GB))
    case bytes >= MB:
        return fmt.Sprintf("%.1fM", float64(bytes)/float64(MB))
    case bytes >= KB:
        return fmt.Sprintf("%.1fK", float64(bytes)/float64(KB))
    default:
        return fmt.Sprintf("%dB", bytes)
    }
}
```
**推荐使用 human readable 格式**——文件管理器（mc、lf、ranger）的通用做法，用户不需要看到 "1048576" 这样的数字。

### 本地目录列表（BROW-01）
```go
func ListLocalDir(path string, showHidden bool) ([]domain.FileInfo, error) {
    entries, err := os.ReadDir(path)
    if err != nil {
        return nil, fmt.Errorf("read dir %s: %w", path, err)
    }
    var result []domain.FileInfo
    for _, e := range entries {
        if !showHidden && strings.HasPrefix(e.Name(), ".") {
            continue
        }
        info, err := e.Info()
        if err != nil {
            continue  // 跳过无法获取信息的文件
        }
        result = append(result, domain.FileInfo{
            Name:    e.Name(),
            Size:    info.Size(),
            Mode:    info.Mode(),
            ModTime: info.ModTime(),
            IsDir:   e.IsDir(),
        })
    }
    return result, nil
}
```

### 视图切换模式（UI-01, 复用现有模式）
```go
// Source: internal/adapters/ui/handlers.go:238-244 (handleServerAdd 模式)
func (t *tui) handleFileBrowser() {
    server, ok := t.serverList.GetSelectedServer()
    if !ok {
        t.showStatusTempColor("No server selected", "#FF6B6B")
        return
    }
    fb := NewFileBrowser(t.app, t.logger, t.fileService, server, func() {
        t.returnToMain()
    })
    t.app.SetRoot(fb, true)  // 全屏覆盖
}
```

### 后台操作 + QueueUpdateDraw 模式（复用 handlePingSelected）
```go
// Source: internal/adapters/ui/handlers.go:291-309
// 建立 SFTP 连接的 goroutine 模式
go func() {
    err := fileService.Connect(server)
    t.app.QueueUpdateDraw(func() {
        if err != nil {
            remotePane.ShowError(fmt.Sprintf("SFTP connection failed: %v", err))
        } else {
            remotePane.ShowDirectory(remoteHome)
        }
    })
}()
```

### Table 行选中回调（UI-04 多选支持）
```go
// 使用 TableCell.SetReference 存储文件信息
// SetSelectedFunc 处理 Enter 键
table.SetSelectedFunc(func(row, column int) {
    cell := table.GetCell(row, column)
    if cell == nil {
        return
    }
    fi, ok := cell.GetReference().(domain.FileInfo)
    if !ok {
        return
    }
    if fi.IsDir {
        // 进入目录
        navigateTo(fi.Name)
    }
    // Phase 2: 文件传输
})
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| os/exec + sftp batch mode | pkg/sftp.NewClientPipe() | 项目决策（D-09） | 消除 P2（解析脆弱）和 P3（连接开销） |
| tview.List (单列) | tview.Table (多列) | 项目决策（D-01） | 支持显示 name/size/date/permissions 四列 |
| 手动 SSH 参数拼接 | 复用 BuildSSHCommand 辅助函数 | 现有代码模式 | 保持 SSH 连接行为一致性 |

**Deprecated/outdated:**
- `schollz/progressbar`: 直接写入终端，不可嵌入 tview，不适用
- `pkg/sftp/v2`: 仍为 alpha 版本，API 不稳定

## Open Questions

1. **SSH 参数构建复用方式**
   - What we know: 现有 `BuildSSHCommand` 返回完整命令字符串，SFTP 连接需要参数数组
   - What's unclear: 是重构 `BuildSSHCommand` 为返回 `[]string`，还是提取共享的参数构建函数
   - Recommendation: 提取 `buildSSHArgs(server) []string` 函数，`BuildSSHCommand` 和 SFTP 客户端都调用它

2. **TableContent 接口是否值得在 Phase 1 引入**
   - What we know: tview 提供 `TableContent` 接口支持自定义数据后端
   - What's unclear: Phase 1 是否需要懒加载
   - Recommendation: Phase 1 使用默认内存实现（直接 `SetCell`），`TableContent` 留给大目录优化时引入

3. **隐藏文件切换快捷键**
   - What we know: 需要一个快捷键切换隐藏文件显示（BROW-04）
   - What's unclear: 使用 `.` 还是 `Ctrl+H`（两者都是常见选择）
   - Recommendation: 使用 `.`（单键操作更符合 lazyssh 风格），参考 mc 的 Alt+. 模式

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | 全部 | Yes | 1.25.8 | -- |
| tview | UI 框架 | Yes | v0.0.0-20250625164341 | -- |
| tcell/v2 | 终端操作 | Yes | v2.9.0 | -- |
| pkg/sftp | SFTP 连接 | No (未安装) | -- | `go get github.com/pkg/sftp@v1.13.10` |
| ssh (系统命令) | SFTP 后端 | Yes | OpenSSH | -- |
| go-runewidth | Unicode 宽度 | Yes | v0.0.16 | -- |

**Missing dependencies with no fallback:**
- 无

**Missing dependencies with fallback:**
- `pkg/sftp` — 需要通过 `go get` 安装，这是 Phase 1 的第一步

## Validation Architecture

> nyquist_validation 在 config.json 中设为 false，跳过此部分。

## Sources

### Primary (HIGH confidence)
- `go doc github.com/rivo/tview.Table` — Table API 完整方法列表、内置 j/k/arrow 导航、SetInputCapture/Selectable/SelectedFunc
- `go doc github.com/rivo/tview.Flex` — Flex 布局 API
- `go doc github.com/rivo/tview.TableCell` — TableCell 完整字段和方法
- `go doc github.com/rivo/tview.TableContent` — 自定义数据后端接口
- `go list -m -versions github.com/pkg/sftp` — v1.13.10 为最新稳定版
- 现有代码 `handlers.go` — 视图切换模式 (SetRoot)、QueueUpdateDraw 模式、错误显示
- 现有代码 `tui.go` — 组件初始化链 (buildComponents -> buildLayout -> bindEvents)
- 现有代码 `server_list.go` — tview 组件封装模式 (struct + NewXxx + build)
- 现有代码 `sort.go` — SortMode 枚举和排序逻辑模式

### Secondary (MEDIUM confidence)
- [pkg/sftp NewClientPipe 官方文档](https://pkg.go.dev/github.com/pkg/sftp) — NewClientPipe(rd io.Reader, wr io.WriteCloser, opts ...ClientOption) (*Client, error)
- [pkg/sftp GitHub](https://github.com/pkg/sftp) — 项目主页，活跃维护
- `.planning/research/STACK.md` — pkg/sftp 选型决策
- `.planning/research/PITFALLS.md` — P2/P3/P4/P5/P9/P11 坑分析

### Tertiary (LOW confidence)
- `.planning/research/FEATURES.md` — Midnight Commander 快捷键参考

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — 所有库版本通过 go doc 和 go list 验证，pkg/sftp v1.13.10 确认为最新稳定版
- Architecture: HIGH — 基于现有代码的直接观察和 tview API 文档验证
- Pitfalls: HIGH — P4/P5/P9/P11 来自项目已有的 pitfall 研究，Table h 键冲突通过 go doc 验证

**Research date:** 2026-04-13
**Valid until:** 30 days（tview API 稳定，pkg/sftp v1 API 稳定）
