# Research Summary

**Synthesized:** 2026-04-13
**Domain:** TUI File Transfer for Go SSH Manager

## Key Findings

### Stack: pkg/sftp 是关键决策

所有研究维度 converge 到同一个结论：**使用 `pkg/sftp` 而非裸 `os/exec` 调用 sftp/scp**。

- 通过 `NewClientPipe()` 使用系统 SSH 二进制（保持安全原则）
- 提供原生 Go API 用于目录列表、进度追踪、连接复用
- 同时消除 3 个最严重的 pitfall（无进度、文本解析脆弱、每次调用新连接）

### Features: 双栏浏览器是标准 UX 模式

参考 Midnight Commander（1994 年至今的标杆）：
- 左栏 = 本地文件，右栏 = 远程文件
- 键盘驱动：Enter = 传输，Tab = 切换面板，Space = 选择
- Table stakes：目录浏览、排序、隐藏文件切换、进度显示
- 差异化：零配置远程访问（利用已有 SSH config）

### Architecture: 集成面极小

- 仅需修改 4 个现有文件（handlers.go, app.go, main.go, status_bar.go）
- 所有新功能通过新文件实现，遵循现有 Clean Architecture
- 新增组件：FileTransferService, SFTPClient, LocalFS, 双栏 UI

### Pitfalls: `pkg/sftp` 一举解决 3 个 CRITICAL/HIGH 问题

| Pitfall | 严重度 | pkg/sftp 是否解决 |
|---------|--------|-------------------|
| SCP 无 TTY 零进度 | CRITICAL | Yes — 原生进度回调 |
| SFTP ls 文本解析脆弱 | HIGH | Yes — 结构化 FileInfo |
| 每次调用新建连接 | HIGH | Yes — 持久连接 |
| `f` 键冲突 | HIGH | N/A — 改用 `F` |
| 进度回调阻塞 UI | HIGH | N/A — goroutine + QueueUpdateDraw |

## Key Decisions from Research

| Decision | Recommendation | Confidence |
|----------|---------------|------------|
| 传输后端 | `pkg/sftp` (系统 SSH pipe) | HIGH |
| 快捷键 | `F` (Shift+f) — `f` 已被端口转发占用 | HIGH |
| 进度显示 | 自定义 tview primitive | MEDIUM |
| 递归目录 | SFTP Walk + Put/Get，大目录回退 scp -r | MEDIUM |

## Suggested Phase Structure

| Phase | 名称 | 核心内容 |
|-------|------|---------|
| 1 | Foundation | Ports 接口 + SFTP 适配器 + 本地 FS 适配器 + 双栏 UI 骨架 |
| 2 | Core Transfer | 文件传输服务、目录浏览、单文件传输、进度显示 |
| 3 | Polish | 递归目录传输、冲突处理、取消支持、跨平台、边界情况 |

## Watch Out For

- Windows OpenSSH 行为差异（路径分隔符、符号链接、权限）
- tview 单线程模型 — 所有 UI 更新必须通过 QueueUpdateDraw
- 大目录列表需要分页/懒加载
- 传输取消后需清理部分文件

---
*Research summary: 2026-04-13*
