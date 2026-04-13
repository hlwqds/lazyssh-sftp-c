# Roadmap: LazySSH File Transfer

## Overview

在 lazyssh 的终端 SSH 管理器中集成双栏文件传输功能。从 UI 骨架和本地浏览起步，经过核心传输能力交付，最终覆盖取消、冲突处理和跨平台鲁棒性。三个阶段层层递进，每个阶段交付用户可验证的完整能力。

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Foundation** - Dual-pane UI shell, local file browsing, SFTP connection setup (completed 2026-04-13)
- [ ] **Phase 2: Core Transfer** - Remote browsing, file/directory transfer, progress display
- [ ] **Phase 3: Polish** - Transfer cancel, conflict handling, cross-platform support

## Phase Details

### Phase 1: Foundation
**Goal**: Users can open a dual-pane file browser and browse local files with keyboard-driven navigation
**Depends on**: Nothing (first phase)
**Requirements**: UI-01, UI-02, UI-03, UI-04, UI-05, UI-07, UI-08, BROW-01, BROW-03, BROW-04, BROW-05, BROW-06, INTG-01, INTG-02
**Success Criteria** (what must be TRUE):
  1. User presses `F` on a selected server and sees a dual-pane file browser open (left=local, right=remote placeholder)
  2. User can navigate local directories using arrow keys and j/k, seeing files listed with name, size, date, and permissions
  3. User can navigate to parent directory, toggle hidden file visibility, and sort files by name/size/date in both panes
  4. User sees current path displayed for both panes and a status bar showing connection info
  5. User can switch pane focus with Tab, select multiple files with Space, and see clear error messages when operations fail
**Plans**: 3 plans

Plans:
- [x] 01-01-PLAN.md — Domain types, port interfaces, LocalFS adapter, SFTP client adapter, buildSSHArgs extraction
- [x] 01-02-PLAN.md — Dual-pane file browser UI component (local pane, remote pane placeholder, keyboard handlers, status bar)
- [x] 01-03-PLAN.md — Wire file browser into existing TUI (F key entry, dependency injection, status bar update)

### Phase 2: Core Transfer
**Goal**: Users can browse remote files via SFTP and transfer files and directories between local and remote with progress feedback
**Depends on**: Phase 1
**Requirements**: BROW-02, UI-06, TRAN-01, TRAN-02, TRAN-03, TRAN-04, TRAN-05
**Success Criteria** (what must be TRUE):
  1. User can browse remote directories and see files listed with the same detail columns as local files
  2. User can select file(s) and press Enter to upload to remote or download to local
  3. User can select a directory and transfer it recursively to the other side, preserving directory structure
  4. User sees a progress bar with current speed and estimated remaining time during transfers
**Plans**: TBD
**UI hint**: yes

### Phase 3: Polish
**Goal**: Users can safely handle edge cases with cancel support, conflict resolution, and reliable cross-platform operation
**Depends on**: Phase 2
**Requirements**: TRAN-06, TRAN-07, INTG-03
**Success Criteria** (what must be TRUE):
  1. User can cancel a transfer in progress and the system cleans up any partial files left on the destination
  2. When a file already exists at the destination, user is prompted with overwrite/skip/rename options before proceeding
  3. File browsing and transfer work correctly on Linux, Windows, and macOS without platform-specific breakage
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 -> 2 -> 3

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation | 0/3 | Complete    | 2026-04-13 |
| 2. Core Transfer | 0/? | Not started | - |
| 3. Polish | 0/? | Not started | - |
