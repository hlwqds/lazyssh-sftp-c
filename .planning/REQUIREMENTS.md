# Milestone v1.2 Requirements — File Operations

## P1: 基础文件操作

- [x] **DEL-01**: 用户在任一面板选中文件后按 `d` 键，弹出确认对话框显示待删除文件名和大小，用户确认后执行删除
- [x] **DEL-02**: 用户删除目录时系统递归删除目录内所有文件和子目录，删除过程中显示进度
- [x] **DEL-03**: 用户通过 Space 多选文件后按 `d` 键，确认对话框显示待删除文件数量和总大小，确认后批量删除
- [x] **DEL-04**: 删除操作完成后自动刷新当前目录列表，光标定位到合理位置
- [x] **REN-01**: 用户在任一面板选中文件/目录后按 `R` 键，弹出输入框预填当前文件名，用户编辑后按 Enter 完成重命名，按 Esc 取消
- [x] **REN-02**: 重命名时如果目标名称已存在，提示名称冲突，用户可选择覆盖或取消
- [x] **MKD-01**: 用户在任一面板按 `m` 键，弹出输入框输入目录名，按 Enter 在当前目录下创建子目录，按 Esc 取消
- [x] **MKD-02**: 新建目录完成后自动刷新列表，光标定位到新创建的目录

## P2: 复制与移动

- [x] **CPY-01**: 用户选中文件/目录后按 `c` 键，文件被标记为复制源（剪贴板状态），文件列表中显示 `[C]` 前缀标记
- [x] **CPY-02**: 用户导航到目标目录后按 `p` 键，系统将标记的文件/目录复制到当前目录
- [x] **CPY-03**: 复制目录时递归复制所有内容，远程端通过 download+re-upload 实现
- [ ] **MOV-01**: 用户选中文件/目录后按 `x` 键，文件被标记为移动源（剪贴板状态），文件列表中显示 `[M]` 前缀标记
- [ ] **MOV-02**: 用户导航到目标目录后按 `p` 键，系统将标记的文件/目录移动到当前目录（复制+删除源文件）
- [ ] **MOV-03**: 移动操作失败时（如权限不足），不删除源文件，保留原始状态
- [ ] **PRG-01**: 复制/移动大文件或目录时显示进度条（复用 TransferModal 或状态栏进度显示）
- [ ] **CNF-01**: 复制到目标目录时如果同名文件已存在，弹出冲突对话框（覆盖/跳过/重命名）
- [ ] **CNF-02**: 多文件复制/移动时，每个冲突文件单独询问处理方式

## P3: 辅助功能

- [x] **CLP-01**: 剪贴板有标记时，被标记文件在列表中显示 `[C]`（复制）或 `[M]`（移动）前缀
- [x] **CLP-02**: 用户导航到其他目录后，剪贴板标记仍然保留（跨目录复制/移动）
- [x] **CLP-03**: 用户按 `Esc` 或新的 `c`/`x` 操作时清除之前剪贴板标记
- [x] **RCP-01**: 远程复制（download+re-upload）显示统一进度视图，包含已复制文件数和总大小

## Future Requirements (Deferred)

- 跨面板复制/移动（本地→远程=上传，远程→本地=下载）— v1.3+
- 拖拽排序 — v2+
- 文件属性编辑（chmod/chown）— v2+
- 符号链接创建和管理 — v2+

## Out of Scope

| Feature | Reason |
|---------|--------|
| 远程端原生 copy | SFTP 协议无原生 copy 操作，需 download+re-upload |
| 跨文件系统移动 | 单面板操作在同一文件系统内，跨文件系统场景罕见 |
| 撤销操作 | 实现复杂度高，需要操作日志和逆操作链 |
| 文件内容编辑 | 只做文件管理操作，不做内容编辑 |
| 文件搜索/过滤 | 独立功能，适合单独 milestone |

## Traceability

| REQ-ID | Phase | Plan | Task | Status |
|--------|-------|------|------|--------|
| DEL-01 | Phase 6 | 06-01, 06-02, 06-03 | - | Implemented |
| DEL-02 | Phase 6 | 06-01, 06-02, 06-03 | - | Implemented |
| DEL-03 | Phase 6 | 06-02, 06-03 | - | Implemented |
| DEL-04 | Phase 6 | 06-03 | - | Implemented |
| REN-01 | Phase 6 | 06-01, 06-02, 06-03 | - | Implemented |
| REN-02 | Phase 6 | 06-02, 06-03 | - | Implemented |
| MKD-01 | Phase 6 | 06-01, 06-02, 06-03 | - | Implemented |
| MKD-02 | Phase 6 | 06-03 | - | Implemented |
| CPY-01 | Phase 7 | - | - | Pending |
| CPY-02 | Phase 7 | - | - | Pending |
| CPY-03 | Phase 7 | - | - | Pending |
| MOV-01 | Phase 8 | - | - | Pending |
| MOV-02 | Phase 8 | - | - | Pending |
| MOV-03 | Phase 8 | - | - | Pending |
| PRG-01 | Phase 8 | - | - | Pending |
| CNF-01 | Phase 8 | - | - | Pending |
| CNF-02 | Phase 8 | - | - | Pending |
| CLP-01 | Phase 7 | - | - | Pending |
| CLP-02 | Phase 7 | - | - | Pending |
| CLP-03 | Phase 7 | - | - | Pending |
| RCP-01 | Phase 7 | - | - | Pending |

---
*Requirements defined: 2026-04-15 — v1.2 File Operations*
*Traceability updated: 2026-04-15 — roadmap created*
