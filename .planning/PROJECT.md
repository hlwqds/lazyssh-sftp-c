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

### Active

- [ ] 双栏文件浏览器 UI（左侧本地、右侧远程）
- [ ] 本地文件/目录浏览（遍历本地文件系统）
- [ ] 远程文件/目录浏览（通过 SFTP 列出远程目录）
- [ ] 文件上传（本地→远程）
- [ ] 文件下载（远程→本地）
- [ ] 目录递归传输（支持整个目录上传/下载）
- [ ] 传输进度显示（进度条、速度、剩余时间）
- [ ] 文件冲突处理（覆盖/跳过/重命名询问）
- [ ] 传输取消（中途取消正在进行的传输）
- [ ] 快捷键入口（在服务器列表按 f 键触发）

### Out of Scope

- Go 原生 SSH 库实现 — 底层使用系统 scp/sftp 命令，保持与现有架构一致
- 文件编辑 — 只做传输，不做远程文件内容编辑
- 断点续传 — 复杂度高，scp 不原生支持
- 多文件并行传输 — v1 单线程传输，保持简单
- 传输历史记录 — 后续版本考虑

## Context

lazyssh 是一个 Go 编写的终端 SSH 管理器，采用 Clean Architecture + 六边形设计。核心依赖 tview/tcell 构建 TUI，通过调用系统 ssh 命令实现连接。现有代码结构清晰，分为 Domain/Ports/Services/Adapters 四层。

文件传输功能是 README 中标记为 "Upcoming" 的功能之一，用户普遍期望在 SSH 管理工具中集成文件传输能力。

## Constraints

- **安全原则**: 不引入新的安全风险，复用系统 scp/sftp 命令，不存储/传输/修改密钥
- **跨平台**: 必须在 Linux/Windows/Darwin 上正常工作
- **架构一致**: 遵循现有 Clean Architecture 模式，通过 Port/Adapter 解耦
- **UI 框架**: 基于 tview/tcell 构建，不可引入其他 UI 框架
- **零外部依赖**: 不引入需要额外安装的依赖，sc/sftp 必须是系统自带的

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| 使用系统 scp/sftp 命令 | 与现有 SSH 连接方式一致，不引入新安全风险，跨平台兼容 | — Pending |
| 双栏浏览器 UI | 最直观的文件传输体验，类似 FileZilla | — Pending |
| 快捷键 f 触发 | 不改变主界面布局，最小化对现有功能的影响 | — Pending |
| 远程浏览通过 SFTP 子命令 | `sftp` 可用于 ls 等操作，无需 Go SSH 库 | — Pending |

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
*Last updated: 2026-04-13 after initialization*
