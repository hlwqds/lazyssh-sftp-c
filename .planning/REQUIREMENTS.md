# Requirements: LazySSH v1.3

**Defined:** 2026-04-15
**Core Value:** 在终端内完成 SSH 文件传输，无需切换到 FileZilla 或记忆 scp 命令——选中服务器、选文件、传输，全部键盘驱动。

## v1.3 Requirements

### Dup SSH Connection

- [ ] **DUP-01**: 用户可以在服务器列表按 D 键复制当前选中服务器的全部配置
- [ ] **DUP-02**: 复制后自动生成唯一别名（原名-copy, 原名-copy-2, ...递增后缀）
- [ ] **DUP-03**: 复制后自动打开编辑表单（ServerForm），用户可修改字段后保存为新条目
- [ ] **DUP-04**: 复制条目清除运行时元数据（metadata/ping 等非配置字段）

## Out of Scope

| Feature | Reason |
|---------|--------|
| 本地路径历史持久化 | 延迟到 v1.4 |
| 双远端文件互传 | 延迟到 v1.4+，架构复杂度高 |
| 批量复制多个服务器 | 超出 v1.3 范围 |
| 复制时修改部分字段再保存 | 用户在表单中自行修改 |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| DUP-01 | Phase 9 | Pending |
| DUP-02 | Phase 9 | Pending |
| DUP-03 | Phase 9 | Pending |
| DUP-04 | Phase 9 | Pending |

**Coverage:**
- v1.3 requirements: 4 total
- Mapped to phases: 4/4
- Unmapped: 0

---
*Requirements defined: 2026-04-15*
*Last updated: 2026-04-15 — traceability updated after roadmap creation*
