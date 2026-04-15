# Phase 7: Copy & Clipboard - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-15
**Phase:** 7-copy-clipboard
**Areas discussed:** Remote copy strategy, Multi-select copy, Copy progress display, Clipboard clearing, Same-dir paste, Clipboard data model, Local copy metadata

---

## Remote copy strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Reuse TransferService (Recommended) | CopyRemoteFile/CopyRemoteDir added to TransferService. Reuses existing 32KB buffer, onProgress, conflict handler, ctx cancel. Local copy on FileService. | ✓ |
| New CopyService port | Dedicated CopyService interface. SFTPClient implements with internal download+re-upload. Cleaner separation but duplicates infrastructure. | |

**User's choice:** Reuse TransferService
**Notes:** Remote copy reuses TransferService infrastructure. Local copy goes on FileService.

---

## Multi-select copy

| Option | Description | Selected |
|--------|-------------|----------|
| Single selection only (Recommended) | c only marks current cursor file. Consistent with CLP-01/03 singular description. | ✓ |
| Multi-select support | Space + c marks multiple files. Clipboard stores multiple paths. Consistent with batch delete UX. | |

**User's choice:** Single selection only
**Notes:** Clipboard holds one item. Multi-select留给未来版本.

---

## Copy progress display

| Option | Description | Selected |
|--------|-------------|----------|
| Reuse TransferModal (Recommended) | TransferModal new modeCopy mode. Reuses progress bar, cancel, conflict dialog, summary. Download shows Download progress, upload shows Upload progress. | ✓ |
| Status bar only | Status bar shows copy progress text. Lightweight but no cancel/conflict/summary. | |

**User's choice:** Reuse TransferModal
**Notes:** TransferModal gets modeCopy. Local copy uses no progress (instant).

---

## Clipboard clearing

| Option | Description | Selected |
|--------|-------------|----------|
| Auto-clear after paste (Recommended) | Clipboard clears after successful paste. Each copy needs fresh c mark. Consistent with Midnight Commander behavior. | ✓ |
| Keep after paste | Clipboard stays active after paste, allowing paste to multiple directories. User must Esc to clear. | |

**User's choice:** Auto-clear after paste
**Notes:** Paste failure does NOT clear (allow retry).

---

## Same-dir paste

| Option | Description | Selected |
|--------|-------------|----------|
| Auto-rename (Recommended) | Auto-generates file.1.txt using existing nextAvailableName logic. No user interaction. | ✓ |
| Prompt with conflict dialog | Show "file exists" prompt with overwrite/rename/cancel. Safer but breaks flow. | |

**User's choice:** Auto-rename
**Notes:** Reuses nextAvailableName() from file_browser.go:587.

---

## Clipboard data model

| Option | Description | Selected |
|--------|-------------|----------|
| Store pane + FileInfo + source path (Recommended) | Stores source pane index (0/1), FileInfo, source directory path. Validates paste target is same pane. | ✓ |
| Minimal: path + filename only | Only stores file path. No pane validation. Simpler but could accidentally trigger cross-pane paste. | |

**User's choice:** Store pane + FileInfo + source path
**Notes:** Paste validates same-pane to prevent cross-pane operations (v1.3+ feature).

---

## Local copy metadata

| Option | Description | Selected |
|--------|-------------|----------|
| Preserve permissions + mtime (Recommended) | Copy preserves source file permissions (os.Chmod) and modification time (os.Chtimes). Matches user expectation of "copy". | ✓ |
| Content only | Only copies content, target uses default permissions and time. Simpler but loses metadata. | |

**User's choice:** Preserve permissions + mtime
**Notes:** Remote copy preserves metadata naturally through SFTP protocol transfer.

---

## Claude's Discretion

- Clipboard state field naming and location on FileBrowser
- [C] prefix rendering style (color, position in Name column)
- TransferModal modeCopy UI layout details (title text, progress format)
- Status bar hint text ("1 file copied", "Clipboard: file.txt", etc.)
- Remote copy error recovery (cleanup when download succeeds but upload fails)

## Deferred Ideas

None — discussion stayed within phase scope
