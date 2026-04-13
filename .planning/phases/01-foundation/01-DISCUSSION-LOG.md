# Phase 1: Foundation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-13
**Phase:** 1-foundation
**Areas discussed:** File list component, Dual-pane layout, SFTP connection behavior, Navigation behavior

---

## File List Component

| Option | Description | Selected |
|--------|-------------|----------|
| tview.Table | Multi-column display (name, size, date, permissions), more info-dense | ✓ |
| tview.List | Single-column filenames, simpler, matches existing server list style | |

**User's choice:** tview.Table

| Option | Description | Selected |
|--------|-------------|----------|
| Full columns | Name, Size, Modified date, Permissions (drwxr-xr-x) | ✓ |
| Without permissions | Name, Size, Modified date only | |
| Minimal | Name, Size only | |

**User's choice:** Full columns (Name, Size, Modified date, Permissions)

---

## Dual-Pane Layout

| Option | Description | Selected |
|--------|-------------|----------|
| 50:50 | Equal split, symmetrical | ✓ |
| 60:40 | Left 60%, right 40%, local files usually more important | |

**User's choice:** 50:50

| Option | Description | Selected |
|--------|-------------|----------|
| Inside each pane | Current path as pane title, like file manager | ✓ |
| Status bar only | Both paths shown in bottom status bar | |

**User's choice:** Inside each pane header

---

## SFTP Connection Behavior

| Option | Description | Selected |
|--------|-------------|----------|
| On open | Establish SFTP connection immediately when pressing F | ✓ |
| Lazy connect | Show "Connecting..." in right pane, connect when user first focuses it | |

**User's choice:** On open

| Option | Description | Selected |
|--------|-------------|----------|
| Show error in pane | Right pane shows error + reason, left pane still browsable | ✓ |
| Modal dialog | Popup modal, user closes and returns to main view | |

**User's choice:** Show error in pane

---

## Navigation Behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Home dirs | Local ~ (home dir), remote ~ (SSH default home) | ✓ |
| Local cwd, remote home | Local current working directory, remote home | |

**User's choice:** Home dirs

| Option | Description | Selected |
|--------|-------------|----------|
| Backspace + h | Both keys go to parent directory | ✓ |
| Backspace only | Only Backspace goes to parent | |

**User's choice:** Backspace + h

---

## Claude's Discretion

File size format, directory-first sorting, empty directory display, file type colors, column width allocation — left to Claude's judgment.

## Deferred Ideas

None.
