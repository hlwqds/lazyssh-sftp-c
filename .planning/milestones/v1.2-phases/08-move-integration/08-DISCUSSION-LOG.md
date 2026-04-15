# Phase 8: Move & Integration - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-15
**Phase:** 8-move-integration
**Areas discussed:** 冲突对话框, 移动失败清理, 移动进度, 本地进度

---

## 冲突对话框触发范围

| Option | Description | Selected |
|--------|-------------|----------|
| 所有粘贴都弹对话框 | 替换 D-06 自动重命名，无论同目录还是跨目录 | |
| 跨目录弹对话框，同目录保持自动重命名 | 保留 D-06，两种行为并存 | |
| 所有粘贴弹对话框，同目录默认选中「重命名」 | 弹对话框但预选 Rename，Enter 即可 | ✓ |

**User's choice:** 所有粘贴都弹对话框
**Notes:** 后续确认复制和移动粘贴都弹冲突对话框（不仅限于移动）

## 冲突对话框适用操作

| Option | Description | Selected |
|--------|-------------|----------|
| 复制和移动粘贴都弹冲突对话框 | TransferModal 已有 conflictDialog 模式可复用 | ✓ |
| 仅移动粘贴弹冲突对话框 | 保持复制粘贴的自动重命名 | |

**User's choice:** 复制和移动粘贴都弹冲突对话框

## 移动失败后的目标副本处理

| Option | Description | Selected |
|--------|-------------|----------|
| 保留目标副本，仅提示错误 | 用户手动清理，简单可靠 | |
| 尝试清理目标副本 | 自动删除目标副本恢复原始状态 | ✓ |

**User's choice:** 尝试清理目标副本

## 移动进度显示方式

| Option | Description | Selected |
|--------|-------------|----------|
| 新增 modeMove | 独立模式显示 Moving + Deleting source 阶段 | ✓ |
| 复用 modeCopy，标题区分 | 减少代码量但模式语义不精确 | |

**User's choice:** 新增 modeMove

## 本地复制进度

| Option | Description | Selected |
|--------|-------------|----------|
| 本地复制也加进度 | 改动较大，需要异步+回调模式 | |
| 本地复制保持同步无进度 | 本地操作快，仅远程显示进度 | ✓ |

**User's choice:** 本地复制保持同步无进度

---

## Claude's Discretion

- [M] 前缀颜色、modeMove UI 布局、"Deleting source..." 显示样式、状态栏提示文本、冲突对话框默认选中项

## Deferred Ideas

None
