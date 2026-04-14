# Milestone v1.1 Requirements — Recent Remote Directories

## P1: 目录历史记录

- [x] **HIST-01**: 用户在远程面板中导航到新目录时（Enter 进入子目录、h 返回上级），系统自动将该目录路径记录到最近目录列表
- [x] **HIST-02**: 最近目录列表按 MRU（Most Recently Used）排序，最近访问的路径排在最前
- [x] **HIST-03**: 同一目录路径多次访问时自动去重，仅保留最新位置（移到列表头部）
- [x] **HIST-04**: 最近目录列表最多保留 10 条记录，超出时移除最旧的条目

## P2: 弹出列表 UI

- [ ] **POPUP-01**: 用户在远程面板获得焦点时按 `r` 键，弹出一个居中的最近目录列表弹窗
- [ ] **POPUP-02**: 弹窗中用户可以用 `j`/`k` 或上下方向键在列表中移动选中项
- [ ] **POPUP-03**: 用户按 `Enter` 选中一条路径后，远程面板直接跳转到该目录并刷新文件列表，弹窗关闭
- [ ] **POPUP-04**: 用户按 `Esc` 关闭弹窗，焦点恢复到远程面板
- [ ] **POPUP-05**: 列表为空时显示"暂无最近目录"提示文本

## P3: 辅助功能

- [ ] **AUX-01**: 弹窗列表中，与当前远程面板路径相同的条目用不同颜色高亮显示
- [ ] **AUX-02**: 修复 `RemotePane.NavigateToParent()` 缺少 `onPathChange` 回调的预存 bug（导致返回上级目录时路径未被记录且终端未 Sync）

## Future Requirements (Deferred)

- 高亮当前目录 — **Included in v1.1 as AUX-01**
- 路径缩写显示（过长路径缩写中间部分）— v1.x
- 数字键快速选择（按 1-9 直接跳转）— v1.x
- 持久化书签/收藏夹 — v2+
- 按本地目录分组 — v2+（如支持多标签页）

## Out of Scope

| Feature | Reason |
|---------|--------|
| 持久化到磁盘 | 安全风险（记录用户访问的服务器目录），缓存失效问题，违反零外部依赖约束 |
| 频率加权排序 | 实现复杂度高，10 条上限下意义不大 |
| 跨服务器目录列表 | 破坏实例隔离，服务器 A 无法访问服务器 B 路径 |
| 目录预览 | 需 SFTP 预取，网络开销大，实现复杂度高 |
| 书签/收藏夹管理 | 需手动管理 + 持久化，超出 v1.1 范围 |

## Traceability

| REQ-ID | Phase | Plan | Task | Status |
|--------|-------|------|------|--------|
| HIST-01 | Phase 4 | — | — | Pending |
| HIST-02 | Phase 4 | — | — | Pending |
| HIST-03 | Phase 4 | — | — | Pending |
| HIST-04 | Phase 4 | — | — | Pending |
| POPUP-01 | Phase 5 | — | — | Pending |
| POPUP-02 | Phase 5 | — | — | Pending |
| POPUP-03 | Phase 5 | — | — | Pending |
| POPUP-04 | Phase 5 | — | — | Pending |
| POPUP-05 | Phase 5 | — | — | Pending |
| AUX-01 | Phase 5 | — | — | Pending |
| AUX-02 | Phase 4 | — | — | Pending |

---
*Requirements defined: 2026-04-14 — v1.1 Recent Remote Directories*
