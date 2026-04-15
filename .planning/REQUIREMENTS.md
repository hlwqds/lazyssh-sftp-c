# Requirements: LazySSH v1.4

**Defined:** 2026-04-15
**Core Value:** 在终端内完成 SSH 文件传输，无需切换到 FileZilla 或记忆 scp 命令——选中服务器、选文件、传输，全部键盘驱动。

## v1.4 Requirements

### Dup Fix

- [ ] **DUP-FIX-01**: D 键复制后直接调用 AddServer() 添加新条目到列表，不自动打开 ServerForm 编辑表单
- [ ] **DUP-FIX-02**: 复制后自动滚动列表到新条目（复用现有 dupPendingAlias 滚动逻辑）

### T Key Marking

- [ ] **MARK-01**: 用户可以在服务器列表按 T 键标记第一个服务器为源端（Shift+t，不与小写 t 标签编辑冲突）
- [ ] **MARK-02**: 再按 T 键标记第二个服务器为目标端，标记完成后自动打开双远端文件浏览器
- [ ] **MARK-03**: 标记状态下按 Esc 清除所有标记，恢复普通选择状态
- [ ] **MARK-04**: 防止标记同一服务器两次（显示错误提示或忽略）
- [ ] **MARK-05**: 已标记的服务器在列表中有视觉提示（如 [S] 源端、[T] 目标端前缀）

### Dual Remote Browser

- [ ] **DRB-01**: 创建独立的 DualRemoteFileBrowser 组件（不复用 FileBrowser），左栏为远端 A（源端），右栏为远端 B（目标端）
- [ ] **DRB-02**: 双栏复用 RemotePane 组件，各自持有独立的 SFTPClient 实例
- [ ] **DRB-03**: 支持键盘导航（Tab 切换面板、上下左右浏览、Enter 进入目录、h 返回上级）
- [ ] **DRB-04**: 退出浏览器（Esc/q）时关闭两个 SFTP 连接并清理资源

### Cross-Remote Transfer

- [ ] **XFR-01**: 用户可以在双远端浏览器中通过 Enter/F5 触发跨远端文件传输（download A → temp → upload B）
- [ ] **XFR-02**: 支持跨远端目录递归传输
- [ ] **XFR-03**: 跨远端传输显示进度（复用 TransferModal，两阶段进度：下载进度 → 上传进度）
- [ ] **XFR-04**: 跨远端传输支持取消（Esc），取消后清理本地临时文件
- [ ] **XFR-05**: 跨远端文件冲突处理（覆盖/跳过/重命名，复用 ConfirmDialog）
- [ ] **XFR-06**: 支持跨远端复制（c 标记 + p 粘贴，绿色 [C] 前缀）
- [ ] **XFR-07**: 支持跨远端移动（x 标记 + p 粘贴，红色 [M] 前缀，复制+删除源文件）

## Out of Scope

| Feature | Reason |
|---------|--------|
| 本地路径历史持久化 | 延迟到后续版本 |
| 双远端传输磁盘空间检查 | 增加复杂度，temp 清理已足够 |
| 同服务器标记允许 | 防止混淆，直接禁止 |
| FileBrowser 重构为接口驱动 | 短期创建独立组件更安全 |
| r 键最近目录（双远端） | v1.4 先不做，保持简单 |
| 多文件并行传输 | v1 单线程，保持简单 |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| DUP-FIX-01 | TBD | Pending |
| DUP-FIX-02 | TBD | Pending |
| MARK-01 | TBD | Pending |
| MARK-02 | TBD | Pending |
| MARK-03 | TBD | Pending |
| MARK-04 | TBD | Pending |
| MARK-05 | TBD | Pending |
| DRB-01 | TBD | Pending |
| DRB-02 | TBD | Pending |
| DRB-03 | TBD | Pending |
| DRB-04 | TBD | Pending |
| XFR-01 | TBD | Pending |
| XFR-02 | TBD | Pending |
| XFR-03 | TBD | Pending |
| XFR-04 | TBD | Pending |
| XFR-05 | TBD | Pending |
| XFR-06 | TBD | Pending |
| XFR-07 | TBD | Pending |

**Coverage:**
- v1.4 requirements: 17 total
- Mapped to phases: 0/17 (roadmap not yet created)
- Unmapped: 17

---
*Requirements defined: 2026-04-15*
*Last updated: 2026-04-15*
