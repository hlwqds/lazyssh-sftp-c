# LazySSH File Transfer

## What This Is

为 lazyssh（终端 SSH 管理器）添加内置的双栏文件传输功能。用户在服务器列表中选中服务器后，按快捷键打开双栏文件浏览器（本地 vs 远程），支持上传/下载文件和目录，提供详细的传输进度显示。底层复用系统 SCP/SFTP 命令，保持 lazyssh "不引入新安全风险" 的原则。

## Core Value

在终端内完成 SSH 文件传输，无需切换到 FileZilla 或记忆 scp 命令——选中服务器、选文件、传输，全部键盘驱动。

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

## Current Milestone: v1.1 Recent Remote Directories

**Goal:** 在文件浏览器的远程面板中，记录并快速重新访问最近浏览过的远程目录。

**Target features:**
- 按 `r` 键弹出最近访问的远程目录列表（最近 10 条）
- 记录粒度为「本机目录 + 服务器」组合
- 弹出式列表交互（j/k 选择，Enter 跳转，Esc 关闭）
- 仅内存中保存，退出后清空

### Active

- 最近远程目录记录（按本机目录 + 服务器分组，最多 10 条）
- 快捷键 `r` 弹出历史目录列表
- 弹出式列表交互（j/k 导航，Enter 跳转，Esc 关闭）

### Out of Scope

- Go 原生 SSH 库实现 — 底层使用系统 scp/sftp 命令，保持与现有架构一致
- 文件编辑 — 只做传输，不做远程文件内容编辑
- 断点续传 — 复杂度高，scp 不原生支持
- 多文件并行传输 — v1 单线程传输，保持简单
- 传输历史记录 — 后续版本考虑

## Context

lazyssh 是一个 Go 编写的终端 SSH 管理器，采用 Clean Architecture + 六边形设计。核心依赖 tview/tcell 构建 TUI，通过调用系统 ssh 命令实现连接。

v1.0 已完成文件传输功能：双栏文件浏览器、SFTP 上传/下载、目录递归传输、进度显示、取消支持、冲突处理、跨平台兼容。技术栈 ~14,490 行 Go 代码，23 个单元测试。

文件传输功能是 README 中标记为 "Upcoming" 的功能，现已实现。

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

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd:transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd:complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-04-14 — v1.1 milestone started*
