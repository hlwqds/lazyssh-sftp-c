# Phase 12: Dual Remote Browser - Research

**Researched:** 2026-04-15
**Domain:** tview/tcell 双栏远程文件浏览器组件 + SFTP 并行连接管理
**Confidence:** HIGH

## Summary

Phase 12 创建独立的 DualRemoteFileBrowser 组件，左右栏各显示一个远程服务器的文件系统。核心挑战在于：(1) 管理**两个独立 SFTPClient 实例**的生命周期（并行连接、独立关闭、资源清理）；(2) 在 DualRemoteFileBrowser 中**复用现有 RemotePane** 组件，但需要为其提供独立的 SFTPService 实例；(3) 复用 ConfirmDialog/InputDialog overlay 模式处理同面板文件操作；(4) 处理 `cmd.Stderr = os.Stderr` 在双实例场景下的输出竞争问题。

现有代码库已经具备所有必需的构建块：RemotePane 是完整的远程文件浏览组件（DRB-02 明确复用），ConfirmDialog/InputDialog 遵循 overlay 模式可直接复用（D-05），FileBrowser 提供了完整的布局和事件路由参考模式。关键的技术决策是：DualRemoteFileBrowser **不** 复用 FileBrowser（CONTEXT.md D-01 已锁定），而是独立创建一个新组件，遵循相同的架构模式。

**Primary recommendation:** 创建 `dual_remote_browser.go` 作为 `file_browser` 包的新文件，结构与 FileBrowser 高度对称，但持有两个 RemotePane 和两个 SFTPService 实例。新建两个 SFTPClient 实例（而非复用 tui 中的单一 sftpService），通过 `sftp_client.New(log)` 工厂函数创建。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** 50:50 Flex 布局，与现有 FileBrowser 一致。上方添加 header bar 显示两个服务器的别名和 IP 地址，格式如 "Source: myserver (1.2.3.4) | Target: otherserver (5.6.7.8)"。
- **D-02:** 面板内文件列表复用 FileBrowser 的 4 列格式（Name, Size, Modified, Permissions），保持一致性。
- **D-03:** 活跃面板通过高亮边框或不同背景色标识，与 FileBrowser 的 Tab 切换体验一致。
- **D-04:** 每个面板顶部显示服务器别名 + IP，如 "Source: myserver (1.2.3.4)"。与 FileBrowser 的路径显示风格一致。目标端面板显示 "Target: otherserver (5.6.7.8)"。
- **D-05:** Phase 12 包含每个远程面板内的删除（d）、重命名（R）、新建目录（m）操作。直接复用 Phase 6 的 ConfirmDialog 和 InputDialog overlay 组件。
- **D-06:** 不包含同服务器内的复制/移动（c/x + p），这些操作在 Phase 13 与跨远端传输一起实现。
- **D-07:** 两个 SFTP 连接并行建立（goroutine 并发），用户体验更快。连接状态在每个面板内显示（Connecting/Connected/Error）。
- **D-08:** 一个连接失败时，失败面板显示错误信息，另一个正常面板可继续浏览。用户可手动按 Esc 退出。不自动关闭整个浏览器。
- **D-09:** 底部状态栏显示：两个服务器别名、两个连接状态（Connected/Error）、活跃面板指示、快捷键提示。格式与 FileBrowser 状态栏一致（bullet 分隔）。
- **D-10:** 快捷键方案与 FileBrowser 完全一致：Tab 切换面板、Esc 退出、d 删除、R 重命名、m 新建目录、/ 搜索、. 隐藏文件、Enter 进入目录、h 返回上级、Space 多选。
- **D-11:** Esc 关闭 DualRemoteFileBrowser 并清理两个 SFTP 连接（DRB-04 已锁定）。

### Claude's Discretion
- Header bar 的具体颜色和样式（基于 tview/tcell 现有颜色方案）
- 活跃面板高亮的具体实现方式（边框颜色 vs 背景色 vs 两者）
- 状态栏的具体文本格式和布局
- 连接失败时的具体错误信息措辞
- ConfirmDialog/InputDialog 在 DualRemoteFileBrowser 中的集成方式（作为独立字段还是共享引用）

### Deferred Ideas (OUT OF SCOPE)
- None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DRB-01 | 创建独立的 DualRemoteFileBrowser 组件（不复用 FileBrowser），左栏为远端 A（源端），右栏为远端 B（目标端） | 新建 `dual_remote_browser.go`，结构与 FileBrowser 对称，持有两个 RemotePane 实例 |
| DRB-02 | 双栏复用 RemotePane 组件，各自持有独立的 SFTPClient 实例 | 两个 `sftp_client.New(log)` 实例，各自 `Connect(server)` |
| DRB-03 | 支持键盘导航（Tab 切换面板、上下左右浏览、Enter 进入目录、h 返回上级） | RemotePane 内建 Tab/Enter/h/Space 支持，DualRemoteFileBrowser 的 handleGlobalKeys 路由 |
| DRB-04 | 退出浏览器（Esc/q）时关闭两个 SFTP 连接并清理资源 | goroutine 并行 Close() + SetAfterDrawFunc(nil) + onClose 回调 |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| tview | v0.0.0 (git) | TUI 框架，Flex/Table/Box 布局 | 项目已有依赖，所有 UI 组件基于此 |
| tcell/v2 | v2.9.0 | 终端 cell 操作，颜色定义 | 项目已有依赖，所有颜色和屏幕操作基于此 |
| pkg/sftp | (via go.mod) | SFTP 协议客户端 | SFTPClient 基于此实现，通过 SSH pipe 建立连接 |
| zap | v1.27.0 | 结构化日志 | 项目已有依赖，所有组件使用 `*zap.SugaredLogger` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| domain.Server | (internal) | SSH 服务器配置实体 | 传递给 SFTPClient.Connect() 建立连接 |
| ports.SFTPService | (internal) | SFTP 服务接口 | DualRemoteFileBrowser 持有两个实例 |
| sftp_client.SFTPClient | (internal) | SFTP 服务实现 | 通过 `sftp_client.New(log)` 创建独立实例 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| 两个独立 SFTPClient 实例 | 复用 tui.sftpService 并在退出时重连 | 不行——tui.sftpService 是单例，关闭后无法同时服务两个服务器 |

## Architecture Patterns

### Recommended Project Structure
```
internal/adapters/ui/file_browser/
├── dual_remote_browser.go          # 新建：DualRemoteFileBrowser 组件
├── dual_remote_browser_handlers.go # 新建：handleGlobalKeys, switchFocus, close 等
├── remote_pane.go                  # 已有：复用（不做修改）
├── confirm_dialog.go               # 已有：复用（不做修改）
├── input_dialog.go                 # 已有：复用（不做修改）
├── file_browser.go                 # 已有：参考模式（不做修改）
└── file_browser_handlers.go        # 已有：参考模式（不做修改）
```

### Pattern 1: 组件结构与 FileBrowser 对称

**What:** DualRemoteFileBrowser 在结构上与 FileBrowser 高度对称，但移除了 LocalPane 相关逻辑，替换为第二个 RemotePane。

**When to use:** 这是 DRB-01 的核心实现方式——独立组件，不复用 FileBrowser。

**Example:**
```go
// 结构对比（概念性，非完整代码）
type DualRemoteFileBrowser struct {
    *tview.Flex
    app          *tview.Application
    log          *zap.SugaredLogger
    sourcePane   *RemotePane  // 左栏：源端服务器
    targetPane   *RemotePane  // 右栏：目标端服务器
    sourceSFTP   ports.SFTPService  // 源端独立 SFTP 实例
    targetSFTP   ports.SFTPService  // 目标端独立 SFTP 实例
    statusBar    *tview.TextView
    headerBar    *tview.TextView    // D-01: 服务器信息 header
    confirmDialog *ConfirmDialog    // D-05: 复用 overlay
    inputDialog   *InputDialog      // D-05: 复用 overlay
    activePane   int                // 0 = source, 1 = target
    sourceServer domain.Server
    targetServer domain.Server
    onClose      func()
}
```

### Pattern 2: Overlay 生命周期管理

**What:** ConfirmDialog 和 InputDialog 作为 DualRemoteFileBrowser 的字段嵌入，遵循 FileBrowser 的 overlay chain 模式——Draw() 中手动绘制，handleGlobalKeys 中优先拦截。

**When to use:** D-05 要求复用 Phase 6 的 overlay 组件。

**Example:**
```go
// Draw overrides Flex.Draw to draw overlays after the main content
func (drb *DualRemoteFileBrowser) Draw(screen tcell.Screen) {
    drb.Flex.Draw(screen)
    if drb.confirmDialog != nil && drb.confirmDialog.IsVisible() {
        drb.confirmDialog.Draw(screen)
    }
    if drb.inputDialog != nil && drb.inputDialog.IsVisible() {
        drb.inputDialog.Draw(screen)
    }
}

// handleGlobalKeys: overlay 优先拦截
func (drb *DualRemoteFileBrowser) handleGlobalKeys(event *tcell.EventKey) *tcell.EventKey {
    if drb.inputDialog != nil && drb.inputDialog.IsVisible() {
        return drb.inputDialog.HandleKey(event)
    }
    if drb.confirmDialog != nil && drb.confirmDialog.IsVisible() {
        return drb.confirmDialog.HandleKey(event)
    }
    // ... then handle Tab, Esc, d, R, m, etc.
}
```

### Pattern 3: 并行 SFTP 连接建立

**What:** 两个 SFTP 连接通过 goroutine 并行建立（D-07），各自独立回调 UI 更新。

**When to use:** DRB-02 要求两个独立 SFTPClient 实例。

**Example:**
```go
// 在 DualRemoteFileBrowser.build() 中并行连接
go func() {
    err := drb.sourceSFTP.Connect(sourceServer)
    drb.app.QueueUpdateDraw(func() {
        if err != nil {
            drb.sourcePane.ShowError(err.Error())
        } else {
            drb.sourcePane.ShowConnected()
        }
        drb.updateStatusBarConnection()
    })
}()

go func() {
    err := drb.targetSFTP.Connect(targetServer)
    drb.app.QueueUpdateDraw(func() {
        if err != nil {
            drb.targetPane.ShowError(err.Error())
        } else {
            drb.targetPane.ShowConnected()
        }
        drb.updateStatusBarConnection()
    })
}()
```

### Pattern 4: 资源清理与连接关闭

**What:** Esc 退出时并行关闭两个 SFTP 连接（DRB-04），遵循 FileBrowser.close() 的模式——goroutine 中关闭 + SetAfterDrawFunc(nil) + onClose 回调。

**When to use:** DRB-04 要求退出时清理两个 SFTP 连接。

**Example:**
```go
func (drb *DualRemoteFileBrowser) close() {
    drb.app.SetAfterDrawFunc(nil) // 移除状态栏 redraw callback
    go func() {
        _ = drb.sourceSFTP.Close()
        _ = drb.targetSFTP.Close()
    }()
    if drb.onClose != nil {
        drb.onClose()
    }
}
```

### Anti-Patterns to Avoid
- **复用 tui.sftpService:** tui 中的 sftpService 是单例，handleFileBrowser() 每次都会先 Close() 再使用。DualRemoteFileBrowser 需要两个独立实例，必须通过 `sftp_client.New(log)` 创建。
- **在 RemotePane 中添加 "source/target" 概念:** RemotePane 应保持通用性，source/target 标识只在 DualRemoteFileBrowser 层面体现（通过 header bar 和状态栏）。
- **在 handleGlobalKeys 中使用 event.Key() == tcell.KeyEnter:** RemotePane 内建的 SetSelectedFunc 已处理 Enter 键，不需要在 DualRemoteFileBrowser 层面拦截。
- **将 DualRemoteFileBrowser 放在 ui 包而非 file_browser 包:** file_browser 包是所有文件浏览组件的归属地，DualRemoteFileBrowser 应放在这里以复用包内的 overlay 组件和辅助函数。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 远程文件列表渲染 | 自定义 Table 填充逻辑 | RemotePane | 已有完整的 4 列格式、排序、隐藏文件、连接状态显示 |
| 确认对话框 | 自定义 modal 弹窗 | ConfirmDialog | 已有 overlay 模式、y/n/Esc 处理、居中布局 |
| 输入对话框 | 自定义 input 弹窗 | InputDialog | 已有 overlay 模式、Enter/Esc 处理、InputField 嵌入 |
| SFTP 连接管理 | 自定义 SSH 进程管理 | sftp_client.SFTPClient | 已有 Connect/Close/IsConnected/HomeDir 完整生命周期 |
| 文件删除/重命名/新建目录 | 自定义 SFTP 操作 | SFTPService.Remove/Rename/Mkdir | 已有错误处理和连接状态检查 |
| 面板焦点切换 | 自定义 Tab 处理 | RemotePane.SetFocused() | 已有边框颜色切换（Color248/Color238） |

**Key insight:** DualRemoteFileBrowser 的核心价值不在于重新实现文件浏览功能，而在于**组合**两个已有的 RemotePane 实例并提供统一的双栏交互层。所有底层能力都已存在，Phase 12 的工作量在于"胶水代码"——布局、事件路由、状态栏、overlay 集成。

## Common Pitfalls

### Pitfall 1: SFTPClient.cmd.Stderr = os.Stderr 输出竞争

**What goes wrong:** 当前 SFTPClient.Connect() 将 SSH 进程的 stderr 重定向到 `os.Stderr`（sftp_client.go:78）。两个并行连接的 SSH 进程会同时向 os.Stderr 写入，在 tview UI 中导致输出污染（SSH debug 信息、警告等直接打印到终端覆盖 UI）。

**Why it happens:** `cmd.Stderr = os.Stderr` 是原始设计决策，用于 SSH 调试。单实例时问题不明显（FileBrowser 使用时 stderr 输出较少），但双实例并行连接时，两个 SSH 进程的 stderr 输出会交错。

**How to avoid:** 为 DualRemoteFileBrowser 的两个 SFTPClient 提供 `io.Discard` 作为 stderr（或通过 logger 捕获）。具体方案：

方案 A（推荐，最小改动）：在 DualRemoteFileBrowser 中创建 SFTPClient 后不修改其内部 stderr，但在 Connect 前临时将 os.Stderr 重定向。这需要 SFTPClient 暴露一个 SetStderr 方法。

方案 B（更干净）：为 SFTPClient 添加一个可选的 `stderr io.Writer` 字段，通过 `NewWithStderr(log, stderr)` 工厂函数创建。默认行为保持 `os.Stderr` 不变。

方案 C（最简单）：接受当前行为。SSH stderr 输出仅在连接建立期间出现（短暂的 debug 输出），连接成功后不再输出。并行连接的短暂输出交错对用户体验影响有限。

**Warning signs:** 连接时终端出现乱码或覆盖 UI 的文本。

### Pitfall 2: SFTPClient 不是线程安全的并发操作

**What goes wrong:** SFTPClient 使用 `sync.Mutex` 保护内部状态，但 `Connect()` 和 `Close()` 操作的是同一个 client/cmd/stdin 字段。如果两个 goroutine 同时操作同一个 SFTPClient 实例会产生数据竞争。

**Why it happens:** 每个 SFTPClient 实例内部持有一个 SSH 进程和一个 SFTP 客户端连接。并发操作需要通过 Mutex 序列化。

**How to avoid:** 为源端和目标端各创建**独立的** SFTPClient 实例（`sftp_client.New(log)`），不共享任何实例。每个实例的连接和操作天然串行化（通过各自的 Mutex）。

**Warning signs:** `data race` 检测报警、panic、连接状态异常。

### Pitfall 3: Overlay 组件需要独立的实例

**What goes wrong:** 如果 DualRemoteFileBrowser 和 FileBrowser 共享同一个 ConfirmDialog/InputDialog 实例，在一个组件中显示 overlay 会影响另一个组件的渲染。

**Why it happens:** ConfirmDialog 和 InputDialog 使用 `visible` 标志控制显示状态，不是 tview 的 focus 系统组件。共享实例意味着状态冲突。

**How to avoid:** DualRemoteFileBrowser 创建自己的 ConfirmDialog 和 InputDialog 实例（`NewConfirmDialog(app)` / `NewInputDialog(app)`）。这与 FileBrowser 的做法一致。

**Warning signs:** 对话框出现在错误的屏幕上、键盘事件被错误的路由。

### Pitfall 4: SetAfterDrawFunc 的状态栏 redraw callback 冲突

**What goes wrong:** FileBrowser 使用 `app.SetAfterDrawFunc()` 来强制重绘状态栏（解决 tcell v2.9.0 dirty tracking 问题）。如果 DualRemoteFileBrowser 也设置 SetAfterDrawFunc，且用户从 FileBrowser 切换到 DualRemoteFileBrowser 时未清理前者的 callback，会导致状态栏渲染异常。

**Why it happens:** `app.SetAfterDrawFunc()` 是全局的——后设置的会覆盖先设置的。FileBrowser.close() 中有 `SetAfterDrawFunc(nil)` 清理，DualRemoteFileBrowser 也需要相同的清理逻辑。

**How to avoid:** 在 DualRemoteFileBrowser.close() 中调用 `app.SetAfterDrawFunc(nil)` 清理 callback（与 FileBrowser.close() 模式一致）。

**Warning signs:** 状态栏不更新、内容闪烁、ghost artifacts。

### Pitfall 5: handleDualRemoteBrowser 中的 SFTPClient 实例化位置

**What goes wrong:** 如果在 `handleDualRemoteBrowser()` 中创建 SFTPClient 实例，需要确保这些实例在 DualRemoteFileBrowser 关闭后被正确清理（Go GC 会回收，但 SSH 进程需要显式 Kill）。

**Why it happens:** SFTPClient 持有 `*exec.Cmd`（SSH 子进程），必须通过 Close() 显式清理进程，否则会留下僵尸进程。

**How to avoid:** DualRemoteFileBrowser 持有 SFTPClient 引用，在 close() 方法中显式调用 Close()。SFTPClient.Close() 已经实现了 Kill + Wait 清理。

**Warning signs:** `ps aux | grep ssh` 显示残留的 SSH 进程。

### Pitfall 6: handleFileBrowser 的 sftpService 冲突

**What goes wrong:** tui 中的 `sftpService` 是单例，handleFileBrowser() 每次打开前会 Close() 它。如果用户先打开 FileBrowser（建立了 sftpService 连接），然后通过 T 标记打开 DualRemoteFileBrowser（FileBrowser 可能未正确关闭），会导致 sftpService 状态异常。

**Why it happens:** 当前 handleFileBrowser() 直接使用 tui.sftpService（单例），而 DualRemoteFileBrowser 需要创建独立实例。两者之间没有冲突，因为 DualRemoteFileBrowser 不使用 tui.sftpService。

**How to avoid:** DualRemoteFileBrowser 创建全新的 SFTPClient 实例，与 tui.sftpService 完全隔离。在 handleDualRemoteBrowser() 中不需要操作 tui.sftpService。

**Warning signs:** "not connected" 错误出现在错误的组件中。

## Code Examples

### 从 handleDualRemoteBrowser 创建组件（参考 handleFileBrowser 模式）

```go
// Source: handlers.go:189 (现有占位函数) + file_browser.go:516 (参考 handleFileBrowser)
func (t *tui) handleDualRemoteBrowser(source, target domain.Server) {
    fb := file_browser.NewDualRemoteFileBrowser(
        t.app,
        t.logger,
        source,
        target,
        func() {
            t.returnToMain()
        },
    )
    t.app.SetRoot(fb, true)
    t.app.Sync()
}
```

### DualRemoteFileBrowser 构造函数签名

```go
// Source: 设计推荐，参考 FileBrowser.NewFileBrowser 签名
func NewDualRemoteFileBrowser(
    app *tview.Application,
    log *zap.SugaredLogger,
    source, target domain.Server,
    onClose func(),
) *DualRemoteFileBrowser
```

注意：与 FileBrowser 不同，DualRemoteFileBrowser **不** 接受外部传入的 SFTPService 实例，因为需要两个独立实例。它在内部通过 `sftp_client.New(log)` 创建。

### 同面板文件操作（参考 FileBrowser.handleDelete/handleRename/handleMkdir）

```go
// 删除操作——需要根据 activePane 选择正确的 SFTPService
func (drb *DualRemoteFileBrowser) handleDelete() {
    sftp := drb.currentSFTPService()
    pane := drb.currentPane()

    if !pane.IsConnected() {
        drb.showStatusError("Not connected")
        return
    }

    row, _ := pane.GetSelection()
    cell := pane.GetCell(row, 0)
    if cell == nil { return }
    fi, ok := cell.GetReference().(domain.FileInfo)
    if !ok { return }

    currentPath := pane.GetCurrentPath()
    fullPath := joinPath(currentPath, fi.Name)

    // ... ConfirmDialog + goroutine 执行 sftp.Remove/sftp.RemoveAll
}
```

### 状态栏格式（D-09）

```go
// 格式参考 FileBrowser.setStatusBarDefault，增加双服务器信息
func (drb *DualRemoteFileBrowser) setStatusBarDefault() {
    srcStatus := "[#A0FFA0]Connected[-]"
    if !drb.sourcePane.IsConnected() {
        srcStatus = "[#FF6B6B]Error[-]"
    }
    tgtStatus := "[#A0FFA0]Connected[-]"
    if !drb.targetPane.IsConnected() {
        tgtStatus = "[#FF6B6B]Error[-]"
    }

    drb.statusBar.SetText(
        fmt.Sprintf("[#5FAFFF]%s[-] %s  [#5FAFFF]%s[-] %s  %s  [white]Tab[-] Switch  [white]d[-] Delete  [white]R[-] Rename  [white]m[-] Mkdir  [white]Esc[-] Back",
            drb.sourceServer.Alias, srcStatus,
            drb.targetServer.Alias, tgtStatus,
            drb.activePanelLabel(),
        ),
    )
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| FileBrowser 复用 local+remote | DualRemoteFileBrowser 独立组件 remote+remote | Phase 12 | 避免修改 FileBrowser 的 activePane 二元假设 |
| 单 SFTPService 实例（tui 单例） | 双独立 SFTPClient 实例（组件内部创建） | Phase 12 | 支持同时连接两台服务器 |
| overlay 作为 tview focus 组件 | overlay 手动 Draw + HandleKey（不受 focus 影响） | Phase 6 | DualRemoteFileBrowser 直接复用此模式 |

**Deprecated/outdated:**
- FileBrowser 的 activePane 0=local/1=remote 二元假设：DualRemoteFileBrowser 不应依赖此模式，但可参考其 switchFocus 实现方式。

## Open Questions

1. **SFTPClient cmd.Stderr 处理方案**
   - What we know: 当前 `cmd.Stderr = os.Stderr`，双实例并行连接时会产生输出交错
   - What's unclear: 是否需要在 Phase 12 解决，还是可以推迟到后续优化
   - Recommendation: Phase 12 采用方案 C（接受当前行为），因为 SSH stderr 输出仅在连接建立短暂期间出现，连接成功后不再输出。如果用户反馈体验差，再在后续 phase 优化。理由：避免为非阻塞性问题增加 SFTPClient API 变更。

2. **tui struct 是否需要持有 DualRemoteFileBrowser 引用**
   - What we know: FileBrowser 通过 `app.SetRoot(fb, true)` 显示，不存储在 tui struct 中
   - What's unclear: DualRemoteFileBrowser 是否需要相同的处理
   - Recommendation: 不需要。遵循 FileBrowser 的模式——通过 app.SetRoot() 显示，onClose 回调 returnToMain()。tui struct 不需要新字段。

3. **headerBar 使用 tview.TextView 还是直接在 Flex 布局中渲染**
   - What we know: FileBrowser 没有独立的 header bar，状态栏通过 AfterDrawFunc 渲染
   - What's unclear: header bar 是用 tview.TextView 作为 Flex 子项，还是用 AfterDrawFunc 渲染
   - Recommendation: 使用 tview.TextView 作为 FlexRow 的子项（固定 1 行高度），与状态栏对称。理由：更简单、更符合 tview 的布局模型、不需要 AfterDrawFunc 复杂性。

## Environment Availability

Step 2.6: SKIPPED (no external dependencies identified — Phase 12 is purely code changes within the existing Go project)

## Validation Architecture

> nyquist_validation is explicitly set to false in .planning/config.json. Skipping.

## Sources

### Primary (HIGH confidence)
- `internal/adapters/ui/file_browser/file_browser.go` — FileBrowser 组件完整实现，布局/overlay/close 模式参考
- `internal/adapters/ui/file_browser/remote_pane.go` — RemotePane 组件完整实现，Phase 12 直接复用
- `internal/adapters/ui/file_browser/file_browser_handlers.go` — handleGlobalKeys 事件路由模式
- `internal/adapters/ui/file_browser/confirm_dialog.go` — ConfirmDialog overlay 组件
- `internal/adapters/ui/file_browser/input_dialog.go` — InputDialog overlay 组件
- `internal/adapters/data/sftp_client/sftp_client.go` — SFTPClient 实现，Connect/Close/HomeDir 生命周期
- `internal/core/ports/file_service.go` — SFTPService 接口定义
- `internal/adapters/ui/tui.go` — tui struct 结构，handleDualRemoteBrowser 入口
- `internal/adapters/ui/handlers.go` — handleDualRemoteBrowser 占位函数
- `cmd/main.go` — 依赖注入模式，sftp_client.New(log) 工厂函数

### Secondary (MEDIUM confidence)
- `.planning/phases/11-t-key-marking/11-CONTEXT.md` — T 键标记上下文，handleDualRemoteBrowser 入口设计
- `.planning/STATE.md` — Phase 12 blocker/concern: cmd.Stderr 竞争问题

### Tertiary (LOW confidence)
- 无需外部搜索，所有发现基于代码库直接分析

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - 全部来自项目已有依赖，无新增
- Architecture: HIGH - 基于 FileBrowser/RemotePane 的已验证模式，代码已审阅
- Pitfalls: HIGH - 所有 pitfall 来自代码直接分析，cmd.Stderr 问题已确认

**Research date:** 2026-04-15
**Valid until:** 30 天（项目内部代码库，变化频率低）
