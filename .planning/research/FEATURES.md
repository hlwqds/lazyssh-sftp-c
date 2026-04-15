# Feature Research: File Operations (v1.2)

**Analysis Date:** 2026-04-15
**Domain:** TUI File Management Operations — Delete, Rename, Mkdir, Copy, Move
**Confidence:** HIGH (cross-referenced across mc, ranger, vifm, lf + existing codebase analysis)
**Mode:** Ecosystem

## Research Sources

- **Midnight Commander (mc)** — F5/F6/F7/F8/Shift+F6 keybindings, recursive delete confirmation dialog (canonical dual-pane TUI)
- **ranger** — yy/dd/pp mark-put model, `d` opens destructive actions submenu, `cw` vim-style rename
- **vifm** — yy/dd/p vim-compatible mark-put, `cw` inline rename, `:mkdir` command mode
- **lf** — y/d/p mark-put, `r` inline rename, trash-cli integration for safe delete
- **nnn** — per-file delete confirmation (anti-pattern: alert fatigue), 0 to select all
- **Nielsen Norman Group** — confirmation dialog UX guidelines (use sparingly, for irreversible actions)
- **UX StackExchange** — "Confirm or Undo" debate (undo preferred for recoverable, confirm for irreversible)
- **lazyssh existing codebase** — TransferModal multi-mode state machine, RecentDirs overlay pattern, FileService/SFTPService ports, Clipboard concept not yet implemented

---

## Part 1: File Operations Feature Landscape (v1.2)

### Table Stakes (Users Expect These)

Missing any of these makes the file browser feel incomplete. Users coming from mc/ranger/vifm expect these as bare minimum.

#### Delete (d key)

| Aspect | Detail | Confidence |
|--------|--------|------------|
| **Trigger** | `d` key on selected item (cursor or space-selected) | HIGH — mc F8, ranger `dD`, lf `d`, vifm `d` |
| **Single file** | Delete immediately with confirmation dialog | HIGH |
| **Directory** | Recursive delete with enhanced confirmation (warn about contents) | HIGH — mc shows recursive delete dialog for non-empty dirs |
| **Multi-select** | Delete all space-selected items in batch | HIGH — mc, ranger, lf all support bulk delete |
| **Confirmation** | Single dialog for entire operation (NOT per-file) | HIGH — nnn's per-file prompting is widely criticized as anti-pattern |
| **Dialog content** | Show: item name, type (file/dir), size, recursive warning for dirs | MEDIUM — UX best practice: communicate scope |
| **Post-delete** | Refresh listing, move cursor to nearest sibling (not to top) | MEDIUM — mc preserves cursor position; ranger does too |
| **Local pane** | `os.Remove()` for files, `os.RemoveAll()` for dirs | HIGH — standard Go |
| **Remote pane** | SFTP `Remove()` for files, need `RemoveDirectory()` for recursive | HIGH — pkg/sftp supports both |
| **Error handling** | Show error in status bar, partial success for multi-delete | MEDIUM |

**Edge cases for delete:**
- Deleting the currently open directory (self) — should be blocked
- Deleting a directory that is the parent of the other pane's cwd — mc allows this
- Permission denied on some files in recursive delete — report partial failure
- Symlinks: delete the link itself, not follow it (standard behavior)
- Read-only files — `os.Remove` handles this; SFTP may need mode change first
- Very large directories (1000+ files) — async with progress indication
- Empty selection — status bar message "No files selected"

#### Rename (R key)

| Aspect | Detail | Confidence |
|--------|--------|------------|
| **Trigger** | `R` key on current item (single item only, not multi-select) | HIGH — mc Shift+F6, ranger `cw`, vifm `cw`, lf `r` |
| **UI pattern** | Inline edit: InputField overlay on the file name column | HIGH — all terminal file managers use inline edit |
| **Pre-fill** | Current filename as default text | HIGH — universal pattern |
| **Selection** | Select filename without extension (stem only) | MEDIUM — ranger `cw` selects stem; vifm `cw` selects full name. Selecting stem is more useful. |
| **Confirm** | Enter commits rename, Esc cancels | HIGH |
| **Empty name** | Block and show error "Filename cannot be empty" | HIGH |
| **Name collision** | If new name exists in same directory, show overwrite confirmation | HIGH |
| **Post-rename** | Refresh listing, keep cursor on renamed item | MEDIUM |
| **Local pane** | `os.Rename()` | HIGH |
| **Remote pane** | SFTP `Rename()` | HIGH — pkg/sftp has `client.Rename()` |
| **Extension change** | Allowed — user may want to change file type | HIGH |

**Edge cases for rename:**
- Rename to existing name (no-op) — silently ignore or show "same name"
- Rename to name with invalid characters — `/`, `\0` on Unix; `<`, `>`, `:`, etc. on Windows
- Leading/trailing spaces — trim and warn, or preserve (OS-dependent)
- Very long filenames — truncate display in InputField but accept full name
- Rename parent directory — other pane's cwd becomes invalid; mark as stale
- Concurrent modification (file changed between list and rename) — retry or report error

#### Create Directory (m key)

| Aspect | Detail | Confidence |
|--------|--------|------------|
| **Trigger** | `m` key (mkdir) | HIGH — mc F7, vifm `:mkdir` |
| **UI pattern** | Small centered popup with InputField | HIGH — mc shows input dialog, vifm shows command input |
| **Pre-fill** | Empty input field with cursor ready | HIGH |
| **Confirm** | Enter creates directory, Esc cancels | HIGH |
| **Empty name** | Block and show error | HIGH |
| **Already exists** | Show error "Directory already exists" | HIGH |
| **Nested creation** | Support `path/to/dir` — create all intermediate directories | MEDIUM — mc does NOT support nested; ranger does not. lazyssh should support it since `MkdirAll` already exists. |
| **Post-create** | Refresh listing, scroll to and select the new directory | MEDIUM |
| **Local pane** | `os.MkdirAll()` | HIGH |
| **Remote pane** | `sftpService.MkdirAll()` — already exists | HIGH — confirmed in sftp_client.go |

**Edge cases for mkdir:**
- Permission denied — show error in status bar
- Invalid characters in path — OS-dependent validation
- Creating in read-only filesystem — error handling
- Path traversal attempts (`../../etc`) — allow if user has permissions (admin tool, not sandbox)

#### Copy (c mark + p paste)

| Aspect | Detail | Confidence |
|--------|--------|------------|
| **Mark trigger** | `c` key marks current item (or selected items) for copy | HIGH — ranger `yy`, vifm `yy`, lf `y` |
| **Mark indicator** | Status bar shows "N file(s) marked for copy" | MEDIUM — ranger shows status line, lf shows copy count |
| **Paste trigger** | `p` key pastes (copies) marked files to current pane's directory | HIGH — ranger `pp`, vifm `p`, lf `p` |
| **Same-directory paste** | Create copy with `.1` suffix (e.g., `file.1.txt`) | MEDIUM — follows existing `nextAvailableName()` pattern |
| **Cross-pane paste** | Copy from local to remote or remote to local (transfer) | HIGH — this is the key value of dual-pane copy |
| **Multi-file** | All marked files copied in batch | HIGH |
| **Directory copy** | Recursive copy of entire directory tree | HIGH |
| **Progress** | Reuse TransferModal for cross-pane (transfer); simple status for same-pane | MEDIUM |
| **Post-paste** | Clear marks, refresh target pane | HIGH |
| **Local-local** | `os.Copy` or `io.Copy` loop for same-pane | HIGH |
| **Remote-remote** | SFTP server-side copy (if supported) or download+reupload | LOW — SFTP protocol doesn't have server-side copy; need download+reupload |

**Edge cases for copy:**
- Mark files, navigate, then mark more files in different directory — should replace previous mark (standard behavior) or extend (ranger extends with `ya`)
- Paste into a directory that doesn't exist yet — create it automatically (MkdirAll)
- Large file copy on same filesystem — could use hard links for speed, but out of scope
- Copy symlink — copy the link target (follow symlink), not the link itself
- Permission errors during recursive copy — report partial failure
- Disk full during copy — stop and report, clean up partial files

#### Move (x mark + p paste)

| Aspect | Detail | Confidence |
|--------|--------|------------|
| **Mark trigger** | `x` key marks current item (or selected items) for move/cut | HIGH — ranger `dd`, vifm `dd`, lf `d` (cut) |
| **Paste trigger** | `p` key pastes (moves) marked files to current pane's directory | HIGH — ranger `pp` (context-dependent: copy or move), vifm `p`, lf `p` |
| **Same-directory paste** | This is effectively a rename — show rename UI instead | MEDIUM — ranger treats same-dir move as rename |
| **Cross-pane move** | Move = copy + delete source | HIGH — this is the dual-pane value |
| **Multi-file** | All marked files moved in batch | HIGH |
| **Directory move** | Recursive move of entire directory tree | HIGH |
| **Confirmation** | For cross-pane move, confirm before deleting source | MEDIUM — destructive operation |
| **Post-paste** | Clear marks, refresh both source and target panes | HIGH |
| **Source cleanup** | After successful cross-pane copy, delete source files | HIGH |
| **Error recovery** | If delete fails after copy, leave source files and report error | HIGH |

**Edge cases for move:**
- Move to same location — no-op (or rename if name changed)
- Move parent directory into itself — block (infinite loop)
- Move directory into its own subdirectory — block
- Partial failure in multi-file move — some files copied but not all deleted from source — report discrepancy
- Cross-filesystem move (local-local) — requires copy+delete, not atomic rename
- Network interruption during remote move — partial state, need cleanup

---

### Differentiators (What Sets lazyssh Apart)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Unified c/x/p model for cross-pane operations** | In mc/ranger, copy and transfer are separate actions (F5 for transfer). In lazyssh, `c` mark + `p` paste on the other pane = transfer. This unifies local copy and cross-pane transfer under one mental model. | MEDIUM | Clipboard struct tracks source pane; paste detects cross-pane vs same-pane. |
| **Cross-pane move (copy + delete)** | Most dual-pane managers treat F6 (move) as transfer + delete. lazyssh's `x` + `p` achieves the same but with the same mark-put consistency. | MEDIUM | Reuses existing TransferService for the copy phase, then DeleteService for cleanup. |
| **Status bar mark indicator** | After pressing `c` or `x`, status bar shows "3 file(s) marked for copy" or "2 file(s) marked for move". This gives immediate feedback without a separate panel. | LOW | Simple text update in statusBar. |
| **Recursive delete with scope display** | Confirmation dialog shows item count and total size for directory deletes, not just "Delete directory?" | LOW | Reuse WalkDir for counting. |

### Anti-Features (Explicitly NOT Build)

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **Trash/recycle bin integration** | 1. `trash-cli` not available on all systems (zero dependency constraint)<br>2. Remote SFTP has no trash concept<br>3. Different trash APIs per platform (gio, trash-cli, macOS Trash) | Permanent delete with confirmation. User can use system file manager for recoverable delete. |
| **Undo (u key)** | 1. Implementing undo for file operations requires operation logging and reverse execution<br>2. SFTP has no transaction support — undo after network failure is unreliable<br>3. Significant complexity for v1.2 | Confirmation dialog before destructive operations. This is the "confirm" side of the confirm-vs-undo debate. |
| **Bulk rename (visual mode)** | 1. Ranger's `:bulkrename` launches external `$EDITOR` — lazyssh doesn't have an editor<br>2. Pattern-based rename (regex, numbering) is complex<br>3. Out of scope for v1.2 | Single-item rename with inline edit. Bulk rename can be v2+ with pattern syntax. |
| **File permissions editing** | 1. Different permission models across platforms (Unix vs Windows)<br>2. SFTP chmod may not be available on all servers<br>3. Display-only permissions already shown | Read-only permission display (already exists). chmod as future feature. |
| **Copy/move progress for same-filesystem operations** | Same-filesystem copy/move are fast (rename is atomic). Progress UI adds complexity without value. | Status bar message "Copying..." or "Moving..." with completion notification. |
| **Drag-and-drop between panes** | TUI is keyboard-driven. Mouse support exists in tview but adds complexity for drag gestures. | Mark (c/x) + navigate + paste (p) model. |

---

### Feature Dependencies

```
[Delete (d)]
    └──requires──> [ConfirmationDialog UI component]
                      └──new──> overlay component (follows TransferModal pattern)
    └──requires──> [DeleteService port]
                      └──new──> ports.FileService extension or new FileOpsService
    └──requires──> [SFTPClient.RemoveDirectory()]
                      └──partially exists──> Remove() for files, need recursive RemoveAll()

[Rename (R)]
    └──requires──> [InputField overlay]
                      └──new──> tview.InputField as overlay component
    └──requires──> [SFTPClient.Rename()]
                      └──exists──> pkg/sftp client.Rename() (not yet wrapped in SFTPClient)

[Mkdir (m)]
    └──requires──> [InputField overlay]
                      └──shared──> same component as Rename
    └──requires──> [SFTPClient.MkdirAll()]
                      └──exists──> already implemented in sftp_client.go

[Copy (c + p)]
    └──requires──> [Clipboard state management]
                      └──new──> struct in FileBrowser with SourcePane, Files, Operation
    └──requires──> [DeleteService] (for source cleanup in move)
    └──requires──> [SFTPClient.WalkDir()]
                      └──exists──> already implemented
    └──cross-pane──> [TransferService]
                        └──exists──> reuse UploadFile/UploadDir/DownloadFile/DownloadDir

[Move (x + p)]
    └──requires──> [Clipboard state management]
                      └──shared──> same as Copy
    └──requires──> [Copy implementation]
                      └──depends──> move = copy + delete source
    └──requires──> [DeleteService]
    └──cross-pane──> [TransferService] + [DeleteService]
```

### Dependency Notes

1. **InputField overlay is shared between Rename and Mkdir.** Both need a text input popup. Build once, use for both. The InputField overlay should support: pre-filled text, placeholder text, label, and a confirm/cancel callback pattern.

2. **ConfirmationDialog is shared between Delete and Move confirmation.** Both need a yes/no dialog. The existing TransferModal's cancel-confirm mode (modeCancelConfirm) demonstrates this pattern exactly. Build a reusable ConfirmDialog component.

3. **Clipboard state must live in FileBrowser**, not in individual panes. Copy/move are inherently cross-pane operations. The FileBrowser is the only component that knows about both panes and can coordinate mark-put operations.

4. **SFTPClient already has `Remove()` and `MkdirAll()` and `WalkDir()`.** What's missing is `Rename()` (pkg/sftp supports it but it's not wrapped) and `RemoveAll()` (recursive directory removal). These are thin wrappers.

5. **Cross-pane copy/move reuses existing TransferService.** When pasting to the other pane, the copy phase is just an upload or download. This means we get progress tracking, conflict handling, and cancellation for free.

6. **Same-pane copy needs new local copy logic.** `os.Rename()` only works on same filesystem. For robustness, implement `copyFile(src, dst)` using `io.Copy` with buffered reads, plus `copyDir(src, dst)` for recursive directory copy.

---

### MVP Definition (v1.2)

#### Launch With (P1)

- [ ] **Delete single file** — `d` key, confirmation dialog, local + remote
- [ ] **Delete directory (recursive)** — `d` key, enhanced confirmation showing scope, local + remote
- [ ] **Delete multi-selected files** — space-select + `d`, batch confirmation
- [ ] **Rename file/directory** — `R` key, inline InputField overlay, local + remote
- [ ] **Create directory** — `m` key, InputField popup, local + remote (nested path support)
- [ ] **Copy mark** — `c` key marks file(s), status bar indicator
- [ ] **Copy paste (same pane)** — `p` copies marked files to current directory
- [ ] **Copy paste (cross-pane)** — `p` transfers marked files to other pane (reuses TransferService)
- [ ] **Move mark** — `x` key marks file(s), status bar indicator
- [ ] **Move paste (same pane)** — `p` renames (same as move within dir)
- [ ] **Move paste (cross-pane)** — `p` transfers + deletes source

#### Add After Validation (v1.x)

- [ ] **Mark indicator in file listing** — visual marker (e.g., `C` or `M` prefix) on marked files
- [ ] **Mark all files** — `0` or `Ctrl+A` to select all files in current directory
- [ ] **Copy progress for large same-pane operations** — progress bar for large directory copies
- [ ] **Conflict resolution for same-pane copy** — overwrite/skip/rename dialog (reuse TransferModal pattern)
- [ ] **Keyboard shortcut help** — `?` key shows all keybindings including new ones

#### Future Consideration (v2+)

- [ ] **Undo last operation** — operation log with reverse execution
- [ ] **Bulk rename with patterns** — regex, numbering, case conversion
- [ ] **File permissions editing** — chmod for local and remote
- [ ] **Symlink creation** — `ln -s` equivalent
- [ ] **File ownership changes** — chown (local only, limited remote support)
- [ ] **Search within directory** — filter/narrow file listing by pattern

---

### Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority | Phase Suggestion |
|---------|------------|---------------------|----------|------------------|
| Delete (file + dir) | HIGH | LOW-MEDIUM | P1 | Phase 1 (simplest, establishes confirmation dialog pattern) |
| Rename | HIGH | LOW | P1 | Phase 2 (establishes InputField overlay pattern) |
| Mkdir | HIGH | LOW | P1 | Phase 2 (shares InputField with rename) |
| Copy (mark + paste) | HIGH | MEDIUM | P1 | Phase 3 (requires clipboard state, cross-pane reuses TransferService) |
| Move (mark + paste) | HIGH | MEDIUM | P1 | Phase 3 (depends on copy + delete infrastructure) |
| Multi-select delete | MEDIUM | LOW | P1 | Phase 1 (extends single delete) |
| Cross-pane copy/move | HIGH | MEDIUM | P1 | Phase 3 (depends on clipboard + transfer service) |
| Mark indicator in listing | MEDIUM | LOW | P2 | Phase 3 or post-v1.2 |
| Conflict resolution (same-pane) | MEDIUM | MEDIUM | P2 | Post-v1.2 |
| Bulk rename | MEDIUM | HIGH | P3 | v2+ |
| Undo | HIGH | HIGH | P3 | v2+ |
| Permissions editing | LOW | MEDIUM | P3 | v2+ |

---

### Competitor Feature Analysis (File Operations)

| Feature | Midnight Commander | Ranger | vifm | lf | lazyssh (planned) |
|---------|-------------------|--------|------|----|-------------------|
| **Delete** | F8, confirm dialog, recursive | `dD` or `d` submenu | `d`, confirm | `d`, trash-cli | `d`, confirm dialog |
| **Rename** | Shift+F6 | `cw` (vim-style) | `cw` (vim-style) | `r` (inline) | `R`, inline InputField |
| **Mkdir** | F7, input dialog | `:mkdir` command | `:mkdir` command | `:mkdir` command | `m`, InputField popup |
| **Copy** | F5 (dual-pane, no mark) | `yy` + `pp` (mark-put) | `yy` + `p` (mark-put) | `y` + `p` (mark-put) | `c` + `p` (mark-put) |
| **Move** | F6 (dual-pane, no mark) | `dd` + `pp` (mark-put) | `dd` + `p` (mark-put) | `d` + `P` (mark-put) | `x` + `p` (mark-put) |
| **Cross-pane** | Implicit (other pane is target) | N/A (single pane) | Dual-pane aware | N/A (single pane) | Explicit mark + paste to other pane |
| **Undo** | No | Yes (`u`) | Yes (`u`) | Yes (`u`) | No (v1.2) |
| **Trash** | No (permanent) | No (permanent) | Optional | Yes (trash-cli) | No (permanent + confirm) |
| **Multi-select** | Insert key | Space/v/V | t/v/V | Space | Space (existing) |

**Key insight:** lazyssh follows the **mark-put model** (ranger/vifm/lf) rather than the **dual-pane implicit model** (mc). This is because:
1. The mark-put model is more flexible — user can navigate anywhere before pasting
2. It works naturally with the existing Space multi-select
3. It unifies local copy and cross-pane transfer under one mental model
4. Status bar feedback ("3 files marked for copy") makes the state visible

**MC's lesson:** MC uses F5/F6 which implicitly target the other pane. This is simpler but less flexible — you can't copy to a subdirectory of the other pane without first navigating there. The mark-put model is strictly more capable.

**ranger/vifm's lesson:** These use vim-style `yy`/`dd` which conflicts with lazyssh's `y` (potential future yank/copy-to-clipboard). lazyssh uses `c` for copy and `x` for move, which is more intuitive for non-vim users and avoids key conflicts.

---

### How Each Operation Works in Practice

#### Delete Flow

```
1. User navigates to file/directory
2. User optionally space-selects multiple items
3. User presses `d`
4. Confirmation dialog appears:
   - Single file: "Delete 'filename.txt' (1.2 MB)? [y] Yes [n] No [Esc] Cancel"
   - Directory: "Delete 'mydir/' (15 files, 3.4 MB)? [y] Yes [n] No [Esc] Cancel"
   - Multi-select: "Delete 5 selected items? [y] Yes [n] No [Esc] Cancel"
5. User presses `y` or Enter to confirm, `n` or Esc to cancel
6. If confirmed:
   - Files: os.Remove / SFTP Remove
   - Directories: os.RemoveAll / SFTP RemoveAll (recursive)
   - Multi-file: iterate, report partial failures
7. Refresh listing, show status message
```

#### Rename Flow

```
1. User navigates to file/directory
2. User presses `R`
3. InputField overlay appears on the name column, pre-filled with current name
4. User edits the name (cursor starts at stem, before extension)
5. User presses Enter to confirm:
   - Validate: non-empty, no invalid characters
   - Check for name collision in same directory
   - If collision: show overwrite confirmation
   - Execute: os.Rename / SFTP Rename
   - Refresh listing, keep cursor on renamed item
6. User presses Esc to cancel: dismiss overlay, no changes
```

#### Mkdir Flow

```
1. User presses `m`
2. InputField popup appears centered: "Create directory: [input field]"
3. User types directory name (supports nested: "path/to/newdir")
4. User presses Enter to confirm:
   - Validate: non-empty, no invalid characters
   - Check if already exists
   - Execute: os.MkdirAll / SFTP MkdirAll
   - Refresh listing, scroll to and select new directory
5. User presses Esc to cancel: dismiss popup
```

#### Copy Flow

```
1. User navigates to file/directory
2. User optionally space-selects multiple items
3. User presses `c` (copy mark)
4. Status bar updates: "3 file(s) marked for copy"
5. User navigates to target directory (same pane, other pane, or subdirectory)
6. User presses `p` (paste):
   a. Same pane, same directory → copy with .1 suffix
   b. Same pane, different directory → copy to target
   c. Other pane → upload/download (reuses TransferService)
7. If cross-pane: TransferModal shows progress
8. Status bar: "Copied 3 file(s)"
9. Marks cleared, target pane refreshed
```

#### Move Flow

```
1. User navigates to file/directory
2. User optionally space-selects multiple items
3. User presses `x` (move/cut mark)
4. Status bar updates: "3 file(s) marked for move"
5. User navigates to target directory
6. User presses `p` (paste):
   a. Same pane, same directory → no-op (or show rename if name differs)
   b. Same pane, different directory → move within filesystem
   c. Other pane → transfer + delete source
7. If cross-pane:
   a. TransferModal shows copy progress
   b. After successful copy, delete source files
   c. If delete fails: report error, source files remain
8. Status bar: "Moved 3 file(s)"
9. Both panes refreshed, marks cleared
```

---

### Keyboard Binding Summary (v1.2 Additions)

| Key | Context | Action | Conflicts |
|-----|---------|--------|-----------|
| `d` | Any pane | Delete selected item(s) | None (current bindings: no `d` in panes) |
| `R` | Any pane | Rename current item | None (current bindings: no `R` in panes; lowercase `r` is recent dirs on remote) |
| `m` | Any pane | Create directory | None (current bindings: no `m` in panes) |
| `c` | Any pane | Mark for copy | None (current bindings: no `c` in panes) |
| `x` | Any pane | Mark for move | None (current bindings: no `x` in panes) |
| `p` | Any pane | Paste (copy or move) | None (current bindings: no `p` in panes) |
| `y` | Confirm dialog | Confirm (yes) | None in pane context (future: yank to clipboard) |
| `n` | Confirm dialog | Cancel (no) | None in pane context |

**Key conflict analysis:** All proposed keys are free in the current key routing chain. The only potential conflict is `r` (lowercase) which is used for recent directories on remote pane. `R` (uppercase, Shift+R) is free in all contexts. `y` and `n` are only consumed by confirmation dialogs (overlay intercepts all keys when visible), so they don't conflict with any future use in pane context.

---

### Existing Infrastructure Reuse

| Component | Current Use | v1.2 Reuse |
|-----------|-------------|------------|
| `TransferModal` | File transfer progress/cancel/conflict | Cross-pane copy/move progress display |
| `TransferService.UploadFile/UploadDir` | File upload | Cross-pane copy (local → remote) |
| `TransferService.DownloadFile/DownloadDir` | File download | Cross-pane copy (remote → local) |
| `SFTPService.MkdirAll` | Transfer directory creation | Remote mkdir |
| `SFTPService.WalkDir` | Transfer directory walking | Delete scope counting, recursive copy |
| `SFTPService.Stat` | Conflict resolution | Rename collision detection, delete scope |
| `SFTPService.Remove` | Not yet used by UI | Single file delete |
| `SFTPService.CreateRemoteFile` | Transfer | Not needed for file ops |
| `domain.ConflictHandler` | Transfer conflicts | Same-pane copy conflict resolution |
| `nextAvailableName()` | Transfer rename on conflict | Same-pane copy naming |
| `Pane.SelectedFiles()` | Transfer multi-select | Delete/copy/move multi-select |
| `Pane.GetCurrentPath()` | Transfer path resolution | All operations |
| `Pane.Refresh()` | Post-transfer refresh | Post-operation refresh |
| `handleGlobalKeys` | Global key routing | Add d/R/m/c/x/p routing |
| Overlay draw chain (`Draw()`) | TransferModal + RecentDirs | Add ConfirmDialog + InputField overlays |

**New infrastructure needed:**
- `ConfirmDialog` — reusable yes/no dialog overlay (for delete confirmation)
- `InputFieldOverlay` — reusable text input popup (for rename and mkdir)
- `Clipboard` — mark state struct in FileBrowser
- `FileOpsService` (or extend `FileService`) — delete, rename, mkdir, local copy
- `SFTPClient.Rename()` — wrapper for pkg/sftp `client.Rename()`
- `SFTPClient.RemoveAll()` — recursive directory removal

---

## Sources

- [Midnight Commander Cheat Sheet (GitHub Gist)](https://gist.github.com/samiraguiar/9cd4264445545cfd459d)
- [MC Official Man Page](https://source.midnight-commander.org/man/mc.html)
- [MC Recursive Delete Dialog — Ubuntu Manpage](https://manpages.ubuntu.com/manpages/focal//man1/mc.1.html)
- [Ranger Cheatsheet (GitHub Gist)](https://gist.github.com/heroheman/aba73e47443340c35526755ef79647eb)
- [Ranger PDF Keybindings](https://debian-install-notes.pages.dev/files/ranger-keybinds_quinton.pdf)
- [Better Terminal File Management with Ranger](https://obaranovskyi.com/environments/better-terminal-file-management-with-ranger)
- [lf Terminal File Manager Documentation](https://github.com/gokcehan/lf/blob/master/doc.md)
- [lf Delete Behavior Discussion (GitHub Issue)](https://github.com/gokcehan/lf/issues/45)
- [nnn Recursive Delete Per-File Prompt (Superuser)](https://superuser.com/questions/1623048/remove-a-folder-without-confirm-deletion-of-every-single-file-in-nnn-file-manage)
- [UX StackExchange — Confirm or Undo](https://ux.stackexchange.com/questions/71960/deletion-confirm-or-undo-which-is-the-better-option-and-why)
- [Nielsen Norman Group — Confirmation Dialog Guidelines](https://www.nngroup.com/articles/confirmation-dialog/)
- [TUI File Navigation and Batch Rename with tview](https://joshalletto.com/posts/terminal-batch-rename/)
- [tview InputField Wiki](https://github.com/rivo/tview/wiki/InputField)
- Existing codebase: `internal/adapters/ui/file_browser/` (all files)
- Existing codebase: `internal/core/ports/file_service.go`
- Existing codebase: `internal/adapters/data/sftp_client/sftp_client.go`
- Existing codebase: `internal/core/domain/transfer.go`

---
*Features research: 2026-04-15 — v1.2 File Operations focus*
