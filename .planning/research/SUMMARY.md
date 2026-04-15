# Project Research Summary

**Project:** LazySSH File Transfer
**Domain:** TUI SSH Manager -- Enhanced File Browser (v1.3)
**Researched:** 2026-04-15
**Confidence:** HIGH

## Executive Summary

v1.3 为 lazyssh 的终端文件浏览器添加三个独立增强功能：本地路径历史持久化、Dup SSH 连接复制、双远端文件互传。所有三个功能都基于现有技术栈实现，**零新外部依赖**。核心发现是这三个功能完全独立，可以按任意顺序构建，但按复杂度递增排序（Dup -> 本地路径 -> 双远端）能最大化早期交付并降低风险。

关键架构决策已经明确：(1) 本地路径历史需要独立于现有 `RecentDirs` 的数据层（全局而非按服务器），存储在 `~/.lazyssh/local-path-history.json`；(2) Dup SSH 因为 `d` 键已被删除功能占用，必须使用不同快捷键（推荐 `D` 或 `y`）；(3) 双远端传输需要全新的 `DualRemoteFileBrowser` 组件和 `RelayTransferService`，而非修改现有 `FileBrowser`。最大风险是双远端传输的架构复杂度——同时管理两个 SFTP 连接、本地临时文件中转、分阶段进度显示。所有风险都有明确的缓解策略，基于 v1.0-v1.2 的实际踩坑经验。

主要风险在于双远端传输的本地磁盘空间需求（下载到临时目录再上传）和 SSH 密码认证导致的 goroutine 阻塞。这些风险都有成熟缓解方案：传输前检查磁盘空间、设置连接超时、确保进程清理。`scp -3` 方案因无进度条被排除，确认采用 download-to-temp + re-upload 的两阶段方案。

## Key Findings

### Recommended Stack

**零新外部依赖。** 三个功能所需的所有技术原语已在当前技术栈中：

- **Go 标准库** (`encoding/json`, `os`, `path/filepath`) — 本地路径历史持久化，复用现有 `metadata.json` 模式
- **现有 TransferService 两阶段模式** (`CopyRemoteFile`) — 双远端传输的基础模式，download-to-temp + re-upload
- **独立 SFTPClient 实例** — 双远端需要两个独立 `sftp_client.New(log)` 连接，不经过 `cmd/main.go` 单例
- **tview overlay 组件模式** (`RecentDirs`, `InputDialog`) — 所有 UI 交互复用已有模式

### Expected Features

**Must have (table stakes):**
- 本地路径历史持久化 — JSON 存储，MRU 弹出列表，`r` 键（本地面板）
- Dup SSH 连接 — 一键复制服务器配置，alias 去重，metadata 清除
- 双远端文件浏览器 — 两台服务器的双 RemotePane 布局
- 双远端文件传输 — download-to-temp + re-upload，分阶段进度显示，取消支持

**Should have (competitive):**
- 分阶段进度显示（Phase 1/2: Download, Phase 2/2: Upload）— 比 `scp -3`（无进度条）的显著优势
- 键盘驱动的服务器选择流程 — 比 Termius/MC 的鼠标操作更适合终端用户
- Alias 递增后缀策略 — 连续 Dup 同一服务器不会产生冲突

**Defer (v2+):**
- 流式中转（边下载边上传）— 减少磁盘占用但增加复杂度
- 双远端文件管理（删除/重命名/mkdir）— v1.x 考虑
- Named bookmarks（命名书签）— MRU 已足够
- Dup 字段对比编辑器 — 复制后用 `e` 编辑即可

### Architecture Approach

三个功能对应三个架构层级的影响：(1) 本地路径历史需要新的 Port 接口 (`PathHistoryService`) 和数据适配器 + UI overlay 组件；(2) Dup SSH 是纯 TUI 层功能，零 Port/Adapter 变更，仅 ~50 行 handler 代码；(3) 双远端传输需要全新的 `RelayTransferService` 接口和 `DualRemoteFileBrowser` 根组件，是 v1.3 中最大的架构变更。

关键设计原则：不修改现有 `FileBrowser`（双远端用独立组件）、不修改现有 `TransferService`（中转用独立 service）、不在 `cmd/main.go` 创建 RelayTransferService（运行时动态创建，因为服务器对由 UI 选择决定）。

**Major components:**
1. **PathHistoryService** — 本地路径 MRU 的纯数据层 Port，JSON 持久化到 `~/.lazyssh/local-path-history.json`
2. **LocalRecentDirs** — 本地面板的路径历史弹出 overlay，复用 RecentDirs 的 Draw/HandleKey 模式
3. **handleServerDup** — Dup SSH handler，组合 `ListServers` + 修改 alias + `AddServer`
4. **RelayTransferService** — 双远端传输服务 Port，接收两个独立 SFTP 连接，编排两阶段传输
5. **DualRemoteFileBrowser** — 新的根 UI 组件，两个 RemotePane + TransferModal，独立于 FileBrowser
6. **ServerPickerOverlay** — 服务器选择弹出层，复用 overlay 模式，支持过滤和键盘导航

### Critical Pitfalls

1. **本地路径历史模型与 RecentDirs 不一致** — `RecentDirs` 按服务器维度持久化（`user@host`），本地路径是全局的。必须创建独立的 `PathHistory` 数据层，不复用 `RecentDirs` 的 serverKey 逻辑。
2. **Dup 的 `d` 键冲突** — `d` 已绑定到 `handleServerDelete()`。Dup 必须使用 `D`（Shift+d）或 `y`。推荐 `y`（vim yank 语义）或 `D`（与 `s`/`S` 排序模式一致）。
3. **双远端传输不能复用 FileBrowser** — FileBrowser 硬编码为 LocalPane + RemotePane 模式，且持有单个 SFTPService。双远端需要两个 RemotePane 和两个独立连接，必须创建新组件 `DualRemoteFileBrowser`。
4. **本地临时文件磁盘空间耗尽** — 大文件双远端传输需要本地磁盘空间 >= 文件大小。需要在传输前检查可用空间，失败时 `defer os.RemoveAll()` 清理临时文件。
5. **SSH 密码认证阻塞 goroutine** — 密码认证的服务器在 `cmd.Start()` 后等待输入，导致 SFTP 连接永远挂起。需要设置连接超时（10s）和 `cmd.Process.Kill()` 进程清理。

## Implications for Roadmap

Based on research, suggested phase structure:

### Phase 1: Dup SSH Connection
**Rationale:** 最低复杂度（~50 行，单文件修改），零架构风险，快速交付建立信心。所有依赖都存在于现有 `ServerService.AddServer()` 中。
**Delivers:** 服务器列表中 `D` 键一键复制配置，alias 自动去重后缀，清除非配置类 metadata（PinnedAt、SSHCount），复制后可 `e` 编辑。
**Addresses:** Dup SSH Connection (FEATURES.md)
**Avoids:** P2 (`d` 键冲突 -- 使用 `D`)，P6 (metadata 继承 -- 清除 PinnedAt/SSHCount/LastSeen)，P9 (alias 冲突 -- 递增后缀)

### Phase 2: Local Path History Persistence
**Rationale:** 扩展现有 `RecentDirs` 模式，新增 Port + Adapter + UI overlay，架构变更明确可控。按 ARCHITECTURE.md 估算约 300 行新代码 + 20 行修改。
**Delivers:** 本地面板 `r` 键弹出路径历史，JSON 持久化（`~/.lazyssh/local-path-history.json`），上传/下载成功后自动记录，最多 20 条 MRU。
**Addresses:** Persistent Local Path History (FEATURES.md)
**Avoids:** P1 (模型不一致 -- 独立 PathHistory 数据层)，P8 (路径规范化 -- `filepath.Clean()`)，P12 (已删除路径 -- 显示警告但不移除)
**Uses:** STACK.md 中的 JSON 持久化模式

### Phase 3: Dual-Remote File Browser
**Rationale:** 最大架构复杂度，需要新 Port (`RelayTransferService`)、新 Adapter (`relay_transfer_service.go`)、新根组件 (`DualRemoteFileBrowser`)、新 overlay (`ServerPickerOverlay`)。放在最后是因为实现者能从 Phase 1/2 中积累 overlay 和 handler 模式经验。
**Delivers:** 服务器列表 `M` 键进入双远端模式，两步选择服务器对，双 RemotePane 文件浏览器，`RelayTransferService` 两阶段传输（download-to-temp + re-upload），分阶段进度显示（Phase 1/2, Phase 2/2），取消支持，临时文件自动清理。
**Addresses:** Dual-Remote File Transfer (FEATURES.md)
**Avoids:** P3 (单连接限制 -- 独立 DualRemoteFileBrowser)，P4 (磁盘空间 -- 传输前检查)，P5 (密码阻塞 -- 连接超时)，P7 (进度重置困惑 -- Phase 1/2 标签)，P10 (入口 UX -- 两步选择模式)，P11 (并发问题 -- 独立 SFTPClient 实例)
**Implements:** RelayTransferService 架构组件

### Phase Ordering Rationale

- **Dup 先行** — 最简单，零依赖，快速建立 v1.3 交付节奏。handler 代码可被双远端 phase 参考模式。
- **本地路径居中** — 扩展现有模式（RecentDirs），有明确的 Port + Adapter + UI 三层参考实现。为 DualRemoteFileBrowser 的 overlay 模式积累经验。
- **双远端最后** — 架构最复杂（新 service + 新组件 + 修改 TransferModal），放在最后能确保前两个 phase 的代码变更已稳定，减少集成冲突风险。
- **所有三个功能完全独立** — 没有代码依赖关系，可以并行开发。排序仅基于复杂度递增和信心积累。

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 3 (Dual-Remote):** `RelayTransferService` 的目录传输编排逻辑需要详细设计——`WalkDir` 获取文件列表后如何分阶段传输和追踪失败文件。`TransferModal.modeRelay` 的 Draw 逻辑需要 UI 原型验证。`ServerPickerOverlay` 的交互流程需要设计评审。
- **Phase 2 (Local Path History):** 本地路径最大条目数（20 vs 10）的权衡需要在实现时验证。`LocalRecentDirs` 是否需要 `currentPath` 高亮功能（本地面板的当前路径始终已知，可能不需要）。

Phases with standard patterns (skip research-phase):
- **Phase 1 (Dup SSH):** 完全复用现有 `AddServer` + `validateServer` + InputDialog 模式，无新架构。handler 逻辑约 50 行，模式明确。

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | 基于项目源码直接分析，所有技术原语已确认存在于当前代码库中，零新外部依赖 |
| Features | MEDIUM-HIGH | 双远端传输参考了 mc/lf/yazi/Termius 等竞品和 SCP 手册；`scp -3` 无进度条限制已确认；Dup 和本地路径基于明确的用户需求和竞品验证 |
| Architecture | HIGH | 基于直接代码分析（ports、adapters、file_browser、transfer_service），所有文件路径和方法签名已确认；新组件设计有明确的现有模式参考 |
| Pitfalls | HIGH | 基于现有代码深度审查 + v1.0-v1.2 实际踩坑经验；13 个 pitfall 的预防策略已明确到代码层面 |

**Overall confidence:** HIGH

### Gaps to Address

- **RelayTransferService 目录传输的具体实现路径**：`WalkDir` 在远程服务器上的 API 调用方式、文件列表缓存策略、失败文件的汇总报告格式——需要在 Phase 3 规划时详细设计。
- **`ServerPickerOverlay` 是否复用现有 ServerList 组件**：架构研究中建议新建 overlay，但 `server_list.go` 本身是 `tview.Table` 的封装。是否抽取公共表格渲染逻辑以避免代码重复，需要在 Phase 3 规划时决定。
- **TransferModal.modeRelay 的进度条重置交互**：阶段切换时进度条从 0 重新开始，用户可能困惑。是否需要额外的 "Phase complete" 过渡动画，需要在 UI 原型中验证。
- **Dup 的 Tags 处理**：STACK.md 建议"保留 Tags"，但 ARCHITECTURE.md 建议"清空 Tags"。两种方案都合理，需要在 Phase 1 实现时做出最终决策。

## Sources

### Primary (HIGH confidence)
- 项目源码直接分析 — `internal/core/ports/`, `internal/core/domain/`, `internal/adapters/data/`, `internal/adapters/ui/file_browser/`, `cmd/main.go`
- `scp -3` 手册验证 — 确认 relay 模式无进度条输出
- 现有 CopyRemoteFile/CopyRemoteDir 模式 — 确认两阶段传输已验证可用
- v1.0-v1.2 实际踩坑经验 — overlay 绘制链、goroutine + QueueUpdateDraw、快捷键冲突

### Secondary (MEDIUM confidence)
- Midnight Commander — 双远端面板 via VFS/SFTP link（需要 in-app SSH library，与 lazyssh 约束冲突）
- lf 文档 — 路径历史持久化模式（`~/.local/share/lf/history`），已知并发写入 bug（Issue #1450）
- Termius — 双 SFTP 面板和服务器复制功能的 UX 参考
- yazi — Session-based path history + 社区书签插件（yamb.yazi）

### Tertiary (LOW confidence)
- `scp -3` 无进度条的具体限制 — 基于 StackOverflow 讨论和实践经验，未找到 OpenSSH 官方文档明确说明
- 流式中转（pipe download to upload）的可行性 — 理论上可行但复杂度高，未实际验证

---
*Research completed: 2026-04-15*
*Ready for roadmap: yes*
