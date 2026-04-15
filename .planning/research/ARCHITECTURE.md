# Architecture Research: v1.4 Dual-Remote File Transfer

**Domain:** TUI SSH Manager -- Dual-Remote File Transfer
**Researched:** 2026-04-15
**Overall confidence:** HIGH (based on direct code analysis of all relevant files: ports, adapters, UI components, transfer service, SFTP client)

## Executive Summary

v1.4 的双远端文件互传需要在服务器列表层添加 T 键标记机制，然后打开一个全新的 `DualRemoteFileBrowser` 组件，左右两栏各连接一台远端服务器。传输通过本地中转实现：从源服务器下载到临时文件/目录，再上传到目标服务器。这个方案复用了现有 `CopyRemoteFile`/`CopyRemoteDir` 的两阶段模式（download-to-temp + re-upload），但不复用现有 `TransferService`，因为它硬编码了单个 `SFTPService` 引用。

核心架构决策：(1) T 键标记状态存储在 TUI 层（非 `ServerList`），因为需要跨组件访问；(2) `DualRemoteFileBrowser` 是独立于 `FileBrowser` 的新组件，避免条件分支污染；(3) `RelayTransferService` 是新端口+适配器，内部持有两个 `SFTPService` 引用；(4) 进度显示复用 `TransferModal`，通过新的 `ShowRelay()` 方法初始化，两阶段各显示一次进度条（与 `CopyRemoteFile` 现有行为一致）。

## 1. T 键标记机制：服务器列表集成

### 1.1 标记状态存储

标记状态必须存储在 TUI struct 上，而非 `ServerList` 组件内部。原因：`ServerList` 是纯展示组件（`tview.List`），不应持有应用级交互状态。T 键处理逻辑在 `handleGlobalKeys` 中（`handlers.go`），而打开 `DualRemoteFileBrowser` 的逻辑也在 `handlers.go`，两者需要共享标记数据。

```go
// internal/adapters/ui/tui.go -- 新增字段

type tui struct {
    // ...existing fields...

    // Dual-remote transfer marking state
    markedServers   [2]domain.Server  // [0]=source, [1]=target
    markedCount     int               // 0, 1, or 2
    markRenderDirty bool              // triggers list re-render to show [T1]/[T2] prefixes
}
```

**为什么用固定大小数组 `[2]domain.Server` 而非 `[]domain.Server`：**
- 最多标记 2 台服务器，数组大小固定，无需动态分配
- 索引语义清晰：`markedServers[0]` 是源端，`markedServers[1]` 是目标端
- 避免 slice 越界检查

### 1.2 T 键处理流程

```
case 'T':
    handleMarkForDualRemote()

handleMarkForDualRemote():
  1. 获取当前选中服务器
  2. 如果该服务器已被标记 -> 取消标记（toggle），重置该位置
  3. 如果 markedCount < 2:
     - 将服务器存入 markedServers[markedCount]
     - markedCount++
  4. 标记完成后的 UI 反馈：
     - markedCount == 1: 状态栏显示 "[#A0FFA0]Marked 1/2: source = alias@host[-]"
     - markedCount == 2: 状态栏显示 "[#A0FFA0]Marked 2/2. Press T again to transfer, or Esc to clear.[-]"
  5. 如果 markedCount == 2 且用户再次按 T -> 打开 DualRemoteFileBrowser
```

**替代方案（被否决）：**
- 按 T 后弹出选择面板让用户指定"源端/目标端"：增加交互步骤，T 键标记两次更简洁
- 按一次 T 打开 browser，再选第二台服务器：需要在 browser 内嵌 server picker，复杂度高

### 1.3 Esc 清除标记

```
case tcell.KeyESC:
    if t.markedCount > 0 {
        t.markedCount = 0
        t.markedServers = [2]domain.Server{}
        t.markRenderDirty = true
        t.showStatusTemp("Marks cleared")
        return nil
    }
    // 原有 ESC 行为...
```

### 1.4 服务器列表标记可视化

需要在服务器列表项上显示标记前缀，类似现有剪贴板的 `[C]`/`[M]` 前缀。

**实现位置：** `formatServerLine()` 函数（`handlers.go` 中），根据 `t.markedServers` 检查当前服务器是否被标记。

```go
func formatServerLine(server domain.Server, markLabel string) (string, string) {
    // 现有格式化逻辑
    if markLabel != "" {
        primary = fmt.Sprintf("[%s] %s", markLabel, primary)
    }
    return primary, secondary
}
```

`UpdateServers()` 调用时需要传入标记信息。但 `ServerList.UpdateServers()` 只接受 `[]domain.Server`，不携带标记状态。

**解决方案：** 在 `tui` 层处理标记渲染，通过修改 `UpdateServers` 调用前后的处理逻辑：

方案 A（推荐）：在 `refreshServerList()` 中，对已标记的服务器修改 `primary` 文本前缀。

```go
func (t *tui) refreshServerList() {
    // ...existing logic...
    t.serverList.UpdateServers(filtered) // 先正常更新
    // 再更新标记前缀
    t.serverList.UpdateMarkLabels(t.markedServers[:t.markedCount])
}
```

方案 B：将标记状态传入 `ServerList`，让它内部渲染前缀。但这需要修改 `ServerList` 的 API。

**推荐方案 A**，因为 `ServerList` 保持纯展示职责，标记状态属于 TUI 层。

### 1.5 涉及文件

| 文件 | 变更 | 估计行数 |
|------|------|---------|
| `internal/adapters/ui/tui.go` | 新增 `markedServers`, `markedCount`, `markRenderDirty` 字段 | +5 |
| `internal/adapters/ui/handlers.go` | 新增 `handleMarkForDualRemote()`，`T` 键绑定，Esc 清除标记 | +60 |
| `internal/adapters/ui/server_list.go` | 新增 `UpdateMarkLabels()` 方法，支持行前缀更新 | +30 |

## 2. DualRemoteFileBrowser：新组件架构

### 2.1 为什么是独立组件而非 FileBrowser 的模式切换

现有 `FileBrowser` 有以下硬编码假设，无法通过模式切换干净地适配双远端：

1. **Pane 类型硬编码：** `localPane *LocalPane` + `remotePane *RemotePane`（file_browser.go:69-70）。双远端需要两个 `*RemotePane`。
2. **单 SFTP 连接：** `sftpService ports.SFTPService`（file_browser.go:66）。双远端需要两个独立连接。
3. **TransferService 绑定：** `transferSvc ports.TransferService`（file_browser.go:67），其内部持有单个 `SFTPService`。双远端需要不同的传输服务。
4. **Pane 索引语义：** `activePane == 0` 表示本地，`activePane == 1` 表示远程（贯穿所有 handler）。双远端中两个 pane 都是远程，索引语义不同。
5. **Clipboard 跨 pane 限制：** `handlePaste()` 明确拒绝跨 pane 粘贴（file_browser.go:971-973）。双远端的核心功能就是跨 pane 传输。
6. **RecentDirs 绑定单服务器：** `NewRecentDirs(fb.log, fb.server.Host, fb.server.User)`（file_browser.go:139）。双远端需要两个 RecentDirs 实例。

**结论：** 如果强行在 FileBrowser 中添加 `isDualRemote bool` 标志，几乎每个方法都需要 `if isDualRemote { ... } else { ... }` 分支。独立组件将复杂度隔离在单一文件中。

### 2.2 组件结构

```go
// internal/adapters/ui/file_browser/dual_remote_browser.go

type DualRemoteFileBrowser struct {
    *tview.Flex                          // root layout
    app            *tview.Application
    log            *zap.SugaredLogger
    sftpA          *sftp_client.SFTPClient  // Server A 连接
    sftpB          *sftp_client.SFTPClient  // Server B 连接
    relaySvc       *relay_transfer.RelayTransferService  // 中转传输服务
    leftPane       *RemotePane             // 连接到 sftpA (Server A)
    rightPane      *RemotePane             // 连接到 sftpB (Server B)
    statusBar      *tview.TextView
    transferModal  *TransferModal
    confirmDialog  *ConfirmDialog
    serverA        domain.Server
    serverB        domain.Server
    activePane     int                     // 0=left, 1=right
    transferring   bool
    transferCancel context.CancelFunc
    onClose        func()
}
```

### 2.3 构造函数与连接生命周期

```go
func NewDualRemoteFileBrowser(
    app *tview.Application,
    log *zap.SugaredLogger,
    serverA, serverB domain.Server,
    onClose func(),
) *DualRemoteFileBrowser {
    drfb := &DualRemoteFileBrowser{
        Flex:     tview.NewFlex(),
        app:      app,
        log:      log,
        serverA:  serverA,
        serverB:  serverB,
        onClose:  onClose,
    }

    // 创建两个独立 SFTP 连接
    drfb.sftpA = sftp_client.New(log)
    drfb.sftpB = sftp_client.New(log)

    // 创建中转传输服务
    drfb.relaySvc = relay_transfer.New(log, drfb.sftpA, drfb.sftpB, serverA, serverB)

    drfb.build()
    return drfb
}
```

**连接生命周期（与 FileBrowser.build() 一致的模式）：**

```
build():
  1. 创建 leftPane = NewRemotePane(log, sftpA, serverA)
  2. 创建 rightPane = NewRemotePane(log, sftpB, serverB)
  3. 两个 pane 都显示 "Connecting..." 状态
  4. 启动两个 goroutine 并行连接：
     go connectAndShow(sftpA, leftPane, serverA)
     go connectAndShow(sftpB, rightPane, serverB)
  5. 每个 goroutine 连接完成后通过 QueueUpdateDraw 更新 pane 状态
```

**并行连接 vs 串行连接：**
- 推荐**并行**。两台服务器相互独立，并行连接缩短用户等待时间。
- 现有 FileBrowser 是串行连接（单个 goroutine），因为只有一个连接。
- 并行连接没有额外风险：每个 SFTPClient 有独立的 mutex、独立的 ssh 进程。

### 2.4 布局

```
DualRemoteFileBrowser (*tview.Flex, FlexRow)
  ├── content (*tview.Flex, FlexColumn)
  │   ├── leftPane  (*RemotePane) -- 50% width, initially focused
  │   └── rightPane (*RemotePane) -- 50% width
  └── statusBar (*tview.TextView) -- 1 row height
```

布局与 FileBrowser 完全一致（50:50 FlexColumn + 1-row StatusBar），但两个 pane 都是 `*RemotePane`。

### 2.5 键路由

```
DualRemoteFileBrowser.handleGlobalKeys(event):
  1. Overlay 拦截（transferModal > confirmDialog）
  2. Tab -> switchFocus()
  3. Esc -> close()（关闭两个 SFTP 连接）
  4. F5 -> initiateRelayDirTransfer()
  5. Enter on file -> initiateRelayFileTransfer()
  6. c/x/p -> clipboard (同 pane 内复制/移动/粘贴)
  7. d/R/m -> delete/rename/mkdir（同 pane 内操作）
  8. s/S -> sort（作用于当前 pane）
  9. 传递到 focused pane 的 InputCapture（h/Space/./Backspace/j/k/arrows/Enter）
```

**关键差异 vs FileBrowser 键路由：**
- 没有 `r` 键弹出最近目录（双远端中两个 pane 的 RecentDirs 管理更复杂，v1.4 不实现）
- Enter on file 触发中转传输（而非 ignore，因为双远端没有本地文件"预览"概念）
- c/x/p 仍支持**同 pane 内**的复制/移动/粘贴（通过 `RelayTransferService` 或 SFTP 原生操作）

### 2.6 close() 方法

```go
func (drfb *DualRemoteFileBrowser) close() {
    drfb.app.SetAfterDrawFunc(nil)
    go func() {
        _ = drfb.sftpA.Close()
    }()
    go func() {
        _ = drfb.sftpB.Close()
    }()
    if drfb.onClose != nil {
        drfb.onClose()
    }
}
```

两个连接都在 goroutine 中关闭，与 FileBrowser.close() 模式一致（file_browser.go:135-143）。

### 2.7 涉及文件

| 文件 | 变更 | 估计行数 |
|------|------|---------|
| `internal/adapters/ui/file_browser/dual_remote_browser.go` | 新组件 | ~400 |
| `internal/adapters/ui/file_browser/dual_remote_handlers.go` | 键路由、传输启动 | ~300 |

## 3. RelayTransferService：中转传输服务

### 3.1 端口接口

```go
// internal/core/ports/relay_transfer.go

// RelayTransferService transfers files between two remote servers via local relay.
// The local machine downloads from source, then uploads to target.
// Temp files are managed internally and cleaned up after transfer.
type RelayTransferService interface {
    // RelayFile downloads a file from source remote and uploads to target remote.
    RelayFile(ctx context.Context, srcPath, dstPath string,
        onProgress func(domain.TransferProgress),
        onConflict domain.ConflictHandler) error

    // RelayDir downloads a directory from source remote and uploads to target remote.
    // Returns list of failed file paths (empty = all success).
    RelayDir(ctx context.Context, srcPath, dstPath string,
        onProgress func(domain.TransferProgress),
        onConflict domain.ConflictHandler) ([]string, error)
}
```

### 3.2 适配器实现

```go
// internal/adapters/data/transfer/relay_transfer_service.go

type relayTransferService struct {
    log      *zap.SugaredLogger
    srcSFTP  ports.SFTPService
    dstSFTP  ports.SFTPService
    srcLabel string  // "user@host" for progress display
    dstLabel string  // "user@host" for progress display
}
```

### 3.3 RelayFile 实现策略

直接复用现有 `CopyRemoteFile` 的模式（transfer_service.go:436-472），但使用两个不同的 SFTPService 实例：

```
Phase 1: Download from srcSFTP to temp file
  - srcSFTP.OpenRemoteFile(srcPath) -> io.ReadCloser
  - os.CreateTemp("", "lazyssh-relay-*") -> temp file
  - copyWithProgress(ctx, remoteReader, tempWriter, ..., onProgress)
  - onProgress label: "Downloading from {srcLabel}: filename"

Phase 2: Upload temp file to dstSFTP
  - dstSFTP.CreateRemoteFile(dstPath) -> io.WriteCloser
  - os.Open(tempPath) -> temp file reader
  - copyWithProgress(ctx, tempReader, remoteWriter, ..., onProgress)
  - onProgress label: "Uploading to {dstLabel}: filename"
  - onConflict handler for target file conflicts

Phase 3: Cleanup
  - os.Remove(tempPath)
  - defer 确保清理（与 CopyRemoteFile 的 Pitfall 3 模式一致）
```

**关键实现细节：**
- 复用 `transfer_service.go` 中的 `copyWithProgress()` 方法。但由于 `relayTransferService` 是独立 struct，需要复制或提取这个方法。
- **推荐：** 将 `copyWithProgress` 提取为包级函数（`internal/adapters/data/transfer/copy_progress.go`），两个 service 共享。
- 或者，`relayTransferService` 内部创建两个 `transferService` 实例（一个用 srcSFTP，一个用 dstSFTP），分别调用 `DownloadFile` 和 `UploadFile`。

**方案对比：**

| 方案 | 优点 | 缺点 |
|------|------|------|
| 创建两个 transferService 实例 | 零代码重复，完全复用现有逻辑 | 每个实例都持有一个 SFTPService + 文件系统操作，语义上有点奇怪 |
| 提取 copyWithProgress 为包级函数 | 干净的代码复用 | 需要重构现有代码 |
| relayTransferService 内部重新实现 | 完全独立，不影响现有代码 | 代码重复（~80行 copyWithProgress） |

**推荐方案 1：创建两个 transferService 实例。**

```go
func New(log *zap.SugaredLogger, srcSFTP, dstSFTP ports.SFTPService,
    srcServer, dstServer domain.Server) *relayTransferService {
    return &relayTransferService{
        log:      log,
        srcSFTP:  srcSFTP,
        dstSFTP:  dstSFTP,
        srcLabel: fmt.Sprintf("%s@%s", srcServer.User, srcServer.Host),
        dstLabel: fmt.Sprintf("%s@%s", dstServer.User, dstServer.Host),
    }
}

func (rs *relayTransferService) RelayFile(ctx context.Context,
    srcPath, dstPath string,
    onProgress func(domain.TransferProgress),
    onConflict domain.ConflictHandler) error {

    // 创建临时文件
    tmpFile, err := os.CreateTemp("", "lazyssh-relay-*")
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

**这个方案的优雅之处：**
- `transfer.New(log, sftpService)` 创建的 `transferService` 会自动使用传入的 `SFTPService` 做远端操作、使用 `os.*` 做本地操作
- 对于 `dlSvc`（下载服务）：`srcSFTP` 扮演远端角色，临时文件扮演本地角色
- 对于 `ulSvc`（上传服务）：`dstSFTP` 扮演远端角色，临时文件扮演本地角色
- 零代码重复，完全复用 `DownloadFile` 和 `UploadFile` 的所有逻辑（冲突处理、取消传播、错误清理）

**RelayDir 同理：**
```go
func (rs *relayTransferService) RelayDir(ctx context.Context,
    srcPath, dstPath string,
    onProgress func(domain.TransferProgress),
    onConflict domain.ConflictHandler) ([]string, error) {

    tmpDir, err := os.MkdirTemp("", "lazyssh-relaydir-*")
    defer func() { _ = os.RemoveAll(tmpDir) }()

    srcBase := filepath.Base(srcPath)
    tmpBase := filepath.Join(tmpDir, srcBase)

    // Phase 1: 从源服务器下载整个目录到临时目录
    dlSvc := transfer.New(rs.log, rs.srcSFTP)
    dlFailed, err := dlSvc.DownloadDir(ctx, srcPath, tmpBase, onProgress, nil)

    // Phase 2: 从临时目录上传到目标服务器
    ulSvc := transfer.New(rs.log, rs.dstSFTP)
    ulFailed, err := ulSvc.UploadDir(ctx, tmpBase, dstPath, onProgress, onConflict)

    // 合并失败列表
    allFailed := mergeFailed(dlFailed, ulFailed)
    return allFailed, err
}
```

### 3.4 涉及文件

| 文件 | 变更 | 估计行数 |
|------|------|---------|
| `internal/core/ports/relay_transfer.go` | 新端口接口 | ~20 |
| `internal/adapters/data/transfer/relay_transfer_service.go` | 新适配器 | ~120 |

## 4. TransferModal 适配

### 4.1 现有模式分析

`TransferModal` 已经有多种模式（transfer_modal.go）：
- `modeProgress` -- 单文件传输进度
- `modeCancelConfirm` -- 取消确认
- `modeConflictDialog` -- 冲突解决
- `modeSummary` -- 传输完成摘要
- `modeCopy` -- 远端复制（download + re-upload）
- `modeMove` -- 远端移动

### 4.2 新增模式：modeRelay

双远端传输与 `modeCopy`（远端复制）的进度显示几乎相同：都是两阶段（下载 -> 上传），都需要在阶段切换时重置进度条。

**推荐：复用 `modeCopy` 模式，不新增 `modeRelay`。**

理由：
1. `CopyRemoteFile` 的 UI 流程（download progress -> reset -> upload progress）与 `RelayFile` 完全一致
2. `ShowCopy(filename)` 的显示文本（"Copying: filename"）可以改为更通用的标签
3. 中转传输的 `handleRemotePaste()` 已经使用 `modeCopy` 模式（file_browser.go:1239-1288）
4. `fileLabel` 字段已经支持动态更新（如 "Downloading: filename" / "Uploading: filename"）

**需要的修改：**
- `ShowCopy()` 方法可以保持不变，或者新增 `ShowRelay(srcLabel, dstLabel, filename)` 方法提供更精确的标签
- 进度回调中的 `fileLabel` 更新逻辑（类似 `remotePasteFile` 和 `handleRemoteMove` 中的模式）

```go
// DualRemoteFileBrowser 中的传输启动
func (drfb *DualRemoteFileBrowser) initiateRelayFileTransfer() {
    // ...收集文件信息...

    fb.transferModal.SetDismissCallback(/* ... */)
    fb.transferModal.ShowCopy(fi.Name)  // 复用 ShowCopy

    go func() {
        var dlDone bool
        combinedProgress := func(p domain.TransferProgress) {
            if p.Done && !dlDone {
                dlDone = true
                drfb.app.QueueUpdateDraw(func() {
                    drfb.transferModal.ResetProgress()
                    drfb.transferModal.fileLabel = fmt.Sprintf("Uploading to %s: %s",
                        drfb.relaySvc.DstLabel(), fi.Name)
                })
                return
            }
            label := fmt.Sprintf("Downloading from %s: %s",
                drfb.relaySvc.SrcLabel(), fi.Name)
            drfb.app.QueueUpdateDraw(func() {
                drfb.transferModal.fileLabel = label
                drfb.transferModal.Update(p)
            })
        }

        err := drfb.relaySvc.RelayFile(ctx, srcPath, dstPath, combinedProgress, onConflict)
        // ...处理结果...
    }()
}
```

### 4.3 涉及文件

| 文件 | 变更 | 估计行数 |
|------|------|---------|
| `internal/adapters/ui/file_browser/transfer_modal.go` | 可能新增 `SrcLabel()`/`DstLabel()` 暴露方法 | +10 |

**或者不修改 TransferModal**：所有标签逻辑在 `DualRemoteFileBrowser` 的回调中处理（通过直接设置 `transferModal.fileLabel`），与现有 `remotePasteFile` 模式一致。

## 5. 同 Pane 内文件操作

双远端浏览器中，用户可能需要在同一台服务器上复制/移动/删除文件。这些操作需要通过各自的 SFTPService 执行。

### 5.1 同 Pane 复制/移动

复用现有 `CopyRemoteFile`/`CopyRemoteDir` 模式（单 SFTP 内的 download-to-temp + re-upload）：

```go
func (drfb *DualRemoteFileBrowser) handleRemotePaste() {
    // 类似 FileBrowser.handleRemotePaste()
    // 但使用 leftPane 或 rightPane 的 sftpService
    sftpSvc := drfb.getActiveSFTPService()
    // 创建 transfer.New(log, sftpSvc) 执行同服务器复制
}
```

### 5.2 同 Pane 删除/重命名/新建目录

直接使用 `SFTPService` 的 `Remove`/`RemoveAll`/`Rename`/`Mkdir` 方法，与 FileBrowser 中的 `handleDelete`/`handleRename`/`handleMkdir` 模式完全一致。

### 5.3 辅助方法

```go
// getActiveSFTPService 返回当前活跃 pane 对应的 SFTPService
func (drfb *DualRemoteFileBrowser) getActiveSFTPService() ports.SFTPService {
    if drfb.activePane == 0 {
        return drfb.sftpA
    }
    return drfb.sftpB
}
```

## 6. 数据流图

### 6.1 T 键标记 -> 打开双远端浏览器

```
Server List
  │ user presses 'T' on server-a
  ├─> handleMarkForDualRemote()
  │     markedServers[0] = server-a
  │     markedCount = 1
  │     status bar: "Marked 1/2: source = user@host-a"
  │
  │ user navigates to server-b, presses 'T'
  ├─> handleMarkForDualRemote()
  │     markedServers[1] = server-b
  │     markedCount = 2
  │     status bar: "Marked 2/2. Press T to transfer, Esc to clear."
  │
  │ user presses 'T' again (markedCount == 2)
  └─> handleOpenDualRemote()
        DualRemoteFileBrowser(serverA, serverB)
        app.SetRoot(dualBrowser, true)
```

### 6.2 中转文件传输

```
DualRemoteFileBrowser
  │ user selects file in leftPane, presses Enter
  ├─> initiateRelayFileTransfer()
  │     srcPath = leftPane.currentPath + "/" + filename
  │     dstPath = rightPane.currentPath + "/" + filename
  │     direction = "Relaying"
  │
  │     TransferModal.ShowCopy(filename)
  │
  │     goroutine:
  │     └─> relaySvc.RelayFile(ctx, srcPath, dstPath, onProgress, onConflict)
  │           │
  │           ├─ Phase 1: dlSvc.DownloadFile(ctx, srcPath, tmpPath, ...)
  │           │   srcSFTP.OpenRemoteFile(srcPath) -> reader
  │           │   os.Create(tmpPath) -> writer
  │           │   copyWithProgress(32KB buffer)
  │           │   onProgress -> QueueUpdateDraw -> TransferModal.Update()
  │           │
  │           ├─ Phase 2: ulSvc.UploadFile(ctx, tmpPath, dstPath, ...)
  │           │   os.Open(tmpPath) -> reader
  │           │   dstSFTP.CreateRemoteFile(dstPath) -> writer
  │           │   copyWithProgress(32KB buffer)
  │           │   onProgress -> QueueUpdateDraw -> TransferModal.Update()
  │           │
  │           └─ Cleanup: os.Remove(tmpPath)
  │
  └─> QueueUpdateDraw: refresh rightPane, hide TransferModal
```

## 7. 组件边界与职责

| 组件 | 职责 | 依赖 | 通信方式 |
|------|------|------|---------|
| `tui` (tui.go) | 持有标记状态，处理 T 键和打开 browser | ServerList, DualRemoteFileBrowser | handleMarkForDualRemote -> app.SetRoot |
| `ServerList` (server_list.go) | 展示服务器列表，支持标记前缀渲染 | tview.List | UpdateMarkLabels() 更新行文本 |
| `DualRemoteFileBrowser` | 双远端文件浏览器根组件 | RemotePane x2, TransferModal, ConfirmDialog, RelayTransferService | SetInputCapture 键路由 |
| `RelayTransferService` (port) | 中转传输端口接口 | TransferProgress, ConflictHandler | RelayFile/RelayDir 方法 |
| `relayTransferService` (adapter) | 中转传输实现 | SFTPService x2, transfer.New() x2 | 创建临时 service 实例 |
| `RemotePane` (复用) | 远端文件浏览，无修改 | SFTPService | OnFileAction 回调 |

## 8. 依赖注入链

### 8.1 现有 DI 链（不变）

```
cmd/main.go
  -> sftp_client.New(log) -> sftpService
  -> transfer.New(log, sftpService) -> transferService
  -> NewTUI(log, serverSvc, fileSvc, sftpService, transferService, ...)
```

### 8.2 双远端 DI 链（内部创建）

```
handleOpenDualRemote() [handlers.go]
  -> sftp_client.New(log) -> sftpA
  -> sftp_client.New(log) -> sftpB
  -> relay_transfer.New(log, sftpA, sftpB, serverA, serverB) -> relaySvc
  -> NewDualRemoteFileBrowser(app, log, serverA, serverB, onClose)
```

**关键决策：不在 main.go 中创建 RelayTransferService。**
- 服务器对在运行时由用户选择（T 键标记），编译时未知
- SFTPClient 实例需要在打开 browser 时创建（与特定服务器绑定）
- 内部创建保持 main.go 的 DI 链简洁

## 9. 反模式警告

### 9.1 不要在 FileBrowser 中添加 isDualRemote 标志

如前分析，FileBrowser 有 6+ 处硬编码假设。添加模式标志会导致每个方法都需要条件分支，代码可读性和可维护性急剧下降。

### 9.2 不要共享 SFTPClient 实例

每个 SFTPClient 管理一个独立的 `exec.Cmd("ssh")` 进程。共享实例会导致：
- 并发操作时的 mutex 争用
- `Close()` 调用杀死共享进程
- 连接状态不一致

### 9.3 不要尝试直连传输（跳过本地中转）

SFTP 协议不支持服务器到服务器的直连传输。`scp -3` 可以做到但无法提供进度反馈。本地中转（download + upload）是唯一可行方案。

### 9.4 不要在 close() 中阻塞等待 SFTP 连接关闭

`SFTPClient.Close()` 会 Kill SSH 进程并 Wait。必须在 goroutine 中执行，否则会阻塞 UI 线程。现有 FileBrowser.close() 已遵循此模式。

### 9.5 不要让标记状态泄漏到 FileBrowser

`markedServers` 只存在于 TUI 层。打开 `DualRemoteFileBrowser` 后，标记状态应该清除（因为已经消费了）。返回主界面时，`markedServers` 应该是空的。

## 10. 构建顺序

### Phase 1: T 键标记机制（服务器列表层）
**估计：** ~100 行，1 个 plan

依赖：无
交付物：
- `tui.go` 新增标记状态字段
- `handlers.go` 新增 `handleMarkForDualRemote()`，T 键绑定，Esc 清除
- `server_list.go` 新增 `UpdateMarkLabels()` 方法

### Phase 2: RelayTransferService（端口 + 适配器）
**估计：** ~140 行，1 个 plan

依赖：无（独立于 Phase 1）
交付物：
- `ports/relay_transfer.go` 新端口接口
- `transfer/relay_transfer_service.go` 新适配器
- 单元测试

### Phase 3: DualRemoteFileBrowser 组件
**估计：** ~700 行，2-3 个 plan

依赖：Phase 1（T 键标记），Phase 2（RelayTransferService）
交付物：
- `dual_remote_browser.go` 根组件（build, layout, close, Draw）
- `dual_remote_handlers.go` 键路由（handleGlobalKeys, switchFocus, initiateRelayTransfer）
- 同 pane 操作（handleDelete, handleRename, handleMkdir, handleCopy, handleMove, handlePaste）
- `handlers.go` 中 `handleOpenDualRemote()` 连接 Phase 1 和 Phase 3

### Phase 顺序理由

- Phase 1 和 Phase 2 可以并行开发（无依赖）
- Phase 3 依赖两者，必须最后构建
- Phase 1 最简单，可以快速验证 T 键 UX
- Phase 2 是纯数据层，可以独立单元测试
- Phase 3 是集成层，将 Phase 1 的 UI 入口和 Phase 2 的传输逻辑连接起来

## 11. 与现有模式的对应关系

| 双远端概念 | 对应的现有模式 | 源文件 |
|-----------|--------------|--------|
| 两个并行 SFTP 连接 | FileBrowser 的单个 SFTP 连接 | file_browser.go:243-255 |
| 中转传输（download + upload） | CopyRemoteFile 的两阶段模式 | transfer_service.go:436-472 |
| 进度显示（两阶段重置） | remotePasteFile 的 combinedProgress | file_browser.go:1352-1370 |
| 临时文件管理 | CopyRemoteFile 的 defer os.Remove | transfer_service.go:449 |
| 冲突处理 channel 同步 | buildConflictHandler 模式 | file_browser.go:565-623 |
| Overlay draw chain | FileBrowser.Draw() | file_browser.go:262-278 |
| close() 中 goroutine 关闭连接 | FileBrowser.close() | file_browser.go:135-143 |
| 键路由层级（overlay > global > pane） | handleGlobalKeys 模式 | file_browser_handlers.go:33-114 |

## Sources

- **HIGH confidence (project source code):**
  - `internal/core/ports/file_service.go` -- SFTPService interface
  - `internal/core/ports/transfer.go` -- TransferService interface (RelayTransferService 参考模板)
  - `internal/adapters/data/transfer/transfer_service.go` -- copyWithProgress, CopyRemoteFile, DownloadFile, UploadFile 实现
  - `internal/adapters/data/sftp_client/sftp_client.go` -- SFTPClient 连接模型、独立实例模式
  - `internal/adapters/ui/file_browser/file_browser.go` -- FileBrowser 组件结构、build/close/Draw 模式
  - `internal/adapters/ui/file_browser/file_browser_handlers.go` -- 键路由、handlePaste/handleRemotePaste/handleRemoteMove
  - `internal/adapters/ui/file_browser/remote_pane.go` -- RemotePane 组件（双远端复用两个实例）
  - `internal/adapters/ui/file_browser/local_pane.go` -- LocalPane（双远端不使用，但作为对比参考）
  - `internal/adapters/ui/handlers.go` -- TUI 层键路由、handleFileBrowser() 模式
  - `internal/adapters/ui/server_list.go` -- ServerList 组件
  - `internal/adapters/ui/tui.go` -- TUI struct、DI 链
  - `.planning/PROJECT.md` -- v1.4 需求定义（T 键标记、双远端互传）

---
*Architecture research: 2026-04-15 (v1.4 Dual-Remote File Transfer)*
