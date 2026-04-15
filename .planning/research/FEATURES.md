# Feature Research: Dual-Remote File Transfer (v1.4)

**Analysis Date:** 2026-04-15
**Domain:** TUI SSH Manager -- Dual-remote file browser, cross-remote copy/move, T key server marking
**Confidence:** HIGH (based on existing codebase analysis, Midnight Commander FISH protocol behavior, scp -3 pattern, and lazyssh's existing CopyRemoteFile/CopyRemoteDir implementation)
**Mode:** Ecosystem

## Research Sources

- **Midnight Commander (mc)** -- Dual remote panels via SFTP/Shell link VFS, server-to-server copy through local relay (source -> local mc -> destination), F5 copy between remotes
- **scp -3 pattern** -- OpenSSH 7.3+ relay flag: `scp -3 user1@host1:/path user2@host2:/path` routes data through local machine; lazyssh implements equivalent via SFTP download + re-upload
- **lazyssh existing CopyRemoteFile/CopyRemoteDir** -- Already implements the download-to-temp + re-upload pattern with progress reporting and conflict handling (file_browser.go lines 1290-1370)
- **lazyssh existing SFTPClient** -- Single SFTPService instance per FileBrowser; dual-remote requires two independent SFTPClient instances (one per remote pane)
- **Termius** -- Commercial reference: dual SFTP panels, select two servers, browse both simultaneously
- **vifm** -- Dual-pane via FUSE/sshfs mounts; no native dual-remote SSH

---

## Part 1: T Key Server Marking in Server List

### Background: How Terminal Tools Handle Multi-Selection

| Tool | Selection Mechanism | Visual Feedback | Activation |
|------|--------------------|-----------------|------------|
| **Midnight Commander** | Insert key per item | Highlight color change | Immediate, items stay marked |
| **lazyssh (existing)** | Space key in file browser | `*` prefix + gold color | `ToggleSelection()` per file |
| **lazyssh (proposed)** | T key in server list | Tagged indicator in list item | Mark up to 2 servers |

### What Exists in lazyssh Already

The server list (`server_list.go`) currently supports:
- `tview.List` with `ShowSecondaryText(false)` and custom `formatServerLine()` rendering
- `UpdateServers()` rebuilds all items from `[]domain.Server` slice
- `GetSelectedServer()` returns the currently highlighted server
- `SetInputCapture()` intercepts Backspace/Esc/Left/Right for search return
- No concept of marking/tagging servers

The `formatServerLine()` function in `utils.go` already renders visual indicators:
- Pinned icon (emoji `📌` vs `📡`)
- Forwarding glyph `Ⓕ` in green
- Tag badges (up to 2 shown, `+N` overflow)
- Alias, host, last SSH time

The `tui` struct holds `dupPendingAlias string` for the Dup feature, demonstrating the pattern of transient state on the TUI.

### Feature Specification

**Table Stakes:**

| Feature | Why Expected | Complexity | Notes |
|---------|-------------|------------|-------|
| **T key marks current server** | User needs to select two servers for dual-remote mode | LOW | Add `case 'T':` to `handleGlobalKeys()`, toggle mark state |
| **Visual indicator for marked servers** | User needs to see which servers are selected | LOW | Add `[A]`/`[B]` prefix or color to `formatServerLine()` |
| **Max 2 marks** | Dual-remote only needs source + destination | LOW | Guard: if 2 already marked, show error or replace oldest |
| **Esc clears marks** | Standard cancel pattern, consistent with clipboard Esc clear | LOW | Clear mark state in Esc handler (after clipboard check) |
| **Auto-open dual-remote browser on 2nd mark** | Reduces friction -- marking second server triggers the action | LOW | After second T press, call `handleDualRemoteFileBrowser(serverA, serverB)` |
| **Status bar hint while marking** | User needs to know marking mode is active | LOW | `showStatusTemp("1/2 servers marked (T to mark second, Esc to cancel)")` |

**Differentiators:**

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Order matters: first=A (left), second=B (right)** | Predictable pane assignment reduces confusion | LOW | First T press = left pane source, second = right pane destination |
| **Mark persists across list navigation** | User can scroll to find second server without losing first mark | LOW | Mark state stored on `tui` struct, not tied to list selection |
| **Mark shown in server details panel** | Reinforces which servers are selected | LOW | Update `ServerDetails` to show "Selected as source/destination" |

**Anti-Features:**

| Anti-Feature | Why Avoid | What to Do Instead |
|-------------|-----------|-------------------|
| Mark more than 2 servers | Dual-pane is always 2 panels; 3+ marks add complexity with no benefit | Cap at 2, show error on 3rd |
| Persist marks across sessions | Marks are transient actions, not configuration | Clear on app exit naturally |
| Mark + F key conflict | F opens local+remote browser; T opens dual-remote -- different flows | T and F are independent paths from server list |

### Feature Dependencies

```
T key marking (server list) --> Dual-remote file browser UI
Dual-remote file browser UI --> Cross-remote copy/move
Cross-remote copy/move --> Transfer progress for cross-remote
```

### Edge Cases

1. **Same server marked twice** -- T on already-marked server should unmark it (toggle behavior)
2. **Marked server deleted** -- If user marks server A, then deletes it via `d`, clear the mark
3. **Marked server edited** -- If alias/credentials change, keep the mark (Server struct is immutable during session)
4. **Search filter hides marked server** -- Mark state survives filter changes; marked server may not be visible
5. **T during search focus** -- Should not trigger (search bar consumes keys); only works when server list has focus
6. **Refresh (`r`) while marking** -- `refreshServerList()` rebuilds items; mark state on `tui` struct survives since it tracks alias, not list index

### Implementation Notes

**State structure (on `tui`):**
```go
type transferMark struct {
    serverA domain.Server  // first marked (left pane)
    serverB domain.Server  // second marked (right pane)
}

// On tui struct:
transferMarks transferMark // zero value = no marks
```

**Rendering change in `formatServerLine()`:**
- Accept an optional `markRole` parameter (0=none, 1=source/A, 2=dest/B)
- Render `[A]` in cyan for source, `[B]` in magenta for destination
- Position: before the pinned icon, or as a suffix

---

## Part 2: Dup Fix (No Auto-Open Form)

### Background: Current Dup Behavior

The current `handleServerDup()` (handlers.go line 288):
1. Deep copies the selected server struct
2. Clears runtime metadata (PinnedAt, SSHCount, LastSeen)
3. Generates unique alias (`original-copy`, `original-copy-2`, ...)
4. Sets `t.dupPendingAlias = dup.Alias`
5. Opens `ServerForm` in Add mode with pre-filled dup data
6. After save, scrolls to the new entry

### What Needs to Change

**Table Stakes:**

| Feature | Why Expected | Complexity | Notes |
|---------|-------------|------------|-------|
| **D key saves directly without opening form** | The whole point of Dup is quick clone -- opening form defeats the purpose | LOW | Call `serverService.AddServer(dup)` directly instead of opening form |
| **Auto-scroll to new entry** | User should see the new entry immediately | LOW | Already implemented via `dupPendingAlias` pattern |
| **Status bar confirmation** | User needs feedback that dup succeeded | LOW | `showStatusTemp("Duplicated: newalias")` |
| **Clear metadata same as before** | Consistent with existing dup behavior | LOW | Already done in current code |

**Edge Cases:**

1. **Dup fails (e.g., alias collision race)** -- Show error in status bar, don't crash
2. **Dup during search filter** -- New entry appears in filtered list if it matches query
3. **Dup followed by immediate D again** -- Each D creates a new `-copy` entry; the `-copy-2`, `-copy-3` pattern handles this

### Implementation Notes

The fix is straightforward: replace the form-opening code path with a direct `AddServer()` call. The existing `generateUniqueAlias()`, metadata clearing, and `dupPendingAlias` scroll logic all remain. The `dupPendingAlias` field on `tui` can be removed if we scroll directly in the handler.

---

## Part 3: Dual-Remote File Browser UI

### Background: How Midnight Commander Handles Dual-Remote

In Midnight Commander, dual-remote works by:
1. Opening Shell link (`sh://user@host/path`) or SFTP link (`sftp://user@host/path`) in one or both panels
2. Each panel connects independently via SSH
3. F5 copies from active panel to inactive panel (regardless of whether either is remote)
4. Data flows through local mc process (VFS read from source, VFS write to destination)

This is the canonical UX pattern for dual-remote in terminal file managers.

### What Exists in lazyssh Already

The current `FileBrowser` (file_browser.go):
- Has `localPane *LocalPane` and `remotePane *RemotePane` in a 50:50 FlexColumn layout
- `NewFileBrowser()` accepts a single `domain.Server` and a single `ports.SFTPService`
- `RemotePane` wraps `SFTPService` for listing, navigation, and file operations
- `LocalPane` wraps `FileService` for local filesystem operations
- TransferModal already supports `modeCopy` and `modeMove` for remote-to-remote operations
- The `handlePaste()` guard at line 971 explicitly blocks cross-pane paste: `"Cross-pane paste not supported (v1.3+)"`

**Key architectural constraint:** The current `FileBrowser` is hardcoded for local+remote. Dual-remote needs remote+remote, which requires:
- Two `SFTPService` instances (two SSH connections)
- Both panes behave as `RemotePane`
- A new constructor or mode flag

### Feature Specification

**Table Stakes:**

| Feature | Why Expected | Complexity | Notes |
|---------|-------------|------------|-------|
| **Two RemotePane instances** | Both sides need SFTP browsing | MEDIUM | Requires two independent SFTPClient instances |
| **Independent SFTP connections** | Each remote server needs its own SSH/SFTP channel | MEDIUM | Current `tui.sftpService` is a singleton; need factory pattern |
| **50:50 dual-pane layout** | Consistent with existing local+remote layout | LOW | Reuse existing FlexColumn layout, swap LocalPane for RemotePane |
| **Pane titles show server identity** | User must distinguish which server they're browsing | LOW | `RemotePane.UpdateTitle()` already shows `user@host:path` |
| **Tab switches focus between remotes** | Consistent with existing Tab behavior | LOW | Same `switchFocus()` logic, just remote-to-remote |
| **h/Backspace navigates parent** | Standard directory navigation | LOW | Already implemented in RemotePane |
| **Space multi-select** | Consistent with existing behavior | LOW | Already implemented in RemotePane |
| **Sort (s/S)** | Consistent with existing behavior | LOW | Already implemented per-pane |
| **Esc closes browser** | Standard exit pattern | LOW | Must close BOTH SFTP connections on exit |
| **Both panes show connection status** | User needs to know if either connection fails | LOW | Each RemotePane independently shows Connecting/Connected/Error |

**Differentiators:**

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Color-coded pane borders** | Distinguish source (A) from destination (B) at a glance | LOW | Left pane: cyan border, right pane: magenta border |
| **Connection progress for both** | User sees both connections establishing simultaneously | LOW | Both `Connect()` calls in goroutines, independent status |
| **Status bar shows "Dual Remote" mode** | Clear that this is a different mode from local+remote | LOW | Different default status text |
| **Independent hidden file toggle per pane** | Different servers may have different hidden file visibility needs | LOW | Already per-pane via `showHidden` field |

**Anti-Features:**

| Anti-Feature | Why Avoid | What to Do Instead |
|-------------|-----------|-------------------|
| Local pane in dual-remote mode | Dual-remote is specifically for server-to-server; local+remote already exists via F | F key opens local+remote, T+T opens dual-remote |
| Swap pane sides | Adds complexity; the A=left, B=right convention is sufficient | User can mark servers in desired order |
| Sync directory navigation | Not useful when servers have different directory structures | Independent navigation per pane |

### Edge Cases

1. **One connection fails** -- Show error in that pane only, other pane remains usable. Transfer operations that need the failed pane should show error.
2. **Both connections fail** -- Show error in both panes, allow Esc to close.
3. **Connection drops during transfer** -- SFTPClient returns error; TransferModal shows failure summary. Consistent with existing error handling.
4. **Same server marked for A and B** -- Should be prevented: T on already-marked server toggles it off (see Part 1 edge case 1).
5. **Server requires password auth** -- `SFTPClient.Connect()` will block on SSH password prompt in `cmd.Stderr = os.Stderr`. This is the same as existing behavior for F key.
6. **Server uses ProxyJump** -- `buildSSHArgs()` already handles ProxyJump; SFTPClient reuses it.

### Implementation Notes

**Architecture options:**

Option A: **Separate DualRemoteFileBrowser struct** -- New struct that mirrors FileBrowser but uses two RemotePanes. Pros: clean separation, no risk of breaking existing FileBrowser. Cons: code duplication.

Option B: **Parameterized FileBrowser** -- Add a `mode` field (local+remote vs remote+remote) and conditionally create panes. Pros: reuse existing code. Cons: adds branching complexity.

**Recommendation: Option A (separate struct).** The existing FileBrowser has deep assumptions about local+remote (activePane 0=local, 1=remote, buildPath uses filepath.Join vs joinPath based on pane index, conflict handler checks activePane for stat function selection). A separate `DualRemoteFileBrowser` avoids touching any of this working code. The shared logic (TransferModal, clipboard, confirm/input dialogs) can be extracted to helper functions or embedded via composition.

**SFTPService factory:** The current `tui` has a single `sftpService ports.SFTPService`. For dual-remote, we need two independent instances. Options:
- `sftp_client.New(log)` creates a new `SFTPClient` -- this already works, just need to call it twice
- Store as local variables in `handleDualRemoteFileBrowser()`, not on `tui` struct

---

## Part 4: Cross-Remote File Transfer (A -> temp -> B)

### Background: Server-to-Server Transfer Patterns

| Pattern | Data Flow | Bandwidth | Complexity | Used By |
|---------|-----------|-----------|------------|---------|
| **Local relay** | A -> local -> B | 2x network | LOW | mc FISH, lazyssh CopyRemoteFile, scp -3 |
| **Direct server-to-server** | A -> B (direct) | 1x network | HIGH | `ssh hostA "scp file hostB:path"`, rsync --rsync-path |
| **FUSE mount + local copy** | A (sshfs) -> local cp -> B (sshfs) | 2x network + FUSE overhead | MEDIUM | vifm, ranger |

**lazyssh constraint:** "Reuse system scp/sftp commands, no new dependencies." The local relay pattern (A -> temp -> B) is the only option that satisfies this constraint. It's exactly what `CopyRemoteFile`/`CopyRemoteDir` already implement for same-server remote copies.

### What Exists in lazyssh Already

The `TransferService` already has:
- `CopyRemoteFile(ctx, remoteSrc, remoteDst, onProgress, onConflict)` -- Downloads to temp, re-uploads
- `CopyRemoteDir(ctx, remoteSrc, remoteDst, onProgress, onConflict)` -- Downloads dir to temp, re-uploads

**Critical limitation:** These methods use a single `SFTPService` instance (same server for src and dst). For cross-remote, we need:
- Download from Server A's SFTPService
- Upload to Server B's SFTPService

This means we need either:
1. A new `CrossRemoteTransfer(srcSFTP, dstSFTP)` method
2. Or reuse existing `DownloadFile`/`UploadFile` with explicit temp directory management

**Recommendation:** Use option 2 -- compose from existing primitives. The flow is:
1. `DownloadFile(ctx, srcRemotePath, localTempPath, onProgress, nil)` using Server A's SFTPService
2. `UploadFile(ctx, localTempPath, dstRemotePath, onProgress, onConflict)` using Server B's SFTPService
3. Clean up temp directory

This reuses all existing progress reporting, conflict handling, and cancellation logic.

### Feature Specification

**Table Stakes:**

| Feature | Why Expected | Complexity | Notes |
|---------|-------------|------------|-------|
| **Cross-remote copy (c + p)** | Copy file from remote A to remote B | MEDIUM | Download to temp, upload to B. Reuse clipboard pattern. |
| **Cross-remote move (x + p)** | Move file from remote A to remote B | MEDIUM | Copy + delete source on A. Same as remotePasteFile + Remove. |
| **Cross-remote directory copy** | Copy entire directory between remotes | MEDIUM | DownloadDir + UploadDir with temp dir. |
| **Cross-remote directory move** | Move entire directory between remotes | MEDIUM | Copy dir + RemoveAll source. |
| **Transfer progress for cross-remote** | User sees download progress then upload progress | MEDIUM | Two-phase progress: "Downloading from A..." then "Uploading to B..." |
| **Conflict handling on destination (B)** | If file exists on B, show overwrite/skip/rename | LOW | Reuse existing `buildConflictHandler()` pattern |
| **Cancellation (double-Esc)** | Consistent with existing cancel pattern | LOW | `context.WithCancel` propagates to both phases |
| **Temp directory cleanup** | Don't leak temp files on success/failure/cancel | LOW | `defer os.RemoveAll(tmpDir)` |
| **F5 cross-remote directory transfer** | Consistent with existing F5 for local<->remote | MEDIUM | Same pattern as `initiateDirTransfer()` but cross-remote |

**Differentiators:**

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Phase label in progress modal** | User understands the two-phase transfer clearly | LOW | "Phase 1/2: Downloading from server-a" -> "Phase 2/2: Uploading to server-b" |
| **Combined progress** | Show overall progress across both phases | MEDIUM | Track bytes in each phase, report combined percentage |
| **Speed/ETA across both phases** | Realistic ETA accounting for both download and upload | MEDIUM | Need to estimate upload time during download phase |

**Anti-Features:**

| Anti-Feature | Why Avoid | What to Do Instead |
|-------------|-----------|-------------------|
| Direct server-to-server SCP | Requires SSH from A to B (may not be possible due to firewalls) | Local relay (download + upload) |
| Parallel download+upload | Would require streaming (download chunk -> upload chunk), complex and fragile | Sequential: complete download, then upload |
| Persistent temp storage | Temp files should never survive app restart | `os.MkdirTemp` + `defer RemoveAll` |

### Edge Cases

1. **Download succeeds, upload fails** -- Temp file preserved for potential retry? No -- follow existing pattern: clean up, show error summary. User can retry manually.
2. **Download fails partially** -- Cancel context, clean up partial temp dir, show error. Consistent with existing `DownloadDir` behavior.
3. **Cancellation during download phase** -- Context cancel stops download, temp dir cleaned up by defer. Show canceled summary.
4. **Cancellation during upload phase** -- Context cancel stops upload. Download phase already completed (temp exists). Defer cleans up temp. Show canceled summary.
5. **Disk full during download to temp** -- `os.Create` or `io.Copy` returns error. Show in TransferModal.
6. **Disk full during upload from temp** -- SFTP write returns error. Show in TransferModal. Temp cleaned up by defer.
7. **Large directory transfer** -- Progress modal shows per-file progress with file counter, same as existing directory transfer.
8. **Filename conflicts on B** -- `buildConflictHandler()` shows overwrite/skip/rename dialog. Conflict check uses Server B's SFTPService.Stat().
9. **Permissions difference between A and B** -- Download preserves local permissions, upload preserves remote permissions per SFTP protocol. No special handling needed.
10. **Symbolic links** -- SFTP `WalkDir` may or may not follow symlinks. Existing behavior is sufficient; don't add special handling.

### Implementation Notes

**Cross-remote transfer flow (single file):**
```
1. User marks file with 'c' on pane A (source)
2. User switches to pane B (Tab)
3. User presses 'p' (paste)
4. handleCrossRemotePaste():
   a. Create temp file: os.CreateTemp("", "lazyssh-xfer-*")
   b. Phase 1: DownloadFile(ctx, srcPath, tmpPath, downloadProgress, nil) via sftpA
   c. Check ctx for cancellation
   d. Phase 2: UploadFile(ctx, tmpPath, dstPath, uploadProgress, onConflict) via sftpB
   e. Cleanup: os.Remove(tmpPath)
   f. Refresh both panes
```

**Cross-remote transfer flow (directory):**
```
1. User presses F5 on pane A (or c+p for directory)
2. initiateCrossRemoteDirTransfer():
   a. Create temp dir: os.MkdirTemp("", "lazyssh-xfer-*")
   b. Phase 1: DownloadDir(ctx, srcDir, tmpBase, dlProgress, nil) via sftpA
   c. Check ctx for cancellation
   d. Reset progress (TransferModal.ResetProgress())
   e. Phase 2: UploadDir(ctx, tmpBase, dstDir, ulProgress, onConflict) via sftpB
   f. Cleanup: os.RemoveAll(tmpDir)
   g. Refresh both panes
```

**Progress reporting:**
The existing `TransferModal` already has `modeCopy` and `modeMove` which show "Downloading: filename" / "Uploading: filename" labels. For cross-remote, we can add a new mode or reuse `modeCopy`/`modeMove` with updated title text (e.g., "Copying: file.txt (server-a -> server-b)").

**The `ResetProgress()` call between phases** is critical -- it clears the progress bar, speed samples, and ETA so the upload phase starts fresh. This pattern is already used in `remotePasteDir()` (line 1325).

---

## Part 5: Feature Dependencies and Ordering

### Dependency Graph

```
[Dup Fix]                    (independent, can ship alone)
    |
[T Key Marking]             (server list changes only)
    |
[Dual-Remote Browser UI]    (depends on T key marking for entry point)
    |
[Cross-Remote Copy/Move]    (depends on dual-remote browser)
```

### Recommended Phase Structure

**Phase 1: Dup Fix** -- Smallest possible change, immediate value
- D key saves directly without form
- Auto-scroll to new entry
- Status bar confirmation
- Remove `dupPendingAlias` field (or keep for scroll logic)

**Phase 2: T Key Marking** -- Server list changes only
- T key handler on `tui.handleGlobalKeys()`
- Mark state on `tui` struct (up to 2 servers)
- Visual indicator in `formatServerLine()` or list item rendering
- Esc clears marks
- Status bar hints during marking

**Phase 3: Dual-Remote Browser UI** -- New component
- `DualRemoteFileBrowser` struct (or `RemoteFileBrowser`)
- Two `SFTPClient` instances created in `handleDualRemoteFileBrowser()`
- Two `RemotePane` instances
- Connection establishment for both (parallel goroutines)
- Pane navigation, sort, hidden toggle (reuse existing RemotePane logic)
- TransferModal integration
- Close both connections on Esc

**Phase 4: Cross-Remote Copy/Move** -- Transfer logic
- Cross-remote clipboard (c/x on one remote, p on other)
- Single file: DownloadFile + UploadFile via temp
- Directory: DownloadDir + UploadDir via temp
- Two-phase progress reporting
- Conflict handling on destination
- Move = copy + delete source
- F5 directory transfer

### What NOT to Build (v1.4)

- Direct server-to-server SCP (requires SSH from A to B)
- Streaming transfer (download chunk -> upload chunk concurrently)
- Transfer queue (multiple files queued for cross-remote)
- Sync/merge mode (rsync-like differential transfer)
- Bandwidth limiting
- Persistent temp storage for resume

---

## Complexity Assessment

| Feature | Complexity | Risk | Reason |
|---------|-----------|------|--------|
| Dup fix | LOW | LOW | Trivial: remove form, call AddServer directly |
| T key marking | LOW | LOW | Pure UI state on tui struct, no new components |
| Dual-remote browser | MEDIUM | MEDIUM | New component, two SFTP connections, but reuses RemotePane heavily |
| Cross-remote copy (file) | MEDIUM | MEDIUM | Composes existing DownloadFile+UploadFile, new temp management |
| Cross-remote copy (dir) | MEDIUM | MEDIUM | Composes existing DownloadDir+UploadDir, larger temp usage |
| Cross-remote move | MEDIUM | MEDIUM | Copy + delete source, same as existing remote move pattern |
| Two-phase progress | LOW | LOW | Reuses TransferModal with phase labels |
| Conflict handling | LOW | LOW | Reuses buildConflictHandler with dst SFTPService |

**Overall v1.4 complexity: MEDIUM** -- The highest-risk item is the dual-remote browser because it introduces a second SFTP connection. Everything else composes from existing, well-tested patterns.

---

## Sources

- [Midnight Commander dual remote via Shell link](https://unix.stackexchange.com/questions/794062/how-to-connect-and-browse-files-of-remote-server-via-midnight-commanders-shell) -- HIGH confidence: established canonical behavior
- [scp -3 proxy transfer](https://askubuntu.com/questions/1116153/transfer-files-between-two-remote-ssh-servers) -- HIGH confidence: OpenSSH documentation
- [FISH protocol internals](https://www.reddit.com/r/linux/comments/pgpmxf/what_is_the_fish_protocol_for/) -- MEDIUM confidence: community discussion
- [Midnight Commander cheat sheet](https://fekir.info/post/mc-cheat-sheet/) -- MEDIUM confidence: community resource
- [lazyssh source code analysis](internal/adapters/ui/file_browser/) -- HIGH confidence: direct code inspection
- [lazyssh SFTPClient implementation](internal/adapters/data/sftp_client/sftp_client.go) -- HIGH confidence: direct code inspection
- [lazyssh TransferService port](internal/core/ports/transfer.go) -- HIGH confidence: direct code inspection
