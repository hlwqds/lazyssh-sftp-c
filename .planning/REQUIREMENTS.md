# Requirements: LazySSH File Transfer

**Defined:** 2026-04-13
**Core Value:** 在终端内完成 SSH 文件传输，无需切换到 FileZilla 或记忆 scp 命令

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### File Browsing

- [x] **BROW-01**: User can browse local directories with file list display (name, size, date, permissions)
- [ ] **BROW-02**: User can browse remote directories via SFTP with file list display
- [x] **BROW-03**: User can navigate to parent directory (../) in both panes
- [x] **BROW-04**: User can toggle hidden file visibility in both panes
- [x] **BROW-05**: User can see current path displayed for both local and remote panes
- [x] **BROW-06**: User can sort files by name, size, or date in both panes

### File Transfer

- [ ] **TRAN-01**: User can upload a single file from local to remote
- [ ] **TRAN-02**: User can download a single file from remote to local
- [ ] **TRAN-03**: User can upload a directory recursively from local to remote
- [ ] **TRAN-04**: User can download a directory recursively from remote to local
- [ ] **TRAN-05**: User can see detailed transfer progress (progress bar, speed, ETA)
- [ ] **TRAN-06**: User can cancel an in-progress transfer
- [ ] **TRAN-07**: User is prompted when destination file already exists (overwrite/skip/rename)

### User Interface

- [ ] **UI-01**: User can open file browser by pressing `F` on a selected server
- [ ] **UI-02**: User sees dual-pane layout (left=local, right=remote)
- [ ] **UI-03**: User can navigate files with arrow keys and j/k
- [ ] **UI-04**: User can select multiple files with Space key
- [ ] **UI-05**: User can switch pane focus with Tab key
- [ ] **UI-06**: User can initiate transfer with Enter key on selected file(s)
- [ ] **UI-07**: User sees status bar with connection info and transfer status
- [ ] **UI-08**: User sees error messages displayed clearly in the UI

### Integration

- [x] **INTG-01**: File browser uses existing SSH config from selected server (zero-config)
- [x] **INTG-02**: SFTP connection established via system SSH binary (respects ~/.ssh/config, ssh-agent)
- [ ] **INTG-03**: File browser works on Linux, Windows, and macOS

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Transfer

- **TRAN-V2-01**: Multi-file parallel transfer
- **TRAN-V2-02**: Transfer queue management
- **TRAN-V2-03**: Resume partial transfers

### Advanced Browsing

- **BROW-V2-01**: File preview (view file content in terminal)
- **BROW-V2-02**: Archive browsing (zip/tar as directories)
- **BROW-V2-03**: Transfer history log

## Out of Scope

| Feature | Reason |
|---------|--------|
| Go native SSH library (golang.org/x/crypto/ssh) | Uses its own transport, bypasses user's SSH config |
| File content editing | lazyssh scope is transfer, not editing |
| Drag-and-drop emulation | TUI tools are keyboard-driven |
| Shell link / fish protocol | mc-specific VFS abstractions, unnecessary complexity |
| Bookmark management | Server list already serves this purpose |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| BROW-01 | Phase 1 | Complete |
| BROW-02 | Phase 2 | Pending |
| BROW-03 | Phase 1 | Complete |
| BROW-04 | Phase 1 | Complete |
| BROW-05 | Phase 1 | Complete |
| BROW-06 | Phase 1 | Complete |
| TRAN-01 | Phase 2 | Pending |
| TRAN-02 | Phase 2 | Pending |
| TRAN-03 | Phase 2 | Pending |
| TRAN-04 | Phase 2 | Pending |
| TRAN-05 | Phase 2 | Pending |
| TRAN-06 | Phase 3 | Pending |
| TRAN-07 | Phase 3 | Pending |
| UI-01 | Phase 1 | Pending |
| UI-02 | Phase 1 | Pending |
| UI-03 | Phase 1 | Pending |
| UI-04 | Phase 1 | Pending |
| UI-05 | Phase 1 | Pending |
| UI-06 | Phase 2 | Pending |
| UI-07 | Phase 1 | Pending |
| UI-08 | Phase 1 | Pending |
| INTG-01 | Phase 1 | Complete |
| INTG-02 | Phase 1 | Complete |
| INTG-03 | Phase 3 | Pending |

**Coverage:**
- v1 requirements: 24 total
- Mapped to phases: 24
- Unmapped: 0

---
*Requirements defined: 2026-04-13*
*Last updated: 2026-04-13 after roadmap creation*
