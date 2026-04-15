# Phase 9: Dup SSH Connection - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-15
**Phase:** 09-dup-ssh-connection
**Areas discussed:** 别名生成策略, 复制字段范围, 复制后光标位置

---

## 别名生成策略

| Option | Description | Selected |
|--------|-------------|----------|
| -copy 后缀递增 | 原名-copy，冲突时原名-copy-2，类似 macOS | ✓ |
| (N) 数字后缀 | 原名 (2)，类似 Windows | |

**User's choice:** -copy 后缀递增
**Notes:** 参考 macOS Finder 文件复制行为

---

## 复制字段范围

| Option | Description | Selected |
|--------|-------------|----------|
| 全部复制，仅清元数据 | 复制所有 SSH 配置字段，清除 PinnedAt/SSHCount/LastSeen | ✓ |
| 仅基本字段 | 仅复制 Host/User/Port，其他留空 | |

**User's choice:** 全部复制，仅清元数据
**Notes:** IdentityFiles/ProxyJump 等全部保留，用户在表单中自行修改不需要的字段

---

## 复制后光标位置

| Option | Description | Selected |
|--------|-------------|----------|
| 定位到新条目 | 保存后自动选中并滚动到新创建的条目 | ✓ |
| 留在原位 | 光标不动 | |

**User's choice:** 定位到新条目
**Notes:** 需要在 handleServerSave 中通过 alias 匹配新条目位置

---

## Claude's Discretion

- 别名唯一性检查的具体实现方式
- handleServerDup 函数内部代码组织

## Deferred Ideas

None
