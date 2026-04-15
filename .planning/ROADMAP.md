# Roadmap: LazySSH File Transfer

## Milestones

- **v1.0 File Transfer** - Phases 1-3 (shipped 2026-04-13)
- **v1.1 Recent Remote Directories** - Phases 4-5 (shipped 2026-04-14)
- **v1.2 File Operations** - Phases 6-8 (shipped 2026-04-15)
- **v1.3 Dup SSH Connection** - Phase 9 (shipped 2026-04-15)
- **v1.4 Dup Fix & Dual Remote Transfer** - Phases 10-13 (in progress)

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

<details>
<summary>v1.2 File Operations (Phases 6-8) - SHIPPED 2026-04-15</summary>

- [x] Phase 6: Basic File Operations (3/3 plans) - completed 2026-04-15
- [x] Phase 7: Copy & Clipboard (2/2 plans) - completed 2026-04-15
- [x] Phase 8: Move & Integration (2/2 plans) - completed 2026-04-15

</details>

<details>
<summary>v1.3 Dup SSH Connection (Phase 9) - SHIPPED 2026-04-15</summary>

- [x] Phase 9: Dup SSH Connection (1/1 plans) - completed 2026-04-15

</details>

### v1.4 Dup Fix & Dual Remote Transfer (In Progress)

**Milestone Goal:** 修复 Dup 行为 + 支持两个远端服务器之间的文件互传

- [ ] **Phase 10: Dup Fix** - D 键复制后直接添加到列表，不自动打开编辑表单
- [ ] **Phase 11: T Key Marking** - T 键标记两个服务器，标记完成后自动打开双远端浏览器
- [ ] **Phase 12: Dual Remote Browser** - 独立双远端文件浏览器组件，两个 RemotePane 并列显示
- [ ] **Phase 13: Cross-Remote Transfer** - 双远端之间文件复制/移动，含进度和冲突处理

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

<details>
<summary>v1.2 File Operations (Phases 6-8) - SHIPPED 2026-04-15</summary>

### Phase 6: Basic File Operations
**Goal**: 用户可以在本地和远程面板中删除文件/目录、重命名文件/目录、新建子目录
**Plans**: 3 plans

Plans:
- [x] 06-01: FileService port 接口扩展 + SFTPClient/LocalFS adapter 实现
- [x] 06-02: ConfirmDialog 和 InputDialog overlay 组件
- [x] 06-03: FileBrowser 集成：按键路由 + delete/rename/mkdir handlers + overlay wiring

### Phase 7: Copy & Clipboard
**Goal**: 用户可以通过 c 标记 + p 粘贴在面板内复制文件/目录，剪贴板标记跨目录导航保持
**Plans**: 2 plans

Plans:
- [x] 07-01: FileService/TransferService port 扩展 + LocalFS/transfer adapter 实现
- [x] 07-02: Clipboard UI：handleCopy/handlePaste + [C] 前缀 + TransferModal modeCopy + Esc 清除

### Phase 8: Move & Integration
**Goal**: 用户可以通过 x 标记 + p 粘贴在面板内移动文件/目录，移动失败时保留源文件
**Plans**: 2 plans

Plans:
- [x] 08-01: OpMove + modeMove + [M] prefix + handleMove + x key + status bar hints
- [x] 08-02: handlePaste refactor: conflict dialog + move dispatch + handleLocalMove/handleRemoteMove + cleanup

</details>

<details>
<summary>v1.3 Dup SSH Connection (Phase 9) - SHIPPED 2026-04-15</summary>

### Phase 9: Dup SSH Connection
**Goal**: 用户可以在服务器列表中按 D 键快速复制当前选中服务器的配置，自动生成唯一别名后打开编辑表单
**Depends on**: Nothing (uses existing ServerService.AddServer and ServerForm)
**Requirements**: DUP-01, DUP-02, DUP-03, DUP-04
**Plans**: 1 plan

Plans:
- [x] 09-01: D key dup handler, alias generation, metadata clearing, form wiring, hint updates

</details>

### Phase 10: Dup Fix
**Goal**: 用户按 D 键复制服务器后，新条目直接出现在列表中，不自动打开编辑表单
**Depends on**: Phase 9 (existing dup implementation)
**Requirements**: DUP-FIX-01, DUP-FIX-02
**Success Criteria** (what must be TRUE):
  1. 用户按 D 键复制服务器后，新条目立即出现在服务器列表底部（或正确排序位置），不弹出编辑表单
  2. 复制完成后列表自动滚动到新条目，新条目处于选中状态
  3. 状态栏显示确认信息（如 "Server duplicated: newserver-copy"）
**Plans**: 1 plan

Plans:
- [x] 10-01: Rewrite handleServerDup() for direct save with search-aware positioning + dupPendingAlias cleanup
**UI hint**: yes

### Phase 11: T Key Marking
**Goal**: 用户可以在服务器列表按 T 键依次标记两个服务器为源端和目标端，标记完成后自动打开双远端文件浏览器
**Depends on**: Phase 10 (no code dependency, but sequential delivery)
**Requirements**: MARK-01, MARK-02, MARK-03, MARK-04, MARK-05
**Success Criteria** (what must be TRUE):
  1. 用户选中服务器后按 T 键，该服务器前显示 [S] 源端标记，状态栏提示 "Press T on target server"
  2. 用户选中另一台服务器后按 T 键，该服务器前显示 [T] 目标端标记，自动打开双远端文件浏览器
  3. 标记状态下按 Esc 清除所有标记，恢复普通选择状态
  4. 用户尝试标记同一服务器两次时，显示错误提示（如 "Cannot mark same server twice"），标记状态不变
**Plans**: TBD
**UI hint**: yes

### Phase 12: Dual Remote Browser
**Goal**: 用户可以在独立的 DualRemoteFileBrowser 中浏览两台远程服务器的文件系统，支持键盘导航和同面板内文件操作
**Depends on**: Phase 11 (T 键标记作为入口)
**Requirements**: DRB-01, DRB-02, DRB-03, DRB-04
**Success Criteria** (what must be TRUE):
  1. 双远端浏览器打开后，左栏显示源端服务器文件列表，右栏显示目标端服务器文件列表，各自独立连接
  2. 用户可以 Tab 键在左右面板间切换焦点，上下方向键浏览文件，Enter 进入目录，h 返回上级目录
  3. 同面板内的文件操作（删除 d、重命名 R、新建目录 m）正常工作，操作执行在对应远程服务器上
  4. 按 Esc 或 q 退出浏览器后，两个 SFTP 连接关闭，资源清理完成，返回服务器列表
**Plans**: TBD
**UI hint**: yes

### Phase 13: Cross-Remote Transfer
**Goal**: 用户可以在双远端浏览器中通过 c/x + p 机制在两台服务器之间复制/移动文件和目录，含两阶段进度显示和冲突处理
**Depends on**: Phase 12 (DualRemoteFileBrowser 组件)
**Requirements**: XFR-01, XFR-02, XFR-03, XFR-04, XFR-05, XFR-06, XFR-07
**Success Criteria** (what must be TRUE):
  1. 用户在左栏按 c 标记文件后切换到右栏按 p，文件从源端服务器传输到目标端服务器（download-to-temp + re-upload），绿色 [C] 前缀显示
  2. 用户按 x 标记文件后粘贴，文件传输到目标端并从源端删除，红色 [M] 前缀显示
  3. 传输过程中 TransferModal 显示两阶段进度："Downloading from A..." 完成后重置为 "Uploading to B..."
  4. 传输过程中按 Esc 取消，本地临时文件和目标端部分文件被清理
  5. 目标文件已存在时弹出冲突对话框（覆盖/跳过/重命名），用户选择后继续或中止
**Plans**: TBD
**UI hint**: yes

## Progress

**Execution Order:**
Phases execute in numeric order: 10 -> 11 -> 12 -> 13

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Foundation | v1.0 | 3/3 | Complete | 2026-04-13 |
| 2. Core Transfer | v1.0 | 3/3 | Complete | 2026-04-13 |
| 3. Polish | v1.0 | 3/3 | Complete | 2026-04-13 |
| 4. Directory History Core | v1.1 | 2/2 | Complete | 2026-04-14 |
| 5. Recent Directories Popup | v1.1 | 1/1 | Complete | 2026-04-14 |
| 6. Basic File Operations | v1.2 | 3/3 | Complete | 2026-04-15 |
| 7. Copy & Clipboard | v1.2 | 2/2 | Complete | 2026-04-15 |
| 8. Move & Integration | v1.2 | 2/2 | Complete | 2026-04-15 |
| 9. Dup SSH Connection | v1.3 | 1/1 | Complete | 2026-04-15 |
| 10. Dup Fix | v1.4 | 1/1 | Complete   | 2026-04-15 |
| 11. T Key Marking | v1.4 | 0/? | Not started | - |
| 12. Dual Remote Browser | v1.4 | 0/? | Not started | - |
| 13. Cross-Remote Transfer | v1.4 | 0/? | Not started | - |
