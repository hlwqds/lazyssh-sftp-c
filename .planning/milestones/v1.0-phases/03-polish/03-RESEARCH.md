# Phase 3: Polish — Research

**Researched:** 2026-04-13
**Domain:** Go context.Context 取消传播、文件冲突检测/解决 UI 对话框、跨平台文件系统操作
**Confidence:** HIGH

## Summary

Phase 3 在 Phase 2 建立的单文件和目录传输基础上，添加三个关键的可靠性能力。核心研究围绕三个技术领域展开：(1) 使用 Go 标准 `context.Context` 在传输 goroutine 中传播取消信号，并在 `copyWithProgress` 的 32KB buffer 循环中检查 `ctx.Done()` 中断传输；(2) 传输前通过 SFTP Stat / os.Stat 检测目标文件是否已存在，在 TransferModal 中内嵌冲突对话框提供 Overwrite/Skip/Rename 三选一；(3) 使用 Go build tags (`file_windows.go`, `file_unix.go`) 处理路径分隔符、文件权限、符号链接等平台差异。

研究结论：所有三个需求都可以在不引入新外部依赖的情况下实现。取消机制完全基于 Go 标准库 `context` 包；冲突解决复用 TransferModal 的 modal overlay 模式；跨平台差异通过已有的 build tag 模式（参见 `sysprocattr_windows.go` / `sysprocattr_unix.go`）和 Go 标准库 `filepath` 包处理。pkg/sftp 的 `client.Stat()`、`client.Remove()` 方法已在 go doc 中验证可用，用于冲突检测和取消后的部分文件清理。

**Primary recommendation:** 使用 `context.WithCancel` + TransferModal 模式切换（progress mode / conflict dialog mode / cancel confirm mode）实现三个需求，不需要引入新的 TUI 组件框架。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** 使用 `context.Context` 传播取消信号 — 添加到所有 TransferService 方法签名（UploadFile, DownloadFile, UploadDir, DownloadDir）
- **D-02:** `copyWithProgress` 循环中检查 `ctx.Done()`，收到取消信号时中断 io.Copy
- **D-03:** Esc 键第一次按下显示 "Cancel transfer? (y/n)" 确认提示，第二次按下确认取消（防止误操作）
- **D-04:** 取消后总是删除目标侧的部分文件（不留 orphaned half-files）
- **D-05:** 取消操作不需要关闭 SFTP 连接 — 连接可以复用，只取消当前传输 goroutine
- **D-06:** 传输前检查目标文件是否存在（Stat），存在则暂停传输、弹出冲突对话框
- **D-07:** 冲突对话框提供三个选项：Overwrite / Skip / Rename（自动添加 .1, .2 后缀）
- **D-08:** 冲突对话框显示在 TransferModal 区域内（替换进度显示），不切换到单独的 view
- **D-09:** 目录传输中每个冲突文件单独提示（非 apply-all），用户可以为每个文件选择不同操作
- **D-10:** 使用 Go build tags（`file_windows.go`, `file_unix.go`）处理平台差异，不使用 runtime.GOOS 散弹枪检查
- **D-11:** 路径处理：本地路径统一使用 `filepath.Join`/`filepath.Clean`，远程路径使用 `path.Join`（Unix 风格，SFTP 标准）
- **D-12:** 符号链接：默认跟随符号链接（follow symlinks），不做符号链接保留（保留需要额外复杂度）
- **D-13:** 文件权限：传输时尝试设置权限（chmod），但如果目标系统不支持（如 Windows NTFS）则静默忽略错误
- **D-14:** 显示格式：文件大小使用 `humanize` 风格自动切换（B/KB/MB/GB），日期使用 locale-independent 格式

### Claude's Discretion
- context.Context 的 WithTimeout 值（是否需要超时保护）
- Rename 的具体后缀格式（.1, .2 vs _copy vs timestamp）
- 冲突对话框的精确布局和颜色
- 符号链接检测的具体实现方式
- Windows 上 filepath.ToSlash 的具体调用位置
- 文件权限失败时的日志级别（warn vs debug）

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| TRAN-06 | User can cancel an in-progress transfer | D-01~D-05: context.Context 传播取消信号，copyWithProgress 检查 ctx.Done()，TransferModal 取消确认模式，取消后删除部分文件 |
| TRAN-07 | User is prompted when destination file already exists (overwrite/skip/rename) | D-06~D-09: 传输前 Stat 检测，TransferModal 冲突对话框模式（内嵌三选一 UI），目录传输逐文件提示 |
| INTG-03 | File browser works on Linux, Windows, and macOS | D-10~D-14: Go build tags 分离平台代码，filepath.Join/path.Join 路径处理，符号链接跟随，权限静默降级 |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **安全原则**: 不引入新的安全风险，复用系统 scp/sftp 命令，不存储/传输/修改密钥
- **跨平台**: 必须在 Linux/Windows/Darwin 上正常工作
- **架构一致**: 遵循现有 Clean Architecture 模式，通过 Port/Adapter 解耦
- **UI 框架**: 基于 tview/tcell 构建，不可引入其他 UI 框架
- **零外部依赖**: 不引入需要额外安装的依赖，scp/sftp 必须是系统自带的
- **GSD 工作流**: 所有文件变更必须通过 GSD 命令发起
- **命名**: snake_case.go 文件名，PascalCase 导出，camelCase 私有
- **错误处理**: 返回 error 作为最后一个返回值，使用 log.Errorw() 记录

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go context | stdlib (Go 1.24.6) | 取消信号传播 | Go 标准 cancellation pattern，零外部依赖 |
| tview | v0.0.0-20250625164341 | TransferModal 模式切换（progress/conflict/cancel） | 已有依赖，复用现有 overlay 模式 |
| tcell/v2 | v2.9.0 | 键盘事件处理（Esc, y/n） | 已有依赖 |
| pkg/sftp | v1.13.10 | `client.Stat()` 冲突检测，`client.Remove()` 取消清理 | 已有 indirect 依赖，API 已通过 go doc 验证 |
| filepath | stdlib | 本地路径处理（跨平台 `/` vs `\`） | Go 标准库，自动处理平台差异 |
| path | stdlib | 远程路径处理（始终 Unix 风格 `/`） | SFTP 是 Unix 协议，远程路径始终使用 `/` |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| os | stdlib | `os.Stat()` 本地冲突检测，`os.Remove()` 本地部分文件清理 | 上传冲突检测和取消清理 |
| go.uber.org/zap | v1.27.0 | 取消/冲突操作的日志记录 | 权限失败日志、取消日志 |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| context.WithCancel | 自定义 atomic bool cancel flag | context.Context 是 Go 标准 pattern，与 select/ctx.Done() 原生配合，atomic bool 无法在 Read/Write 中断中使用 |
| TransferModal 模式切换 | tview.Pages 多页切换 | Pages 需要管理多个 Primitive 的生命周期，模式切换在同一个 Draw() 中更简单，D-08 明确要求在 TransferModal 区域内 |
| .1, .2 后缀 rename | timestamp 后缀 / _copy 后缀 | .1, .2 简洁且是 mc 的默认行为，timestamp 过长，_copy 不支持多版本 |
| filepath.Join + path.Join | 统一使用 filepath.Join | 远程 SFTP 路径是 Unix 风格，Windows 上 filepath.Join 会产生 `\`，但 SFTP 服务端期望 `/` |

**Installation:**
```bash
# 无需安装新依赖 — 所有需要的库已在 go.mod 中
```

**Version verification:**
- `pkg/sftp` v1.13.10 — 已在 go.mod 中锁定为 indirect dependency
- `client.Stat()` — 通过 `go doc github.com/pkg/sftp.Client.Stat` 验证可用，返回 `os.FileInfo`
- `client.Remove()` — 通过 `go doc github.com/pkg/sftp.Client.Remove` 验证可用，可删除文件和空目录
- `client.Chmod()` — 通过 `go doc github.com/pkg/sftp.Client.Chmod` 验证可用

## Architecture Patterns

### Pattern 1: context.Context 取消传播

**What:** 在 TransferService 的所有方法签名中添加 `ctx context.Context` 作为第一个参数，在 `copyWithProgress` 循环中通过 `select` 检查 `ctx.Done()`。

**When to use:** TRAN-06 — 用户取消传输时传播取消信号到传输 goroutine。

**技术原理:**
`context.Context` 是 Go 标准库的取消传播机制。`context.WithCancel(parent)` 返回一个 derived context 和 cancel 函数。当 cancel() 被调用时，ctx.Done() channel 被关闭，所有 select 监听 ctx.Done() 的 goroutine 都会收到信号。

**关键设计细节:**
- `copyWithProgress` 的 Read/Write 循环不能被直接中断（io.Read 是阻塞调用），但可以在每次 32KB chunk 之间检查 ctx.Done()
- 取消后，`copyWithProgress` 返回 `context.Canceled` 错误，上层函数进行清理（删除部分文件）
- context 不关闭 SFTP 连接（D-05），只取消当前传输操作

**Example:**
```go
func (ts *transferService) copyWithProgress(ctx context.Context, src io.Reader, dst io.Writer, ...) error {
    buf := make([]byte, 32*1024)
    var transferred int64
    for {
        // Check cancellation before each read
        select {
        case <-ctx.Done():
            return context.Canceled
        default:
        }

        n, readErr := src.Read(buf)
        if n > 0 {
            _, writeErr := dst.Write(buf[:n])
            // ... progress callback ...
        }
        if readErr != nil { break }
    }
    return nil
}
```

**接口变更影响:**
- `ports.TransferService` 接口的 4 个方法全部需要添加 `ctx context.Context` 第一个参数
- `transfer_service_test.go` 中的 `mockSFTPService` 不需要变更（它实现的是 `SFTPService` 接口，不是 `TransferService`）
- `file_browser.go` 中 `initiateTransfer()` 和 `initiateDirTransfer()` 需要创建 `context.WithCancel()` 并传递给 service

### Pattern 2: TransferModal 多模式切换

**What:** TransferModal 在同一个 Box 内通过模式标志切换显示内容：(1) progress mode — 正常进度显示，(2) cancel confirm mode — "Cancel transfer? (y/n)" 确认，(3) conflict dialog mode — "File exists: [Overwrite] [Skip] [Rename]"。

**When to use:** TRAN-06（取消确认）和 TRAN-07（冲突解决）的 UI 呈现。

**技术原理:**
现有的 TransferModal 已经有 `showSummary` 标志控制 summary 模式的渲染。扩展这个模式概念，使用枚举类型管理多个模式：

```go
type modalMode int
const (
    modeProgress  modalMode = iota // 正常进度
    modeCancelConfirm               // 取消确认
    modeConflictDialog              // 冲突解决
    modeSummary                     // 传输完成摘要
)
```

`Draw()` 方法根据当前 `modalMode` 渲染不同的内容。模式切换不需要创建/销毁组件，只需要修改模式标志和模式数据。

**冲突对话框的阻塞式交互:**
冲突对话框需要用户选择后传输才能继续。由于传输运行在 goroutine 中，需要使用 channel 进行同步：

```go
// 在 file_browser.go 中定义冲突响应类型
type ConflictAction int
const (
    ConflictOverwrite ConflictAction = iota
    ConflictSkip
    ConflictRename
)

// 传输 goroutine 在遇到冲突时：
actionCh := make(chan ConflictAction, 1)
fb.app.QueueUpdateDraw(func() {
    fb.transferModal.ShowConflict(fileName, existingSize, newSize, actionCh)
})
action := <-actionCh // 阻塞等待用户选择
```

**Cancel 确认的双 Esc 设计（D-03）:**
- 第一次 Esc：TransferModal 切换到 modeCancelConfirm，显示 "Cancel transfer? (y/n)"
- 在 cancel confirm 模式中：
  - `y` 或 Enter → 执行取消（调用 cancel()）
  - `n` 或 Esc → 返回 modeProgress（继续传输）
- 在非 confirm 模式中：Esc → 进入 cancel confirm 模式

### Pattern 3: SFTP 端冲突检测与文件清理

**What:** 在传输前通过 `sftp.Client.Stat()` 检测远程文件是否存在；在取消后通过 `sftp.Client.Remove()` 清理部分文件。

**When to use:** TRAN-07（冲突检测）和 TRAN-06（取消清理）的远程端操作。

**技术原理:**
pkg/sftp 的 `client.Stat(path)` 返回 `os.FileInfo`，如果文件不存在返回 error。这是最轻量的存在性检查方式——不需要打开文件，只发送一个 SFTP stat 请求。

```go
// 冲突检测（传输前）
_, err := ts.sftp.Stat(remotePath)
if err == nil {
    // 文件存在 → 弹出冲突对话框
}

// 取消后清理（删除部分文件）
func (ts *transferService) cleanupRemoteFile(path string) {
    if err := ts.sftp.Remove(path); err != nil {
        ts.log.Warnw("failed to cleanup partial remote file", "path", path, "error", err)
    }
}
```

**端口接口扩展:**
当前 `SFTPService` 接口没有 `Stat` 和 `Remove` 方法。需要添加：

```go
// ports/file_service.go SFTPService 接口中添加：
Stat(path string) (os.FileInfo, error)
Remove(path string) error
```

**本地端对称操作:**
```go
// 本地冲突检测
_, err := os.Stat(localPath)

// 本地取消后清理
os.Remove(localPath)
```

本地端不需要扩展端口接口，直接使用 `os` 包。

### Pattern 4: Go Build Tags 平台分离

**What:** 使用 Go build tags 将平台差异代码分离到不同文件，避免 `runtime.GOOS` 散弹枪式条件检查。

**When to use:** INTG-03 — 跨平台兼容性。

**技术原理:**
Go 的 build tags 机制允许在文件顶部通过注释指定编译条件：

```go
// file_windows.go
//go:build windows

// file_unix.go
//go:build !windows
```

现有项目中已有此模式（`sysprocattr_windows.go` / `sysprocattr_unix.go`），Phase 3 遵循相同约定。

**需要平台分离的操作:**

| 操作 | Unix | Windows | 实现位置 |
|------|------|---------|---------|
| 符号链接检测 | `os.Lstat()` + `fs.ModeSymlink` | `os.Lstat()` 相同 | 不需要分离 — Go 标准库已跨平台 |
| 文件权限设置 | `os.Chmod()` 正常工作 | `os.Chmod()` 可能失败（NTFS 无 Unix 权限） | 用 `filepath_windows.go` 和 `filepath_unix.go` 封装 |
| 路径清理 | `filepath.Clean()` 产生 `/` | `filepath.Clean()` 产生 `\` | 不需要分离 — `filepath.Join` 自动处理 |
| 远程路径 | 始终使用 `/` | 始终使用 `/`（SFTP 协议） | 不需要分离 — 远程路径不用 `filepath` 包 |

**关键发现：大部分平台差异不需要 build tags。** Go 标准库的 `filepath` 包已经处理了路径分隔符差异。真正需要平台分离的主要是文件权限设置（D-13），因为 Windows NTFS 不支持 Unix 风格权限位。

### Pattern 5: Rename 后缀递增

**What:** 当用户选择 Rename 时，自动在文件名后添加 `.1`, `.2`, `.3` 递增后缀，直到找到不冲突的文件名。

**When to use:** TRAN-07 — 冲突解决中的 Rename 选项。

**Example:**
```go
func nextAvailableName(path string, statFunc func(string) (os.FileInfo, error)) string {
    base := filepath.Ext(path)          // .txt
    name := filepath.Base(path)         // file.txt
    dir := filepath.Dir(path)           // /some/dir
    stem := name[:len(name)-len(base)]  // file

    for i := 1; i <= 100; i++ {
        candidate := filepath.Join(dir, fmt.Sprintf("%s.%d%s", stem, i, base))
        if _, err := statFunc(candidate); err != nil {
            return candidate // 文件不存在，可用
        }
    }
    return path // fallback: 使用原始路径
}
```

**远程端对应版本使用 `path.Join` 而非 `filepath.Join`。**

### Anti-Patterns to Avoid

- **在 io.Read() 调用中直接使用 select 检查 ctx.Done():** `select` 不能中断阻塞的 `Read()` 调用。正确做法是在 chunk 之间检查（如 Pattern 1 所示）。
- **cancel() 后调用 sftpClient.Close():** D-05 明确取消不关闭连接。关闭连接会导致后续传输无法进行。
- **使用 tview.Pages 创建独立的冲突对话框:** D-08 要求在 TransferModal 区域内显示，不用切换到新 view。
- **在冲突对话框中使用 apply-all 模式:** D-09 要求每个冲突文件单独提示。
- **使用 runtime.GOOS 散弹枪检查:** D-10 明确要求使用 build tags。
- **手动在远程路径中使用 `\`:** SFTP 是 Unix 协议，远程路径始终使用 `/`。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 取消信号传播 | 自定义 atomic bool + 轮询 | context.WithCancel + ctx.Done() | Go 标准模式，与 select/超时天然配合，生态一致 |
| 路径处理（跨平台） | 手动字符串替换 `/` ↔ `\` | filepath.Join（本地）、path.Join（远程） | 标准库已完整处理所有边界情况 |
| 文件存在性检测 | 尝试 Open 然后关闭 | os.Stat / sftp.Client.Stat | Stat 更轻量，不创建文件句柄 |
| UI 对话框渲染 | 手动管理多组 UI 组件的显示/隐藏 | TransferModal 模式枚举 + Draw() 分发 | 单一组件内模式切换，生命周期管理简单 |
| 文件大小格式化 | 已在 local_pane.go 中实现 | formatSize() | 复用现有实现，D-14 要求已满足 |
| 冲突后重命名 | 简单地追加固定后缀 | 递增检测 nextAvailableName() | 避免覆盖已有的 .1 文件 |

**Key insight:** Phase 3 的三个需求本质上是可靠性增强，不是新功能。关键是正确使用 Go 标准库的 context 包和 pkg/sftp 已有的 Stat/Remove API，不引入新的外部依赖。

## Common Pitfalls

### Pitfall 1: context 取消无法中断阻塞的 io.Read/Write

**What goes wrong:** 以为 `select { case <-ctx.Done(): ... }` 能中断正在执行的 `src.Read(buf)` 调用，实际上 Read 会阻塞直到有数据或 EOF。
**Why it happens:** Go 的 I/O 操作不感知 context，ctx.Done() 只在 select 语句中生效。如果 Read 已经在执行，ctx 取消不会中断它。
**How to avoid:** 在 copy 循环的每次 chunk 之间（Read 返回后、下一次 Read 之前）检查 ctx.Done()。这意味着取消有最多 32KB 的延迟（一个 chunk 的时间），这对于用户交互来说完全可以接受。
**Warning signs:** 取消后传输仍然继续写入了一个完整的 32KB chunk。

### Pitfall 2: TransferModal 模式切换时的键盘事件处理

**What goes wrong:** 在 conflict dialog 或 cancel confirm 模式中，按键被错误地传递给其他组件或被忽略。
**Why it happens:** tview 的事件传播链中，FileBrowser.SetInputCapture 和 TransferModal.HandleKey 都可能消费事件。如果模式切换后没有正确更新 HandleKey 的逻辑，按键会穿透。
**How to avoid:** TransferModal.HandleKey 必须根据当前模式决定行为。在 conflict/cancel confirm 模式中，HandleKey 消费所有相关按键（y/n/Enter/Esc），不传递给父组件。
**Warning signs:** 在冲突对话框中按 y 后，y 被传递到 Table 导致文件名搜索。

### Pitfall 3: 冲突对话框的 goroutine 同步死锁

**What goes wrong:** 传输 goroutine 等待用户在冲突对话框中的选择（阻塞读 channel），但 UI 线程也在等待传输完成，造成死锁。
**Why it happens:** 如果在 UI 线程中调用 `action := <-actionCh`（阻塞读），UI 线程无法处理 Draw 更新。如果使用 `app.QueueUpdateDraw()` 但不在 goroutine 中等待，传输会继续执行并覆盖文件。
**How to avoid:** 传输 goroutine 阻塞在 `<-actionCh`（这是正确的——它在后台 goroutine 中，不阻塞 UI 线程）。UI 线程通过 `app.QueueUpdateDraw()` 更新显示对话框。用户按键后，HandleKey 向 actionCh 发送选择，goroutine 解除阻塞继续执行。
**Warning signs:** UI 冻结，传输卡住不动。

### Pitfall 4: SFTPService 接口变更破坏编译

**What goes wrong:** 给 SFTPService 添加 Stat/Remove 方法后，sftp_client_test.go 中的 mockSFTPService 编译失败。
**Why it happens:** mockSFTPService 实现了 SFTPService 接口，添加新方法后 mock 不满足接口约束。
**How to avoid:** 同步更新 mockSFTPService，添加 Stat/Remove 的 mock 实现。TransferService 的 mock 不受影响（它只调用 SFTPService 的方法，不实现该接口）。
**Warning signs:** `go build ./...` 或 `go test ./...` 编译失败。

### Pitfall 5: Windows 上路径混淆（远程 vs 本地）

**What goes wrong:** 在 Windows 上，`filepath.Join("remote", "dir", "file.txt")` 产生 `remote\dir\file.txt`，但 SFTP 服务端期望 `/`。
**Why it happens:** `filepath.Join` 使用当前操作系统的路径分隔符。在 Windows 上是 `\`。
**How to avoid:** 现有代码已经有 `joinRemotePath()` 函数使用字符串拼接 `+ "/"`（见 transfer_service.go:346），以及 `joinPath()` 函数（见 remote_pane.go:418）。远程路径始终使用这些函数，不使用 `filepath.Join`。Phase 3 保持这个约定。
**Warning signs:** Windows 上 SFTP 操作返回 "no such file" 错误，路径中出现 `\`。

### Pitfall 6: 取消后部分文件清理的竞态条件

**What goes wrong:** `copyWithProgress` 返回 context.Canceled 后，defer 语句关闭了 remoteFile handle，然后尝试 Remove 但文件仍被锁定。
**Why it happens:** defer 的执行顺序是 LIFO（后进先出）。如果 remoteFile.Close() 在 cleanup 之前执行，这是正确的（文件 handle 已关闭）。但如果 cleanup 在 Close 之前执行，Remove 可能因文件被占用而失败。
**How to avoid:** 确保在 UploadFile/DownloadFile 的 defer 中，先关闭文件 handle，再执行 cleanup。或者更好的方式：在 `copyWithProgress` 返回错误后，在调用方（UploadFile/DownloadFile）中显式关闭文件并执行 cleanup，不依赖 defer 顺序。

### Pitfall 7: 目录传输取消时已创建的目录未清理

**What goes wrong:** 取消目录传输时，已创建的空目录残留在目标侧。
**Why it happens:** `UploadDir` 在传输每个文件前会创建目录结构。取消后只清理了当前正在传输的文件，没有清理之前已创建的空目录。
**How to avoid:** 记录传输过程中创建的所有远程目录。取消后，从最深层目录开始向上尝试删除空目录（如果目录不为空则跳过）。这与 D-04 "删除部分文件" 不矛盾——D-04 说的是删除部分文件，空目录可以保留（它们不占数据空间）。建议：只删除部分文件，不删除空目录（更安全）。

## Code Examples

### context 取消传播到 TransferService

```go
// ports/transfer.go — 更新接口签名
type TransferService interface {
    UploadFile(ctx context.Context, localPath, remotePath string, onProgress func(domain.TransferProgress)) error
    DownloadFile(ctx context.Context, remotePath, localPath string, onProgress func(domain.TransferProgress)) error
    UploadDir(ctx context.Context, localPath, remotePath string, onProgress func(domain.TransferProgress)) ([]string, error)
    DownloadDir(ctx context.Context, remotePath, localPath string, onProgress func(domain.TransferProgress)) ([]string, error)
}
```

### copyWithProgress 中的取消检查

```go
func (ts *transferService) copyWithProgress(ctx context.Context, src io.Reader, dst io.Writer,
    srcPath, displayPath string, total int64, onProgress func(domain.TransferProgress)) error {

    buf := make([]byte, 32*1024)
    var transferred int64
    fileName := filepath.Base(displayPath)

    for {
        // 检查取消信号
        select {
        case <-ctx.Done():
            return context.Canceled
        default:
        }

        n, readErr := src.Read(buf)
        if n > 0 {
            _, writeErr := dst.Write(buf[:n])
            if writeErr != nil {
                return fmt.Errorf("write to %s: %w", displayPath, writeErr)
            }
            transferred += int64(n)
            if onProgress != nil {
                onProgress(domain.TransferProgress{
                    FileName:  fileName,
                    FilePath:  displayPath,
                    BytesDone: transferred,
                    BytesTotal: total,
                })
            }
        }
        if readErr != nil {
            if readErr == io.EOF {
                break
            }
            return fmt.Errorf("read from %s: %w", srcPath, readErr)
        }
    }

    if onProgress != nil {
        onProgress(domain.TransferProgress{
            FileName:  fileName,
            FilePath:  displayPath,
            BytesDone: transferred,
            BytesTotal: total,
            Done:       true,
        })
    }
    return nil
}
```

### UploadFile 中的取消清理

```go
func (ts *transferService) UploadFile(ctx context.Context, localPath, remotePath string,
    onProgress func(domain.TransferProgress)) error {

    localFile, err := os.Open(localPath)
    if err != nil {
        return fmt.Errorf("open local %s: %w", localPath, err)
    }
    defer localFile.Close()

    // ... stat for total size ...

    remoteFile, err := ts.sftp.CreateRemoteFile(remotePath)
    if err != nil {
        return fmt.Errorf("create remote %s: %w", remotePath, err)
    }

    err = ts.copyWithProgress(ctx, localFile, remoteFile, localPath, localPath, total, onProgress)
    remoteFile.Close()

    if err == context.Canceled {
        // D-04: 取消后删除部分文件
        ts.log.Infow("transfer canceled, cleaning up partial file", "path", remotePath)
        if removeErr := ts.sftp.Remove(remotePath); removeErr != nil {
            ts.log.Warnw("failed to cleanup partial remote file", "path", remotePath, "error", removeErr)
        }
        return context.Canceled
    }
    return err
}
```

### file_browser.go 中的 context 创建和取消传递

```go
func (fb *FileBrowser) initiateTransfer() {
    // ... collect files ...

    ctx, cancel := context.WithCancel(context.Background())
    fb.transferCancel = cancel // 保存到 FileBrowser 结构体中

    // Show modal
    fb.transferModal.Show(direction, files[0].Name)

    go func() {
        defer cancel() // 确保 context 最终被取消
        for i, fi := range files {
            if fb.activePane == 0 {
                err = fb.transferSvc.UploadFile(ctx, localPath, remotePath, progressCb)
            } else {
                err = fb.transferSvc.DownloadFile(ctx, remotePath, localPath, progressCb)
            }
            if ctx.Err() != nil {
                break // 上下文已取消，停止剩余传输
            }
        }
        // ... UI 更新 ...
    }()
}
```

### Esc 取消确认的键盘处理

```go
// file_browser_handlers.go
case tcell.KeyESC:
    if fb.transferModal != nil && fb.transferModal.IsVisible() {
        if fb.transferModal.InCancelConfirm() {
            // 第二次 Esc — 确认取消
            fb.transferModal.SetCancelConfirmed(true)
            if fb.transferCancel != nil {
                fb.transferCancel()
            }
            return nil
        }
        // 第一次 Esc — 进入取消确认模式
        fb.transferModal.ShowCancelConfirm()
        return nil
    }
    fb.close()
    return nil
```

### 冲突对话框的 goroutine 同步

```go
// domain/transfer.go 中添加
type ConflictAction int
const (
    ConflictOverwrite ConflictAction = iota
    ConflictSkip
    ConflictRename
)

// transfer_service.go — UploadFile 中的冲突检测
func (ts *transferService) UploadFile(ctx context.Context, localPath, remotePath string,
    onProgress func(domain.TransferProgress), onConflict func(fileName string) (ConflictAction, string)) error {

    // 冲突检测
    if onConflict != nil {
        if _, err := ts.sftp.Stat(remotePath); err == nil {
            action, newPath := onConflict(filepath.Base(remotePath))
            switch action {
            case ConflictSkip:
                return nil
            case ConflictRename:
                remotePath = newPath
            case ConflictOverwrite:
                // 继续使用原路径
            }
        }
    }
    // ... 正常传输 ...
}
```

### 跨平台文件权限设置（build tags）

```go
// internal/adapters/data/transfer/permissions_windows.go
//go:build windows

package transfer

import (
    "os"
    "go.uber.org/zap"
)

func setFilePermissions(path string, mode os.FileMode, log *zap.SugaredLogger) {
    // Windows: Chmod 只影响只读标志，忽略 Unix 权限位
    // 不调用 os.Chmod，静默降级
    log.Debugw("skipping chmod on Windows", "path", path, "mode", mode)
}

// internal/adapters/data/transfer/permissions_unix.go
//go:build !windows

package transfer

import (
    "os"
    "go.uber.org/zap"
)

func setFilePermissions(path string, mode os.FileMode, log *zap.SugaredLogger) {
    if err := os.Chmod(path, mode); err != nil {
        log.Warnw("failed to set file permissions", "path", path, "mode", mode, "error", err)
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| 无取消支持 | context.WithCancel + ctx.Done() 检查 | Phase 3 | TransferService 接口签名变更，所有调用方需要传递 ctx |
| 直接覆盖目标文件 | 传输前 Stat 检测 + 冲突对话框 | Phase 3 | SFTPService 接口需要添加 Stat/Remove 方法 |
| Esc 直接关闭 modal | 第一次 Esc → 取消确认 → 第二次确认 | Phase 3 | TransferModal 增加模式状态管理 |
| 路径处理无平台分离 | build tags + filepath/path 分离 | Phase 3 | 添加 permissions_windows.go / permissions_unix.go |

**Deprecated/outdated:**
- 直接 `remoteFile.Close()` defer 模式 — 取消场景下需要先关闭再清理，不能简单 defer

## Open Questions

1. **是否需要 WithTimeout 保护**
   - What we know: context.WithCancel 支持手动取消，WithTimeout 会自动超时
   - What's unclear: 用户是否期望长时间无响应的传输自动超时（如网络中断但 SFTP 连接未断）
   - Recommendation: 不添加 WithTimeout（Claude's Discretion）。SFTP 协议有自己的超时机制，应用层超时可能误杀慢速传输。如果网络中断，Read 会返回 error，copyWithProgress 会自然结束。

2. **Rename 后缀格式**
   - What we know: D-07 指定 "自动添加 .1, .2 后缀"
   - What's unclear: 具体位置是 `file.1.txt` 还是 `file.txt.1`
   - Recommendation: 使用 `file.1.txt` 格式（在文件名和扩展名之间插入后缀），这是 mc 和 Windows 的默认行为

3. **目录传输取消后的清理策略**
   - What we know: D-04 要求删除部分文件
   - What's unclear: 已创建的空目录是否需要清理
   - Recommendation: 只删除部分文件，不删除空目录。空目录不占数据空间，且删除非空目录会导致误删（如果用户之前通过其他方式在该目录中放了文件）

4. **冲突回调的接口设计**
   - What we know: TransferService 需要在遇到冲突时暂停并询问用户
   - What's unclear: 冲突回调是通过额外参数传递还是作为 TransferService 的字段
   - Recommendation: 使用额外参数 `onConflict func(string) (ConflictAction, string)` 传递，这样 TransferService 保持无状态（stateless），测试更容易

## Environment Availability

Step 2.6: SKIPPED (no new external dependencies identified — all required libraries already in go.mod)

## Validation Architecture

> nyquist_validation 在 config.json 中设为 false，跳过此部分。

## Sources

### Primary (HIGH confidence)
- `go doc github.com/pkg/sftp.Client.Stat` — 验证 Stat 方法签名和返回类型
- `go doc github.com/pkg/sftp.Client.Remove` — 验证 Remove 方法可用于清理部分文件
- `go doc github.com/pkg/sftp.Client.Chmod` — 验证 Chmod 方法可用于权限设置
- `go doc context` — context.Context、context.WithCancel 标准用法
- 现有代码 `transfer_service.go` — copyWithProgress 实现，需要添加 ctx.Done() 检查
- 现有代码 `transfer_modal.go` — TransferModal 结构，showSummary 模式切换模式
- 现有代码 `file_browser.go` — initiateTransfer/initiateDirTransfer goroutine 模式
- 现有代码 `file_browser_handlers.go` — Esc 键当前处理逻辑
- 现有代码 `ports/file_service.go` — SFTPService 接口，需要添加 Stat/Remove
- 现有代码 `sysprocattr_windows.go` / `sysprocattr_unix.go` — build tag 模式参考

### Secondary (MEDIUM confidence)
- Go blog: "Context" (https://go.dev/blog/context) — context 包设计理念和最佳实践
- Go blog: "Package context" (https://go.dev/blog/context-and-structs) — context 在 struct 中的使用约定
- pkg/sftp GitHub (https://github.com/pkg/sftp) — Stat/Remove/Chmod 方法的跨平台行为

### Tertiary (LOW confidence)
- 无 — 所有研究结论基于已有代码直接观察和 go doc 验证

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — 所有库已在 go.mod 中，pkg/sftp API 通过 go doc 验证
- Architecture: HIGH — 基于现有代码的直接观察，TransferModal 模式切换在现有 showSummary 模式上有直接参考
- Pitfalls: HIGH — 所有 pitfall 来源于对 Go I/O 模型、tview 事件传播、goroutine 同步的深入理解，大部分可在编码前通过代码审查预防

**Research date:** 2026-04-13
**Valid until:** 30 days（Go 标准库 API 稳定，pkg/sftp v1 API 稳定，tview API 稳定）
