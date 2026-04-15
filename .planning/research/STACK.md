# Technology Stack

**Project:** LazySSH v1.4 — Dup Fix & Dual Remote Transfer
**Researched:** 2026-04-15
**Confidence:** HIGH (full codebase analysis, zero external dependency needed)

---

## Executive Summary

v1.4 有四个交付物：(1) Dup 修复（移除自动打开表单行为）、(2) T 键标记服务器、(3) 双远端文件浏览器、(4) 双远端之间文件复制/移动。经过对现有代码的逐行分析，**结论：零新外部依赖。** 所有技术需求均由现有技术栈覆盖。v1.3 的 STACK.md 已前瞻性地分析了双远端架构（`DualRemoteBrowser` + 两个独立 `SFTPClient`），v1.4 仅需将设计落地。

---

## Recommended Stack (v1.4 Additions)

### 核心发现：零新外部依赖

| Category | Before v1.4 | After v1.4 | Change |
|----------|-------------|------------|--------|
| External Go deps | pkg/sftp, tview, tcell/v2, cobra, zap, clipboard, ssh_config, runewidth, uniseg | **完全相同** | **零变更** |
| New Go stdlib packages | -- | **无新增** | 全部已在使用中 |
| New interfaces | FileService, SFTPService, TransferService | **无新增** | 零接口变更 |
| New adapter types | -- | `DualRemoteFileBrowser` (UI) | 内部类型 |
| New tui struct fields | -- | `markedServers`, `markedAliases` | 纯 UI 状态 |

---

## Feature 1: Dup 修复（移除自动打开表单）

### 技术方案

纯 UI 行为变更，零代码新增。移除 `handleServerDup()` 中打开 ServerForm 的代码，改为直接调用 `AddServer()` 后刷新列表。

**当前代码分析（`handlers.go:288-349`）：**

```go
// 当前行为：D 键 → handleServerDup() → NewServerForm(ServerFormAdd, &dup) → SetRoot(form)
// 问题：打开表单是多余步骤，用户期望直接出现在列表中
func (t *tui) handleServerDup() {
    // ... 生成 dup, 清除元数据 ...
    t.dupPendingAlias = dup.Alias
    // BUG: 这里不应该打开表单
    form := NewServerForm(ServerFormAdd, &dup).
        SetApp(t.app).SetVersionInfo(t.version, t.commit).
        OnSave(t.handleServerSave).OnCancel(t.handleFormCancel)
    t.app.SetRoot(form, true)
}
```

**修复方案：直接在 `handleServerDup()` 中调用 `AddServer()`**

```go
// 修复后：D 键 → handleServerDup() → AddServer() → refreshServerList() → 滚动到新条目
func (t *tui) handleServerDup() {
    // ... 生成 dup, 清除元数据（不变） ...
    if err := t.serverService.AddServer(dup); err != nil {
        t.showStatusTempColor(fmt.Sprintf("Dup failed: %v", err), "#FF6B6B")
        return
    }
    t.refreshServerList()
    // 滚动到新条目（复用现有 dupPendingAlias 逻辑）
    servers, _ := t.serverService.ListServers("")
    for i, s := range servers {
        if s.Alias == dup.Alias {
            t.serverList.SetCurrentItem(i)
            break
        }
    }
}
```

### 影响范围

| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `internal/adapters/ui/handlers.go` | **修改** | `handleServerDup()` — 移除 ServerForm 创建，直接 AddServer |
| `internal/adapters/ui/tui.go` | 可能移除 | `dupPendingAlias` 字段可能不再需要（但保留也无害） |

### 复用

- `generateUniqueAlias()` — 已存在（`handlers.go:44-63`），alias 冲突检测逻辑完全复用
- `t.serverService.AddServer()` — 已存在，ServerService 接口方法
- `t.refreshServerList()` — 已存在，列表刷新
- `t.serverList.SetCurrentItem()` — 已存在，滚动定位

---

## Feature 2: T 键标记服务器

### 技术方案

在 `tui` 结构体上添加两个 `domain.Server` 字段存储标记状态。T 键标记当前选中服务器为源端/目标端（第一次 T = 源端，第二次 T = 目标端），两个都标记后自动打开双远端文件浏览器。

### 数据结构

```go
// tui struct 新增字段
type tui struct {
    // ... 现有字段 ...
    markedSource  domain.Server // T 键第一次标记：源端服务器
    markedTarget  domain.Server // T 键第二次标记：目标端服务器
    hasMarkedSrc  bool           // 是否已标记源端
    hasMarkedTgt  bool           // 是否已标记目标端
}
```

**为什么不用 `[]domain.Server` slice：** 只需要恰好两个服务器，固定字段比 slice 更清晰，避免了空/超长 slice 的边界检查。

### 快捷键分配

| 键 | 当前占用 | v1.4 分配 |
|----|---------|----------|
| `t` | `handleTagsEdit()` (标签编辑) | **需要变更** |
| `T` | 未占用 | **不使用**（避免与 `t` 混淆） |

**关键发现：** `t` 键当前被标签编辑功能占用（`handlers.go:109`）。PROJECT.md 要求 T 键标记服务器。

**解决方案：将标签编辑移到 `T`（Shift+t），标记功能使用 `t`（小写）。**

理由：
- 标签编辑是低频操作，用户已经习惯在表单中编辑，Shift+t 不影响肌肉记忆
- 标记服务器是新功能的默认入口，应该使用更容易按到的键
- 或者，如果不想改变标签编辑的键位，可以改用其他键如 `b`（batch/dual）

**建议采用 PROJECT.md 的原始设计：`T`（大写）用于标记服务器，`t`（小写）保持标签编辑不变。**

### UI 反馈

标记状态需要在服务器列表中可见。两种方案：

**方案 A: 状态栏提示（推荐 — 最小改动）**

```
第一次 T: "Marked [serverA] as source. Mark another server with T."
第二次 T: "Opening dual remote browser: [serverA] ↔ [serverB]"
Esc:      "Marking cleared"
```

理由：
- ServerList 是 `tview.List`，不支持自定义行内标记颜色（不像 Table 的 `SetReference` + `populateTable` 模式）
- 改造 ServerList 为 Table 是大工程且超出 v1.4 范围
- 状态栏提示遵循现有的 `showStatusTemp` 模式

**方案 B: 列表项文本前缀（备选）**

在 `formatServerLine()` 中添加 `[S]`/`[T]` 前缀。需要修改 `UpdateServers()` 接受标记状态。

理由：更直观但需要侵入 ServerList 组件。

### 影响范围

| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `internal/adapters/ui/tui.go` | **修改** | 新增 `markedSource`, `markedTarget`, `hasMarkedSrc`, `hasMarkedTgt` 字段 |
| `internal/adapters/ui/handlers.go` | **修改** | 新增 `case 'T':` 路由到 `handleServerMark()`，Esc 清除标记 |
| `internal/adapters/ui/server_list.go` | 可能修改 | 如果采用方案 B（前缀标记），需修改 `formatServerLine` 或 `UpdateServers` |

### 复用

- `t.serverList.GetSelectedServer()` — 已存在，获取当前选中服务器
- `t.showStatusTemp()` — 已存在，状态栏反馈
- `t.showStatusTempColor()` — 已存在，彩色状态栏
- `t.app.QueueUpdateDraw()` — 已存在，UI 线程安全更新

---

## Feature 3: 双远端文件浏览器

### 架构设计

**核心思路：** 新建 `DualRemoteFileBrowser` 组件，复用 `RemotePane` 两次（左栏=源服务器，右栏=目标服务器），各自持有独立的 `SFTPService` 实例。

**为什么新建组件而不是改造 `FileBrowser`：**

| 因素 | 改造 FileBrowser | 新建 DualRemoteFileBrowser |
|------|-----------------|---------------------------|
| 本地面板 | 需要条件逻辑隐藏/替换 | 自然不含本地面板 |
| SFTP 连接数 | 从 1 变为 2，需修改构造函数 | 构造时就创建 2 个 |
| 传输方向 | 原来只有 upload/download | 新增 cross-remote（A→temp→B） |
| c/x/p 操作 | 原来有 cross-pane 限制 | 需要支持跨面板粘贴 |
| 进度标签 | "Uploading"/"Downloading" | 需新增 "Relaying" 标签 |
| 复杂度风险 | 高（大量条件分支） | 低（独立组件，清晰职责） |

**推荐：新建 `DualRemoteFileBrowser`。** 虽然 `RemotePane` 完全复用，但 `FileBrowser` 的传输编排逻辑（`initiateTransfer`、`handlePaste`、`handleRemoteMove` 等）是为 local+remote 设计的，强行改造会引入大量条件分支。

### 组件结构

```go
// DualRemoteFileBrowser 是双远端文件浏览器的根组件
// 布局: FlexRow(content(FlexColumn: RemotePane(left) + RemotePane(right)) + StatusBar)
type DualRemoteFileBrowser struct {
    *tview.Flex
    app            *tview.Application
    log            *zap.SugaredLogger

    // 两个独立的 SFTP 连接
    leftSFTP       ports.SFTPService  // 源端服务器
    rightSFTP      ports.SFTPService  // 目标端服务器

    // 两个远程面板（完全复用 RemotePane）
    leftPane       *RemotePane
    rightPane      *RemotePane

    // 服务器信息（用于面板标题和连接）
    leftServer     domain.Server
    rightServer    domain.Server

    // UI 组件
    statusBar      *tview.TextView
    transferModal  *TransferModal
    confirmDialog  *ConfirmDialog
    inputDialog    *InputDialog

    // 剪贴板（跨面板）
    clipboard      Clipboard
    activePane     int  // 0 = left, 1 = right
    transferring   bool
    transferCancel context.CancelFunc
    onClose        func()
}
```

### SFTP 连接管理

**关键设计：两个独立 SFTPClient 实例，不共享 `cmd/main.go` 的单例。**

当前架构：
```
cmd/main.go: sftpService := sftp_client.New(log)  // 单例
             transferService := transfer.New(log, sftpService)  // 绑定到单例
             tui := ui.NewTUI(log, ..., sftpService, transferService, ...)
```

双远端方案：
```
// 在 DualRemoteFileBrowser 构造时创建独立实例
leftSFTP := sftp_client.New(fb.log)
rightSFTP := sftp_client.New(fb.log)

// 各自独立连接
leftSFTP.Connect(leftServer)
rightSFTP.Connect(rightServer)
```

**理由：**
- `SFTPClient.Connect()` 每次调用创建独立的 SSH 子进程（`exec.Command`），天然支持多实例
- `SFTPClient` 内部有 `sync.Mutex` 保护，线程安全
- 不需要修改 `cmd/main.go` 的 DI 容器 -- `DualRemoteFileBrowser` 自管理连接生命周期
- 关闭时两个连接都需要清理（类似 `FileBrowser.close()` 的 goroutine 模式）

**与 FileBrowser 的 SFTP 交互：** 当用户从服务器列表按 F 键打开 FileBrowser 时，FileBrowser 使用 `tui.sftpService`（`cmd/main.go` 单例）。当按 T 键打开 DualRemoteFileBrowser 时，会先关闭 FileBrowser 的 SFTP 连接（如果有的话），然后创建两个新的。这在 `handleFileBrowser()` 中已有先例（`handlers.go:466-468`）。

### 传输编排（Cross-Remote Transfer）

**核心问题：** 当前 `TransferService` 绑定到一个 `SFTPService`，无法直接做跨服务器传输。

**方案：UI 层编排两阶段传输，复用两个 `TransferService` 实例。**

```go
func (drb *DualRemoteFileBrowser) initiateCrossRemoteTransfer() {
    // 1. 确定源和目标
    var srcSFTP, dstSFTP ports.SFTPService
    var srcPane, dstPane *RemotePane
    if drb.activePane == 0 {
        srcSFTP = drb.leftSFTP
        dstSFTP = drb.rightSFTP
        srcPane = drb.leftPane
        dstPane = drb.rightPane
    } else {
        srcSFTP = drb.rightSFTP
        dstSFTP = drb.leftSFTP
        srcPane = drb.rightPane
        dstPane = drb.leftPane
    }

    // 2. 创建临时文件
    tmpFile, _ := os.CreateTemp("", "lazyssh-dual-*")
    tmpPath := tmpFile.Name()
    tmpFile.Close()
    defer os.Remove(tmpPath)

    // 3. Phase 1: 下载到临时文件
    downloadSvc := transfer.New(drb.log, srcSFTP)
    downloadSvc.DownloadFile(ctx, remoteSrcPath, tmpPath, dlProgress, nil)

    // 4. Phase 2: 上传到目标服务器
    uploadSvc := transfer.New(drb.log, dstSFTP)
    uploadSvc.UploadFile(ctx, tmpPath, remoteDstPath, ulProgress, conflictHandler)
}
```

**这个模式已经被 `CopyRemoteFile`（`transfer_service.go:436-472`）验证过** -- 下载到临时文件再上传。区别仅在于下载和上传使用不同的 `SFTPService` 实例。

### 目录传输（Cross-Remote Dir）

```go
// 目录传输同样分两阶段
tmpDir, _ := os.MkdirTemp("", "lazyssh-dual-dir-*")
defer os.RemoveAll(tmpDir)

// Phase 1: 下载整个目录
downloadSvc := transfer.New(drb.log, srcSFTP)
downloadSvc.DownloadDir(ctx, remoteSrcDir, tmpDir, dlProgress, nil)

// Phase 2: 上传整个目录
uploadSvc := transfer.New(drb.log, dstSFTP)
uploadSvc.UploadDir(ctx, tmpDir, remoteDstDir, ulProgress, conflictHandler)
```

### 复制/移动（c/x/p）

双远端浏览器的 c/x/p 机制需要支持跨面板粘贴（与现有 FileBrowser 不同）。

**现有 FileBrowser 的限制（`file_browser.go:970-974`）：**
```go
// Guard: cross-pane paste not supported (v1.3+)
if fb.clipboard.SourcePane != fb.activePane {
    fb.showStatusError("Cross-pane paste not supported (v1.3+)")
    return
}
```

**双远端浏览器必须移除此限制** -- 跨面板粘贴是核心功能。

- **c 标记 + p 粘贴（复制）：** 从 A 下载到 temp → 上传到 B（同 initiateCrossRemoteTransfer）
- **x 标记 + p 粘贴（移动）：** 复制成功后删除源（同 `handleRemoteMove` 模式）

### 进度显示

`TransferModal` 已支持两阶段进度显示（`CopyRemoteFile` 模式）。双远端传输可以复用：

```
Phase 1: "Downloading: filename" (进度条从 0→100%)
Phase 2: "Uploading: filename"   (进度条重置，从 0→100%)
```

对于移动操作，增加第三阶段：
```
Phase 3: "Deleting source..." (无进度条，纯文本提示)
```

### Esc 处理

双远端浏览器的 Esc 需要处理两层：
1. 如果剪贴板有内容 → 清除剪贴板（同 FileBrowser）
2. 如果传输中 → 传输取消确认（同 FileBrowser）
3. 否则 → 关闭浏览器，清理两个 SFTP 连接

### 影响范围

| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `internal/adapters/ui/file_browser/dual_remote_browser.go` | **新增** | 双远端文件浏览器主组件 |
| `internal/adapters/ui/file_browser/dual_remote_handlers.go` | **新增** | 快捷键处理（Tab/Esc/c/x/p/d/R/m/s/S/F5） |
| `internal/adapters/ui/file_browser/dual_remote_transfer.go` | **新增** | 跨远端传输编排（两阶段下载+上传） |
| `internal/adapters/ui/file_browser/remote_pane.go` | **无变更** | 完全复用 |
| `internal/adapters/ui/handlers.go` | **修改** | T 键标记逻辑 + 打开双远端浏览器入口 |
| `internal/adapters/ui/tui.go` | **修改** | 新增标记状态字段 |

### 复用清单

| 组件 | 复用方式 | 来源文件 |
|------|---------|---------|
| `RemotePane` | 完全复用，实例化两次 | `remote_pane.go` |
| `TransferModal` | 完全复用 | `transfer_modal.go` |
| `ConfirmDialog` | 完全复用 | `confirm_dialog.go` |
| `InputDialog` | 完全复用 | `input_dialog.go` |
| `Clipboard` / `ClipboardOp` | 完全复用 | `file_browser.go:39-56` |
| `FileSortMode` | 完全复用 | `file_sort.go` |
| `formatSize` | 完全复用 | `local_pane.go:359-370` |
| `buildConflictHandler` 模式 | 复制并适配（SFTP 实例不同） | `file_browser.go:565-623` |
| `transfer.New(log, sftp)` | 创建两个实例 | `transfer/transfer_service.go:43` |
| `sftp_client.New(log)` | 创建两个实例 | `sftp_client/sftp_client.go:51` |

---

## Temp File Management

### 临时文件策略

双远端传输的临时文件管理遵循现有模式（`CopyRemoteFile` / `CopyRemoteDir`）：

| 操作 | 临时文件 | 清理方式 | 参考 |
|------|---------|---------|------|
| 单文件传输 | `os.CreateTemp("", "lazyssh-dual-*")` | `defer os.Remove(tmpPath)` | `transfer_service.go:443-449` |
| 目录传输 | `os.MkdirTemp("", "lazyssh-dual-dir-*")` | `defer os.RemoveAll(tmpDir)` | `transfer_service.go:483-487` |

**关键：** `defer` 确保即使传输失败或取消，临时文件也会被清理。这与现有 `CopyRemoteFile` 的 `defer func() { _ = os.Remove(tmpPath) }()` 模式完全一致。

### 磁盘空间考虑

双远端传输会在本机创建临时文件，磁盘占用等于传输文件大小。对于大文件传输，可能需要预检查磁盘空间。

**v1.4 建议：** 不做预检查，依赖传输失败时的错误提示（`os.Create` 返回 `ENOSPC`）。理由：
- 预检查增加复杂度且不可靠（并发写入可能改变可用空间）
- 传输失败已有完善的错误处理和部分文件清理机制
- 这不是 MVP 的关键路径

---

## Concurrent SFTP Connections

### 连接并发安全性

`SFTPClient` 内部使用 `sync.Mutex` 保护所有操作（`sftp_client.go:46`）。两个独立实例各自有独立的 mutex，不存在竞争。

```go
type SFTPClient struct {
    log     *zap.SugaredLogger
    client  *sftp.Client
    cmd     *exec.Cmd
    stdin   io.WriteCloser
    mu      sync.Mutex     // 每个 SFTPClient 独立的锁
    homeDir string
}
```

### 连接建立时序

两个 SFTP 连接应并发建立（goroutine），而非串行：

```go
go func() {
    err := leftSFTP.Connect(leftServer)
    drb.app.QueueUpdateDraw(func() {
        if err != nil {
            drb.leftPane.ShowError(err.Error())
        } else {
            drb.leftPane.ShowConnected()
        }
    })
}()

go func() {
    err := rightSFTP.Connect(rightServer)
    drb.app.QueueUpdateDraw(func() {
        if err != nil {
            drb.rightPane.ShowError(err.Error())
        } else {
            drb.rightPane.ShowConnected()
        }
    })
}()
```

这与现有 `FileBrowser.build()` 中的 SFTP 连接模式一致（`file_browser.go:243-255`）。

### 连接关闭时序

Esc 关闭时，两个连接应并发关闭：

```go
func (drb *DualRemoteFileBrowser) close() {
    drb.app.SetAfterDrawFunc(nil)
    go func() {
        _ = drb.leftSFTP.Close()
        _ = drb.rightSFTP.Close()
    }()
    if drb.onClose != nil {
        drb.onClose()
    }
}
```

---

## Dup 修复 + T 键的交互设计

### T 键与 D 键的共存

| 键 | 功能 | 条件 |
|----|------|------|
| `D` | 复制服务器（直接出现在列表） | 无标记状态 |
| `T` | 标记当前服务器 | 第一次=源端，第二次=目标端 |
| `Esc` | 清除标记 | 有标记状态时 |
| `Esc` | 退出应用 | 无标记状态时 |

### 标记后打开双远端浏览器的流程

```
用户按 T（第一次）→ 标记 serverA 为源端 → 状态栏: "Source: serverA. Mark target with T."
用户导航到 serverB → 按 T（第二次）→ 标记 serverB 为目标端
→ 自动创建 DualRemoteFileBrowser(serverA, serverB)
→ 自动 SetRoot(drb, true)
```

### 边界情况

| 场景 | 处理方式 |
|------|---------|
| T 标记同一个服务器两次 | 第二次 T 应提示 "Cannot mark same server as both source and target" |
| T 标记时无选中服务器 | 忽略（`GetSelectedServer()` 返回 false） |
| 标记后用户按 D 键 | 正常执行 Dup（标记状态不影响 Dup） |
| 标记后用户按 / 搜索 | 正常搜索（标记状态保持） |
| 标记后用户按 q 退出 | 退出应用（标记状态不阻止退出，但可能需要确认） |
| 双远端浏览器中 SFTP 连接失败 | 各面板独立显示错误，不影响另一个面板 |

---

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| Go SSH 原生库 (`golang.org/x/crypto/ssh`) | 违反"零外部依赖"原则，无法复用用户 SSH config | 系统 `ssh` + `pkg/sftp`（当前方案） |
| `ssh -W` 端口转发直传 | 无法显示分阶段进度，取消传播复杂 | 本机临时文件中转（已验证的 CopyRemoteFile 模式） |
| 改造 FileBrowser 为双远端模式 | 大量条件分支，local/remote 逻辑纠缠 | 新建 DualRemoteFileBrowser，复用 RemotePane |
| 修改 TransferService 接口 | 不需要新方法，UI 层编排即可 | 两个 transfer.New() 实例 |
| 修改 SFTPService 接口 | 当前接口已满足多实例需求 | 创建第二个 sftp_client.New() 实例 |
| ServerList 改造为 Table | 大工程，超出 v1.4 范围 | 状态栏标记反馈（最小改动） |
| 磁盘空间预检查 | 不可靠，增加复杂度 | 依赖 ENOSPC 错误处理 |
| 标记状态持久化 | 标记是临时交互状态，不是用户数据 | 内存字段，Esc 清除 |

---

## Dependency Summary

| Category | v1.4 Change |
|----------|-------------|
| go.mod | **零变更** |
| 新外部依赖 | **0 个** |
| 新标准库包 | **0 个** |
| 新 Port 接口 | **0 个** |
| 新 Adapter 类型 | 1 个（`DualRemoteFileBrowser`） |
| 新 tui 字段 | 2-4 个（标记状态） |
| 新文件 | ~3 个（`dual_remote_browser.go`, `dual_remote_handlers.go`, `dual_remote_transfer.go`） |
| 修改文件 | ~3 个（`handlers.go`, `tui.go`, `server_list.go` 可能） |

---

## Existing Stack (Preserved)

| Component | Technology | Version |
|-----------|-----------|---------|
| Language | Go | 1.24.6 |
| TUI Framework | tview | latest |
| Terminal | tcell/v2 | 2.9.0 |
| CLI | Cobra | 1.9.1 |
| Logging | Zap | 1.27.0 |
| SSH Config | ssh_config (forked) | 1.4.0 |
| SFTP Client | pkg/sftp | v1.13.10 |
| Clipboard | atotto/clipboard | 0.1.4 |
| Unicode Width | mattn/go-runewidth | 0.0.16 |
| Unicode Segmentation | rivo/uniseg | 0.4.7 |

---

## Sources

- **HIGH confidence（项目源码直接分析）：**
  - `internal/adapters/ui/handlers.go` — handleServerDup 当前实现（行 288-349），handleGlobalKeys 快捷键分配（行 65-134），handleFileBrowser SFTP 连接管理（行 458-483）
  - `internal/adapters/ui/tui.go` — tui 结构体字段定义（行 29-54），DI 构造函数（行 56-70）
  - `internal/adapters/ui/server_list.go` — ServerList 结构体（行 23-29），UpdateServers 渲染（行 70-90），GetSelectedServer（行 92-98）
  - `internal/adapters/ui/file_browser/file_browser.go` — FileBrowser 结构体（行 61-81），Clipboard 定义（行 39-56），build 连接管理（行 108-259），cross-pane paste 限制（行 970-974），handleRemoteMove 两阶段模式（行 1126-1236）
  - `internal/adapters/ui/file_browser/remote_pane.go` — RemotePane 构造和 SFTPService 注入（行 46-61），ShowConnecting/ShowError/ShowConnected 状态（行 124-162）
  - `internal/adapters/ui/file_browser/file_browser_handlers.go` — handleGlobalKeys overlay chain（行 33-114），close SFTP 清理（行 135-143）
  - `internal/core/ports/file_service.go` — SFTPService 接口，Connect/Close 独立生命周期
  - `internal/core/ports/transfer.go` — TransferService 接口，Download/Upload 方法签名
  - `internal/adapters/data/sftp_client/sftp_client.go` — SFTPClient 独立实例创建（行 51-53），sync.Mutex 保护（行 46）
  - `internal/adapters/data/transfer/transfer_service.go` — CopyRemoteFile 两阶段中转模式（行 436-472），CopyRemoteDir 目录中转（行 476-524）
  - `cmd/main.go` — SFTP 单例创建（行 60），TransferService 绑定（行 61），TUI 注入（行 62）

---
*Stack research: 2026-04-15 (v1.4 Dup Fix & Dual Remote Transfer)*
*Previous: 2026-04-15 (v1.3 Enhanced File Browser)*
