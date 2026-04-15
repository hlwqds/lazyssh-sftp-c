# Phase 6: Basic File Operations - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-15
**Phase:** 06-basic-file-operations
**Areas discussed:** Overlay 组件设计, 删除确认 UX, 重命名/新建目录 UX, 错误处理方式

---

## Overlay 组件设计

| Option | Description | Selected |
|--------|-------------|----------|
| 独立组件 | 新建 confirm_dialog.go 和 input_dialog.go，遵循 TransferModal/RecentDirs overlay 模式 | ✓ |
| 扩展 TransferModal | 在 TransferModal 中添加 modeDeleteConfirm 和 modeInput 模式 | |
| tview 原生组件 | 直接用 tview.Modal 和 tview.InputField，让 tview 管理焦点 | |

**User's choice:** 独立组件
**Notes:** 职责清晰，不影响 TransferModal。遵循已验证的 overlay 模式。

---

## 删除确认 UX

### 单文件删除信息量

| Option | Description | Selected |
|--------|-------------|----------|
| 简洁确认 | 文件名+大小，[y] Yes [n] No | |
| 详细信息确认 | 文件名+大小+文件类型+修改时间 | ✓ |

**User's choice:** 详细信息确认

### 多选批量删除

| Option | Description | Selected |
|--------|-------------|----------|
| 批量确认 | "删除 5 个文件？共 12.3MB"，一个确认全部 | ✓ |
| 带列表的批量确认 | 显示文件列表 | |

**User's choice:** 批量确认

### 目录删除警告

| Option | Description | Selected |
|--------|-------------|----------|
| 递归警告 | 额外显示"目录非空，将递归删除所有内容" | ✓ |
| 无额外警告 | 和普通文件一样 | |

**User's choice:** 递归警告

---

## 重命名/新建目录 UX

### 重命名触发方式

| Option | Description | Selected |
|--------|-------------|----------|
| 弹出输入框 | 居中弹出，预填文件名 | ✓ |
| 表格内联编辑 | 直接在表格行内编辑 | |

**User's choice:** 弹出输入框

### 光标初始位置

| Option | Description | Selected |
|--------|-------------|----------|
| 文件名末尾 | 光标在扩展名前，如 "config\|.yaml" | ✓ |
| 全名末尾 | 如 "config.yaml\|" | |
| 全选文件名 | 选中整个名称方便替换 | |

**User's choice:** 文件名末尾

### InputDialog 复用

| Option | Description | Selected |
|--------|-------------|----------|
| 复用 InputDialog | 重命名和新建目录共用同一组件 | ✓ |
| 独立组件 | 单独的 MkdirDialog | |

**User's choice:** 复用 InputDialog

---

## 错误处理方式

| Option | Description | Selected |
|--------|-------------|----------|
| 状态栏闪烁 | 红色错误信息，几秒后自动恢复 | ✓ |
| 错误弹窗 | 弹窗需按键关闭 | |

**User's choice:** 状态栏闪烁

---

## Claude's Discretion

- ConfirmDialog 具体布局细节
- InputDialog 焦点管理方式
- 状态栏闪烁持续时间
- 递归删除进度显示策略
