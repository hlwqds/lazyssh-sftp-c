# Phase 2: Core Transfer - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-13
**Phase:** 2-core-transfer
**Areas discussed:** Transfer trigger, Progress display, Directory transfer, Post-transfer behavior

---

## Transfer Trigger

| Option | Description | Selected |
|--------|-------------|----------|
| Enter + focus determines direction (Recommended) | Enter on file initiates transfer. Direction: local pane focused = upload, remote pane focused = download. Multi-selected files transfer all at once. | ✓ |
| Enter=upload, separate key=download | Enter always uploads. Different key (e.g., F5) downloads. Clearer but two keys to remember. | |
| Enter opens action menu | Enter opens a small action menu (Upload/Download/Cancel). More explicit but adds an extra step. | |

**User's choice:** Enter + focus determines direction
**Notes:** Focus-based direction is intuitive — user looks at target pane, presses Enter.

---

| Option | Description | Selected |
|--------|-------------|----------|
| Enter on files only, dirs separate (Recommended) | Enter on file transfers. Multi-selected files all transfer together. Enter on directory does nothing special (no recursive via Enter). | ✓ |
| Enter on file OR directory | Enter on file transfers single file. Enter on directory recursively transfers whole directory. | |
| Enter shows confirmation first | Enter on file or directory shows confirmation dialog first. | |

**User's choice:** Enter on files only, directories use separate key
**Notes:** Keeps Enter safe (no accidental recursive uploads). Directories get dedicated key.

---

## Progress Display

| Option | Description | Selected |
|--------|-------------|----------|
| Modal overlay with full detail (Recommended) | tview.Modal centered: file name, progress bar, speed, ETA, percentage. Esc cancels. | ✓ |
| Status bar only (minimal) | Progress in existing status bar. Minimal space but limited info. | |
| Status bar + toggle detail view | Status bar summary, 'p' opens detailed overlay. Flexible but adds key binding. | |

**User's choice:** Modal overlay with full detail

---

| Option | Description | Selected |
|--------|-------------|----------|
| Per-file progress (Recommended) | Show progress for current file only. When file completes, show next file. Clear and simple. | ✓ |
| Overall progress across files | Show overall progress (e.g., 3/10 files 45%). Better for multi-file but less granular. | |
| Both per-file and overall | Per-file progress bar + overall counter. Most informative but more complex. | |

**User's choice:** Per-file progress
**Notes:** Simple and clear. User sees exactly what's happening with the current file.

---

## Directory Transfer

| Option | Description | Selected |
|--------|-------------|----------|
| Dedicated key (e.g., F5) on directory (Recommended) | F5 on directory triggers recursive transfer. Direction follows same focus rule. | ✓ |
| Shift+Enter on directory | Shift+Enter triggers recursive. Keeps transfer on Enter but requires modifier. | |
| Enter + confirmation dialog | Enter on directory shows confirmation, then proceeds. Extra step but clear intent. | |

**User's choice:** F5 on directory
**Notes:** F5 is a well-known shortcut in Midnight Commander for copy operations.

---

| Option | Description | Selected |
|--------|-------------|----------|
| Skip failed, continue (Recommended) | Skip failed file, log warning, continue. Show summary at end. | ✓ |
| Abort on first error | Abort entire transfer on first file error. Safer but frustrating. | |
| Prompt for each error | Prompt for each error (Skip/Retry/Abort). Most control but terrible UX for large dirs. | |

**User's choice:** Skip failed, continue
**Notes:** User gets partial result instead of nothing. Summary shows what failed.

---

## Post-Transfer Behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Auto-refresh target pane (Recommended) | Auto-refresh target pane to show transferred files. Scroll to first transferred file. | ✓ |
| Summary overlay + OK button | Show summary overlay ('Uploaded 3 files (1.2MB) in 4s') with OK button. | |
| Status bar message only | Just update status bar with 'Upload complete'. User manually refreshes. | |

**User's choice:** Auto-refresh target pane
**Notes:** Immediate visual confirmation. User sees transferred files appear.

---

## Claude's Discretion

- Progress bar style and color scheme (follow existing theme)
- Speed display format (MB/s vs KB/s auto-switch)
- ETA calculation method (sliding average vs instantaneous)
- Modal layout and border style
- Directory transfer: pre-count total files vs count-as-we-go
