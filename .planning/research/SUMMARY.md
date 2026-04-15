# Project Research Summary

**Project:** LazySSH v1.4 -- Dup Fix & Dual-Remote File Transfer
**Domain:** TUI SSH Manager -- 双远端文件浏览器、跨服务器复制/移动、T 键标记服务器
**Researched:** 2026-04-15
**Confidence:** HIGH

## Executive Summary

v1.4 为 lazyssh 终端 SSH 管理器添加双远端文件互传能力。核心交付物包括：(1) Dup 修复（移除自动打开表单行为），(2) T 键标记服务器（最多两台），(3) 双远端文件浏览器（两个 `RemotePane` 并列显示），(4) 跨远端文件复制/移动（通过本地临时文件中转）。

经过对现有代码库的逐行审查，研究得出一个核心结论：**零新外部依赖。** 所有技术需求均由现有技术栈覆盖。双远端传输复用 `CopyRemoteFile`/`CopyRemoteDir` 的 download-to-temp + re-upload 两阶段模式，`DualRemoteFileBrowser` 是独立于现有 `FileBrowser` 的新组件，避免了对 `activePane` 二元假设（0=本地, 1=远程）的大量条件分支改造。最大的架构风险在于现有 `FileBrowser` 中 15+ 处 `activePane == 0/1` 的硬编码假设，新建独立组件是唯一干净的选择。

主要风险及缓解措施：(1) `activePane` 二元假设导致文件管理操作（删除/重命名/新建目录）调用错误的文件系统 API -- 通过创建独立的 `DualRemoteFileBrowser` 组件彻底隔离；(2) 跨面板粘贴保护阻止双远端互传 -- 新组件中移除此保护，跨面板粘贴触发 relay 传输；(3) 双 SFTP 子进程 stderr 竞争污染终端 UI -- 重定向到 `io.Discard` 或 logger；(4) 临时文件在取消/失败时清理不对称 -- 创建新的 `RelayTransferService`，使用 `defer` + 显式清理。

## Key Findings

### Recommended Stack

**零新外部依赖。** v1.4 不修改 `go.mod`，不引入新的 Go 标准库包，不新增 Port 接口。所有功能通过组合现有组件实现。

**核心复用技术：**
- `pkg/sftp` + `SFTPClient`: 创建两个独立实例，各自连接不同服务器 -- 每个 SFTPClient 有独立 mutex，天然支持多实例并发
- `TransferService`: 创建两个临时实例（一个用源 SFTP 下载，一个用目标 SFTP 上传），零代码重复
- `RemotePane`: 完全复用，实例化两次（左栏=服务器 A，右栏=服务器 B）
- `TransferModal`: 复用 `modeCopy` 进度显示，两阶段重置进度条
- `Clipboard`/`ConfirmDialog`/`InputDialog`: 完全复用

**新增内部类型：**
- `DualRemoteFileBrowser` (UI 组件) -- 新文件 ~3 个，估计 ~700 行
- `RelayTransferService` (Port + Adapter) -- 新端口接口 ~20 行，新适配器 ~120 行
- `tui` struct 新增标记状态字段 2-4 个

### Expected Features

**Must have (table stakes):**

- **Dup 修复**: D 键直接保存复制结果，不打开编辑表单 -- LOW 复杂度
- **T 键标记服务器**: 标记最多 2 台服务器（源端/目标端），状态栏提示标记进度 -- LOW 复杂度
- **Esc 清除标记**: 标准取消模式，与现有剪贴板 Esc 清除一致 -- LOW 复杂度
- **双远端文件浏览器**: 两个 `RemotePane` 并列，独立 SFTP 连接，Tab 切换焦点 -- MEDIUM 复杂度
- **跨远端文件复制 (c+p)**: 从服务器 A 下载到临时文件，上传到服务器 B -- MEDIUM 复杂度
- **跨远端文件移动 (x+p)**: 复制 + 删除源 -- MEDIUM 复杂度
- **两阶段进度显示**: "Downloading from A" -> 重置 -> "Uploading to B" -- LOW 复杂度
- **冲突处理**: 目标文件已存在时显示 overwrite/skip/rename 对话框 -- LOW 复杂度
- **取消清理**: 传输取消时清理本地临时文件和目标远程部分文件 -- MEDIUM 复杂度

**Should have (differentiators):**

- **标记顺序决定面板分配**: 第一次 T = 左栏（源端），第二次 T = 右栏（目标端）-- 减少用户混淆
- **颜色编码面板边框**: 左栏 cyan（A），右栏 magenta（B）-- 快速区分服务器
- **并行 SFTP 连接**: 两个 goroutine 同时连接，缩短等待时间
- **同面板内文件操作**: 复制/移动/删除/重命名/新建目录在同一远程服务器内正常工作
- **F5 跨远端目录传输**: 与现有 F5 操作对称

**Defer (v2+):**

- 直接服务器到服务器 SCP（需要 A 到 B 的 SSH 访问）
- 流式中转（download chunk -> upload chunk 并发）
- 传输队列（多个文件排队传输）
- Sync/merge 模式（rsync 式增量传输）
- 带宽限制

### Architecture Approach

**核心架构决策：新建独立组件，不改造 FileBrowser。**

现有 `FileBrowser` 有 6 处以上硬编码假设（localPane + remotePane 类型固定、单 SFTP 连接、activePane 二元语义、跨面板粘贴保护、RecentDirs 绑定单服务器），强行添加双远端模式会污染每个方法。独立 `DualRemoteFileBrowser` 将复杂度隔离在单一组件中。

**主要组件：**

1. **`tui` (tui.go)** -- 持有 T 键标记状态 (`markedServers [2]domain.Server`, `markedCount int`)，处理 T 键路由和打开双远端浏览器入口。标记状态存储在 TUI 层而非 ServerList，因为需要跨组件访问。
2. **`ServerList` (server_list.go)** -- 新增 `UpdateMarkLabels()` 方法，支持 `[A]`/`[B]` 前缀渲染，标记状态由 TUI 层传入。
3. **`DualRemoteFileBrowser` (dual_remote_browser.go)** -- 双远端文件浏览器根组件。内部创建两个 `sftp_client.New()` 实例（不经过 `cmd/main.go` 的 DI 链），布局为 FlexRow(content(FlexColumn: RemotePane x2) + StatusBar)。
4. **`RelayTransferService` (port + adapter)** -- 新端口接口定义 `RelayFile()`/`RelayDir()` 方法。适配器内部创建两个 `transfer.New()` 实例，分别用于下载和上传阶段。零代码重复。
5. **`RemotePane` (复用)** -- 完全复用，实例化两次。无任何修改。

**关键数据流：**
```
T 标记 serverA -> T 标记 serverB -> handleOpenDualRemote()
  -> sftp_client.New() x2 -> relay_transfer.New()
  -> NewDualRemoteFileBrowser(app, log, serverA, serverB)
  -> app.SetRoot(dualBrowser, true)

跨面板粘贴 (c + p):
  -> initiateRelayFileTransfer()
  -> relaySvc.RelayFile(ctx, srcPath, dstPath, onProgress, onConflict)
    -> Phase 1: dlSvc.DownloadFile(ctx, srcPath, tmpPath) [via sftpA]
    -> Phase 2: ulSvc.UploadFile(ctx, tmpPath, dstPath) [via sftpB]
    -> Cleanup: os.Remove(tmpPath)
```

### Critical Pitfalls

1. **`activePane` 二元假设** -- 现有 15+ 个方法假设 `pane 0 = 本地, pane 1 = 远程`。如果在 FileBrowser 中强行添加双远端模式，`getFileService()` 在左面板返回本地 FileService 而非远程 SFTPService，导致删除/重命名操作执行在本地文件系统上。**预防：创建独立 DualRemoteFileBrowser，所有 pane 选择逻辑都返回远程 SFTP 服务。**

2. **跨面板粘贴保护** -- `handlePaste` 的 `SourcePane != activePane` 保护会阻止所有跨面板操作。**预防：新组件中移除此保护，跨面板粘贴触发 relay 传输。**

3. **`buildConflictHandler` 使用 `activePane` 判断冲突检查目标** -- 双远端中两个面板都是远程，冲突检查应始终检查目标面板的 SFTP 服务。复用现有方法会检查错误的服务器。**预防：重写冲突处理，始终使用目标面板的 SFTPService.Stat()。**

4. **临时文件清理不对称** -- 双远端传输的取消可能发生在 download 中途或 upload 中途，upload 失败时需额外清理目标远程部分文件。**预防：创建新的 RelayFile 方法，defer 清理本地临时文件，upload 失败时显式清理目标远程。**

5. **双 SFTP 子进程 stderr 竞争** -- 两个 SSH 进程的 stderr 都输出到 `os.Stderr`，在 TUI 模式下会污染终端。**预防：重定向 `cmd.Stderr` 到 `io.Discard` 或日志缓冲区。**

## Implications for Roadmap

Based on research, suggested phase structure:

### Phase 1: Dup Fix

**Rationale:** 最小的独立变更，零依赖，可立即交付价值。修复现有行为 bug，为后续 phase 清理代码。

**Delivers:**
- `handleServerDup()` 移除 ServerForm 创建，直接调用 `AddServer()`
- 自动滚动到新条目
- 状态栏确认反馈

**Addresses:** FEATURES.md -- Dup 修复所有 table stakes

**Avoids:** 无特别风险，简单直接的代码路径替换

**Files:** `internal/adapters/ui/handlers.go` (修改), `internal/adapters/ui/tui.go` (可能移除 `dupPendingAlias`)

---

### Phase 2: T Key Marking

**Rationale:** 纯服务器列表层的 UI 状态变更，无新组件依赖。为 Phase 4 提供入口机制。可与 Phase 1 并行开发。

**Delivers:**
- `tui` struct 新增 `markedServers [2]domain.Server`, `markedCount int` 字段
- `handleMarkForDualRemote()` 处理 T 键标记逻辑
- `ServerList.UpdateMarkLabels()` 支持 `[A]`/`[B]` 前缀渲染
- Esc 清除标记状态
- 状态栏提示标记进度

**Addresses:** FEATURES.md -- T 键标记所有 table stakes

**Avoids:** PITFALLS P9 (T/D 键交互), P14 (T/t 大小写), P16 (Esc + T 标记)

**Files:** `internal/adapters/ui/tui.go` (+5 行), `internal/adapters/ui/handlers.go` (+60 行), `internal/adapters/ui/server_list.go` (+30 行)

---

### Phase 3: RelayTransferService (Port + Adapter)

**Rationale:** 纯数据层，可独立单元测试，无 UI 依赖。与 Phase 2 并行开发。为 Phase 4 提供传输能力。

**Delivers:**
- `internal/core/ports/relay_transfer.go` -- `RelayTransferService` 接口
- `internal/adapters/data/transfer/relay_transfer_service.go` -- 中转传输实现
- `RelayFile()`: download-to-temp + re-upload，defer 清理
- `RelayDir()`: 目录级中转传输
- 单元测试

**Addresses:** FEATURES.md -- 跨远端复制/移动的数据层

**Avoids:** PITFALLS P5 (临时文件清理不对称)

**Files:** 2 个新文件，约 140 行

---

### Phase 4: DualRemoteFileBrowser UI

**Rationale:** 集成层，依赖 Phase 2 (T 键入口) 和 Phase 3 (RelayTransferService)。最大的单个 phase，估计 ~700 行新代码。

**Delivers:**
- `dual_remote_browser.go` -- 根组件 (build, layout, close, Draw)
- `dual_remote_handlers.go` -- 键路由 (Tab/Esc/c/x/p/d/R/m/s/S/F5)
- `dual_remote_transfer.go` -- 跨远端传输编排
- 两个并行 SFTP 连接管理
- 同面板内文件操作 (复制/移动/删除/重命名/新建目录)
- TransferModal 集成 (复用 modeCopy 进度)
- 两个 RecentDirs 实例

**Addresses:** FEATURES.md -- 双远端浏览器所有 table stakes 和 differentiators

**Avoids:** PITFALLS P1 (activePane 二元假设), P2 (跨面板粘贴保护), P3 (Clipboard.SourcePane 语义), P4 (buildConflictHandler), P6 (stderr 竞争), P7 (close() 竞态), P8 (goroutine 死锁), P10 (Enter 键语义), P11 (RecentDirs 分离), P12 (状态栏信息), P13 (SetDismissCallback 覆盖), P15 (r 键面板歧义), P17 (速度计算基准)

**Files:** 3 个新文件 (~700 行), 2-3 个修改文件

---

### Phase Ordering Rationale

- **Phase 1 和 Phase 2 并行:** 两者完全独立，Dup 修复是纯 handler 逻辑，T 键标记是纯 UI 状态
- **Phase 2 和 Phase 3 并行:** T 键标记在 UI 层，RelayTransferService 在数据层，无交叉依赖
- **Phase 4 最后构建:** 它是集成层，需要 Phase 2 的 T 键入口机制和 Phase 3 的传输能力
- **按风险递增排序:** Phase 1 (LOW) -> Phase 2 (LOW) -> Phase 3 (MEDIUM) -> Phase 4 (MEDIUM-HIGH)
- **按代码量递增排序:** Phase 1 (~20 行修改) -> Phase 2 (~100 行) -> Phase 3 (~140 行) -> Phase 4 (~700 行)

### Research Flags

**需要研究 (需要 `/gsd:research-phase`):**
- **Phase 4:** 双远端浏览器的键盘绑定设计需要确认 Enter 键行为（触发 relay 传输 vs 不做任何事 vs 进入目录）。当前研究建议选项 B（不触发传输，统一通过 c/p 机制），但需要在实现阶段最终确认。

**标准模式 (可跳过研究):**
- **Phase 1:** 简单的代码路径替换，模式已在现有 `AddServer()` 调用中验证
- **Phase 2:** 纯 UI 状态管理，复用 `dupPendingAlias` 的 transient state 模式
- **Phase 3:** `CopyRemoteFile`/`CopyRemoteDir` 的两阶段模式已被 v1.3 充分验证，RelayTransferService 是同构实现

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | 逐行源码分析确认零新依赖，所有复用点已验证 |
| Features | HIGH | 基于 Midnight Commander FISH 协议行为、scp -3 模式、现有 CopyRemoteFile 实现 |
| Architecture | HIGH | 15+ 个 activePane 硬编码点已逐行确认，独立组件方案有充分依据 |
| Pitfalls | HIGH | 基于 v1.0-v1.3 踩坑经验和源码逐行审查，17 个 pitfall 有具体代码行号 |

**Overall confidence:** HIGH

### Gaps to Address

1. **`cmd.Stderr = os.Stderr` 修改范围**: 当前建议在所有 `SFTPClient.Connect()` 调用中重定向 stderr，但这会影响现有的单连接文件浏览器行为。需要在 Phase 4 开始时确认是否作为独立前置改动处理。
2. **Enter 键在双远端浏览器中的行为**: 研究建议不触发传输（统一通过 c/p），但这是一个 UX 决策，需要在 Phase 4 实现时最终确认。
3. **双远端浏览器的 `r` 键 (RecentDirs)**: 需要两个独立 `RecentDirs` 实例，实现方案明确，但 v1.4 是否必须实现 `r` 键功能需要确认（可以作为 v1.5 延迟交付）。

## Sources

### Primary (HIGH confidence)

- `internal/adapters/ui/handlers.go` -- handleServerDup 当前实现 (行 288-349), handleGlobalKeys 快捷键分配 (行 65-134), handleFileBrowser SFTP 连接管理 (行 458-483)
- `internal/adapters/ui/tui.go` -- tui 结构体字段定义, DI 构造函数
- `internal/adapters/ui/server_list.go` -- ServerList 结构体, UpdateServers 渲染, GetSelectedServer
- `internal/adapters/ui/file_browser/file_browser.go` -- FileBrowser 结构体 (行 61-81), Clipboard 定义 (行 39-56), build 连接管理 (行 108-259), cross-pane paste 限制 (行 970-974), buildConflictHandler (行 565-623), buildPath (行 1415-1420), close() (行 135-143)
- `internal/adapters/ui/file_browser/file_browser_handlers.go` -- handleGlobalKeys overlay chain (行 33-114)
- `internal/adapters/ui/file_browser/remote_pane.go` -- RemotePane 构造和 SFTPService 注入, ShowConnecting/ShowError/ShowConnected 状态
- `internal/adapters/data/sftp_client/sftp_client.go` -- SFTPClient 独立实例创建, sync.Mutex 保护, cmd.Stderr = os.Stderr (行 78)
- `internal/adapters/data/transfer/transfer_service.go` -- CopyRemoteFile 两阶段中转模式 (行 436-472), CopyRemoteDir 目录中转 (行 476-524), 取消清理逻辑
- `internal/core/ports/file_service.go` -- SFTPService 接口
- `internal/core/ports/transfer.go` -- TransferService 接口
- `cmd/main.go` -- SFTP 单例创建, TransferService 绑定, TUI 注入
- `.planning/PROJECT.md` -- v1.4 需求定义

### Secondary (MEDIUM confidence)

- Midnight Commander FISH protocol behavior -- 双远端面板的 canonical UX 模式
- OpenSSH scp -3 relay flag -- 本地中转传输的技术验证
- Termius commercial reference -- 双 SFTP 面板的产品设计参考

---
*Research completed: 2026-04-15*
*Ready for roadmap: yes*
