# Roadmap: LazySSH File Transfer

## Milestones

- **v1.0 File Transfer** - Phases 1-3 (shipped 2026-04-13)
- **v1.1 Recent Remote Directories** - Phases 4-5 (shipped 2026-04-14)
- **v1.2 File Operations** - Phases 6-8 (in progress)

## Phases

<details>
<summary>v1.0 File Transfer (Phases 1-3) - SHIPPED 2026-04-13</summary>

- [x] Phase 1: Foundation (3/3 plans) - completed 2026-04-13
- [x] Phase 2: Core Transfer (3/3 plans) - completed 2026-04-13
- [x] Phase 3: Polish (3/3 plans) - completed 2026-04-13

</details>

<details>
<summary>v1.1 Recent Remote Directories (Phases 4-5) - SHIPPED 2026-04-14</summary>

- [x] Phase 4: Directory History Core (2/2 plans) - completed 2026-04-14
- [x] Phase 5: Recent Directories Popup (1/1 plans) - completed 2026-04-14

</details>

### v1.2 File Operations (In Progress)

**Milestone Goal:** 双面板内完成文件管理操作（删除/重命名/新建/复制/移动），无需退出 lazyssh

- [ ] **Phase 6: Basic File Operations** - Port 接口扩展 + 删除/重命名/新建目录
- [ ] **Phase 7: Copy & Clipboard** - 复制功能（剪贴板 + 可视化标记 + 远程复制进度）
- [ ] **Phase 8: Move & Integration** - 移动功能 + 进度显示 + 冲突处理

## Phase Details

<details>
<summary>v1.0 File Transfer (Phases 1-3) - SHIPPED 2026-04-13</summary>

### Phase 1: Foundation
**Goal**: 文件浏览器基础设施（FileInfo 实体、FileService/SFTPService 端口、LocalFS 和 SFTP 适配器）
**Plans**: 3 plans

Plans:
- [x] 01-01: FileInfo domain entity and FileService/SFTPService port interfaces
- [x] 01-02: LocalFS adapter implementation
- [x] 01-03: SFTPClient adapter with ListDir and sorting

### Phase 2: Core Transfer
**Goal**: 双栏文件浏览器 UI + SFTP 连接管理 + 传输服务
**Plans**: 3 plans

Plans:
- [x] 02-01: Dual-pane file browser UI with keyboard navigation
- [x] 02-02: SFTP connection lifecycle and TransferService port
- [x] 02-03: TransferService implementation with 32KB buffered progress

### Phase 3: Polish
**Goal**: 传输进度 overlay + 冲突处理 + 取消 + 跨平台权限
**Plans**: 3 plans

Plans:
- [x] 03-01: ProgressBar and TransferModal overlay
- [x] 03-02: Keyboard-driven file/directory transfer (Enter/F5)
- [x] 03-03: Conflict detection, cancel, and platform permissions

</details>

<details>
<summary>v1.1 Recent Remote Directories (Phases 4-5) - SHIPPED 2026-04-14</summary>

### Phase 4: Directory History Core
**Goal**: In-memory MRU directory tracking data structure
**Plans**: 2 plans

Plans:
- [x] 04-01: RecentDirs data structure with MRU dedup and callbacks
- [x] 04-02: NavigateToParent fix and RecentDirs wiring through OnPathChange

### Phase 5: Recent Directories Popup
**Goal**: Centered popup overlay for quick navigation to recent remote directories
**Plans**: 1 plan

Plans:
- [x] 05-01: RecentDirs overlay component with j/k navigation and path highlighting

</details>

### Phase 6: Basic File Operations
**Goal**: 用户可以在本地和远程面板中删除文件/目录、重命名文件/目录、新建子目录
**Depends on**: Phase 5
**Requirements**: DEL-01, DEL-02, DEL-03, DEL-04, REN-01, REN-02, MKD-01, MKD-02
**Success Criteria** (what must be TRUE):
  1. 用户在任一面板选中文件后按 `d` 键，弹出确认对话框显示文件名和大小，确认后文件被删除，列表自动刷新
  2. 用户删除目录时系统递归删除所有内容，删除完成后列表自动刷新且光标定位到合理位置
  3. 用户通过 Space 多选文件后按 `d` 键，确认对话框显示待删除文件数量和总大小，确认后批量删除
  4. 用户选中文件/目录后按 `R` 键，弹出输入框预填当前文件名，编辑后 Enter 完成重命名，Esc 取消；目标名称已存在时提示冲突
  5. 用户在任一面板按 `m` 键，弹出输入框输入目录名，Enter 创建子目录，Esc 取消，创建后光标定位到新目录
**Plans**: 3 plans

Plans:
- [x] 06-01-PLAN.md -- FileService port 接口扩展 + SFTPClient/LocalFS adapter 实现
- [x] 06-02-PLAN.md -- ConfirmDialog 和 InputDialog overlay 组件
- [x] 06-03-PLAN.md -- FileBrowser 集成：按键路由 + delete/rename/mkdir handlers + overlay wiring

### Phase 7: Copy & Clipboard
**Goal**: 用户可以通过 c 标记 + p 粘贴在面板内复制文件/目录，剪贴板标记跨目录导航保持
**Depends on**: Phase 6
**Requirements**: CPY-01, CPY-02, CPY-03, CLP-01, CLP-02, CLP-03, RCP-01
**Success Criteria** (what must be TRUE):
  1. 用户选中文件/目录后按 `c` 键，文件在列表中显示 `[C]` 前缀标记，状态栏提示标记数量
  2. 用户导航到目标目录后按 `p` 键，标记的文件/目录被复制到当前目录，目录递归复制所有内容
  3. 远程面板内复制大文件/目录时显示统一进度视图（包含已复制文件数和总大小）
  4. 剪贴板标记在导航到其他目录后仍然保留，按 Esc 或新的 c/x 操作清除之前标记
**Plans**: 2 plans

Plans:
- [x] 07-01-PLAN.md -- FileService/TransferService port 扩展 + LocalFS/transfer adapter 实现
- [x] 07-02-PLAN.md -- Clipboard UI：handleCopy/handlePaste + [C] 前缀 + TransferModal modeCopy + Esc 清除

### Phase 8: Move & Integration
**Goal**: 用户可以通过 x 标记 + p 粘贴在面板内移动文件/目录，移动失败时保留源文件
**Depends on**: Phase 7
**Requirements**: MOV-01, MOV-02, MOV-03, PRG-01, CNF-01, CNF-02
**Success Criteria** (what must be TRUE):
  1. 用户选中文件/目录后按 `x` 键，文件在列表中显示 `[M]` 前缀标记，导航到目标目录后按 `p` 完成移动（源文件被删除）
  2. 复制/移动大文件或目录时显示进度条，复用 TransferModal 或状态栏进度显示
  3. 目标目录存在同名文件时弹出冲突对话框（覆盖/跳过/重命名），多文件操作时每个冲突文件单独询问
  4. 移动操作失败时（如权限不足），源文件保留不变，用户收到错误提示
**Plans**: TBD
**UI hint**: yes

## Progress

**Execution Order:**
Phases execute in numeric order: 6 -> 7 -> 8

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 6. Basic File Operations | v1.2 | 2/3 | Complete    | 2026-04-15 |
| 7. Copy & Clipboard | v1.2 | 0/2 | Not started | - |
| 8. Move & Integration | v1.2 | 0/? | Not started | - |

Full details: .planning/milestones/v1.0-ROADMAP.md, .planning/milestones/v1.1-ROADMAP.md
