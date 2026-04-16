# LazySSH File Transfer

## What This Is

为 lazyssh（终端 SSH 管理器）添加内置的双栏文件传输和管理功能。用户在服务器列表中选中服务器后，按快捷键打开双栏文件浏览器（本地 vs 远程），支持上传/下载文件和目录、删除/重命名/新建/复制/移动文件操作，提供详细的传输进度显示。底层复用系统 SCP/SFTP 命令，保持 lazyssh "不引入新安全风险" 的原则。

## Core Value

在终端内完成 SSH 文件传输和文件管理，无需切换到 FileZilla 或记忆 scp 命令——选中服务器、选文件、操作，全部键盘驱动。

## Current Milestone: v1.4 Dup Fix & Dual Remote Transfer

**Goal:** 修复 Dup 行为 + 支持两个远端服务器之间的文件互传

**Target features:**
- Dup 修复：D 键复制后直接出现在列表，不自动打开表单
- 双远端互传：T 键标记两个服务器，自动打开双远端文件浏览器
- 双远端浏览器中支持复制/移动文件（复用 c/x + p 机制）

## Current State

v1.4 shipped 2026-04-16 — Dup 修复 + 双远端文件互传功能完整。
Phase 13 complete — 跨远端文件中继传输（RelayTransferService），c/x+p 剪贴板操作，F5 快速传输，两阶段进度显示，冲突处理，移动回滚。

**已交付功能：**
- 双栏文件浏览器（本地 vs 远程）
- 文件上传/下载（含目录递归）
- 传输进度显示（进度条、速度、剩余时间）
- 文件冲突处理（覆盖/跳过/重命名）
- 最近远程目录快速跳转
- 文件管理操作：删除/重命名/新建目录/复制/移动/冲突对话框

## Requirements

### Validated

- ✓ 服务器列表展示和导航 — existing
- ✓ SSH 连接管理（快速连接、命令复制）— existing
- ✓ 端口转发（Local/Remote/Dynamic）— existing
- ✓ SSH 配置管理（增删改查、备份、非破坏性写入）— existing
- ✓ SSH config 解析和写入（保留注释和格式）— existing
- ✓ 服务器搜索（别名、IP、标签模糊搜索）— existing
- ✓ 服务器置顶和排序 — existing
- ✓ Ping 检测 — existing
- ✓ 标签管理 — existing
- ✓ 密钥自动补全 — existing
- ✓ 跨平台支持（Linux/Windows/Darwin）— existing
- ✓ 双栏文件浏览器 UI（左侧本地、右侧远程）— v1.0
- ✓ 本地文件/目录浏览（遍历本地文件系统）— v1.0
- ✓ 快捷键入口（在服务器列表按 F 键触发）— v1.0
- ✓ 远程文件/目录浏览（通过 SFTP 列出远程目录）— v1.0
- ✓ 文件上传（本地→远程）— v1.0
- ✓ 文件下载（远程→本地）— v1.0
- ✓ 目录递归传输（支持整个目录上传/下载）— v1.0
- ✓ 传输进度显示（进度条、速度、剩余时间）— v1.0
- ✓ 文件冲突处理（覆盖/跳过/重命名询问）— v1.0
- ✓ 传输取消（中途取消正在进行的传输）— v1.0
- ✓ 跨平台文件权限（Windows/macOS/Linux）— v1.0
- ✓ 取消后部分文件清理（D-04）— v1.0
- ✓ 最近远程目录记录（MRU 10 条，仅内存）— v1.1
- ✓ 最近目录弹出列表（`r` 键，j/k 导航，Enter 跳转）— v1.1
- ✓ 当前路径黄色高亮 + 空状态提示 — v1.1
- ✓ 文件/目录删除（双面板，d 键，确认对话框，递归删除）— v1.2
- ✓ 文件/目录重命名（双面板，R 键，冲突检测）— v1.2
- ✓ 新建目录（双面板，m 键，光标定位）— v1.2
- ✓ 文件/目录复制（c 标记 + p 粘贴，绿色 [C] 前缀）— v1.2
- ✓ 文件/目录移动（x 标记 + p 粘贴，红色 [M] 前缀，copy+delete）— v1.2
- ✓ 粘贴冲突对话框（覆盖/跳过/重命名，所有粘贴操作）— v1.2
- ✓ Dup SSH 连接（D 键复制服务器配置，唯一别名，清除元数据）— v1.3

### Validated

- ✓ Dup 修复：D 键复制后直接保存并返回列表，移除 ServerForm 中间步骤 — v1.4, Phase 10
- ✓ T 键标记服务器（源端 [S]/目标端 [T]），Esc 清除标记，同服务器防护 — v1.4, Phase 11
- ✓ 双远端文件浏览器（左栏远端 A，右栏远端 B，并行 SFTP 连接，Tab 切换焦点）— v1.4, Phase 12

- ✓ 双远端之间文件复制/移动（download A → temp → upload B，两阶段进度，冲突处理，移动回滚）— v1.4, Phase 13
- ✓ F 键仍保留本地+远端文件浏览器（现有行为不变）— v1.4

### Out of Scope

- Go 原生 SSH 库实现 — 底层使用系统 scp/sftp 命令，保持与现有架构一致
- 文件编辑 — 只做传输和管理，不做远程文件内容编辑
- 断点续传 — 复杂度高，scp 不原生支持
- 多文件并行传输 — v1 单线程传输，保持简单
- 传输历史记录 — 后续版本考虑
- 路径缩写显示 — v1.x
- 数字键快速选择 — v1.x
- 持久化书签/收藏夹 — v2+
- 文件搜索/过滤 — 独立功能，适合单独 milestone
- 拖拽排序 — v2+
- 文件属性编辑（chmod/chown）— v2+
- 符号链接创建和管理 — v2+
- 撤销操作 — 实现复杂度高，需要操作日志和逆操作链
- 本地路径历史持久化 — 延迟到后续版本

## Context

lazyssh 是一个 Go 编写的终端 SSH 管理器，采用 Clean Architecture + 六边形设计。核心依赖 tview/tcell 构建 TUI，通过调用系统 ssh 命令实现连接。

**版本历史：**
- v1.0 (2026-04-13): 文件传输核心功能 — 双栏浏览器、上传/下载、进度显示、冲突处理 (3 phases, 9 plans)
- v1.1 (2026-04-14): 最近远程目录快速跳转 — MRU 记录 + 弹出列表 (2 phases, 3 plans)
- v1.2 (2026-04-15): 文件管理操作 — 删除/重命名/新建/复制/移动/冲突对话框 (3 phases, 7 plans)
- v1.3 (2026-04-15): Dup SSH 连接 — 服务器列表快速复制配置创建新条目 (1 phase, 1 plan)
- v1.4 (2026-04-16): Dup 修复 + 双远端文件互传 (4 phases — Dup fix, T key marking, dual remote browser, cross-remote transfer)

**技术栈：** Go 1.24.6, tview/tcell TUI, Cobra CLI, Zap logging, 系统 SSH/SFTP

## Constraints

- **安全原则**: 不引入新的安全风险，复用系统 scp/sftp 命令，不存储/传输/修改密钥
- **跨平台**: 必须在 Linux/Windows/Darwin 上正常工作
- **架构一致**: 遵循现有 Clean Architecture 模式，通过 Port/Adapter 解耦
- **UI 框架**: 基于 tview/tcell 构建，不可引入其他 UI 框架
- **零外部依赖**: 不引入需要额外安装的依赖，sc/sftp 必须是系统自带的

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| 使用系统 scp/sftp 命令 | 与现有 SSH 连接方式一致，不引入新安全风险，跨平台兼容 | ✓ pkg/sftp NewClientPipe via system SSH binary |
| 32KB 缓冲复制循环 | 替代 io.Copy，支持逐块进度回调 | ✓ TransferService 32KB buffer + onProgress callback |
| onFileAction 回调模式 | Pane Enter 事件传递到 FileBrowser 传输编排层 | ✓ local/remote pane → initiateTransfer |
| 双栏浏览器 UI | 最直观的文件传输体验，类似 FileZilla | ✓ tview.Table 50:50 Flex layout |
| 快捷键 F 触发 | 不改变主界面布局，最小化对现有功能的影响 | ✓ case 'F' in handleGlobalKeys |
| 远程浏览通过 SFTP 子命令 | `sftp` 可用于 ls 等操作，无需 Go SSH 库 | ✓ pkg/sftp NewClientPipe via exec.Command("ssh") |
| context.Context 取消传播 | Go 惯用取消模式，32KB chunk 粒度中断 | ✓ TransferService 所有方法接受 ctx 参数 |
| TransferModal 多模式状态机 | 替代 bool 标志，支持 progress/cancelConfirm/conflictDialog/summary 四种模式 | ✓ modalMode enum + HandleKey dispatch |
| 冲突处理 channel 同步 | goroutine 中检测冲突后通过 buffered channel 等待 UI 响应 | ✓ buildConflictHandler → actionCh |
| Build tags 分离平台权限 | Windows 不支持 Unix 权限模型，需要编译时分离 | ✓ permissions_unix.go / permissions_windows.go |
| 快捷键 `r` 弹出最近目录 | 仅远程面板有效，避免与本地面板冲突 | ✓ case 'r' in handleGlobalKeys with activePane==1 |
| 记录粒度为「本机目录 + 服务器」组合 | 避免跨服务器目录列表泄露 | ✓ RecentDirs 实例绑定到 FileBrowser |
| 2-phase 结构（数据层 + UI 层） | 数据结构与 UI 渲染解耦，便于独立测试 | ✓ Phase 4 数据层 + Phase 5 UI 层 |
| RecentDirs 通过 SetCurrentPath 解耦 | 不直接依赖 RemotePane，overlay 组件独立 | ✓ currentPath string parameter |
| Overlay draw chain 修复 | TransferModal.Draw() 从未被调用是预存 bug | ✓ FileBrowser.Draw() 添加 overlay 渲染调用 |
| FileService 统一接口 | 删除/重命名/新建目录方法提升到 FileService（非仅 SFTPService） | ✓ Remove/RemoveAll/Rename/Mkdir/Stat on FileService |
| ConfirmDialog/InputDialog overlay | 独立 overlay 组件，遵循 TransferModal/RecentDirs 模式 | ✓ confirm_dialog.go + input_dialog.go |
| goroutine + QueueUpdateDraw | 所有文件操作异步执行，不阻塞 UI | ✓ handleDelete/handleRename/handleMkdir |
| ClipboardOp 4-tuple | 剪贴板携带操作类型（Copy/Move），区分复制和移动粘贴 | ✓ clipboardProvider returns (bool, string, string, ClipboardOp) |
| 冲突对话框统一化 | 所有粘贴操作（复制/移动/本地/远程）均经过冲突对话框 | ✓ handlePaste wraps all dispatch with buildConflictHandler |
| handleServerDup 直接保存 | D 键复制后直接 AddServer()，移除 ServerForm 中间步骤和 dupPendingAlias 追踪 | ✓ Phase 10 — 直接深拷贝+保存+自动滚动 |
| 独立 SFTPClient 实例 | 双远端浏览器每个面板使用独立 sftp_client.New()，不复用 tui.sftpService | ✓ Phase 12 — 两个独立 SFTP 连接并行建立 |
| 并行 SFTP 连接 | 双远端浏览器两个 SFTP 连接并行 goroutine 建立，失败不影响对方 | ✓ Phase 12 — goroutine + QueueUpdateDraw 错误隔离 |
| RelayTransferService 独立 port | 不污染单连接 TransferService 接口，组合两个 transfer.New() 实例 | ✓ Phase 13 — download→temp→upload 中继模式 |
| modeCrossRemote 两阶段进度 | 复用 TransferModal 现有渲染，dlDone 标志切换下载/上传阶段 | ✓ Phase 13 — combinedProgress + ResetProgress |
| 跨远端剪贴板 c/x+p | 双面板 clipboardProvider 回调，[C]/[M] 前缀，Esc 清除 | ✓ Phase 13 — handleCopy/handleMove/handleCrossRemotePaste |

## Evolution

This document evolves at phase transitions and milestone boundaries.

---
*Last updated: 2026-04-16 — Phase 13 complete (v1.4 shipped)*
