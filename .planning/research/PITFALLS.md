# Pitfalls Research: File Operations in Terminal SFTP File Browser (v1.2)

**Domain:** tview/tcell TUI 双面板文件浏览器中添加文件管理操作（删除/重命名/新建/复制/移动）
**Researched:** 2026-04-15
**Confidence:** HIGH -- 基于代码审查、pkg/sftp 库文档分析、SFTP 协议规范

## Critical Pitfalls

### Pitfall 1: SFTP 协议没有原生 copy 操作 -- 远程复制必须 download+re-upload

**What goes wrong:**
在远程面板按 `c` 标记文件后按 `p` 粘贴到另一个远程目录，实现者假设 SFTP 有服务端复制命令（类似本地 `cp`），直接调用某个 SFTP 方法。结果发现 SFTP 协议没有 copy 操作，代码无法编译或运行时报错。

**Why it happens:**
SFTP 协议（RFC 4254 + drafts）只定义了以下文件操作：
- `open` / `close` / `read` / `write` -- 文件读写
- `remove` / `rmdir` -- 删除
- `rename` / `posix-rename` -- 重命名/移动
- `mkdir` / `opendir` / `readdir` -- 目录操作
- `stat` / `lstat` / `fstat` -- 文件信息
- `symlink` / `readlink` / `realpath` -- 符号链接

**没有** `copy`、`copy-data`、`copy-file` 操作。

OpenSSH 9.0+ 添加了 `copy-data` 扩展（非标准），但其 `sftp` 客户端的 `cp` 命令依赖服务端支持。pkg/sftp 库没有暴露 `copy-data` 扩展 API。

**Consequences:**
- 远程面板内的文件复制（同服务器内从一个目录复制到另一个目录）必须通过 **download 到临时文件 + upload 到目标路径** 实现
- 这意味着远程复制比本地复制慢很多（数据经过本地中转）
- 对于大文件或目录，用户会感受到明显的延迟
- 如果实现者不知道这个限制，可能会花大量时间寻找不存在的 API

**Prevention:**
1. 在 `SFTPService` port 接口中，**不要**添加 `CopyRemoteFile` 方法 -- 这会误导实现者
2. 远程复制应复用现有的 `TransferService.DownloadFile` + `TransferService.UploadFile` 模式
3. 可以添加 `CopyRemoteToRemote(ctx, remoteSrc, remoteDst)` 高层方法，内部用临时文件实现：
   - `DownloadFile(ctx, remoteSrc, tempLocalPath, ...)`
   - `UploadFile(ctx, tempLocalPath, remoteDst, ...)`
   - 清理临时文件
4. 临时文件应放在系统临时目录（`os.TempDir()`），使用唯一名称避免冲突
5. 在 UI 中明确提示用户：远程复制需要经过本地中转，速度取决于网络带宽

**Detection:**
- 实现时搜索 `pkg/sftp` 的方法列表，找不到 `Copy` 方法
- 如果代码中出现 `client.Copy()` 调用，说明踩了这个坑

**Phase to address:**
实现复制功能时（可能需要单独的 phase 处理远程复制的特殊逻辑）

**Confidence:** HIGH -- 基于 pkg/sftp 官方文档和 SFTP 协议规范

---

### Pitfall 2: SFTP Remove 不能删除非空目录 -- 递归删除需要自行实现

**What goes wrong:**
用户在远程面板按 `d` 删除一个包含文件和子目录的目录。实现者调用 `SFTPService.Remove(path)`，结果报错 `SSH_FX_FAILURE`（目录非空）。`pkg/sftp.Client.Remove` 的文档明确写道："if the specified directory is not empty" 会返回错误。

**Why it happens:**
当前 `SFTPService` port 接口只暴露了 `Remove(path string) error`，其实现直接调用 `client.Remove()`。这个方法只能删除空目录或文件。

虽然 `pkg/sftp` 提供了 `RemoveAll(path string) error` 方法，可以递归删除目录，但当前 `SFTPService` 接口没有暴露它。需要扩展接口。

**更深层的问题：**
`pkg/sftp.RemoveAll` 的实现是逐个 `Remove` 文件和子目录。对于深层嵌套的大目录，这意味着大量的 SFTP 请求。如果连接在删除过程中断开，会留下部分删除的状态。

**Prevention:**
1. 在 `SFTPService` port 接口中添加 `RemoveAll(path string) error` 方法
2. 在 `SFTPClient` adapter 中使用 `pkg/sftp.Client.RemoveAll()` 实现
3. 对于大型目录，考虑在 UI 中显示删除进度（已删除文件数/总文件数）
4. 删除前先统计目录中的文件总数（用现有的 `WalkDir`），为进度显示提供数据
5. 删除操作应在 goroutine 中执行，避免阻塞 UI（参考 `initiateTransfer` 的 goroutine 模式）
6. 如果 `RemoveAll` 失败，记录已删除的文件列表，向用户报告部分删除状态

**Detection:**
- 删除非空目录时返回 `SSH_FX_FAILURE` 错误
- 状态栏显示删除错误信息

**Phase to address:**
实现删除功能的第一个 phase -- 必须在 port 接口扩展后才能实现 UI

**Confidence:** HIGH -- 基于 pkg/sftp 官方文档和代码审查

---

### Pitfall 3: 删除确认对话框与文件列表的竞态条件（TOCTOU）

**What goes wrong:**
用户选中一个文件，按 `d` 键弹出确认对话框。对话框显示 "Delete file.txt?"。在用户犹豫是否按 `y` 确认时，后台有其他进程（或另一个 SFTP 客户端）已经删除或重命名了该文件。用户按 `y` 确认后，删除操作失败，或者更糟的情况：另一个进程创建了同名的新文件，用户删除了错误的文件。

**Why it happens:**
这是经典的 Time-of-Check-Time-of-Use (TOCTOU) 问题。确认对话框显示的是 "检查时" 的文件信息，但删除操作执行在 "使用时"。中间的时间窗口内文件系统可能已经改变。

在 lazyssh 的上下文中，这个问题的严重程度取决于：
- 是否有其他 SFTP 客户端同时连接到同一服务器（常见场景）
- 是否有后台进程在操作同一目录（cron jobs, 文件同步工具等）

**Prevention:**
1. 确认对话框显示的信息应包含文件大小和修改时间（便于用户核对）
2. 用户确认后，在执行删除前再次 `Stat` 目标文件，确认它仍然存在且未被修改
3. 如果 `Stat` 返回的信息与对话框显示的不一致（大小或修改时间变化），显示 "File has changed since you selected it" 警告，让用户重新确认
4. 如果文件不存在了，显示 "File no longer exists" 而不是报错
5. 本地面板的竞态条件更容易发生（本地进程可能随时修改文件），远程面板相对少见但不应忽略

**Detection:**
- 删除操作偶尔报 "file not found" 或 "no such file"
- 用户反馈 "我确认删除的文件和实际删除的文件不是同一个"

**Phase to address:**
实现删除确认对话框的 phase

**Confidence:** HIGH -- TOCTOU 是文件操作的经典安全问题

---

### Pitfall 4: SFTP Rename 的跨文件系统限制

**What goes wrong:**
用户在远程面板按 `R` 重命名文件，或者用 `x` 标记后 `p` 粘贴到另一个目录（移动操作），操作失败并报错。某些 SFTP 服务器返回 `SSH_FX_FAILURE` 或 `EPERM`，原因是源路径和目标路径在不同的文件系统或挂载点上。

**Why it happens:**
SFTP 的 `rename` 操作底层映射到服务器的 `rename(2)` 系统调用。在 Unix 系统上，`rename()` 在跨文件系统时会失败（`EXDEV` 错误）。此外：

- `pkg/sftp.Client.Rename` 遵循 SFTP 协议的 rename 语义，如果目标已存在则失败
- `pkg/sftp.Client.PosixRename` 使用 OpenSSH 扩展 `posix-rename@openssh.com`，会替换已存在的目标（类似 Unix `rename(2)` 行为）
- 某些 SFTP 服务器（非 OpenSSH）可能不支持 `PosixRename`

**Prevention:**
1. 移动操作应优先尝试 `PosixRename`（原子操作，最快），如果失败再 fallback 到 copy+delete
2. Fallback 路径：`download + upload + delete source`（跨文件系统或服务器不支持时）
3. 重命名操作应使用 `Rename`（更安全，不会意外覆盖已有文件）
4. 如果重命名目标已存在，提示用户确认覆盖（类似现有的冲突处理对话框模式）
5. 在 port 接口中分别暴露 `Rename` 和 `PosixRename`，让 service 层决定使用哪个

**Detection:**
- 移动文件到不同挂载点时报 `SSH_FX_FAILURE`
- 重命名时目标已存在导致操作失败

**Phase to address:**
实现重命名和移动功能的 phase

**Confidence:** HIGH -- 基于 pkg/sftp 官方文档

---

## High Pitfalls

### Pitfall 5: 递归操作阻塞 UI -- 缺少进度反馈

**What goes wrong:**
用户删除一个包含数千个文件的大型目录，或复制/移动一个大型目录树。操作在 UI 线程中同步执行，终端完全冻结，无法取消，无法看到进度。用户以为程序崩溃了，强制终止进程。

**Why it happens:**
当前代码中，文件传输（upload/download）已经使用了 goroutine + `QueueUpdateDraw` + `TransferModal` 进度显示的模式。但如果文件操作（删除、复制、移动）直接在 UI 线程中同步调用 SFTP 方法，就会阻塞 tview 的事件循环。

特别是 `SFTPClient.RemoveAll` -- 对于一个包含 10000 个文件的目录，它需要发送 10000+ 个 SFTP 请求（每个文件一次 remove + 每个目录一次 rmdir），每个请求需要网络往返（RTT）。在 50ms RTT 的连接上，这需要 500 秒（8+ 分钟）。

**Prevention:**
1. **所有耗时操作必须在 goroutine 中执行**，参考 `initiateTransfer` 的模式
2. 对于递归删除，复用 `TransferModal` 显示进度（已删除 N/M 个文件）
3. 对于远程复制（download+re-upload），复用现有的 `TransferModal` 进度显示
4. 操作开始前先统计文件总数（`WalkDir`），为进度提供分母
5. 支持取消操作：使用 `context.Context`，参考 `transferCancel` 的模式
6. 如果操作耗时可能很长（>2秒），始终显示进度 UI

**Detection:**
- 执行文件操作时终端不响应按键
- 按键有明显的延迟后才生效

**Phase to address:**
所有涉及递归操作的 phase（删除目录、复制目录、移动目录）

**Confidence:** HIGH -- 基于代码审查，现有传输代码已正确处理此问题

---

### Pitfall 6: 剪贴板标记状态在目录导航后失效或丢失

**What goes wrong:**
用户在远程面板 `/home/user/docs` 目录中按 `c` 标记文件 `report.txt`，然后导航到 `/home/user/docs/archive` 目录，按 `p` 粘贴。但粘贴操作失败或粘贴了错误的文件。

**Why it happens:**
当前代码中，`LocalPane` 和 `RemotePane` 都有 `selected map[string]bool`，但这是多选状态（space 键），不是复制/移动的剪贴板。v1.2 需要添加新的剪贴板状态，与现有的 `selected` 多选状态区分。

关键问题：
1. 剪贴板存储的应该是完整路径（`currentPath + fileName`），而不是仅文件名。因为导航后 `currentPath` 改变了
2. 如果只存文件名，导航到另一个包含同名文件的目录后，粘贴的目标路径会出错
3. 剪贴板需要区分操作类型：复制（copy）vs 剪切（move）
4. 剪贴板状态需要在面板间共享（本地标记 -> 远程粘贴，或远程标记 -> 本地粘贴）

**Prevention:**
1. 定义独立的剪贴板结构体，存储完整路径、操作类型和来源面板
2. 导航时不清除剪贴板状态（这是核心 UX 决策 -- 用户标记后导航到目标目录再粘贴是自然的工作流）
3. 剪贴板状态在状态栏显示提示（例如 "2 files marked for copy"），让用户知道当前有标记
4. 切换面板（Tab）或按 Esc 时清除剪贴板（避免用户忘记标记存在）
5. 新的文件操作（删除/重命名/新建）不应清除剪贴板，除非操作影响了标记的文件

```go
// 剪贴板数据结构
type ClipboardState struct {
    Files       []domain.FileInfo  // 标记的文件（含完整路径）
    Operation   ClipboardOp        // Copy or Cut
    SourcePane  int                // 0=local, 1=remote
    SourcePath  string             // 来源目录的完整路径
}

type ClipboardOp int
const (
    ClipboardCopy ClipboardOp = iota
    ClipboardCut
)
```

**Detection:**
- 粘贴时操作了错误的文件（因为路径计算错误）
- 用户不知道当前有文件在剪贴板中，意外粘贴

**Phase to address:**
实现复制/移动标记功能的 phase

**Confidence:** HIGH -- 基于终端文件管理器的常见 UX 模式和代码审查

---

### Pitfall 7: SFTP 连接断开导致文件操作进行到一半

**What goes wrong:**
用户开始删除一个大型目录，删除到一半时 SSH 连接断开（网络中断、服务器重启、SSH 超时）。结果目录处于部分删除状态：一些文件被删除了，一些还在。用户无法知道哪些文件被删除了，也无法撤销。

**Why it happens:**
当前 `SFTPClient` 使用 `exec.Command("ssh", ...)` + `sftp.NewClientPipe()` 建立连接。SSH 连接可能在任何时候断开。`pkg/sftp` 在连接断开时会返回错误，但：
- `RemoveAll` 可能已经删除了部分文件
- 没有 "undo" 机制
- 用户不知道操作进行到了哪一步

**Prevention:**
1. 在 goroutine 中执行操作，通过 `QueueUpdateDraw` 更新进度
2. 操作失败时，在 UI 中显示已完成的操作数和错误信息
3. 对于删除操作，无法撤销 -- 必须在确认对话框中明确警告用户（"This cannot be undone"）
4. 对于移动操作（copy+delete），如果 delete 步骤失败，源文件仍然存在，目标文件可能部分存在。应提示用户检查两边的状态
5. 对于复制操作（download+re-upload），如果 upload 步骤失败，临时文件应被清理（参考现有的 D-04 cancel cleanup 模式）
6. 操作失败后自动刷新两个面板的文件列表

**Detection:**
- 操作中途返回 `EOF` 或 `connection lost` 错误
- 重新连接后文件列表与操作前不一致

**Phase to address:**
所有文件操作的错误处理

**Confidence:** HIGH -- 网络断开是 SFTP 操作的固有风险

---

### Pitfall 8: 符号链接在递归操作中导致无限循环

**What goes wrong:**
远程服务器上有一个目录包含循环符号链接：`/home/user/dir/link -> /home/user/dir`。用户尝试删除或复制这个目录，程序进入无限递归，最终导致栈溢出或 goroutine 泄漏。

**Why it happens:**
当前 `SFTPClient.WalkDir` 的实现（walkDir 方法）递归遍历目录，对每个目录调用 `client.ReadDir`。如果遇到指向父目录的符号链接，会产生无限递归。

查看现有代码：
```go
// walkDir (sftp_client.go:293-312)
func (c *SFTPClient) walkDir(client *sftp.Client, path string, files *[]string) error {
    entries, err := client.ReadDir(path)
    ...
    for _, e := range entries {
        fullPath := path + "/" + e.Name()
        if e.IsDir() {
            if err := c.walkDir(client, fullPath, files); err != nil {
                return err
            }
        } else {
            *files = append(*files, fullPath)
        }
    }
}
```

注意：`ReadDir` 对符号链接的行为取决于 SFTP 服务器。OpenSSH 的 `sftp-server` 默认跟随符号链接（`Readdir` 返回符号链接指向的内容）。如果符号链接指向一个目录，`e.IsDir()` 返回 true，递归会跟随进去。

但 `pkg/sftp` 的 `ReadDir` 实际上返回的是 `os.FileInfo`，其中 `IsDir()` 对于指向目录的符号链接会返回 `false`（因为 mode 包含 `fs.ModeSymlink`）。所以当前实现可能不会跟随符号链接目录，但这是未明确保证的行为。

**Prevention:**
1. 在递归遍历中显式检查符号链接：如果 `e.Mode()&fs.ModeSymlink != 0`，跳过该条目（不跟随）
2. 添加循环检测：维护已访问路径的 set，如果路径已访问过则跳过（防御性编程）
3. 在 `domain.FileInfo` 中已经有 `IsSymlink` 字段，在递归操作中使用它来跳过符号链接
4. 在删除确认对话框中提示用户目录包含符号链接（如果检测到）
5. 递归操作时限制最大深度（例如 100 层），防止意外的深层嵌套

**Detection:**
- 递归操作时内存持续增长（goroutine 栈溢出）
- 操作超时或 panic: runtime: goroutine stack exceeds

**Phase to address:**
实现递归删除和递归复制的 phase -- 必须在 `WalkDir` 中修复符号链接处理

**Confidence:** MEDIUM -- 基于代码分析，当前 walkDir 可能不受影响但行为未明确保证

---

## Medium Pitfalls

### Pitfall 9: 文件名编码问题 -- Unicode 特殊字符导致路径拼接错误

**What goes wrong:**
远程服务器上有包含 Unicode 字符的文件名（中文、日文、emoji、空格、特殊符号等）。用户尝试重命名或删除这些文件时，操作失败或操作了错误的文件。

**Why it happens:**
SFTP 协议使用 UTF-8 编码文件名（RFC），但：
1. 远程服务器可能使用其他编码（GBK, Shift-JIS, Latin-1 等）
2. 某些文件名包含控制字符或不可见字符
3. 文件名中的空格或特殊字符可能导致路径拼接问题
4. Unicode 规范化形式不同（NFC vs NFD）可能导致同一文件名的两种表示不匹配

当前代码中的路径拼接：
```go
// joinPath (remote_pane.go:441-446)
func joinPath(base, name string) string {
    if strings.HasSuffix(base, "/") {
        return base + name
    }
    return base + "/" + name
}
```

这个简单的字符串拼接在大多数情况下工作正常，但如果 `name` 包含 `/`（在某些文件系统中合法），会产生路径遍历。

**Prevention:**
1. 路径拼接后使用 `filepath.Clean()` 或 `path.Clean()` 规范化路径（远程使用 `/` 分隔符，应使用 `path.Clean`）
2. 在显示文件名时使用 `tview.Print` 而不是手动拼接（已正确处理 tview 标记字符）
3. 文件名输入框（重命名/新建目录）应过滤掉控制字符（ASCII 0x00-0x1F）和路径分隔符（`/`, `\`）
4. 验证路径不会产生 `..` 遍历（拼接后的规范路径应仍在预期目录下）
5. 对于重命名操作，使用 SFTP 的 `Rename` API 而不是手动构造路径（SFTP 服务器处理编码转换）

**Detection:**
- 包含中文/日文文件名的远程目录无法正确显示
- 重命名包含空格的文件时操作失败

**Phase to address:**
实现重命名和新建目录输入框的 phase

**Confidence:** MEDIUM -- 编码问题在不同服务器配置下表现不同

---

### Pitfall 10: 快捷键冲突 -- 新操作键与现有键绑定冲突

**What goes wrong:**
v1.2 计划添加的快捷键（`d` 删除, `R` 重命名, `m` 新建目录, `c` 复制, `x` 剪切, `p` 粘贴）与现有键绑定或与 TransferModal 的键绑定冲突。

**Why it happens:**
现有键绑定分析：
- `FileBrowser.handleGlobalKeys`: `Tab`, `Esc`, `F5`, `r`, `s`, `S`
- `LocalPane.InputCapture`: `h`, `Space`, `.`, `Backspace`
- `RemotePane.InputCapture`: `h`, `Space`, `.`, `Backspace`
- `TransferModal.HandleKey`: `Esc`, `y`, `n`, `o`, `s`, `r`（冲突对话框模式）
- `tview.Table` built-in: `j`, `k`, `Up`, `Down`, `Enter`, `PgUp`, `PgDn`

潜在冲突：
1. **`s` 键** -- 当前用于排序（`cycleSortField`），与文件操作无关，无冲突
2. **`r` 键** -- 全局用于最近目录弹出，TransferModal 冲突对话框中用于 Rename。如果 v1.2 添加远程重命名功能（`R` 大写），需确保 `R` 不会与 `r` 冲突（tview 的 `Rune()` 区分大小写，所以 `R` 和 `r` 是不同的键）
3. **`m` 键** -- 当前未使用，但 `M` 也未使用
4. **`c` 键** -- 当前未使用，但 `C` 也未使用
5. **`x` 键** -- 当前未使用
6. **`p` 键** -- 当前未使用，但需注意 `P` 也未使用
7. **`d` 键** -- 当前未使用。**关键冲突风险**：如果 TransferModal 可见时 `d` 被处理，可能导致在传输过程中执行删除操作

**Prevention:**
1. 所有新快捷键的处理必须在 `handleGlobalKeys` 中添加守卫条件：如果 `transferModal.IsVisible()` 或 `recentDirs.IsVisible()`，不处理新快捷键
2. 使用**大写字母**（`R`, `C`, `X`, `D`）作为文件操作键，与现有小写键区分
3. `p` 粘贴使用小写（因为大写 `P` 在某些终端中需要 Shift，不方便）
4. 维护一个完整的按键绑定矩阵文档，包含所有模式和所有按键
5. 在状态栏中显示当前可用的快捷键（已有此模式）

**Detection:**
- 在传输进度显示时按 `d` 意外触发删除
- 在冲突对话框中按 `r` 触发重命名而不是 Rename

**Phase to address:**
实现键盘路由的第一个 phase

**Confidence:** HIGH -- 基于代码审查，已有 TransferModal 按键冲突的先例（Pitfall 1 in v1.1）

---

### Pitfall 11: 移动操作（Move）的原子性 -- copy+delete 不是原子的

**What goes wrong:**
用户用 `x` 标记文件后 `p` 粘贴到另一个目录（移动操作）。实现者先 copy（download+upload）再 delete source。但如果 copy 成功后、delete 前连接断开，结果是文件同时存在于源和目标位置。用户不知道操作是否完成。

**Why it happens:**
远程移动有两种实现路径：
1. **SFTP Rename** -- 原子操作（如果源和目标在同一文件系统）
2. **Copy + Delete** -- 非原子操作（跨文件系统或跨服务器）

`Copy + Delete` 的两步操作之间可能出现错误：
- 网络断开
- 权限不足（能读源但不能删源）
- 磁盘空间不足（能读源但目标空间不够）

**Prevention:**
1. 移动操作优先尝试 `PosixRename`，失败后再 fallback 到 copy+delete
2. copy+delete 的 delete 步骤应在 copy 完全成功后才执行
3. 如果 delete 失败，在 UI 中明确告知用户："File copied but source could not be deleted. You may need to manually delete the source file."
4. 在移动操作的进度显示中区分两个阶段："Copying..." 和 "Removing source..."

**Detection:**
- 移动后源文件仍然存在
- 用户困惑：文件到底移动了还是复制了？

**Phase to address:**
实现移动功能的 phase

**Confidence:** HIGH -- copy+delete 非原子性是文件操作的固有风险

---

### Pitfall 12: 新建目录名称验证不足

**What goes wrong:**
用户按 `m` 键弹出输入框，输入了无效的目录名（空字符串、仅空格、包含 `/` 或 `..` 的路径、已存在的目录名），创建操作失败或创建了意外的目录结构。

**Why it happens:**
输入框接受用户自由输入，但不验证输入是否为合法的目录名。特别危险的情况：
- `../../etc` -- 路径遍历，在父目录之外创建目录
- `existing_dir` -- 创建已存在的目录，SFTP `Mkdir` 返回错误
- `` (空字符串) -- `Mkdir("")` 的行为未定义
- `dir with / slash` -- 在某些系统中创建嵌套目录

**Prevention:**
1. 验证输入非空且不包含路径分隔符（`/`, `\`）
2. 验证输入不包含 `..` 组件
3. 验证输入不包含控制字符
4. 检查目标位置是否已存在同名目录/文件（`Stat`），如果存在则提示用户
5. 限制输入长度（例如 255 字符，大多数文件系统的文件名长度限制）
6. 使用 `MkdirAll` 而不是 `Mkdir`（已存在于当前代码中），但仍然需要验证

**Detection:**
- 创建目录时报 "file exists" 或 "permission denied"
- 意外在父目录之外创建了目录

**Phase to address:**
实现新建目录输入框的 phase

**Confidence:** HIGH -- 输入验证是用户输入处理的基本要求

---

## Low Pitfalls

### Pitfall 13: 远程权限不足时的错误信息不友好

**What goes wrong:**
用户尝试删除或重命名远程文件，但当前 SSH 用户没有足够的权限。SFTP 返回 `SSH_FX_PERMISSION_DENIED`，UI 直接显示原始错误信息，用户不理解。

**Why it happens:**
远程文件权限由服务器控制。常见场景：
- 文件属于 root，当前用户是普通用户
- 目录设置了 sticky bit，只有文件所有者能删除
- 文件系统只读挂载
- SSH 配置限制了 SFTP 操作（`ForceCommand internal-sftp` with chroot）

**Prevention:**
1. 将 `SSH_FX_PERMISSION_DENIED` 错误翻译为用户友好的消息："Permission denied: you don't have rights to modify this file"
2. 在错误信息中包含文件名和当前用户名
3. 对于删除操作，如果权限不足，建议用户检查文件权限

**Phase to address:**
所有文件操作的错误处理

**Confidence:** HIGH

---

### Pitfall 14: 操作后文件列表未刷新或刷新时机不对

**What goes wrong:**
用户删除了一个文件，但文件列表中仍然显示该文件。或者用户复制了一个文件到当前目录，但列表中没有显示新文件。

**Why it happens:**
文件操作后需要刷新受影响面板的文件列表。当前代码中，文件传输成功后会刷新目标面板（`fb.remotePane.Refresh()` 或 `fb.localPane.Refresh()`）。文件操作也需要类似的刷新逻辑。

但刷新时机很关键：
1. 如果在 goroutine 中执行操作，刷新必须在 `QueueUpdateDraw` 中调用（线程安全）
2. 如果操作涉及两个面板（例如从本地复制到本地另一个目录），两个面板都需要刷新
3. 删除当前目录中的文件后，需要刷新当前面板，并且需要调整选中行（如果删除的文件在选中行之上）

**Prevention:**
1. 每个文件操作完成后，刷新所有受影响的面板
2. 刷新后尝试保持选中位置（记住删除前选中的文件名，刷新后找到最近的文件选中）
3. 使用 `QueueUpdateDraw` 确保线程安全

**Phase to address:**
所有文件操作的完成后处理

**Confidence:** HIGH -- 基于代码审查，现有传输代码已正确处理刷新

---

### Pitfall 15: 确认对话框中 `y` 键误触发

**What goes wrong:**
用户在确认对话框中输入 `y` 确认删除，但 `y` 键被 tview 的 InputHandler 处理为其他操作（例如移动到以 `y` 开头的文件）。

**Why it happens:**
如果确认对话框没有正确拦截按键，`y` 可能泄漏到背景的 Table 组件。tview 的 Table 在搜索模式下会跳转到以输入字母开头的文件（虽然这个行为取决于具体配置）。

**Prevention:**
1. 确认对话框必须消费所有按键（返回 nil），不传递给背景组件
2. 参考现有 TransferModal 的模式：在 `modeCancelConfirm` 和 `modeConflictDialog` 中消费所有按键
3. 确认对话框显示时，通过 `handleGlobalKeys` 的守卫条件拦截按键

**Phase to address:**
实现确认对话框的 phase

**Confidence:** HIGH -- 基于现有 TransferModal 的模式

---

## Technical Debt Patterns

| Shortcut | Description | Long-term Cost | When Acceptable |
|----------|-------------|----------------|-----------------|
| 远程复制用 download+upload 实现 | 不需要等待 SFTP copy-data 扩展标准化 | 大文件复制慢（数据经过本地中转），消耗本地临时磁盘空间 | Now -- SFTP copy-data 不是标准协议，pkg/sftp 不支持 |
| 删除操作不记录被删除文件列表 | 代码更简单，不需要维护删除日志 | 无法提供 "undo" 功能，错误删除后无法恢复 | Now -- undo 需要回收站机制，复杂度高 |
| 递归操作使用 RemoveAll 而不是逐文件删除 | 代码更少 | 无法提供细粒度的进度和取消 | Now -- 如果用户反馈需要，后续可以替换为自定义实现 |
| 符号链接在递归操作中直接跳过 | 简单安全 | 用户可能期望符号链接被跟随或被保留 | Now -- 符号链接处理策略需要用户调研 |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| handleGlobalKeys + 新快捷键 | 直接在 `switch event.Rune()` 中添加 case，没有检查 overlay 可见性 | 在处理新快捷键前检查 `transferModal.IsVisible()` 和其他 overlay 状态 |
| SFTPService.Remove + 非空目录 | 调用 `Remove()` 删除非空目录 | 添加 `RemoveAll()` 方法到 port 接口 |
| SFTPService.Rename + 目标已存在 | 调用 `Rename()` 期望覆盖目标 | 使用 `PosixRename` 或先检查目标是否存在再提示用户 |
| ClipboardState + 导航 | 导航时清除剪贴板 | 导航时保留剪贴板状态，在状态栏显示提示 |
| QueueUpdateDraw + 文件操作 | 在 goroutine 中直接调用 `pane.Refresh()` | 所有 UI 更新必须包裹在 `app.QueueUpdateDraw()` 中 |
| TransferModal + 删除确认 | 复用 TransferModal 的冲突对话框模式来显示删除确认 | 删除确认是一个独立的对话框，应该有独立的模式（或在 TransferModal 中添加新模式） |
| FileBrowser.Draw() + 新 overlay | 新的确认对话框 overlay 没有在 Draw() 中绘制 | 在 FileBrowser.Draw() 中添加新 overlay 的绘制调用 |

---

## "Looks Done But Isn't" Checklist

- [ ] **删除非空目录**: 删除包含文件和子目录的远程目录能正确完成 -- verify: 创建多层级目录，删除整个目录
- [ ] **删除进度**: 大型目录删除时显示进度 -- verify: 删除包含 100+ 文件的目录，观察进度显示
- [ ] **删除取消**: 删除过程中可以取消 -- verify: 开始删除大型目录，按 Esc 取消
- [ ] **远程复制**: 远程面板内复制文件到另一个远程目录能成功 -- verify: 在远程面板标记文件，导航到另一个目录，粘贴
- [ ] **远程复制临时文件清理**: 远程复制后临时文件被清理 -- verify: 检查 `os.TempDir()` 无残留文件
- [ ] **移动跨文件系统**: 移动文件到不同挂载点时 fallback 正确 -- verify: 如果可能，测试跨挂载点移动
- [ ] **重命名目标已存在**: 重命名为已存在的文件名时有确认提示 -- verify: 尝试重命名为目录中已有文件名
- [ ] **剪贴板跨面板**: 本地标记文件，Tab 切换到远程面板，粘贴能上传 -- verify: 本地标记 -> Tab -> 远程粘贴
- [ ] **剪贴板状态显示**: 有文件在剪贴板中时状态栏有提示 -- verify: 标记文件后查看状态栏
- [ ] **新建目录验证**: 输入无效目录名时有错误提示 -- verify: 尝试输入空名称、包含 `/` 的名称
- [ ] **符号链接安全**: 递归删除包含循环符号链接的目录不会无限循环 -- verify: 创建循环符号链接目录，尝试删除
- [ ] **Unicode 文件名**: 操作包含中文/日文/空格的文件名能成功 -- verify: 对含 Unicode 文件名的文件执行各种操作
- [ ] **操作后刷新**: 操作完成后文件列表正确更新 -- verify: 删除文件后列表不再显示该文件
- [ ] **权限错误**: 权限不足时显示友好错误信息 -- verify: 尝试删除无权限的远程文件
- [ ] **快捷键不泄漏**: 确认对话框中按键不泄漏到背景 Table -- verify: 在确认对话框中按各种字母键

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| P1: SFTP 无 copy | LOW | 实现为 download+upload，用临时文件，清理临时文件 |
| P2: Remove 不能删非空目录 | LOW | 使用 `RemoveAll()` 或实现自定义递归删除 |
| P3: TOCTOU 竞态 | LOW | 删除前再次 Stat 验证，失败时友好提示 |
| P4: Rename 跨文件系统 | MEDIUM | 先尝试 PosixRename，失败后 fallback 到 copy+delete |
| P5: 递归操作阻塞 UI | MEDIUM | 重构为 goroutine + QueueUpdateDraw 模式 |
| P6: 剪贴板路径失效 | LOW | 使用完整路径存储，导航时不清除剪贴板 |
| P7: 连接断开 | MEDIUM | 显示已完成的操作数，建议用户重新连接后检查 |
| P8: 符号链接循环 | MEDIUM | 检查 IsSymlink 跳过，添加路径已访问检测 |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| P1: SFTP 无 copy | 远程复制实现 | 远程面板内复制文件能成功 |
| P2: Remove 不能删非空目录 | Port 接口扩展 | 删除非空远程目录能成功 |
| P3: TOCTOU 竞态 | 确认对话框实现 | 删除前再次 Stat 验证 |
| P4: Rename 跨文件系统 | 重命名/移动实现 | 跨文件系统移动有 fallback |
| P5: 递归操作阻塞 UI | 所有递归操作 | 大型目录操作时 UI 不冻结 |
| P6: 剪贴板路径失效 | 复制/移动标记实现 | 标记后导航再粘贴能成功 |
| P7: 连接断开 | 错误处理 | 断连后显示已完成操作数 |
| P8: 符号链接循环 | WalkDir 修复 + 递归操作 | 循环符号链接目录不导致无限循环 |
| P9: Unicode 编码 | 输入框实现 | Unicode 文件名操作正常 |
| P10: 快捷键冲突 | 键盘路由实现 | 所有模式下按键不冲突 |
| P11: 移动非原子 | 移动实现 | 移动失败时提示用户检查 |
| P12: 目录名验证 | 新建目录实现 | 输入无效名称有提示 |
| P13: 权限错误信息 | 错误处理 | 权限不足时显示友好消息 |
| P14: 刷新时机 | 所有操作 | 操作后列表正确更新 |
| P15: 确认对话框按键泄漏 | 确认对话框 | 对话框中按键不泄漏 |

---

## Sources

- [pkg/sftp 官方文档](https://pkg.go.dev/github.com/pkg/sftp) -- HIGH confidence: Remove, RemoveAll, Rename, PosixRename, Lstat, ReadLink 方法文档
- [OpenSSH PROTOCOL file](https://github.com/openssh/libopenssh/blob/master/ssh/PROTOCOL) -- HIGH confidence: copy-data 扩展规范
- [SFTP copy without roundtrip - SuperUser](https://superuser.com/questions/1166354/copy-file-on-sftp-to-another-directory-without-roundtrip) -- HIGH confidence: 确认 SFTP 无原生 copy
- [SFTP remote server side copy - rclone forum](https://forum.rclone.org/t/sftp-remote-server-side-copy/41867) -- MEDIUM confidence: 社区对 SFTP copy 限制的讨论
- [pkg/sftp SSH_FX_FAILURE for removing directories #137](https://github.com/pkg/sftp/issues/137) -- HIGH confidence: Remove 非空目录的限制
- [OWASP Unicode Encoding](https://owasp.org/www-community/attacks/Unicode_Encoding) -- MEDIUM confidence: Unicode 编码安全问题
- [Path Traversal in SFTP QUOTE command - HackerOne](https://hackerone.com/reports/3293177) -- HIGH confidence: SFTP 路径遍历漏洞
- [sindresorhus/del race condition #43](https://github.com/sindresorhus/del/issues/43) -- MEDIUM confidence: 异步文件删除竞态条件
- [lf file manager documentation](https://github.com/gokcehan/lf/blob/master/doc.md) -- LOW confidence: 终端文件管理器剪贴板模式参考
- 项目源码分析 -- HIGH confidence: SFTPClient, TransferModal, handleGlobalKeys, WalkDir 现有实现

---
*Pitfalls research for: lazyssh v1.2 File Operations*
*Researched: 2026-04-15*
