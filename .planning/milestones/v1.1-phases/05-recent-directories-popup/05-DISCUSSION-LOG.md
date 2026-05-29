# Phase 5: Recent Directories Popup - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-14
**Phase:** 05-recent-directories-popup
**Areas discussed:** 列表布局和尺寸, 颜色和视觉风格, 快捷键和交互行为, Draw() 渲染实现

---

## 列表布局和尺寸

### 弹窗尺寸和位置

| Option | Description | Selected |
|--------|-------------|----------|
| 居中弹窗，60% 宽度 | 弹窗宽度 = 终端宽度的 60%，最大 80 列；高度 = min(路径数量+2, 15) 行；屏幕正中央 | ✓ |
| 居中弹窗，80% 宽度 | 弹窗宽度 = 终端宽度的 80%，最大 100 列；高度 = min(路径数量+2, 20) 行；屏幕正中央 | |
| 右对齐，贴近远程面板 | 弹窗固定在右侧，与远程面板对齐；宽度占右侧面板的 90% | |

**User's choice:** 居中弹窗，60% 宽度 (Recommended)

### 路径显示格式

| Option | Description | Selected |
|--------|-------------|----------|
| 完整路径 | 显示完整绝对路径，如 /home/user/projects/my-app | ✓ |
| 仅目录名 | 只显示最后一级目录名，如 my-app | |

**User's choice:** 完整路径 (Recommended)

### 列表行数上限

| Option | Description | Selected |
|--------|-------------|----------|
| 固定最多 10 行，不滚动 | 列表最多显示 10 行（与 maxRecentDirs 一致），不滚动 | ✓ |
| 自适应高度，超出时滚动 | 列表高度适应内容，最少 1 行，最多占屏幕 60% 高度 | |

**User's choice:** 固定最多 10 行，不滚动 (Recommended)

---

## 颜色和视觉风格

### 选中项高亮样式

| Option | Description | Selected |
|--------|-------------|----------|
| 深灰背景 + 白字 | tcell.Color236 深灰背景 + tcell.Color250 白色文字 | ✓ |
| 浅灰背景 + 白字 | tcell.Color240 浅灰背景 + 白色文字 | |
| 蓝色背景 + 白字 | tcell.Color57 蓝色背景 + 白色文字 | |

**User's choice:** 深灰背景 + 白字 (Recommended)

### 当前路径条目高亮 (AUX-01)

| Option | Description | Selected |
|--------|-------------|----------|
| 黄色文字标记 | tcell.ColorYellow 黄色文字 | ✓ |
| > 前缀符号标记 | 条目前加 > 或 * 符号 | |
| 青色文字标记 | tcell.Color33 青色文字 | |

**User's choice:** 黄色文字标记 (Recommended)

### 空状态显示

| Option | Description | Selected |
|--------|-------------|----------|
| 居中灰色文字 | 居中显示灰色文字「暂无最近目录」 | ✓ |
| 极简 (empty) | 居中显示「(empty)」小字 | |

**User's choice:** 居中灰色文字 (Recommended)

---

## 快捷键和交互行为

### r 键触发条件

| Option | Description | Selected |
|--------|-------------|----------|
| 仅远程面板 | 仅在远程面板获得焦点时按 r 才弹出 | ✓ |
| 任意面板都可触发 | 任意面板按 r 都弹出 | |

**User's choice:** 仅远程面板 (Recommended)

### Esc 关闭行为

| Option | Description | Selected |
|--------|-------------|----------|
| 关闭弹窗，恢复焦点 | 关闭弹窗，焦点恢复到远程面板 | ✓ |
| 关闭 + Sync() | 关闭弹窗 + 触发 app.Sync() | |

**User's choice:** 关闭弹窗，恢复焦点 (Recommended)

### 事件拦截策略

| Option | Description | Selected |
|--------|-------------|----------|
| 完全拦截 | 弹窗可见时拦截所有按键，不传递到下层 | ✓ |
| 仅拦截已知按键 | 仅拦截弹窗定义的按键 | |

**User's choice:** 完全拦截 (Recommended)

---

## Draw() 渲染实现

### 渲染方式

| Option | Description | Selected |
|--------|-------------|----------|
| 手动渲染 | 用 tview.Print() 逐行渲染，与 TransferModal 一致 | ✓ |
| 嵌入 tview.Table | 在 RecentDirs 内嵌入 tview.Table | |

**User's choice:** 手动渲染 (Recommended)

### 选中项指示符

| Option | Description | Selected |
|--------|-------------|----------|
| 仅背景色 | 选中行用背景色区分，不额外显示指示符 | ✓ |
| > 符号 + 背景色 | 选中行左侧显示 > 或 ▸ 符号 | |

**User's choice:** 仅背景色 (Recommended)

### 标题栏

| Option | Description | Selected |
|--------|-------------|----------|
| 显示标题 | 弹窗顶部显示「 Recent Directories 」标题 | ✓ |
| 无标题 | 不显示标题 | |

**User's choice:** 显示标题 (Recommended)

---

## Claude's Discretion

- Draw() 中边框内边距的具体像素值
- HandleKey 中 j/k 与上下方向键的具体实现细节
- Show() 时是否需要调用 app.ForceDraw()

## Deferred Ideas

- 路径缩写显示 — v1.x
- 数字键快速选择 — v1.x
- 持久化书签/收藏夹 — v2+
