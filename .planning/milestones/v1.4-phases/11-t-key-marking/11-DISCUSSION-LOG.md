# Phase 11: T Key Marking - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-15
**Phase:** 11-t-key-marking
**Areas discussed:** 快捷键绑定, 标记状态与视觉呈现, 标记流程与 Esc 行为, 自动打开双远端浏览器

---

## 快捷键绑定 (Key Binding)

| Option | Description | Selected |
|--------|-------------|----------|
| T (Shift+t) | 与 'D' 复制模式一致。't' 标签编辑不变，'T' 标记服务器 | ✓ |
| Ctrl+T | 用 Ctrl+T 触发标记，避免占用字母键 | |
| Other key | 用其他键触发（如 'm' for mark） | |

**User's choice:** T (Shift+t)
**Notes:** 't' 已被 handleTagsEdit() 占用，'T' 遵循 'd'/'D' 大小写分离模式

---

## 标记状态与视觉呈现 (Visual Presentation)

| Option | Description | Selected |
|--------|-------------|----------|
| 文本前缀 [S]/[T] | 绿色 [S] 源端、蓝色 [T] 目标端，在 formatServerLine() 最前面 | ✓ |
| Emoji 前缀 | 🟢 源端、🔵 目标端，与 📌/📡 风格统一 | |
| 行高亮 | 改变整行背景色 | |

**User's choice:** 文本前缀 [S]/[T]
**Notes:** 颜色区分：绿色源端、蓝色目标端

---

## 标记流程与 Esc 行为 (Marking Flow + Esc)

| Option | Description | Selected |
|--------|-------------|----------|
| Esc 清除标记 | 有标记时 Esc 先清除，再按 Esc 返回搜索栏 | ✓ |
| 导航取消标记 | j/k 上下键自动取消标记 | |

**User's choice:** Esc 清除标记
**Notes:** 标记模式不改变 Esc 的功能层级，只是插入清除逻辑

---

## 同一服务器标记防护 (Same Server Protection)

| Option | Description | Selected |
|--------|-------------|----------|
| 显示错误提示 | 红色提示 "Cannot mark same server twice"，状态不变 | ✓ |
| 静默忽略 | 不做任何反应 | |

**User's choice:** 显示错误提示
**Notes:** 与 ROADMAP MARK-04 一致

---

## 自动打开双远端浏览器 (Auto-open Dual Remote Browser)

| Option | Description | Selected |
|--------|-------------|----------|
| 立即自动打开 | 标记完成后立即调用，Phase 11 用占位函数 | ✓ |
| 需要确认才打开 | 显示提示后用户按 Enter 才打开 | |

**User's choice:** 立即自动打开
**Notes:** Phase 11 用 TODO 占位函数，Phase 12 填充实现

---

## Claude's Discretion

- 标记状态字段命名
- formatServerLine() 参数传递方式
- 状态栏提示文本措辞
- 标记清除时机（打开浏览器前 vs 关闭后）

## Deferred Ideas

None
