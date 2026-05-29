# Phase 12: Dual Remote Browser - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-16
**Phase:** 12-dual-remote-browser
**Areas discussed:** Layout & visual design, In-pane file operations scope, Connection handling, Status bar & key hints

---

## Layout & Visual Design

| Option | Description | Selected |
|--------|-------------|----------|
| 50:50 + header bar | 两栏各占 50%，上方标题行显示服务器信息 | ✓ |
| 60:40 asymmetric | 源端 60%，目标端 40% | |
| 50:50 + no header | 全屏，无标题，服务器信息在状态栏 | |

**User's choice:** 50:50 + header bar
**Notes:** 与现有 FileBrowser 布局一致，上方 header bar 显示服务器别名和 IP。

| Option | Description | Selected |
|--------|-------------|----------|
| Server alias + IP | 面板顶部显示 "Source: myserver (1.2.3.4)" | ✓ |
| Server alias only | 仅显示别名 | |
| No labels | 不区分标签 | |

**User's choice:** Server alias + IP

| Option | Description | Selected |
|--------|-------------|----------|
| Highlight border | 活跃面板用高亮边框标识 | ✓ |
| No visual distinction | 仅通过光标位置判断 | |

**User's choice:** Highlight border

| Option | Description | Selected |
|--------|-------------|----------|
| Same as FileBrowser | 4 列：Name, Size, Modified, Permissions | ✓ |
| Simplified 2-column | Name + Size | |
| You decide | Claude 决定 | |

**User's choice:** Same as FileBrowser (4-column)

---

## In-Pane File Operations Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Navigation only | 仅浏览导航 | |
| Delete/Rename/Mkdir | 复用 Phase 6 overlay 组件 | ✓ |
| Same-server copy/move | c/x + p 同服务器内操作 | |
| No file ops in Phase 12 | 全部推迟到 Phase 13 | |

**User's choice:** Delete/Rename/Mkdir（导航是必须的，加上文件操作）
**Notes:** 同服务器复制/移动属于 Phase 13 跨远端传输范围。

| Option | Description | Selected |
|--------|-------------|----------|
| Reuse existing overlays | 复用 ConfirmDialog/InputDialog | ✓ |
| New dedicated overlays | 创建专用 overlay | |

**User's choice:** Reuse existing overlays

---

## Connection Handling

| Option | Description | Selected |
|--------|-------------|----------|
| Parallel connect | 同时建立两个 SFTP 连接 | ✓ |
| Sequential connect | 先源端后目标端 | |

**User's choice:** Parallel connect

| Option | Description | Selected |
|--------|-------------|----------|
| Show error, keep other pane | 失败面板显示错误，另一个可继续浏览 | ✓ |
| Abort on any failure | 任一失败则关闭整个浏览器 | |

**User's choice:** Show error, keep other pane

---

## Status Bar & Key Hints

| Option | Description | Selected |
|--------|-------------|----------|
| Connection states + key hints | 显示服务器别名、连接状态、活跃面板、快捷键 | ✓ |
| Key hints only | 仅快捷键提示 | |
| You decide | Claude 决定 | |

**User's choice:** Connection states + key hints

| Option | Description | Selected |
|--------|-------------|----------|
| Match FileBrowser keys | 与 FileBrowser 快捷键方案一致 | ✓ |
| Custom key scheme | 自定义快捷键 | |

**User's choice:** Match FileBrowser keys

| Option | Description | Selected |
|--------|-------------|----------|
| Esc to close | 与 FileBrowser 行为一致 | ✓ |
| q to close | Esc 保留给其他操作 | |

**User's choice:** Esc to close

---

## Claude's Discretion

- Header bar 的具体颜色和样式
- 活跃面板高亮的具体实现方式
- 状态栏的具体文本格式
- 连接失败的错误信息措辞
- ConfirmDialog/InputDialog 集成方式

## Deferred Ideas

None — discussion stayed within phase scope
