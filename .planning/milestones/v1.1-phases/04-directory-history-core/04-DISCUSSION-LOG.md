# Phase 4: Directory History Core - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-14
**Phase:** 4-directory-history-core
**Areas discussed:** Initial path handling, Re-record on select, Path normalization

---

## Initial Path Handling

| Option | Description | Selected |
|--------|-------------|----------|
| 不记录相对路径 | 初始路径 "." 和从它 join 出来的 "./docs" 不记录，只有在第一次 NavigateToParent 返回到绝对路径后才开始记录 | ✓ |
| 全部记录并规范化 | 所有导航都记录，包括 "." 和 "./docs"，在记录时规范化为绝对路径 | |

**User's choice:** 不记录相对路径
**Notes:** SFTP 初始路径是 "."，joinPath(".", "docs") = "./docs"，这些相对路径没有记录价值。

---

## Re-record on Select

| Option | Description | Selected |
|--------|-------------|----------|
| 重新提升到头部 | 选择后移动到列表头部，强调使用频率 | ✓ |
| 保持原位置 | 保持原位置不变，避免不必要的列表变动 | |

**User's choice:** 重新提升到头部
**Notes:** Phase 5 中用户从最近列表选择并跳转后，调用 Record(path) 将该路径重新提升。

---

## Path Normalization

| Option | Description | Selected |
|--------|-------------|----------|
| 仅去尾部斜杠 | 去除尾部 /，确保 "/home/user" 和 "/home/user/" 是同一条记录 | ✓ |
| 完整规范化 | 完整规范化：去尾部 /、解析 ./、解析 ../ | |

**User's choice:** 仅去尾部斜杠
**Notes:** SFTP 远程路径通常是绝对路径，主要问题是尾部斜杠不一致。完整规范化不必要。

---

## Claude's Discretion

- RecentDirs Draw() 实现细节留给 Phase 5
- NavigateTo 是否调用 UpdateTitle() 留给实现 discretion

## Deferred Ideas

None
