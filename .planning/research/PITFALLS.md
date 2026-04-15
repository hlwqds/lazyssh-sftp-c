# Pitfalls Research: v1.4 Dual Remote File Transfer

**Domain:** 为现有 local+remote 文件浏览器添加 dual-remote 文件互传功能
**Researched:** 2026-04-15
**Confidence:** HIGH -- 基于现有代码逐行审查、v1.0-v1.3 踩坑经验、架构约束分析

## Executive Summary

v1.4 的核心挑战是在一个为「单远程 + 本地」设计的架构上叠加「双远程」能力。现有代码在多个层面做了硬编码假设：`activePane == 0` 就是本地、`activePane == 1` 就是远程、`TransferService` 持有单个 `sftp` 引用、`FileBrowser` 只有一个 `sftpService` 字段。这些假设在添加双远端时会导致从编译错误到运行时数据损坏的各类问题。

本研究聚焦于 **集成陷阱**（integration pitfalls），即「代码能编译通过但运行时出错」的隐蔽问题。v1.3 的 PITFALLS.md 已覆盖了架构层面的设计建议（P3-P5, P7, P10-P11），本研究不重复那些建议，而是深入到具体的代码路径和状态机交互。

---

## Critical Pitfalls

### Pitfall 1: `activePane` 二元假设导致双远端浏览器无法区分两个远程面板

**What goes wrong:**
现有 FileBrowser 中所有操作都基于 `activePane` 的二元判断：`0 = local, 1 = remote`。这个假设遍布在 15+ 个方法中：

```go
// file_browser.go line 307-311
func (fb *FileBrowser) currentPane() tview.Primitive {
    if fb.activePane == 0 {
        return fb.localPane
    }
    return fb.remotePane
}

// file_browser.go line 373-378
func (fb *FileBrowser) getFileService() ports.FileService {
    if fb.activePane == 0 {
        return fb.fileService
    }
    return fb.sftpService
}

// file_browser.go line 399-404
func (fb *FileBrowser) getCurrentPanePath() string {
    if fb.activePane == 0 {
        return fb.localPane.GetCurrentPath()
    }
    return fb.remotePane.GetCurrentPath()
}
```

如果双远端浏览器复用 `FileBrowser`，将 `localPane` 替换为第二个 `RemotePane`，那么 `getFileService()` 在 `activePane == 0` 时返回的是 `fb.fileService`（本地文件服务），而不是第二个远程的 SFTPService。这会导致 `handleDelete`、`handleRename`、`handleMkdir` 等所有文件管理操作在左面板上调用本地 API 而非远程 SFTP API。

**Why it happens:**
`activePane` 的二元语义是架构级的隐式契约。`FileBrowser` 的构造函数签名 `NewFileBrowser(..., fs ports.FileService, sftp ports.SFTPService, ...)` 将 `fileService` 和 `sftpService` 作为不同参数传入，然后在方法中通过 pane index 选择。双远端场景下两个面板都是远程的，但 `getFileService()` 仍然在 `activePane == 0` 时返回本地服务。

**Consequences:**
- 左面板的删除/重命名/新建目录操作调用本地文件系统 API 而非远程 SFTP
- 粘贴操作（handlePaste）在左面板执行本地复制而非远程复制
- `handleRemotePaste`/`handleRemoteMove` 永远不会被左面板触发（因为它们只在 `fb.activePane == 1` 时调用）
- 删除操作成功（删除了本地文件）但用户以为删除了远程文件

**Prevention:**
1. **不要直接复用 `FileBrowser`**，创建新的 `DualRemoteBrowser` 组件，或者将 `FileBrowser` 重构为泛型/接口驱动
2. 如果选择创建 `DualRemoteBrowser`，它的方法实现应与 `FileBrowser` 类似，但所有 pane 选择逻辑都返回远程 SFTP 服务：
   ```go
   func (drb *DualRemoteBrowser) getFileService() ports.FileService {
       if drb.activePane == 0 {
           return drb.sftpServiceA  // 两个都是远程
       }
       return drb.sftpServiceB
   }
   ```
3. 如果选择重构 `FileBrowser`，引入 `Pane` 接口抽象面板类型差异：
   ```go
   type Pane interface {
       tview.Primitive
       GetService() ports.FileService
       GetCurrentPath() string
       Refresh()
       // ...
   }
   ```
   然后将 `localPane`/`remotePane` 替换为 `panes [2]Pane`
4. 无论哪种方案，**所有 `if fb.activePane == 0` 的分支都必须被审查**

**Detection:**
- 在双远端浏览器中，左面板的删除操作删除了本地文件而非远程文件
- 粘贴操作在左面板执行了本地文件复制
- `handlePaste` 中的 `fb.activePane == 0` 分支进入了 `handleLocalPaste` 而非远程路径

**Phase to address:** 双远端浏览器 UI 组件设计（第一个 phase）

**Confidence:** HIGH -- 逐行审查确认了 15+ 个 `activePane` 二元判断点

---

### Pitfall 2: `handlePaste` 的跨面板粘贴保护阻止双远端互传

**What goes wrong:**
现有 `handlePaste` 有一个明确的保护机制（file_browser.go line 971-974）：

```go
// Guard: cross-pane paste not supported (v1.3+)
if fb.clipboard.SourcePane != fb.activePane {
    fb.showStatusError("Cross-pane paste not supported (v1.3+)")
    return
}
```

这个保护在当前设计下是正确的（跨 local/remote 粘贴通过 Enter 键触发传输，不是 c/p 机制）。但双远端浏览器的核心功能就是**跨面板粘贴**——用户在左面板（服务器 A）标记文件，切到右面板（服务器 B）粘贴。

如果双远端浏览器复用了 `handlePaste`，这个保护会阻止所有跨面板操作。

**Why it happens:**
`handlePaste` 的跨面板保护是为「local <-> remote」场景设计的。在那个场景下，跨面板文件传输通过 `initiateTransfer`（Enter 键）实现，`c/p` 只用于同面板内的文件复制/移动。双远端场景下，跨面板传输**必须**通过 `c/p` 机制实现（因为没有「本地」作为中转的语义）。

**Consequences:**
- 用户在左面板标记文件后切到右面板粘贴，看到 "Cross-pane paste not supported"
- 双远端文件互传功能完全不可用

**Prevention:**
1. 在 `DualRemoteBrowser` 中，**移除或修改跨面板保护**：
   ```go
   // 双远端浏览器允许跨面板粘贴（通过 relay transfer）
   // 不再检查 SourcePane != activePane
   ```
2. 跨面板粘贴的传输路径变为「download from source remote -> temp -> upload to target remote」，与 `CopyRemoteFile`/`CopyRemoteDir` 的模式一致，但使用两个不同的 SFTP 连接
3. 注意：同面板内的复制/移动仍走现有路径（`handleRemotePaste`/`handleRemoteMove`），只有跨面板粘贴才需要 relay 传输
4. 修改后的分发逻辑：
   ```go
   if fb.clipboard.SourcePane == fb.activePane {
       // 同面板操作：复用现有 handleRemotePaste/handleRemoteMove
   } else {
       // 跨面板操作：relay transfer
       fb.handleRelayPaste(...)
   }
   ```

**Detection:**
- 双远端浏览器中跨面板粘贴时出现 "Cross-pane paste not supported" 错误

**Phase to address:** 双远端浏览器的剪贴板和粘贴逻辑

**Confidence:** HIGH -- 代码中明确存在保护逻辑（file_browser.go:971-974）

---

### Pitfall 3: `Clipboard.SourcePane` 语义在双远端场景下冲突

**What goes wrong:**
`Clipboard` 结构体使用 `SourcePane int` 标记来源面板（file_browser.go line 52）：

```go
type Clipboard struct {
    Active     bool
    SourcePane int // 0 = local, 1 = remote
    FileInfo   domain.FileInfo
    SourceDir  string
    Operation  ClipboardOp
}
```

`SourcePane` 的值被用于：
1. 判断是否跨面板粘贴（`fb.clipboard.SourcePane != fb.activePane`）
2. `handlePaste` 中构建源路径：`fb.buildPath(fb.clipboard.SourcePane, fb.clipboard.SourceDir, ...)`
3. `clipboardProvider` 回调中检查 `[C]`/`[M]` 前缀高亮

问题在于 `buildPath` 根据 pane index 选择路径构建方式（file_browser.go line 1415-1420）：

```go
func (fb *FileBrowser) buildPath(paneIdx int, base, name string) string {
    if paneIdx == 0 {
        return filepath.Join(base, name)  // 本地路径（反斜杠在 Windows）
    }
    return joinPath(base, name)           // 远程路径（正斜杠）
}
```

如果双远端浏览器中 `SourcePane == 0`，`buildPath` 会用 `filepath.Join` 构建路径。在 Windows 上这会产生 `C:\Users\...` 格式的路径，但实际上这个路径应该用于远程服务器 A（使用正斜杠）。

**Why it happens:**
`SourcePane` 的值 0/1 与 `buildPath` 中的 `filepath.Join` vs `joinPath` 选择耦合。这种耦合在 local+remote 场景下是正确的（pane 0 永远是本地），但在双远端场景下 pane 0 是远程，应该使用 `joinPath`。

**Consequences:**
- Windows 上跨面板粘贴时，源路径被构建为 `filepath.Join("/home/user", "file.txt")`，在 Windows 上可能产生混合分隔符
- 如果两个远程服务器都是 Linux，路径分隔符虽然碰巧正确（`filepath.Join` 在 Linux 上用 `/`），但语义是错误的
- `clipboardProvider` 的高亮检查仍然工作（因为只比较文件名和目录），但路径构建是错误的

**Prevention:**
1. 在 `DualRemoteBrowser` 中，**重写 `buildPath`** 使其始终使用 `joinPath`（因为两个面板都是远程）：
   ```go
   func (drb *DualRemoteBrowser) buildPath(_ int, base, name string) string {
       return joinPath(base, name)  // 始终使用远程路径格式
   }
   ```
2. 或者，更彻底的方案：让 `Clipboard` 存储完整的源路径（而非依赖 pane index 重建），在 `handleCopy` 时直接计算并存储 `SourcePath string`
3. 如果选择重构，`Clipboard` 结构体增加 `IsRemote bool` 或 `PathJoinMode int` 字段

**Detection:**
- Windows 上双远端跨面板粘贴失败，路径格式不正确
- 跨面板粘贴时日志显示混合路径分隔符

**Phase to address:** 双远端浏览器的剪贴板设计

**Confidence:** HIGH -- `buildPath` 的分支逻辑已确认（file_browser.go:1415-1420）

---

### Pitfall 4: `buildConflictHandler` 使用 `fb.activePane` 判断冲突检查目标

**What goes wrong:**
`buildConflictHandler`（file_browser.go line 565-623）在检测文件冲突时，根据 `fb.activePane` 决定检查远程还是本地文件：

```go
if fb.activePane == 0 {
    // Upload: check remote file info
    if fi, err := fb.sftpService.Stat(joinPath(fb.remotePane.GetCurrentPath(), fileName)); err == nil {
        existingInfo = ...
    }
} else {
    // Download: check local file info
    if fi, err := os.Stat(filepath.Join(fb.localPane.GetCurrentPath(), fileName)); err == nil {
        existingInfo = ...
    }
}
```

在双远端浏览器中，冲突检查应该始终检查目标远程的 SFTP 服务。如果复用此方法，当 `activePane == 0`（左面板 = 服务器 A）时，冲突检查会使用 `fb.sftpService`（在现有架构中是唯一的 SFTP 连接），但双远端场景需要检查服务器 B 的文件。

同样，`nextAvailableName` 中的 `statFunc` 也有相同问题：

```go
if fb.activePane == 0 {
    newPath = nextAvailableName(joinPath(fb.remotePane.GetCurrentPath(), fileName), fb.sftpService.Stat)
} else {
    newPath = nextAvailableName(filepath.Join(fb.localPane.GetCurrentPath(), fileName), os.Stat)
}
```

**Why it happens:**
冲突处理逻辑硬编码了「pane 0 = 本地、pane 1 = 远程」的映射。双远端场景下，两个 pane 都是远程，冲突检查应该始终使用目标远程的 SFTP 服务。

**Consequences:**
- 跨面板粘贴时，冲突检查检查了错误的远程服务器
- 重命名冲突解决时，文件名检查用了本地 `os.Stat` 而非远程 SFTP
- 用户看不到冲突对话框（因为检查的目标文件不存在于错误的服务器上），导致文件被静默覆盖

**Prevention:**
1. 在 `DualRemoteBrowser` 中重写 `buildConflictHandler`，使其始终使用目标面板的 SFTP 服务：
   ```go
   func (drb *DualRemoteBrowser) buildConflictHandler(ctx context.Context) domain.ConflictHandler {
       return func(fileName string) (domain.ConflictAction, string) {
           // 始终检查 activePane（目标面板）的远程文件
           targetService := drb.getSFTPService(drb.activePane)
           // ...
       }
   }
   ```
2. `nextAvailableName` 的 `statFunc` 参数应始终传入目标远程的 `sftpService.Stat`
3. 不复用现有 `buildConflictHandler`——它的本地/远程分支逻辑太深，修改容易引入回归

**Detection:**
- 跨面板粘贴时目标文件被覆盖而不显示冲突对话框
- 重命名冲突解决生成了在本地已存在但远程不存在的文件名

**Phase to address:** 双远端浏览器的冲突处理逻辑

**Confidence:** HIGH -- `buildConflictHandler` 的分支逻辑已确认（file_browser.go:571-608）

---

### Pitfall 5: 双远端传输的临时文件生命周期 -- 取消/失败时的清理不对称

**What goes wrong:**
现有 `CopyRemoteFile`/`CopyRemoteDir` 使用 `defer` 清理临时文件（transfer_service.go line 449）：

```go
defer func() { _ = os.Remove(tmpPath) }()
```

双远端传输也是 "download -> upload" 两阶段，但有两个关键区别：
1. **download 阶段使用的是源服务器的 SFTP 连接**，upload 阶段使用的是目标服务器的 SFTP 连接
2. **取消可能发生在 download 中途**（本地临时文件只有部分数据），也可能发生在 **upload 中途**（本地临时文件完整，但目标远程只有部分数据）

现有的取消清理逻辑（D-04 模式）只清理「部分传输的远程文件」：
- `UploadFile` 取消后删除远程部分文件（transfer_service.go line 88-93）
- `DownloadFile` 取消后删除本地部分文件（transfer_service.go line 138-143）

但双远端传输的取消场景更复杂：

| 取消时机 | download 状态 | upload 状态 | 需要清理 |
|----------|--------------|-------------|---------|
| download 中途 | 本地临时文件部分 | 未开始 | 本地临时文件 |
| download 完成，upload 前 | 本地临时文件完整 | 未开始 | 本地临时文件 |
| upload 中途 | 本地临时文件完整 | 目标远程部分 | 本地临时文件 + 目标远程部分 |
| upload 完成 | 本地临时文件完整 | 目标远程完整 | 本地临时文件（defer 处理） |

如果直接复用 `CopyRemoteFile`，它内部的 `DownloadFile` + `UploadFile` 各自有自己的取消清理。但问题是：
- `DownloadFile` 失败时删除的是「本地临时文件」——这是正确的
- `UploadFile` 失败时删除的是「远程部分文件」——在双远端场景下，这个「远程」是目标服务器 B，不是源服务器 A

现有的 `CopyRemoteFile` 将 `ts.sftp`（单连接）同时用于 download 和 upload。双远端需要两个不同的连接，所以 `CopyRemoteFile` 不能直接复用。

**Why it happens:**
`CopyRemoteFile` 的实现假设 download 和 upload 使用同一个 SFTP 连接（同一台服务器）。双远端传输需要两个不同的 SFTP 连接（两台不同的服务器）。这不是简单的参数替换，而是需要一个新的传输编排函数。

**Consequences:**
- 如果强行复用 `CopyRemoteFile`，它内部调用的 `ts.DownloadFile` 和 `ts.UploadFile` 都使用同一个 `ts.sftp`，只能访问一台服务器
- 如果创建新的 relay 函数但没有正确处理三段式清理（源远程部分 + 本地临时 + 目标远程部分），可能留下残留文件
- 取消后目标服务器上残留部分文件，源服务器上的原始文件完好（这是安全的），但用户可能不知道目标上有残留

**Prevention:**
1. **创建新的 `RelayFile` 方法**（不建议修改现有 `CopyRemoteFile`）：
   ```go
   func (rs *relayService) RelayFile(ctx context.Context,
       srcSFTP ports.SFTPService, srcPath string,
       dstSFTP ports.SFTPService, dstPath string,
       onProgress func(RelayProgress), onConflict domain.ConflictHandler) error {

       tmpFile, _ := os.CreateTemp("", "lazyssh-relay-*")
       tmpPath := tmpFile.Name()
       _ = tmpFile.Close()
       defer func() { _ = os.Remove(tmpPath) }()

       // Phase 1: Download from source remote
       err := downloadViaSFTP(ctx, srcSFTP, srcPath, tmpPath, ...)
       if err != nil {
           return err  // defer 清理 tmpPath
       }

       // Phase 2: Upload to destination remote
       err = uploadViaSFTP(ctx, dstSFTP, tmpPath, dstPath, ...)
       if err != nil {
           // 清理目标远程部分文件
           _ = dstSFTP.Remove(dstPath)
           return err  // defer 清理 tmpPath
       }
       return nil  // defer 清理 tmpPath
   }
   ```
2. 关键：`defer` 保证 tmpPath 被清理。upload 失败时显式清理目标远程部分文件
3. 对于目录传输，`RelayDir` 使用 `defer func() { _ = os.RemoveAll(tmpDir) }()` 清理整个临时目录
4. 考虑在 upload 失败时也清理已创建的目标远程目录结构（`dstSFTP.RemoveAll(dstDir)`）

**Detection:**
- 取消传输后 `os.TempDir()` 下残留 `lazyssh-relay-*` 文件
- 目标服务器上残留部分上传的文件
- 连续传输同一文件时冲突对话框弹出（因为上次残留的文件还在）

**Phase to address:** 双远端传输的数据层实现

**Confidence:** HIGH -- 基于对 `CopyRemoteFile`/`CopyRemoteDir` 源码的逐行审查

---

### Pitfall 6: 双 SFTP 子进程的 `stderr` 输出竞争

**What goes wrong:**
现有 `SFTPClient.Connect()` 将 SSH 进程的 stderr 重定向到 `os.Stderr`（sftp_client.go line 78）：

```go
cmd.Stderr = os.Stderr
```

双远端传输同时运行两个 SSH 进程，两个进程的 stderr 都输出到同一个 `os.Stderr`。在终端 UI 模式下，`os.Stderr` 的输出会直接显示在终端上，与 tview 的渲染混合，导致：
- SSH 警告信息（"Warning: Permanently added..."）覆盖 tview 的 UI
- 两个进程的输出交错，无法区分来自哪个连接
- 用户看到不可预期的终端输出

**Why it happens:**
`os.Stderr` 是全局的，所有 SSH 进程共享。在单连接场景下这不是问题（只有一个 SSH 进程），但双连接场景下两个进程同时写 stderr。

**Consequences:**
- 终端 UI 被 SSH 警告信息污染
- 两个连接的调试信息交错
- 在某些终端模拟器上可能导致 UI 渲染错乱

**Prevention:**
1. 将 SSH 进程的 stderr 重定向到 `io.Discard` 或 logger：
   ```go
   cmd.Stderr = nil  // 丢弃 stderr
   // 或者
   var stderrBuf bytes.Buffer
   cmd.Stderr = &stderrBuf
   // 连接失败时记录 stderrBuf.String() 到日志
   ```
2. 如果需要保留 SSH 调试信息，写入日志文件而非 `os.Stderr`：
   ```go
   logFile, _ := os.CreateTemp("", "lazyssh-ssh-debug-*")
   cmd.Stderr = logFile
   ```
3. 连接失败时，将 stderr 内容包含在错误信息中（截断到合理长度）
4. 这个修改应该应用到**所有** `SFTPClient.Connect()` 调用，不仅仅是双远端场景

**Detection:**
- 终端上出现 SSH 警告信息与 tview UI 混合
- 双远端传输开始时终端出现不可预期的输出

**Phase to address:** SFTP 连接管理（可以是独立的小改动，也可以合并到双远端第一个 phase）

**Confidence:** HIGH -- `cmd.Stderr = os.Stderr` 已确认（sftp_client.go:78）

---

### Pitfall 7: `close()` 中 goroutine 异步关闭 SFTP 导致竞态

**What goes wrong:**
现有 `FileBrowser.close()` 在 goroutine 中异步关闭 SFTP 连接（file_browser.go line 137-139）：

```go
func (fb *FileBrowser) close() {
    fb.app.SetAfterDrawFunc(nil)
    go func() {
        _ = fb.sftpService.Close()
    }()
    if fb.onClose != nil {
        fb.onClose()
    }
}
```

双远端浏览器需要关闭**两个** SFTP 连接。如果两个关闭操作都在 goroutine 中异步执行，而 `onClose` 回调立即触发返回主界面，主界面可能立即打开一个新的文件浏览器（F 键），此时旧的 SSH 进程可能还没有完全退出。

更严重的是，`handleFileBrowser()`（handlers.go line 466-468）在打开新文件浏览器前会关闭现有 SFTP 连接：

```go
if t.sftpService.IsConnected() {
    _ = t.sftpService.Close()
}
```

但 `t.sftpService` 是 TUI 层持有的单例。双远端浏览器的两个 SFTP 连接是内部创建的，TUI 层不知道它们的存在。如果用户从双远端浏览器返回主界面，再按 F 键打开普通文件浏览器，`t.sftpService` 的关闭不会影响双远端浏览器的残留连接。

**Why it happens:**
双远端浏览器的 SFTP 连接生命周期与 `FileBrowser` 不同。现有 `FileBrowser` 的 SFTP 连接由 TUI 层注入（`t.sftpService`），双远端浏览器需要自行创建和管理两个连接。关闭时的清理路径不同。

**Consequences:**
- 从双远端浏览器返回后，两个 SSH 进程仍在运行
- 重复进出双远端浏览器会导致 SSH 进程泄漏
- 系统资源耗尽（文件描述符、进程数）

**Prevention:**
1. 双远端浏览器必须在 `close()` 中**同步**关闭两个 SFTP 连接（不用 goroutine）：
   ```go
   func (drb *DualRemoteBrowser) close() {
       drb.app.SetAfterDrawFunc(nil)
       _ = drb.sftpServiceA.Close()
       _ = drb.sftpServiceB.Close()
       if drb.onClose != nil {
           drb.onClose()
       }
   }
   ```
2. SFTP 关闭操作很快（发送 close 请求 + kill 进程），不需要异步
3. 如果担心关闭阻塞，使用 `context.WithTimeout` 限制关闭时间：
   ```go
   ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
   defer cancel()
   go func() {
       <-ctx.Done()
       if drb.sftpServiceA.IsConnected() {
           _ = drb.sftpServiceA.Close()
       }
   }()
   ```
4. 在 `DualRemoteBrowser` 上实现类似 `tview.Primitive` 的 `Draw` 方法，确保 `close()` 被正确调用

**Detection:**
- 从双远端浏览器返回后，`ps aux | grep ssh` 显示残留的 SSH 进程
- 重复进出双远端浏览器后进程数持续增长

**Phase to address:** 双远端浏览器的连接生命周期管理

**Confidence:** HIGH -- `close()` 的异步模式已确认（file_browser.go:137-139）

---

## High Pitfalls

### Pitfall 8: `handlePaste` 的 goroutine + `buildConflictHandler` channel 同步在跨面板场景下的死锁风险

**What goes wrong:**
`handlePaste` 在 goroutine 中执行所有粘贴逻辑（file_browser.go line 987）：

```go
go func() {
    // Conflict check
    if _, err := statFunc(targetPath); err == nil {
        ctx, cancel := context.WithCancel(context.Background())
        defer cancel()
        onConflict := fb.buildConflictHandler(ctx)
        action, newPath := onConflict(targetName)
        // ...
    }
    // Operation dispatch
    if fb.clipboard.Operation == OpMove { ... }
}()
```

`buildConflictHandler` 内部通过 `actionCh` channel 与 UI 线程同步（file_browser.go line 567-594）：

```go
func (fb *FileBrowser) buildConflictHandler(ctx context.Context) domain.ConflictHandler {
    return func(fileName string) (domain.ConflictAction, string) {
        actionCh := make(chan domain.ConflictAction, 1)
        fb.app.QueueUpdateDraw(func() {
            fb.transferModal.ShowConflict(fileName, existingInfo, actionCh)
        })
        var action domain.ConflictAction
        select {
        case action = <-actionCh:
        case <-ctx.Done():
            return domain.ConflictSkip, ""
        }
        // ...
    }
}
```

在双远端场景下，如果跨面板粘贴需要 relay 传输，而 relay 传输也需要显示 `TransferModal`（用于进度），这里可能出现冲突：
- `handlePaste` 的 goroutine 调用 `buildConflictHandler`，后者通过 `QueueUpdateDraw` 显示冲突对话框
- 如果冲突对话框显示在 `TransferModal` 上（复用现有模式），而 relay 传输也需要显示 `TransferModal`（用于进度）
- 两个 UI 操作可能竞争同一个 `TransferModal` 实例

**Why it happens:**
`handlePaste` 和 `handleRemotePaste` 使用不同的 UI 模式：`handlePaste` 使用 `ConfirmDialog` + `buildConflictHandler` channel，`handleRemotePaste` 使用 `TransferModal`。双远端的跨面板粘贴需要**同时**使用冲突检测和传输进度显示，但现有代码中这两者使用不同的 UI 组件。

**Consequences:**
- 冲突对话框和传输进度模态框竞争同一个屏幕空间
- `TransferModal` 的状态机（progress/cancelConfirm/conflictDialog/summary）可能被跨面板粘贴的错误时序破坏
- 用户在冲突对话框上操作时，后台的 relay 传输可能已经开始了

**Prevention:**
1. 双远端的跨面板粘贴应**复用 `handleRemotePaste` 的模式**（使用 TransferModal），而不是 `handlePaste` 的模式（使用 ConfirmDialog + channel）
2. 在 `TransferModal` 中集成冲突处理（TransferModal 已有 `ShowConflict` 方法和 `modalMode` 状态机），不需要 `buildConflictHandler` 的 channel 同步
3. 或者，更安全的方案：跨面板粘贴的完整流程（冲突检查 + relay 传输 + 进度显示）都通过 `TransferModal` 编排：
   ```
   跨面板粘贴流程：
   1. 检查目标是否存在（同步）
   2. 如果冲突，TransferModal.ShowConflict()（modalMode = conflictDialog）
   3. 用户选择后，TransferModal.Show("Relaying", fileName)（modalMode = progress）
   4. 执行 relay 传输
   5. 完成/取消后显示 summary
   ```

**Detection:**
- 跨面板粘贴时 TransferModal 和 ConfirmDialog 同时出现
- 冲突对话框的确认操作没有响应

**Phase to address:** 双远端浏览器的粘贴和传输编排

**Confidence:** HIGH -- `handlePaste` 的 goroutine 模式和 `buildConflictHandler` 的 channel 同步已确认

---

### Pitfall 9: T 键标记状态与 D 键 Dup 的交互 -- 标记后误触 Dup

**What goes wrong:**
v1.4 的双远端入口使用 T 键标记服务器（PROJECT.md: "T 键标记服务器（源端/目标端），标记两个后自动打开双远端文件浏览器"）。同时，D 键已绑定到 `handleServerDup`（handlers.go line 87-89）。

问题场景：
1. 用户选中服务器 A，按 T 标记（状态：标记了 1 个服务器）
2. 用户不小心按了 D（预期：Dup 服务器 A）
3. Dup 操作执行，生成 `server-A-copy`，打开编辑表单
4. 用户取消表单，回到服务器列表
5. **T 标记状态可能已丢失**（如果 Dup 打开了新表单，标记状态被清除）

或者反过来：
1. 用户选中服务器 A，按 T 标记
2. 用户导航到服务器 B，按 T 标记（自动打开双远端浏览器）
3. 用户按 Esc 返回服务器列表
4. **T 标记状态应该被清除**，但如果清除逻辑有 bug，下次按 T 会以为已经标记了一个服务器

**Why it happens:**
T 键标记是新的全局状态，与现有的服务器列表交互（Dup、编辑、删除、搜索）共享同一个 UI。任何导致服务器列表重建或焦点变化的事件都可能影响标记状态。

**Prevention:**
1. T 标记状态应存储在 TUI 层（`tui` struct），与 `dupPendingAlias` 同级：
   ```go
   type tui struct {
       // ...
       tMarkedServers []domain.Server  // T 键标记的服务器列表（最多 2 个）
   }
   ```
2. 在所有可能导致标记状态失效的操作中清除标记：
   - 搜索过滤变更（`handleSearchInput`）——标记的服务器可能不在过滤结果中
   - 刷新服务器列表（`handleRefreshBackground`）——标记的引用可能失效
   - 退出搜索栏（`blurSearchBar`）——安全起见清除
3. 在状态栏显示标记状态，让用户知道当前标记了几个服务器：
   ```
   T: 1/2 marked (server-a)  |  Select second server and press T
   ```
4. Esc 键清除标记（PROJECT.md 已列出此需求）
5. **在 Dup 操作前不清除 T 标记**——Dup 是独立操作，不影响标记状态。但如果 Dup 导致列表重建，需要重新验证标记的服务器是否仍在列表中

**Detection:**
- T 标记一个服务器后执行搜索，标记指示器消失但状态仍在
- T 标记两个服务器后执行刷新，自动打开了错误的双远端浏览器
- 按 Esc 返回后 T 标记状态残留

**Phase to address:** T 键标记功能（双远端入口的第一个 phase）

**Confidence:** HIGH -- T 键标记是新的全局状态，与现有交互的冲突是必然的

---

### Pitfall 10: 双远端浏览器中 `initiateTransfer`（Enter 键/F5）的语义变化

**What goes wrong:**
现有 `initiateTransfer` 和 `initiateDirTransfer` 通过 Enter 键和 F5 键触发，方向由 `activePane` 决定：
- `activePane == 0`：上传（本地 -> 远程）
- `activePane == 1`：下载（远程 -> 本地）

双远端浏览器没有本地面板。Enter 键在文件上按下时，现有逻辑会进入 `initiateTransfer`，但此时两个面板都是远程的。传输方向应该变为「从 activePane 的远程 -> 到另一个 pane 的远程」，即 relay 传输。

但 `initiateTransfer` 内部的传输逻辑（file_browser.go line 385-446）直接调用 `fb.transferSvc.UploadFile`/`DownloadFile`，这些方法操作的是 TUI 层注入的 `sftpService`（单连接），不是双远端浏览器的两个连接。

**Why it happens:**
`initiateTransfer` 假设传输始终涉及本地文件系统。它的路径构建使用 `filepath.Join`（本地路径）和 `joinPath`（远程路径），文件操作使用 `fb.transferSvc`（持有单个 SFTP 连接）。双远端传输需要完全不同的传输路径。

**Consequences:**
- Enter 键在双远端浏览器中触发了 `initiateTransfer`，但传输方向和目标不正确
- 传输尝试使用错误的 SFTP 连接
- 或者 Enter 键完全无效（如果 `initiateTransfer` 被重写为 relay 传输，但 c/p 机制也需要同样的 relay 传输）

**Prevention:**
1. **决定 Enter 键在双远端浏览器中的行为**：
   - 选项 A：Enter 键触发 relay 传输（从当前面板到对面面板），与 local+remote 的行为对称
   - 选项 B：Enter 键在双远端浏览器中不做任何事（因为跨面板传输通过 c/p 机制）
   - 选项 C：Enter 键在文件上不做任何事（文件不可执行），在目录上进入目录
2. 推荐**选项 B**：双远端浏览器的文件传输统一通过 c/p 机制。理由：
   - c/p 机制更灵活（可以选择复制或移动）
   - Enter 键触发传输是 local+remote 的特殊约定（因为那是最自然的操作方式）
   - 双远端场景下用户需要先选择操作类型（copy vs move），c/p 更合适
3. F5 键（目录传输）在双远端浏览器中可以触发整个目录的 relay 传输（如果需要的话）

**Detection:**
- Enter 键在双远端浏览器中触发错误的传输操作
- Enter 键在目录上执行了传输而非进入目录

**Phase to address:** 双远端浏览器的键盘绑定设计

**Confidence:** HIGH -- `initiateTransfer` 的 activePane 分支已确认

---

### Pitfall 11: `RecentDirs` 在双远端浏览器中的适用性

**What goes wrong:**
现有 `RecentDirs` 绑定到一个服务器（`RecentDirs(fb.log, fb.server.Host, fb.server.User)`），记录该服务器的远程目录历史。双远端浏览器有两台服务器，每台都需要独立的最近目录历史。

如果复用单个 `RecentDirs`，两台服务器的目录历史会混在一起。如果创建两个 `RecentDirs`，需要为每个面板分别触发 `r` 键弹出列表。

**Why it happens:**
`RecentDirs` 在构造时绑定了服务器标识（host + user），在 `Record()` 时通过 serverKey 隔离不同服务器的目录。双远端浏览器有两台服务器，需要两个独立的 `RecentDirs` 实例。

**Consequences:**
- 如果只有一个 `RecentDirs`，两台服务器的目录历史混合
- `r` 键弹出最近目录列表时，用户无法区分哪些路径属于哪台服务器
- 选择了一台服务器上的路径，但当前在另一台服务器的面板上，导航失败

**Prevention:**
1. 双远端浏览器创建两个 `RecentDirs` 实例：
   ```go
   drb.recentDirsA = NewRecentDirs(drb.log, serverA.Host, serverA.User)
   drb.recentDirsB = NewRecentDirs(drb.log, serverB.Host, serverB.User)
   ```
2. `r` 键的行为改为：在当前面板弹出该面板对应服务器的最近目录
3. 面板导航事件（`NavigateInto`、`NavigateToParent`）分别记录到对应的 `RecentDirs`
4. Draw 链中需要绘制正确的 `RecentDirs` overlay（当前面板的那个）

**Detection:**
- `r` 键弹出的目录列表包含不属于当前服务器的路径
- 选择路径后导航失败（路径在另一台服务器上不存在）

**Phase to address:** 双远端浏览器的 RecentDirs 集成

**Confidence:** HIGH -- `RecentDirs` 的 serverKey 绑定已确认

---

## Medium Pitfalls

### Pitfall 12: 双远端浏览器的状态栏信息不足

**What goes wrong:**
现有 `FileBrowser` 的状态栏显示连接信息（file_browser.go line 286-288）：

```go
func (fb *FileBrowser) updateStatusBarConnection(msg string) {
    fb.statusBar.SetText(msg + "  [white]Tab[-] Switch  ...")
}
```

连接信息只显示一个服务器（`[Connected: user@host]`）。双远端浏览器需要显示两个连接状态：
- 服务器 A 的连接状态
- 服务器 B 的连接状态
- 哪个面板对应哪台服务器

如果只显示一个连接状态，用户无法判断哪个面板已连接、哪个面板连接失败。

**Prevention:**
1. 状态栏左半部分显示面板 A 的连接信息，右半部分显示面板 B 的连接信息
2. 面板标题（`SetTitle`）应包含服务器标识（类似 `RemotePane.UpdateTitle` 的格式）
3. 连接失败时，对应面板显示错误信息（复用 `RemotePane.ShowError`）

**Phase to address:** 双远端浏览器的 UI 布局

**Confidence:** HIGH -- 状态栏信息不足是 UI 可用性问题

---

### Pitfall 13: 双远端传输中 `TransferModal` 的 `SetDismissCallback` 被多次覆盖

**What goes wrong:**
现有代码中，`initiateTransfer`、`initiateDirTransfer`、`handleRemotePaste`、`handleRemoteMove` 都会调用 `fb.transferModal.SetDismissCallback(...)` 设置关闭回调。每次调用都会**覆盖**之前的回调。

如果双远端传输的 relay 函数也调用 `SetDismissCallback`，而此时 `TransferModal` 正在显示冲突对话框（由 `buildConflictHandler` 触发），冲突对话框的关闭回调会被 relay 函数的回调覆盖。

**Why it happens:**
`SetDismissCallback` 是一个简单的 setter，没有回调链或优先级机制。后设置的回调覆盖先设置的。

**Prevention:**
1. 确保 relay 传输的完整流程中，`SetDismissCallback` 只在开始传输前调用一次
2. 如果需要冲突处理，不要使用 `buildConflictHandler` + `SetDismissCallback` 的组合，而是在 `TransferModal` 的状态机中统一处理
3. 或者引入回调链机制（但这会增加复杂度，不推荐 v1.4 实现）

**Phase to address:** 双远端传输的 TransferModal 集成

**Confidence:** MEDIUM -- 取决于双远端传输如何与 TransferModal 交互

---

### Pitfall 14: `T` 键与 `t` 键的大小写处理

**What goes wrong:**
PROJECT.md 使用大写 `T` 键标记服务器。现有 `handleGlobalKeys` 中小写 `t` 绑定到 `handleTagsEdit()`（handlers.go line 108-110）：

```go
case 't':
    t.handleTagsEdit()
    return nil
```

tview 的 `event.Rune()` 区分大小写，所以 `T` 和 `t` 是不同的键。但如果用户的终端配置了大小写不敏感，或者用户在 Caps Lock 开启时按键，可能触发错误的操作。

更微妙的问题：用户选中服务器后按 `T`（标记），但手指习惯性按了小写 `t`，触发了标签编辑而不是标记。

**Prevention:**
1. 确认 `T`（大写）和 `t`（小写）确实绑定到不同功能
2. 在状态栏提示中明确显示大写 `T`：`[white]T[-] Mark for relay transfer`
3. 考虑在 `T` 标记一个服务器后，状态栏提示 `Press T on another server to start relay`，引导用户
4. 不需要修改 `t` 键的绑定——标签编辑是已有功能，不应被影响

**Phase to address:** T 键标记功能的实现

**Confidence:** HIGH -- `t` 键绑定已确认（handlers.go:108-110）

---

### Pitfall 15: 双远端浏览器中 `r` 键（最近目录）的面板歧义

**What goes wrong:**
现有 `r` 键在文件浏览器中弹出远程面板的最近目录列表（file_browser_handlers.go line 100-105）：

```go
case 'r':
    if fb.activePane == 1 && fb.remotePane.IsConnected() {
        fb.recentDirs.SetCurrentPath(fb.remotePane.GetCurrentPath())
        fb.recentDirs.Show()
        return nil
    }
```

条件是 `fb.activePane == 1`（只有远程面板有效）。双远端浏览器中两个面板都是远程的，`r` 键应该在两个面板上都有效。但如果直接移除 `activePane == 1` 条件，需要确保弹出的是**当前面板对应服务器的**最近目录。

**Prevention:**
1. 双远端浏览器中 `r` 键在两个面板上都有效
2. 根据当前面板选择对应的 `RecentDirs` 实例（见 Pitfall 11）
3. 修改条件为 `if drb.panels[drb.activePane].IsConnected()`

**Phase to address:** 双远端浏览器的键盘绑定

**Confidence:** HIGH -- `r` 键条件已确认

---

## Low Pitfalls

### Pitfall 16: 双远端浏览器的 `Esc` 行为与 T 标记状态的交互

**What goes wrong:**
现有文件浏览器中，Esc 键首先清除剪贴板（如果活跃），然后关闭浏览器（file_browser_handlers.go line 56-76）：

```go
case tcell.KeyESC:
    if fb.clipboard.Active {
        fb.clipboard = Clipboard{}
        fb.refreshPane(fb.activePane)
        // ...
        return nil
    }
    fb.close()
    return nil
```

如果用户在服务器列表中按 T 标记了一个服务器，然后不小心按了 Esc（想取消标记），Esc 在服务器列表中的行为可能不是清除标记——它可能触发其他操作（如退出应用）。

**Prevention:**
1. 在服务器列表的 `handleGlobalKeys` 中，如果存在 T 标记状态，Esc 应首先清除标记
2. 只有在没有标记状态时，Esc 才执行默认行为（退出应用或无操作）
3. 这与文件浏览器中 Esc 先清除剪贴板的模式一致

**Phase to address:** T 键标记功能的实现

**Confidence:** HIGH -- Esc 键行为需要与标记状态协调

---

### Pitfall 17: 双远端传输的进度显示——速度计算的基准问题

**What goes wrong:**
现有进度条的速度计算基于 `copyWithProgress` 中的 `BytesDone` 和时间差（progress_bar.go）。双远端传输的两个阶段（download + upload）通常速度不同（因为两条网络链路的带宽不同）。

如果进度条在阶段切换时重置速度计算（`ResetProgress()`），用户看到速度从 0 开始重新计算。如果不重置，速度样本中混合了两个阶段的数据，显示的平均速度不准确。

**Prevention:**
1. 阶段切换时调用 `ResetProgress()` 重置速度样本（现有 `remotePasteFile` 已经这样做了）
2. 在进度标签中标注当前阶段（"Phase 1/2: Downloading from A"），让用户理解速度变化
3. 最终汇总中显示两个阶段的速度和总时间

**Phase to address:** 双远端传输的 UI 进度显示

**Confidence:** MEDIUM -- 进度显示的 UX 细节，不影响功能正确性

---

## Technical Debt Patterns

| Shortcut | Description | Long-term Cost | When Acceptable |
|----------|-------------|----------------|-----------------|
| 创建 DualRemoteBrowser 而非重构 FileBrowser | 避免修改现有稳定代码 | 两个浏览器组件的代码重复 | Now -- 重构 FileBrowser 的 activePane 二元假设会影响所有现有功能 |
| 双远端传输用 download+upload 两阶段 | 复用现有 TransferService 模式 | 需要本地临时磁盘空间 | Now -- 流式中转需要两个 SFTP 连接的协调，复杂度高 |
| 复用 TransferModal 的状态机 | 减少 UI 组件数量 | 状态机更复杂，需要区分 local+remote 和 dual-remote 模式 | Now -- 如果状态机过于复杂，创建 RelayTransferModal |
| `cmd.Stderr = os.Stderr` 不修改 | 避免改动现有 SFTPClient | 双连接时 stderr 输出竞争 | Now -- 可以作为独立小改动处理 |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| DualRemoteBrowser + Clipboard | 复用 handlePaste 的跨面板保护 | 移除保护，跨面板粘贴触发 relay 传输 |
| DualRemoteBrowser + buildConflictHandler | 冲突检查使用错误的 SFTP 服务 | 重写 buildConflictHandler，始终检查目标面板的 SFTP |
| DualRemoteBrowser + buildPath | pane 0 使用 filepath.Join | 重写 buildPath，两个面板都使用 joinPath |
| DualRemoteBrowser + initiateTransfer | Enter 键触发 upload/download | Enter 键在双远端浏览器中不触发传输（或触发 relay） |
| DualRemoteBrowser + RecentDirs | 只有一个 RecentDirs 实例 | 创建两个实例，分别绑定两台服务器 |
| DualRemoteBrowser + close() | 异步关闭 SFTP 连接 | 同步关闭两个 SFTP 连接 |
| DualRemoteBrowser + getFileService | pane 0 返回本地 FileService | 两个面板都返回对应的 SFTPService |
| T 标记 + handleGlobalKeys | T 标记状态与搜索/Dup/编辑冲突 | 标记状态变化时更新状态栏，Esc 清除标记 |
| T 标记 + ServerList | 标记的服务器被删除或编辑 | 删除/编辑时清除标记，刷新时验证标记有效性 |
| Relay 传输 + TransferModal | SetDismissCallback 被覆盖 | 确保回调只在传输开始前设置一次 |

---

## "Looks Done But Isn't" Checklist

- [ ] **双远端浏览器两个面板都能连接**: 两台服务器的 SFTP 连接都成功建立，各自显示文件列表
- [ ] **跨面板粘贴**: 在左面板标记文件，切到右面板粘贴，文件从服务器 A relay 到服务器 B
- [ ] **跨面板移动**: 在左面板标记文件（x 键），切到右面板粘贴，文件从服务器 A relay 到服务器 B 并删除源
- [ ] **同面板操作**: 在同一面板内复制/移动文件仍然正常（复用 handleRemotePaste/handleRemoteMove）
- [ ] **冲突处理**: 跨面板粘贴时，如果目标文件已存在，显示冲突对话框
- [ ] **取消清理**: 传输过程中取消，本地临时文件被清理，目标远程部分文件被清理
- [ ] **进度显示**: 进度显示包含阶段信息（Phase 1/2: Downloading from A, Phase 2/2: Uploading to B）
- [ ] **Esc 清除标记**: 在服务器列表中按 Esc 清除 T 标记状态
- [ ] **双远端关闭**: 从双远端浏览器返回后，两个 SSH 进程都已退出
- [ ] **临时文件清理**: 传输完成后 os.TempDir() 下无 lazyssh-relay-* 残留
- [ ] **同服务器拒绝**: T 标记不允许选择同一台服务器作为源和目标
- [ ] **RecentDirs 分离**: r 键在左面板弹出服务器 A 的最近目录，在右面板弹出服务器 B 的
- [ ] **文件管理操作**: 在双远端浏览器中，删除/重命名/新建目录操作在正确的远程服务器上执行
- [ ] **r 键双面板可用**: r 键在两个面板上都有效（不仅仅是 activePane == 1）

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| P1: activePane 二元假设 | HIGH | 创建 DualRemoteBrowser 或重构 FileBrowser 为 Pane 接口驱动 |
| P2: 跨面板粘贴保护 | LOW | 移除保护条件，添加跨面板 relay 分支 |
| P3: Clipboard.SourcePane 语义 | LOW | 重写 buildPath 或让 Clipboard 存储完整路径 |
| P4: buildConflictHandler | MEDIUM | 重写冲突处理，始终使用目标面板的 SFTP 服务 |
| P5: 临时文件清理 | MEDIUM | 创建 RelayFile 方法，defer + 显式清理目标远程 |
| P6: stderr 竞争 | LOW | 重定向 cmd.Stderr 到 io.Discard 或日志 |
| P7: close() 竞态 | LOW | 同步关闭两个 SFTP 连接 |
| P8: goroutine 死锁 | MEDIUM | 统一使用 TransferModal 编排完整流程 |
| P9: T/D 键交互 | LOW | T 标记状态独立管理，与 Dup 操作互不影响 |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| P1: activePane 二元假设 | 双远端浏览器组件设计 | getFileService/buildPath 在两个面板都返回远程服务 |
| P2: 跨面板粘贴保护 | 剪贴板和粘贴逻辑 | 跨面板粘贴触发 relay 传输而非报错 |
| P3: SourcePane 语义 | 剪贴板设计 | buildPath 在两个面板都使用 joinPath |
| P4: buildConflictHandler | 冲突处理逻辑 | 冲突检查使用目标面板的 SFTP 服务 |
| P5: 临时文件清理 | 数据层实现 | 取消后无残留临时文件和远程部分文件 |
| P6: stderr 竞争 | SFTP 连接管理 | 双连接时终端无 SSH 警告信息 |
| P7: close() 竞态 | 连接生命周期 | 关闭后无残留 SSH 进程 |
| P8: goroutine 死锁 | 传输编排 | 冲突对话框和传输进度不竞争 |
| P9: T/D 键交互 | T 标记功能 | Dup 操作不影响 T 标记状态 |
| P10: Enter 键语义 | 键盘绑定设计 | Enter 键行为明确且一致 |
| P11: RecentDirs 分离 | RecentDirs 集成 | r 键弹出对应服务器的目录历史 |
| P12: 状态栏信息 | UI 布局 | 状态栏显示两个连接状态 |
| P13: SetDismissCallback 覆盖 | TransferModal 集成 | 回调设置时序正确 |
| P14: T/t 大小写 | T 标记功能 | T 标记和 t 标签编辑互不干扰 |
| P15: r 键面板歧义 | 键盘绑定 | r 键在两个面板上都有效 |
| P16: Esc + T 标记 | T 标记功能 | Esc 清除 T 标记状态 |
| P17: 速度计算基准 | 进度显示 | 阶段切换时重置速度样本 |

---

## Sources

- 项目源码逐行审查 -- HIGH confidence:
  - `internal/adapters/ui/file_browser/file_browser.go` -- FileBrowser 结构体、build()、initiateTransfer()、buildConflictHandler()、close()
  - `internal/adapters/ui/file_browser/file_browser_handlers.go` -- handleGlobalKeys()、handlePaste()、switchFocus()
  - `internal/adapters/data/sftp_client/sftp_client.go` -- Connect()、Close()、cmd.Stderr
  - `internal/adapters/data/transfer/transfer_service.go` -- CopyRemoteFile()、CopyRemoteDir()、取消清理逻辑
  - `internal/adapters/ui/file_browser/remote_pane.go` -- RemotePane 结构体、OnPathChange、NavigateTo
  - `internal/adapters/ui/handlers.go` -- handleGlobalKeys()、handleFileBrowser()、handleServerDup()
  - `internal/core/ports/transfer.go` -- TransferService 接口
  - `internal/core/ports/file_service.go` -- FileService/SFTPService 接口
- `.planning/PROJECT.md` -- HIGH confidence: v1.4 需求列表、Key Decisions
- `.planning/research/PITFALLS.md` (v1.3) -- HIGH confidence: P3/P4/P5/P7/P10/P11 的架构建议
- Go `os/exec` 文档 -- HIGH confidence: cmd.Stderr 行为、进程生命周期
- Go `os` 文档 -- HIGH confidence: os.CreateTemp、os.RemoveAll 行为

---
*Pitfalls research for: lazyssh v1.4 Dual Remote File Transfer*
*Researched: 2026-04-15*
