# Features Research

**Analysis Date:** 2026-04-13
**Domain:** TUI File Transfer

## Research Sources

- Midnight Commander (mc) — canonical dual-pane TUI file manager since 1994
- lftp — most feature-rich CLI transfer tool
- vifm / lf / ranger / nnn — modern terminal file managers
- FileZilla — GUI reference for transfer UX patterns
- lazyssh existing codebase — current UI patterns and constraints

## Table Stakes (Must Have)

Users expect these in any file transfer tool. Without them, the feature feels incomplete.

### Navigation & Browsing

| Feature | Complexity | Dependency |
|---------|-----------|------------|
| Local directory browsing | LOW | None |
| Remote directory browsing (SFTP) | MEDIUM | SFTP connection |
| Parent directory navigation (../) | LOW | Both browsers |
| Hidden file toggle | LOW | Both browsers |
| Current path display | LOW | Both browsers |
| Sort by name/size/date | LOW | Both browsers |

### Transfer Operations

| Feature | Complexity | Dependency |
|---------|-----------|------------|
| Single file upload | LOW | SFTP connection |
| Single file download | LOW | SFTP connection |
| Directory upload (recursive) | MEDIUM | SFTP walk |
| Directory download (recursive) | MEDIUM | SFTP walk |
| Transfer progress indication | MEDIUM | Progress tracking |
| Transfer cancel | MEDIUM | Process management |

### UX Essentials

| Feature | Complexity | Dependency |
|---------|-----------|------------|
| Keyboard navigation (arrows/j/k) | LOW | tview |
| Quick transfer shortcut (Enter or specific key) | LOW | UI |
| File selection (space to mark) | LOW | UI |
| Status bar with connection info | LOW | UI |
| Error display | LOW | UI |

## Differentiators

Features that set lazyssh apart from standalone tools.

| Feature | Complexity | Why It Matters |
|---------|-----------|----------------|
| Zero-config remote access | LOW | Server list already has SSH config — no manual connection setup like mc requires |
| Seamless server switching | LOW | Switch servers without re-entering credentials |
| Integrated with SSH management | MEDIUM | One tool for connection + transfer + config management |
| Quick-open from server list | LOW | Press `F` on any server → instant file browser |

## Anti-Features (Deliberately NOT Build)

| Feature | Reason |
|---------|--------|
| File preview (F3 in mc) | Adds significant complexity (image previewers, syntax highlighting). lazyssh scope is transfer, not viewing |
| Drag-and-drop emulation | TUI tools are keyboard-driven; emulating drag-and-drop adds complexity without value |
| Archive VFS (browse zip/tar) | Powerful but orthogonal to SSH file transfer |
| File editing | Out of scope per PROJECT.md |
| Shell link / fish protocol | mc-specific VFS abstractions, adds protocol handling complexity |
| Resume/partial transfer | scp/sftp don't natively support; adds significant complexity |
| Multi-threaded transfer | v1 single-threaded for simplicity |
| Transfer queue | Over-engineering for v1 |
| Bookmark management | Server list already serves this purpose |

## Key UX Patterns from Research

### Midnight Commander Key Bindings (Reference)
- F5 = Copy (transfer)
- F6 = Move
- F7 = Mkdir
- F8 = Delete
- F3 = View
- Tab = Switch panels
- Insert = Select file

### FileZilla UX Patterns
- Drag between panes = copy (not move)
- Double-click = transfer
- Non-blocking background transfers
- Non-blocking confirmation dialogs

### lazyssh Adaptation
Given lazyssh is keyboard-driven with single-key shortcuts:
- `Enter` on file = transfer to other pane
- `Tab` = switch pane focus
- `Space` = select/deselect file
- `F5` or `y` = transfer selected
- `Backspace` or `h` = go to parent directory

---
*Features research: 2026-04-13*
