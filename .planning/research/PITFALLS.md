# Pitfalls Research: v1.3 Enhanced File Browser

**Domain:** tview/tcell TUI 文件浏览器中添加三个增强功能：本地路径历史持久化、SSH 连接复制、双远端文件互传
**Researched:** 2026-04-15
**Confidence:** HIGH -- 基于现有代码深度审查、v1.0-v1.2 实际踩坑经验

## Critical Pitfalls

### Pitfall 1: 本地路径历史与远程目录 MRU 的持久化模型不一致

**What goes wrong:**
实现者参考现有 `RecentDirs` 的持久化模式（每个服务器一个 JSON 文件 `~/.lazyssh/recent-dirs/{user@host}.json`），为本地路径历史采用相同结构。但本地路径没有"服务器"维度，导致需要造一个假的 serverKey，或者把所有本地路径混在一个文件里，结果违反了现有记录粒度的设计约束。

**Why it happens:**
现有 `RecentDirs` 的持久化模型是**「本机目录 + 服务器」组合**（PROJECT.md Key Decision: "记录粒度为 本机目录 + 服务器 组合，避免跨服务器目录列表泄露"）。这个粒度对远程目录有意义，但对本地路径历史没有意义 -- 本地路径与具体服务器无关。

更深层问题：`RecentDirs` 当前同时承担了两个职责：
1. **数据层**：MRU 列表的 CRUD + 持久化（`loadFromDisk`, `saveToDisk`, `Record`, `GetPaths`）
2. **UI 层**：overlay 弹出框（`Draw`, `HandleKey`, `Show`, `Hide`）

如果本地路径历史也需要弹出列表（例如 `l` 键弹出本地历史），就会产生代码重复。但如果复用 `RecentDirs` 组件，就需要重构它以支持不同的持久化策略。

**Consequences:**
- 本地路径历史如果没有服务器维度，与现有 MRU 模型不一致
- 试图复用 `RecentDirs` 但发现它的 `serverKey` 硬编码到文件路径中
- 如果为本地路径创建独立的组件，会与 `RecentDirs` 产生大量重复代码
- 持久化路径命名冲突：`~/.lazyssh/recent-dirs/` 下放什么文件？

**Prevention:**
1. 将本地路径历史的存储文件放在 `~/.lazyssh/local-path-history.json`（全局，无服务器维度）
2. 不复用 `RecentDirs` 的 UI 组件（因为弹出列表的交互不同），但可以抽取公共的 MRU 数据逻辑到独立的 struct：
   ```go
   // PathHistory 纯数据层，无 UI 依赖
   type PathHistory struct {
       paths    []string
       maxItems int
       filePath string
       log      *zap.SugaredLogger
   }
   func (ph *PathHistory) Record(path string) { ... }
   func (ph *PathHistory) GetPaths() []string { ... }
   func (ph *PathHistory) loadFromDisk() { ... }
   func (ph *PathHistory) saveToDisk() { ... }
   ```
3. 如果需要弹出列表 UI，创建独立的 overlay 组件（类似 `RecentDirs` 但使用 `PathHistory` 作为数据源）
4. 记录时机：在 `initiateTransfer` 和 `initiateDirTransfer` 成功后记录使用的本地路径（类似现有 `recentDirs.Record(fb.remotePane.GetCurrentPath())` 的模式）
5. 路径记录应区分上传路径和下载路径？建议不区分 -- 用户的"最近本地路径"就是最近用过的路径，不分方向

**Detection:**
- 发现自己在 `RecentDirs` 的构造函数中传入空的 `serverHost` 或 `serverUser`
- 本地路径历史的 JSON 文件路径命名与 `recent-dirs/` 目录下的文件混淆
- 复制粘贴 `RecentDirs` 的 `Draw()` 和 `HandleKey()` 代码到新组件中

**Phase to address:**
本地路径历史持久化的第一个 phase -- 必须先确定数据模型和存储策略

**Confidence:** HIGH -- 基于对 `RecentDirs` 源码的审查

---

### Pitfall 2: Dup SSH 连接的 `d` 键冲突 -- 服务器列表中 `d` 已绑定到删除

**What goes wrong:**
PROJECT.md 写的是 "服务器列表 d 键复制配置创建新条目"，但现有代码中 `d` 键已经绑定到 `handleServerDelete()`（handlers.go:60-62）。直接按描述实现会在按 `d` 时删除服务器而不是复制。

**Why it happens:**
需求描述中的快捷键与现有实现冲突。查看 `handleGlobalKeys`：
```go
case 'd':
    t.handleServerDelete()
    return nil
```

此外，服务器表单的删除确认对话框中 `d`/`D` 也用于确认删除（handlers.go:407-410）。这意味着如果改变 `d` 的含义，删除操作将没有快捷键入口。

**Consequences:**
- 按预期实现会导致删除功能丢失快捷键
- 如果保留 `d` 为删除，Dup 功能需要另一个键
- 用户习惯被打破（v1.0-v1.2 用户已习惯 `d` = 删除）

**Prevention:**
1. Dup 操作必须使用不同的快捷键。推荐 `y`（"yank" 是 Unix 术语中的复制/拉取操作，与 vim 的 `y` 一致），或者 `D`（大写，与 `d` 区分）
2. 如果选择 `D`：tview 的 `event.Rune()` 对大小写敏感，所以 `D` 和 `d` 是不同的键。但需注意某些终端在 Caps Lock 开启时可能不区分
3. Dup 操作的完整流程：
   - 在服务器列表中选中一个服务器
   - 按 `y`（或 `D`）触发 Dup
   - 复制当前 Server 的所有字段
   - 修改 Alias（自动添加 `-copy` 后缀，例如 `myserver` -> `myserver-copy`）
   - 调用 `ServerService.AddServer()` 写入 SSH config
   - 刷新列表并滚动到新条目
4. Dup 后应自动打开编辑表单，让用户修改 Alias 和其他字段（类似先 `Add` 再预填字段）
5. 或者更简单的方案：Dup 后直接添加并选中，用户按 `e` 编辑

**Detection:**
- 检查 `handleGlobalKeys` 中 `d` 的绑定目标
- 如果代码中 `case 'd'` 调用的是 `handleServerDuplicate()` 而不是 `handleServerDelete()`

**Phase to address:**
Dup SSH 连接的第一个 phase -- 开始前必须确认快捷键选择

**Confidence:** HIGH -- 基于代码审查，`d` 键绑定已确认

---

### Pitfall 3: 双远端传输的 SFTP 连接生命周期管理 -- 单连接 vs 双连接

**What goes wrong:**
现有 `FileBrowser` 持有一个 `sftpService`，连接到一台服务器。双远端传输需要同时连接两台服务器。实现者试图复用现有 `FileBrowser` 的架构，发现：
1. `TransferService` 持有一个 `sftpService`（单连接），无法同时访问两台服务器
2. `FileBrowser` 只有一个 `RemotePane`，UI 上只有一个远程面板
3. `TransferModal` 的进度显示假设传输只有两个阶段（上传或下载），但双远端传输有四个阶段

**Why it happens:**
当前架构是围绕 "本地 <-> 单远程" 设计的：
```go
type transferService struct {
    log  *zap.SugaredLogger
    sftp ports.SFTPService  // 单连接
}
```

```go
type FileBrowser struct {
    ...
    sftpService    ports.SFTPService  // 单连接
    transferSvc    ports.TransferService
    localPane      *LocalPane
    remotePane     *RemotePane        // 单远程面板
    ...
}
```

双远端传输需要：
- 两个独立的 SFTP 连接（SFTPClient 实例）
- 两个 RemotePane（或至少两个 SFTPService 引用）
- 新的 TransferService 方法（或独立的 service）处理 "远程 A -> 本地临时 -> 远程 B" 的三段式传输

**Consequences:**
- 无法复用现有 `TransferService` 的 `UploadFile`/`DownloadFile`（它们绑定到单个 `sftp` 字段）
- 如果强行修改 `TransferService` 添加第二个 SFTP 连接，会破坏现有单连接场景的接口语义
- 进度显示模型完全不同：不是 "Uploading file.txt"，而是 "Downloading from A: file.txt" -> "Uploading to B: file.txt"
- 取消逻辑更复杂：需要关闭两个 SFTP 连接，清理本地临时文件

**Prevention:**
1. **不要修改现有 `TransferService`**，它服务于 "本地 <-> 远程" 场景
2. 创建新的 `RelayTransferService`（或 `DualRemoteTransferService`），接口如下：
   ```go
   type RelayTransferService interface {
       // RelayFile transfers a file from one remote server to another via local machine.
       // Phase 1: Download from source remote to local temp
       // Phase 2: Upload from local temp to destination remote
       RelayFile(ctx context.Context, srcServer domain.Server, srcPath string,
           dstServer domain.Server, dstPath string,
           onProgress func(RelayProgress), onConflict ConflictHandler) error

       // RelayDir transfers a directory from one remote server to another via local machine.
       RelayDir(ctx context.Context, srcServer domain.Server, srcPath string,
           dstServer domain.Server, dstPath string,
           onProgress func(RelayProgress), onConflict ConflictHandler) ([]string, error)
   }
   ```
3. `RelayTransferService` 内部创建和管理两个临时的 `SFTPClient` 实例
4. 进度回调 `RelayProgress` 需要包含阶段信息：
   ```go
   type RelayPhase int
   const (
       PhaseDownloadFromSource RelayPhase = iota
       PhaseUploadingToDestination
   )
   type RelayProgress struct {
       domain.TransferProgress
       Phase    RelayPhase
       SrcLabel string // "server-a:/path"
       DstLabel string // "server-b:/path"
   }
   ```
5. 传输完成后清理：关闭两个 SFTP 连接，删除临时文件/目录
6. UI 层面，双远端传输可能需要一个独立于 `FileBrowser` 的新入口（服务器列表中选择两台服务器），而不是在现有 `FileBrowser` 内部添加

**Detection:**
- 发现自己在 `TransferService` 中添加 `srcSFTP` 和 `dstSFTP` 两个字段
- 试图让一个 `SFTPClient` 同时服务两个服务器
- `TransferModal` 的 `Show(direction, fileName)` 无法表达 "Relay from A to B" 的语义

**Phase to address:**
双远端传输的第一个 phase -- 必须先设计连接管理和进度模型

**Confidence:** HIGH -- 基于代码审查，现有架构确认是单连接设计

---

### Pitfall 4: 双远端传输的本地临时文件空间耗尽

**What goes wrong:**
用户从服务器 A 传输一个 50GB 的目录到服务器 B。中转流程：先下载到本地临时目录，再上传到目标。本地磁盘只有 30GB 可用空间，下载到一半时磁盘写满，下载失败，临时文件残留。

**Why it happens:**
双远端传输的 "download -> upload" 模式需要本地磁盘空间 >= 待传输文件大小。这与现有的 `CopyRemoteFile`/`CopyRemoteDir` 有同样的问题，但规模更大：
- 现有远程复制是在同一台服务器内，文件大小通常可控
- 双远端传输涉及不同服务器，文件可能非常大（备份数据库、日志归档等）

更严重的是，如果使用 `DownloadDir` + `UploadDir` 的两阶段模式，中间的临时目录可能包含完整的目标文件树副本，占用空间等于源目录大小。

**Consequences:**
- 本地磁盘写满导致下载失败
- 临时文件残留占用空间
- 后续传输也失败（磁盘已满）
- 用户不知道发生了什么（如果错误信息不清晰）

**Prevention:**
1. **传输前检查本地磁盘可用空间**：
   ```go
   func checkDiskSpace(path string, required int64) error {
       var stat syscall.Statfs_t
       syscall.Statfs(path, &stat)
       available := stat.Bavail * uint64(stat.Bsize)
       if uint64(required) > available {
           return fmt.Errorf("insufficient disk space: need %d, available %d", required, available)
       }
       return nil
   }
   ```
   注意：对于目录传输，可能无法精确预知总大小（远程 `WalkDir` 不返回目录大小）。保守策略：检查可用空间 > 单文件大小（文件传输）或跳过检查但监控（目录传输）。
2. 使用 `os.TempDir()` 确保临时文件在系统临时目录中（通常有自动清理机制）
3. 传输失败时确保 `defer os.RemoveAll(tmpDir)` 清理所有临时文件
4. 在进度 UI 中显示已使用的临时空间大小，让用户感知磁盘使用情况
5. 考虑流式中转（边下载边上传），避免在本地存储完整文件。但这需要两个 SFTP 连接的协调，复杂度更高。v1.3 建议先用简单的两阶段模式，后续优化为流式。

**Detection:**
- 传输到一半报 "no space left on device"
- `os.TempDir()` 下残留大量 `lazyssh-*` 临时文件
- 系统变慢因为磁盘 I/O 或 inode 耗尽

**Phase to address:**
双远端传输的数据层实现

**Confidence:** HIGH -- 磁盘空间是本地中转传输的固有限制

---

### Pitfall 5: 双远端传输中 SSH 认证失败 -- 密码提示阻塞 goroutine

**What goes wrong:**
双远端传输需要同时建立两个 SFTP 连接。其中一个服务器使用密码认证（不是密钥），`ssh` 进程在 `cmd.Start()` 后等待密码输入。但 SFTP 连接是在 goroutine 中建立的，没有终端交互能力。`ssh` 进程挂起等待密码，goroutine 永远不返回，UI 显示 "Connecting..." 但永远不会完成。

**Why it happens:**
现有 `SFTPClient.Connect()` 使用 `exec.Command("ssh", ...)` + `cmd.Start()`。对于密码认证的服务器：
- `cmd.Start()` 启动 SSH 进程
- SSH 进程尝试连接，发现需要密码
- SSH 进程向 stdin 写密码提示（"user@host's password: "）
- 但 `cmd.Stdin` 已经被重定向到 `sftp.NewClientPipe`，不会发送密码
- SSH 进程挂起等待输入

现有的单连接场景中，如果密码认证失败，`sftp.NewClientPipe()` 会超时或返回错误。但超时时间可能很长（SSH 默认超时），导致 UI 卡住。

**Consequences:**
- 连接建立长时间无响应
- 用户以为程序崩溃
- goroutine 泄漏（SSH 进程永远不会退出）

**Prevention:**
1. 在 `Connect()` 中设置 `cmd.WaitDelay`（Go 1.20+）或使用 `context.WithTimeout` 包裹 `cmd.Wait()`
2. 为 SFTP 连接建立设置合理的超时（例如 10 秒）
3. 连接失败时确保清理 SSH 进程（`cmd.Process.Kill()`）
4. 在双远端传输的 UI 中，为每个连接分别显示连接状态（"Connecting to A...", "Connecting to B..."）
5. 如果任一连接失败，立即取消另一个连接（不必要地占用资源）
6. 考虑在双远端传输开始前先 Ping 两台服务器（复用现有的 `ServerService.Ping`），快速筛选不可达的服务器

**Detection:**
- 双远端传输开始后 UI 卡在 "Connecting..." 超过 10 秒
- 系统进程列表中有挂起的 `ssh` 进程

**Phase to address:**
双远端传输的连接管理

**Confidence:** HIGH -- 密码认证阻塞是 `exec.Command("ssh")` 模式的已知问题

---

## High Pitfalls

### Pitfall 6: Dup SSH 连接后 metadata（标签、置顶、计数）的处理策略

**What goes wrong:**
用户复制一个有标签（tags）、置顶（pinned）、SSH 使用次数（ssh_count）等 metadata 的服务器。复制后的新条目继承了所有 metadata，但这是不合理的：
- 置顶状态：新服务器不应该被置顶（用户还没决定是否常用它）
- SSH 使用次数：新服务器的使用次数应该为 0
- LastSeen：新服务器从未连接过
- 标签：标签是否应该复制？可能合理（用户可能想要相同的标签分类）

**Why it happens:**
现有的 `AddServer` 方法会调用 `metadataManager.updateServer(server, server.Alias)`，将 `server.Tags` 写入 metadata。如果 Dup 操作直接复制 `domain.Server` 的所有字段并调用 `AddServer`，所有 metadata 也会被写入。

查看 `metadataManager.updateServer`：
```go
func (m *metadataManager) updateServer(server domain.Server, oldAlias string) error {
    ...
    merged.Tags = server.Tags
    if !server.LastSeen.IsZero() {
        merged.LastSeen = server.LastSeen.Format(time.RFC3339)
    }
    if !server.PinnedAt.IsZero() {
        merged.PinnedAt = server.PinnedAt.Format(time.RFC3339)
    }
    if server.SSHCount > 0 {
        merged.SSHCount = server.SSHCount
    }
    ...
}
```

如果复制的 `Server` 有 `PinnedAt` 非零，新服务器也会被置顶。

**Prevention:**
1. Dup 操作后，清除新 Server 的非配置类 metadata：
   ```go
   dup := original
   dup.Alias = generateDupAlias(original.Alias)
   dup.PinnedAt = time.Time{}   // 不继承置顶
   dup.LastSeen = time.Time{}   // 不继承 LastSeen
   dup.SSHCount = 0             // 不继承使用次数
   // dup.Tags = original.Tags  // 标签可以继承（用户可能需要）
   ```
2. 或者更简洁的方案：Dup 操作只复制 SSH config 字段（Host, Port, User, IdentityFile 等），不复制任何 metadata。Tags 在 `Server` 实体上，metadata 也在 `Server` 实体上，但 SSH config 中只有配置字段
3. 需要区分 "SSH config 字段" 和 "lazyssh metadata 字段"。查看 `domain.Server` 的定义来确认哪些字段属于 SSH config，哪些属于 metadata

**Detection:**
- Dup 后新服务器出现在列表顶部（被置顶了）
- Dup 后新服务器的 SSH 使用次数 > 0

**Phase to address:**
Dup SSH 连接的实现

**Confidence:** HIGH -- 基于对 `metadataManager.updateServer` 的代码审查

---

### Pitfall 7: 双远端传输的进度重置问题 -- 四阶段 vs 两阶段

**What goes wrong:**
现有的 `TransferModal` 为两阶段传输设计（例如远程复制：下载 -> 上传）。`TransferModal.ResetProgress()` 在阶段切换时重置进度条。但双远端传输也是两阶段（下载 -> 上传），如果直接复用 `TransferModal`，用户看到的是：
- 阶段 1: "Downloading: file.txt" 进度 0-100%
- 阶段 2: "Uploading: file.txt" 进度 0-100%

这看起来和远程复制完全一样，用户无法区分 "本地中转" 和 "远程复制"。而且进度条在阶段切换时重置为 0，用户可能误以为传输失败了又重新开始。

**Why it happens:**
现有的远程复制（`CopyRemoteFile`/`CopyRemoteDir`）和双远端传输都是 "download -> upload" 两阶段模式。`TransferModal` 的进度显示无法区分两者的区别。双远端传输需要显示更多信息：
- 源服务器标签（"From: server-a"）
- 目标服务器标签（"To: server-b"）
- 当前阶段（"Phase 1/2: Downloading from server-a"）
- 总体进度（如果可能的话）

**Prevention:**
1. 扩展 `TransferModal`（或创建 `RelayTransferModal`）支持四段式信息显示：
   ```
   Relay: server-a -> server-b
   Phase 1/2: Downloading from server-a
   file.txt  [=========>          ] 45%  12.3 MB/s  ETA 2m
   ```
   ```
   Relay: server-a -> server-b
   Phase 2/2: Uploading to server-b
   file.txt  [=========>          ] 30%  8.7 MB/s  ETA 3m
   ```
2. 阶段切换时不重置进度条为 0，而是显示 "Phase 2/2" 标签让用户理解进度重置是正常的
3. 或者使用两个独立的进度条（下载进度 + 上传进度），但这需要修改 `TransferModal` 的布局
4. 传输完成后显示汇总：
   ```
   Relay Complete: server-a -> server-b
   Downloaded: 150 MB in 12s (12.5 MB/s)
   Uploaded: 150 MB in 18s (8.3 MB/s)
   Temp files cleaned up
   ```

**Detection:**
- 双远端传输时 `TransferModal` 的标题显示 "Uploading" 或 "Downloading" 而不是 "Relay"
- 进度重置时用户困惑

**Phase to address:**
双远端传输的 UI 层实现

**Confidence:** HIGH -- 基于对 `TransferModal` 源码的审查

---

### Pitfall 8: 本地路径历史的路径规范化和跨平台一致性

**What goes wrong:**
用户在 Windows 上使用 lazyssh，上传了 `C:\Users\test\Documents\file.txt`。路径被记录到 `~/.lazyssh/local-path-history.json`。下次用户打开文件浏览器时，历史列表显示 `C:\Users\test\Documents`，但用户当前的工作目录是 `C:\Users\test\documents`（大小写不同，但 Windows 文件系统不区分大小写）。路径匹配失败，高亮不工作。

**Why it happens:**
路径规范化在不同平台上不一致：
- Linux/macOS：路径区分大小写，`/home/user` != `/Home/User`
- Windows：路径不区分大小写，`C:\Users` == `c:\users`
- Windows 有驱动器号（`C:\`），Linux/macOS 没有
- macOS 文件系统（APFS）默认不区分大小写
- 符号链接可能导致同一路径有不同表示

现有的 `RecentDirs` 使用简单的字符串比较：
```go
isCurrent := path == rd.currentPath
```

**Prevention:**
1. 记录路径时使用 `filepath.Clean()` 规范化（去除多余的 `.`、`..`、重复分隔符）
2. 在 Linux 上不区分大小写比较（因为记录的路径可能来自不同的规范形式）
3. 在 Windows 上使用 `filepath.Equal()` 或 `strings.EqualFold()` 比较
4. 在 macOS 上，APFS 默认不区分大小写，应使用不区分大小写的比较
5. 或者简化处理：只对显示路径做精确匹配，不做大小写归一化。用户的路径历史就是他们用过的路径，如果大小写不同就是不同的路径
6. 跨平台路径分隔符：`filepath.Join()` 已经处理了这个问题（Windows 用 `\`，Unix 用 `/`）

**Detection:**
- Windows 上路径历史中的路径无法高亮当前目录
- macOS 上同一路径的不同大小写形式被记录为两个条目

**Phase to address:**
本地路径历史的数据层实现

**Confidence:** MEDIUM -- 跨平台路径规范化是已知难题，但 lazyssh 的使用场景（终端用户）通常在单一平台上

---

## Medium Pitfalls

### Pitfall 9: Dup 后的 Alias 命名冲突 -- 循环后缀

**What goes wrong:**
用户对 `myserver` 执行 Dup，生成 `myserver-copy`。再对 `myserver` 执行 Dup，又生成 `myserver-copy`，与已有条目冲突（`AddServer` 会报 "alias already exists"）。

**Why it happens:**
简单的后缀策略（添加 `-copy`）不考虑已有别名。需要递增后缀或更智能的命名。

**Prevention:**
1. 实现递增后缀策略：`myserver` -> `myserver-copy` -> `myserver-copy-2` -> `myserver-copy-3`
2. 在生成别名后、调用 `AddServer` 前，检查别名是否已存在（`serverExists`）
3. 如果已存在，递增后缀直到找到可用的别名
4. 或者使用时间戳后缀：`myserver-copy-20260415`，保证唯一性但不够友好
5. Dup 后自动进入编辑模式，让用户修改别名（最佳 UX）

**Detection:**
- 第二次 Dup 同一个服务器时报 "alias already exists"

**Phase to address:**
Dup SSH 连接的实现

**Confidence:** HIGH -- 别名冲突是 `AddServer` 的已知约束

---

### Pitfall 10: 双远端传输的入口 UX -- 如何选择两台服务器

**What goes wrong:**
用户需要选择两台不同的服务器进行互传。实现者设计了一个复杂的多步骤选择流程：
1. 进入服务器列表
2. 标记第一台服务器（Space 键）
3. 标记第二台服务器
4. 按某个键开始传输

但这个流程与现有的服务器列表交互模式不一致。现有列表使用单选（j/k 导航 + Enter 连接），没有多选概念。

**Why it happens:**
当前服务器列表 UI（`tview.Table`）没有多选支持。远程面板有 `selected map[string]bool` 的多选（Space 键），但服务器列表没有。

**Prevention:**
1. 使用两步选择模式：
   - 步骤 1：用户选中服务器 A，按 `T`（Transfer）键
   - 步骤 2：UI 进入 "Select destination server" 模式，标题栏提示 "Select destination server for relay transfer"
   - 步骤 3：用户导航到服务器 B，按 Enter 确认
   - 步骤 4：打开双远端文件浏览器（或直接开始传输）
2. 或者使用 Space 键多选（两台），然后按 `T` 开始
3. 状态栏提示应清晰指导用户操作："Select first server, press T"
4. 不允许选择同一台服务器（源和目标必须不同）
5. 选择完成后打开一个类似 `FileBrowser` 的双远端浏览器（左面板 = 服务器 A 的远程文件，右面板 = 服务器 B 的远程文件）

**Detection:**
- 服务器列表的 Space 键没有反应（因为没有多选实现）
- 用户不知道如何选择第二台服务器

**Phase to address:**
双远端传输的 UI 入口实现

**Confidence:** HIGH -- UX 流程设计是双远端传输的核心挑战

---

### Pitfall 11: 双远端传输中两个 SFTP 连接的并发问题

**What goes wrong:**
双远端传输同时操作两个 SFTP 连接。如果两个连接共享某些状态（例如同一个 `*zap.SugaredLogger` 的并发写入、同一个临时目录的文件操作），可能产生竞态条件。

**Why it happens:**
Go 的 `sftp.Client` 不是线程安全的（尽管有 mutex 保护了 Connect/Close）。如果多个 goroutine 同时使用同一个 client，可能产生问题。

在双远端传输中，如果使用两个独立的 `SFTPClient` 实例，每个实例有自己的 mutex 和 SSH 进程，理论上不会冲突。但如果实现不当（例如共享临时目录、共享 logger 的某些状态），仍可能有问题。

**Prevention:**
1. 每个远程服务器使用独立的 `SFTPClient` 实例（独立的 SSH 进程、独立的 SFTP 连接）
2. 临时目录使用 `os.MkdirTemp("", "lazyssh-relay-*")`，确保唯一性
3. `zap.SugaredLogger` 是线程安全的，可以共享
4. 下载和上传阶段是顺序执行的（不是并发的），因为上传需要下载完成后才能开始。所以不存在真正的并发访问
5. 但如果未来想优化为流式中转（边下载边上传），需要考虑并发安全性

**Detection:**
- 双远端传输偶尔出现 "broken pipe" 或 "connection reset" 错误
- 临时目录中出现损坏的文件

**Phase to address:**
双远端传输的数据层实现

**Confidence:** MEDIUM -- Go 的 `sftp.Client` 并发安全性取决于具体使用方式

---

## Low Pitfalls

### Pitfall 12: 本地路径历史的路径有效性 -- 已删除的目录仍在历史中

**What goes wrong:**
用户上传了 `/home/user/project/build` 目录。该目录后来被删除了。但本地路径历史中仍然保留了这个路径。用户在弹出列表中选择这个路径，本地面板导航到该路径，显示空目录或错误。

**Why it happens:**
`PathHistory.Record()` 只记录路径，不验证路径是否仍然存在。这是正确的行为（历史记录不应该因为路径被删除而自动移除），但在用户选择已删除路径时需要优雅处理。

**Prevention:**
1. 用户选择历史路径后，先检查路径是否存在（`os.Stat(path)`）
2. 如果路径不存在，在状态栏显示警告："Directory no longer exists: /path"，但不从历史中移除
3. 如果路径存在但不可访问（权限问题），显示相应的错误信息
4. 参考现有 `RecentDirs` 的行为：它也是直接导航到路径，不预先检查

**Detection:**
- 选择历史路径后本地面板显示空目录或 "permission denied"

**Phase to address:**
本地路径历史的 UI 层实现

**Confidence:** HIGH -- 路径有效性是历史记录功能的常见问题

---

### Pitfall 13: Dup SSH 连接的 SSH config 注入 -- 复制包含 Match/Include 的配置块

**What goes wrong:**
用户的 SSH config 中有一个服务器配置使用了 `Match` 块或引用了 `Include` 指令。Dup 操作复制了这个配置块，但 `Match` 条件引用的是原服务器的特征（例如 `Match host old-server`），复制后的新服务器不匹配这些条件，导致配置不生效。

**Why it happens:**
SSH config 的 `Host` 块可能包含复杂的指令，包括 `Match`（在文件末尾，不是在 Host 块内）、`Include`（引用外部文件）、`ProxyJump`（引用其他 Host 别名）。Dup 操作只复制 `Host` 块内的指令，不处理这些引用关系。

查看现有的 `createHostFromServer` 和 `updateHostNodes` 方法，它们只处理 `Server` 实体上的字段，不涉及 `Match` 块。

**Prevention:**
1. Dup 操作只复制 `Host` 块内的字段（`Host`, `HostName`, `User`, `Port`, `IdentityFile` 等），不涉及 `Match` 块
2. 复制后检查 `ProxyJump` 字段：如果引用了其他 Host 别名，提示用户可能需要更新
3. `Include` 指令通常在文件级别，不在 Host 块内，Dup 不会影响
4. 大多数情况下 Dup 操作是安全的，因为 `Host` 块是自包含的

**Detection:**
- Dup 后 SSH 连接失败（因为 `ProxyJump` 引用了不存在的别名）
- SSH config 语法错误（因为复制引入了不完整的配置）

**Phase to address:**
Dup SSH 连接的实现

**Confidence:** MEDIUM -- 取决于用户 SSH config 的复杂度

---

## Technical Debt Patterns

| Shortcut | Description | Long-term Cost | When Acceptable |
|----------|-------------|----------------|-----------------|
| 双远端传输用 download+upload 两阶段实现 | 实现简单，复用现有 TransferService | 需要本地临时磁盘空间，传输速度受限于较慢的链路 | Now -- 流式中转（边下载边上传）需要更复杂的协调逻辑 |
| 本地路径历史用全局单一文件存储 | 不需要按服务器分文件 | 路径数量可能增长过大，加载/保存性能下降 | Now -- 10-20 条 MRU 不会造成性能问题 |
| Dup 操作不处理 ProxyJump 引用 | 减少实现复杂度 | 复制使用 ProxyJump 的服务器后连接可能失败 | Now -- 大多数用户的 ProxyJump 引用在 Dup 后仍然有效 |
| 双远端传输复用 TransferModal | 减少 UI 组件数量 | TransferModal 的两阶段模型可能无法完美适配四阶段信息 | Now -- 如果发现不够用再创建 RelayTransferModal |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| PathHistory + FileBrowser | 在 `initiateTransfer` 中直接操作路径历史 | 通过回调或事件通知路径历史记录新路径 |
| Dup + ServerForm | 直接复制 Server struct 并调用 AddServer | 清除 metadata 字段，修改 Alias 后再 AddServer |
| Dup + handleGlobalKeys | 将 `d` 键改为 Dup 功能 | 使用不同的键（`y` 或 `D`），保留 `d` 为删除 |
| RelayTransfer + TransferModal | 复用 TransferModal 的两阶段进度显示 | 扩展 TransferModal 或创建 RelayTransferModal，显示源/目标服务器标签 |
| RelayTransfer + SFTPClient | 复用 FileBrowser 的 sftpService 连接 | 创建独立的 SFTPClient 实例，传输完成后关闭 |
| RelayTransfer + context.Cancel | 只取消一个 SFTP 连接 | 使用同一个 ctx 取消两个连接，确保对称清理 |
| 本地路径历史 + RecentDirs | 复用 RecentDirs 组件处理本地路径 | 抽取 PathHistory 数据层，RecentDirs 只处理远程路径 |

---

## "Looks Done But Isn't" Checklist

- [ ] **本地路径持久化**: 关闭 lazyssh 后重新打开，路径历史仍然存在 -- verify: 记录路径 -> 退出 -> 重新启动 -> 检查 JSON 文件
- [ ] **本地路径弹出列表**: 按 `l` 键（或设计的快捷键）弹出本地路径历史列表 -- verify: 记录至少 2 条路径，弹出列表，选择路径导航
- [ ] **本地路径记录时机**: 上传和下载都记录本地路径 -- verify: 上传文件后检查本地路径是否被记录，下载文件后检查
- [ ] **Dup 快捷键不冲突**: Dup 快捷键不与删除冲突 -- verify: 在服务器列表按 `d` 仍然删除，按 Dup 键复制
- [ ] **Dup Alias 唯一**: 连续 Dup 同一服务器不产生别名冲突 -- verify: 对同一服务器执行 3 次 Dup
- [ ] **Dup 不继承 metadata**: 复制后的服务器不被置顶、SSH 计数为 0 -- verify: 复制一个置顶的服务器，检查新条目
- [ ] **Dup 后可编辑**: 复制后可以编辑新服务器的配置 -- verify: Dup 后按 `e` 编辑
- [ ] **双远端连接建立**: 两台服务器的 SFTP 连接都能成功建立 -- verify: 选择两台不同服务器，连接都显示 "Connected"
- [ ] **双远端文件传输**: 从服务器 A 下载到本地再上传到服务器 B 能成功 -- verify: 传输一个小文件
- [ ] **双远端进度显示**: 进度显示包含源/目标服务器信息 -- verify: 观察传输进度 UI
- [ ] **双远端取消**: 传输过程中可以取消 -- verify: 开始传输大文件，按 Esc 取消
- [ ] **双远端临时文件清理**: 传输完成后临时文件被清理 -- verify: 检查 `os.TempDir()` 无残留
- [ ] **双远端连接清理**: 传输完成后两个 SFTP 连接都被关闭 -- verify: 检查无残留 `ssh` 进程
- [ ] **同服务器选择拒绝**: 双远端传输不允许选择同一台服务器 -- verify: 尝试选择同一台服务器作为源和目标
- [ ] **跨平台路径历史**: Windows/macOS/Linux 上路径历史正常工作 -- verify: 在不同平台上记录和显示路径

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| P1: 路径历史模型不一致 | MEDIUM | 早期确定数据模型，重构成本取决于代码量 |
| P2: `d` 键冲突 | LOW | 选择不同快捷键，零代码修改 |
| P3: 单连接架构限制 | HIGH | 创建独立的 RelayTransferService，不修改现有代码 |
| P4: 磁盘空间耗尽 | LOW | 传输前检查空间，失败时 defer 清理临时文件 |
| P5: 密码认证阻塞 | MEDIUM | 设置连接超时，确保进程清理 |
| P6: metadata 继承 | LOW | Dup 时清除非配置类字段 |
| P7: 进度重置困惑 | LOW | 在 UI 中标注阶段信息 |
| P8: 路径规范化 | LOW | 使用 `filepath.Clean()`，不区分大小写比较 |
| P9: 别名冲突 | LOW | 实现递增后缀策略 |
| P10: 入口 UX | MEDIUM | 两步选择模式，清晰的状态栏提示 |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| P1: 路径历史模型 | 本地路径历史数据层 | 路径历史独立于 RecentDirs，有独立存储文件 |
| P2: `d` 键冲突 | Dup SSH 连接 UI 层 | Dup 使用非 `d` 快捷键 |
| P3: 单连接架构 | 双远端传输数据层 | RelayTransferService 使用独立 SFTPClient 实例 |
| P4: 磁盘空间 | 双远端传输数据层 | 传输前检查空间，失败时清理临时文件 |
| P5: 密码认证阻塞 | 双远端传输连接管理 | 连接有超时，失败时清理进程 |
| P6: metadata 继承 | Dup SSH 连接实现 | 复制后 PinnedAt=0, SSHCount=0 |
| P7: 进度重置 | 双远端传输 UI 层 | 进度显示包含阶段信息 |
| P8: 路径规范化 | 本地路径历史数据层 | `filepath.Clean()` 处理，跨平台测试 |
| P9: 别名冲突 | Dup SSH 连接实现 | 连续 Dup 不产生别名冲突 |
| P10: 入口 UX | 双远端传输 UI 入口 | 两步选择模式，状态栏提示清晰 |
| P11: 并发问题 | 双远端传输数据层 | 两个独立 SFTPClient 实例 |
| P12: 路径有效性 | 本地路径历史 UI 层 | 选择已删除路径时显示警告 |
| P13: SSH config 注入 | Dup SSH 连接实现 | 复制后 SSH 连接正常 |

---

## Sources

- 项目源码深度审查 -- HIGH confidence: `RecentDirs`, `TransferService`, `SFTPClient`, `FileBrowser`, `ServerService`, `Repository`, `metadataManager`, `handlers.go`
- PROJECT.md Key Decisions -- HIGH confidence: 记录粒度决策、快捷键设计决策
- `pkg/sftp` 文档 -- HIGH confidence: SFTP 协议限制、客户端并发安全性
- Go `os/exec` 文档 -- HIGH confidence: `cmd.Start()` + `cmd.StdinPipe()` 的密码认证行为
- Go `filepath` 文档 -- HIGH confidence: `filepath.Clean()`, `filepath.Equal()` 跨平台行为
- v1.0-v1.2 实际踩坑经验 -- HIGH confidence: overlay 绘制链、goroutine + QueueUpdateDraw、快捷键冲突

---
*Pitfalls research for: lazyssh v1.3 Enhanced File Browser*
*Researched: 2026-04-15*
