# Roadmap: LazySSH File Transfer

## Milestones

- **v1.0 File Transfer** - Phases 1-3 (shipped 2026-04-13)
- **v1.1 Recent Remote Directories** - Phases 4-5 (shipped 2026-04-14)
- **v1.2 File Operations** - Phases 6-8 (shipped 2026-04-15)

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

## Progress

Full details: .planning/milestones/v1.0-ROADMAP.md, .planning/milestones/v1.1-ROADMAP.md, .planning/milestones/v1.2-ROADMAP.md
