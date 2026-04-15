# Technology Stack Research

**Analysis Date:** 2026-04-15
**Domain:** TUI File Browser Enhancements for Go SSH Manager (v1.3)
**Confidence:** HIGH

---

## Executive Summary

v1.3 需要三个新功能：(1) 本地路径历史持久化、(2) Dup SSH 连接复制、(3) 双远端文件互传。经过对代码库的深入分析，**结论是：零新外部依赖。** 三个功能所需的技术原语全部存在于当前技术栈中，具体为 Go 标准库（`encoding/json`、`os`、`path/filepath`）和现有项目模式（metadata JSON 持久化、Port/Adapter 架构、双 SFTP 连接）。

---

## v1.3 Stack Additions

**结论: 零新外部依赖。** v1.3 三个功能的所有技术需求均由现有技术栈覆盖。

### Feature 1: 本地路径历史持久化

**存储方式:** JSON 文件，遵循已有 `metadata.json` 模式。

| 需求 | 技术方案 | 现有参考 |
|------|---------|---------|
| 持久化存储 | `encoding/json` + `os.WriteFile` | `metadata_manager.go:67-85` -- `saveAll()` |
| 读取已存储数据 | `encoding/json` + `os.ReadFile` | `metadata_manager.go:44-65` -- `loadAll()` |
| 目录创建 | `os.MkdirAll` | `metadata_manager.go:168-173` -- `ensureDirectory()` |
| 存储路径 | `~/.lazyssh/local-path-history.json` | `~/.lazyssh/metadata.json` 模式 |
| MRU 去重/截断 | 内存 slice 操作 | `recent_dirs.go:79-98` -- `Record()` |

**设计决策: 独立 JSON 文件 vs 扩展 metadata.json**

选择独立 JSON 文件 `~/.lazyssh/local-path-history.json`，理由：
- `metadata.json` 的 key 是 server alias（`map[string]ServerMetadata`），路径历史不绑定到特定服务器
- 路径历史是应用级数据（跨服务器共享），不是服务器级元数据
- 独立文件避免读写 metadata 时的并发竞争（metadata 在增删改查服务器时频繁读写）
- 与 `~/.lazyssh/recent-dirs/{user@host}.json` 模式一致（per-use-case 独立文件）

**数据结构设计:**

```go
// LocalPathHistory 持久化本地路径的 MRU 列表
// 存储在 ~/.lazyssh/local-path-history.json
type LocalPathHistory struct {
    paths    []string          // MRU 路径列表，最近使用在前
    maxEntries int             // 默认 10
    filePath string            // ~/.lazyssh/local-path-history.json
}

// JSON 格式: 直接存储 []string
// ["~/projects/lazyssh", "/tmp/uploads", "~/Downloads"]
```

**为什么不扩展 RecentDirs:**
- `RecentDirs` 绑定到 `serverKey`（`user@host`），且包含 tview.Box 嵌入和 UI 渲染逻辑
- 本地路径历史不区分服务器，需要全局 MRU
- 数据层和 UI 层分离原则 -- 遵循 v1.1 的 2-phase 结构

**集成点:**
- 在 `FileBrowser.initiateTransfer()` 成功后调用 `Record(fb.localPane.GetCurrentPath())`
- 在 `FileBrowser.initiateDirTransfer()` 成功后调用 `Record(dirPath)`
- 在 `LocalPane` 的路径导航中提供 `r` 键弹出历史（类似远程面板的 RecentDirs）
- 通过 `cmd/main.go` 注入到 FileBrowser（或直接在 FileBrowser 构造函数中创建）

---

### Feature 2: Dup SSH 连接（复制服务器配置）

**方案:** 纯 Server 实体拷贝 + 修改 alias + 调用现有 `AddServer()`。

| 需求 | 技术方案 | 现有参考 |
|------|---------|---------|
| 复制服务器配置 | Go 值拷贝 `newServer := server` | Go 结构体值语义 -- 直接赋值即可完整拷贝 |
| 修改 alias | 字符串操作 `newServer.Alias = server.Alias + "-copy"` | -- |
| 验证新 alias | 复用 `validateServer()` | `server_service.go:77-110` |
| 写入 SSH config | 复用 `AddServer()` | `server_service.go:126-134` |
| 清除元数据字段 | 零值重置 `LastSeen, PinnedAt, SSHCount` | `domain.Server` 零值语义 |
| UI 入口 | `handleGlobalKeys` 中添加 `D` 键 | `handlers.go:47-99` 现有快捷键模式 |

**设计决策: alias 后缀策略**

```
原始 alias: "myserver"
复制后 alias: "myserver-copy"
如果 "myserver-copy" 已存在: "myserver-copy-2", "myserver-copy-3", ...
```

理由：
- `-copy` 后缀明确语义，用户一眼就知道这是复制品
- 数字递增处理冲突，与 `nextAvailableName()` 模式一致（`file_browser.go:628-641`）
- 仅修改 alias，Host/Port/User/所有 SSH 选项完全复制

**设计决策: 哪些字段需要清除**

| 字段 | 操作 | 理由 |
|------|------|------|
| `Alias` | 修改（加 `-copy` 后缀） | 必须唯一 |
| `Aliases` | 清空 `[]string{}` | 别名列表可能包含原始 alias，需清除避免冲突 |
| `Tags` | 保留 | 用户可能希望复制品继承标签 |
| `LastSeen` | 清零 `time.Time{}` | 新条目无访问历史 |
| `PinnedAt` | 清零 `time.Time{}` | 新条目默认不置顶 |
| `SSHCount` | 清零 `0` | 新条目无连接计数 |
| 其他所有字段 | 完整保留 | 用户明确要复制配置 |

**Port 接口变更:**

`ServerService` 接口需要新增方法（或直接在 UI 层组合现有方法）：

```go
// 方案 A: 新增接口方法
DuplicateServer(sourceAlias string) error

// 方案 B: UI 层组合（推荐 -- 零接口变更）
// 1. ListServers(sourceAlias) 获取 Server
// 2. 修改 alias 字段
// 3. AddServer(modifiedServer)
```

**推荐方案 B** -- 理由：
- `DuplicateServer` 本质就是 `Get + Modify + Add`，无需新的 Port 方法
- 减少接口膨胀，保持 `ServerService` 职责清晰
- alias 冲突检测逻辑（数字递增）属于 UI 关注点，不应下沉到 Service 层
- 如果 alias 递增需要查询现有服务器列表，UI 层已有 `ListServers` 能力

**注意:** 当前 `handleGlobalKeys` 中 `d` 键已被 `handleServerDelete` 占用。Dup 功能需要使用不同的快捷键。建议使用 `D`（Shift+d）或 `y`。

---

### Feature 3: 双远端文件互传（Local Relay Transfer）

**方案:** 本机作为中转站，下载到临时目录后上传到目标服务器。复用现有 `TransferService` 的 Download + Upload 能力。

| 需求 | 技术方案 | 现有参考 |
|------|---------|---------|
| 两台服务器连接 | 两个独立 `SFTPClient` 实例 | `sftp_client.go` -- 每个 `Connect()` 创建独立 SSH 进程 |
| 下载到临时目录 | `os.CreateTemp` + `os.MkdirTemp` | `transfer_service.go:474-480` -- CopyRemoteFile 模式 |
| 上传到目标服务器 | 复用 `UploadFile`/`UploadDir` | `transfer_service.go:51-110` |
| 进度显示 | 分阶段 TransferProgress | `transfer_service.go:467-503` -- CopyRemoteFile 两阶段进度 |
| 取消传播 | `context.Context` | 全项目统一模式 |
| 冲突处理 | `domain.ConflictHandler` | `file_browser.go:565-623` -- `buildConflictHandler` |
| 临时文件清理 | `defer os.Remove` / `defer os.RemoveAll` | `transfer_service.go:480` -- `defer func() { _ = os.Remove(tmpPath) }()` |

**设计决策: 为什么不用 SSH ProxyJump 直传**

| 方案 | 优点 | 缺点 | 结论 |
|------|------|------|------|
| **SSH ProxyJump 直传** | 速度更快（不经过本机磁盘） | 需要 Go SSH 库（违反"零外部依赖"原则），无法显示中转进度，不支持所有 SSH config 组合 | **不采用** |
| **本机临时文件中转** | 复用现有 SFTP + TransferService 架构，零新依赖，进度可分阶段显示 | 磁盘空间临时占用，速度稍慢（双倍传输量） | **采用** |

**设计决策: 两个独立 SFTPClient 实例**

当前架构中 `SFTPService` 是单例（`cmd/main.go:60`），通过 `tui` 传递到 `FileBrowser`。双远端互传需要两个独立连接。

方案：
1. **在双远端文件浏览器中创建第二个 SFTPClient** -- 不经过 `cmd/main.go` 的单例
2. `DualRemoteBrowser` 直接 `sftp_client.New(log)` 创建第二个实例
3. 关闭时清理两个连接

```go
// DualRemoteBrowser holds two independent SFTP connections
type DualRemoteBrowser struct {
    *tview.Flex
    app            *tview.Application
    log            *zap.SugaredLogger
    leftSFTP       ports.SFTPService  // 第一个服务器连接
    rightSFTP      ports.SFTPService  // 第二个服务器连接（新创建）
    leftServer     domain.Server
    rightServer    domain.Server
    // ...
}
```

**TransferService 适配问题:**

当前 `TransferService` 接口绑定到一个 `SFTPService` 实例（`transfer_service.go:34-37`）。双远端互传需要：

- 从 `rightSFTP` 下载到临时文件
- 上传到 `leftSFTP`

方案 A: 创建两个 TransferService 实例（各绑定一个 SFTPService）
- 问题: `CopyRemoteFile` 内部用同一个 `sftp` 实例做下载和上传，不支持跨实例

方案 B: 在 UI 层编排两阶段传输（推荐）
- Phase 1: 用 `rightSFTP` 创建的 TransferService 下载到临时目录
- Phase 2: 用 `leftSFTP` 创建的 TransferService 上传到目标
- 这正是 `CopyRemoteFile` 的实现模式（`transfer_service.go:467-503`），只是拆分到两个 TransferService 实例

```go
// 伪代码: 双远端文件传输
func (drb *DualRemoteBrowser) transferFile(ctx context.Context, srcPath, dstPath string) {
    tmpFile, _ := os.CreateTemp("", "lazyssh-dual-remote-*")
    tmpPath := tmpFile.Name()
    tmpFile.Close()
    defer os.Remove(tmpPath)

    // Phase 1: 从源服务器下载
    downloadSvc := transfer.New(log, rightSFTP)
    downloadSvc.DownloadFile(ctx, srcPath, tmpPath, progressCb("Downloading: "), nil)

    // Phase 2: 上传到目标服务器
    uploadSvc := transfer.New(log, leftSFTP)
    uploadSvc.UploadFile(ctx, tmpPath, dstPath, progressCb("Uploading: "), conflictHandler)
}
```

**UI 入口设计:**

双远端文件浏览器需要一个服务器选择流程：
1. 用户在服务器列表按 `X` 键（或其他未占用键）进入双远端模式
2. 第一个服务器选择 -- 当前选中的服务器作为左侧
3. 第二个服务器选择 -- 弹出服务器列表让用户选择右侧服务器
4. 打开双远端文件浏览器（左面板=服务器A，右面板=服务器B）
5. 传输方向由用户在左/右面板选中文件后按 Enter 决定

**Port 接口变更:**

无需修改任何现有 Port 接口。`TransferService` 保持不变，通过组合两个实例实现双远端传输。

`SFTPService` 也无需修改 -- 它已经支持独立的 `Connect()`/`Close()` 生命周期。

---

## UI Components (tview/tcell)

| UI Component | Technology | Pattern Reference |
|-------------|------------|-------------------|
| 本地路径历史弹出 | 复用 RecentDirs overlay 模式 | `recent_dirs.go` -- 仅需替换数据源 |
| Dup 确认/alias 编辑 | `InputDialog` overlay | `input_dialog.go` -- 已有模式 |
| 双远端文件浏览器 | 复用 FileBrowser 双栏布局 | `file_browser.go` -- 替换左面板为 RemotePane |
| 双远端服务器选择 | `tview.List` 弹窗 | `server_list.go` -- 弹出式服务器选择列表 |
| 分阶段进度 | 复用 TransferModal | `transfer_modal.go` -- 两阶段进度已验证 |

---

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| Go SSH 原生库 (`golang.org/x/crypto/ssh`) | 违反"零外部依赖"原则，且无法复用用户 SSH config（密钥、agent、known_hosts） | 系统 `ssh` + `pkg/sftp`（当前方案） |
| `ssh -W` 端口转发直传 | 无法在传输过程中显示进度，取消传播复杂 | 本机临时文件中转（清晰的两阶段进度） |
| 新的 UI 框架 | 项目约束"基于 tview/tcell" | 现有 tview/tcell 组件 |
| 数据库存储路径历史 | 过度工程，路径历史是简单 MRU 列表 | JSON 文件（与 metadata.json 模式一致） |
| `DuplicateServer` Port 方法 | 本质是 Get+Modify+Add，无需新接口方法 | UI 层组合 `ListServers` + `AddServer` |
| 修改现有 `SFTPService` 接口 | 双远端通过组合两个独立实例实现，无需修改接口 | 创建第二个 `sftp_client.New(log)` 实例 |

---

## Dependency Summary

| Category | Before v1.3 | After v1.3 | Change |
|----------|-------------|------------|--------|
| External Go deps | pkg/sftp v1.13.10, tview, tcell/v2, cobra, zap, clipboard, ssh_config, runewidth, uniseg | **完全相同** | **零变更** |
| New Go packages | -- | `os`, `path/filepath`, `encoding/json`, `fmt`, `strings` | 全部标准库，已在项目中使用 |
| New tview widgets | -- | **无** | 复用现有 Table, InputField, Modal |
| New interfaces | FileService, SFTPService, TransferService | **无新增** | 零接口变更 |
| New adapter types | -- | `DualRemoteBrowser` (UI), `LocalPathHistory` (data) | 内部类型，非外部依赖 |

**总新增外部依赖: 0 个。**

---

## Integration Points Summary

### Feature 1: 本地路径历史持久化

| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `internal/adapters/data/local_fs/local_path_history.go` | **新增** | LocalPathHistory 数据层（load/save/record/getPaths） |
| `internal/adapters/ui/file_browser/local_path_history.go` | **新增** | LocalPathHistory overlay UI（Draw/HandleKey/Show/Hide） |
| `internal/adapters/ui/file_browser/file_browser.go` | 修改 | 构造函数中创建 LocalPathHistory，成功传输后 Record |
| `internal/adapters/ui/file_browser/file_browser_handlers.go` | 修改 | 添加 `l` 键处理（本地面板弹出路径历史） |
| `internal/adapters/ui/file_browser/local_pane.go` | 可能修改 | 暴露 GetCurrentPath 给历史记录 |

### Feature 2: Dup SSH 连接

| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `internal/adapters/ui/handlers.go` | 修改 | 添加 `D` 键处理（`handleServerDuplicate`） |
| `internal/adapters/ui/file_browser/file_browser_handlers.go` | 无变更 | Dup 功能在服务器列表层，不在文件浏览器层 |

### Feature 3: 双远端文件互传

| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `internal/adapters/ui/file_browser/dual_remote_browser.go` | **新增** | 双远端文件浏览器（双 RemotePane 布局） |
| `internal/adapters/ui/handlers.go` | 修改 | 添加 `X` 键处理（进入双远端模式） |
| `internal/adapters/data/transfer/transfer_service.go` | 可能修改 | 或不修改 -- 视是否需要新方法（如 RelayTransferFile） |
| `internal/adapters/ui/file_browser/transfer_modal.go` | 可能修改 | 支持分阶段标签（"下载中..." -> "上传中..."） |

---

## Existing Stack (Preserved)

| Component | Technology | Version |
|-----------|-----------|---------|
| Language | Go | 1.24.6 |
| TUI Framework | tview/tcell | tview latest, tcell/v2 2.9.0 |
| CLI | Cobra | 1.9.1 |
| Logging | Zap | 1.27.0 |
| SSH Config | ssh_config (forked) | 1.4.0 |
| SFTP Client | pkg/sftp | v1.13.10 |

---

## Sources

- **HIGH confidence (项目源码直接分析):**
  - `internal/core/ports/transfer.go` -- TransferService 接口，确认 Upload/Download 方法签名
  - `internal/core/ports/file_service.go` -- SFTPService 接口，确认 Connect/Close 独立生命周期
  - `internal/core/ports/services.go` -- ServerService 接口，确认 AddServer 方法可用
  - `internal/core/ports/repositories.go` -- ServerRepository 接口
  - `internal/core/domain/server.go` -- Server 实体，确认值拷贝语义和所有字段
  - `internal/core/domain/transfer.go` -- TransferProgress/ConflictHandler 类型
  - `internal/adapters/data/ssh_config_file/metadata_manager.go` -- JSON 持久化模式
  - `internal/adapters/data/transfer/transfer_service.go` -- CopyRemoteFile 两阶段中转模式
  - `internal/adapters/data/sftp_client/sftp_client.go` -- SFTPClient 独立实例创建
  - `internal/adapters/ui/file_browser/recent_dirs.go` -- MRU overlay 模式
  - `internal/adapters/ui/file_browser/file_browser.go` -- FileBrowser 构造和传输编排
  - `internal/adapters/ui/file_browser/file_browser_handlers.go` -- 快捷键分配和 overlay chain
  - `internal/adapters/ui/handlers.go` -- 主界面快捷键，确认 `d` 已被 delete 占用
  - `internal/adapters/ui/tui.go` -- DI 容器和依赖注入结构
  - `cmd/main.go` -- 应用启动和依赖组装
  - `go.mod` -- 依赖列表确认

---
*Stack research: 2026-04-15 (v1.3 Enhanced File Browser)*
*Previous: 2026-04-15 (v1.2 File Management Operations)*
