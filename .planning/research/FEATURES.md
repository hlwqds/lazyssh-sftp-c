# Feature Research: Enhanced File Browser (v1.3)

**Analysis Date:** 2026-04-15
**Domain:** TUI SSH File Manager Enhancements — Persistent local path history, Server duplication, Dual-remote file transfer
**Confidence:** MEDIUM (cross-referenced across mc, lf, yazi, Termius, scp manual + existing codebase analysis; dual-remote transfer patterns are niche, less documented)
**Mode:** Ecosystem

## Research Sources

- **Midnight Commander (mc)** — Dual remote panels via SFTP/Shell link VFS, F5 copy between remotes (canonical dual-pane server-to-server transfer)
- **lf** — Persistent history via `~/.local/share/lf/history` (known concurrency overwrite bug), `~/.local/share/lf/dirs` for directory stack
- **yazi** — Session-based path history via `j` key; persistent bookmarks via community plugins (yamb.yazi, bookmarks.yazi using DDS state)
- **ranger** — Directory history is in-memory only, persistent history is open feature request (Issue #1741)
- **Termius** — Dual SFTP panels side by side (closest commercial equivalent to dual-remote feature)
- **SCP manual** — `scp -3` flag for local relay transfer; known limitation: no progress bar output in relay mode
- **SFTP protocol** — No native server-to-server support; proxy requires download+reupload or SSH agent forwarding
- **lazyssh existing codebase** — RecentDirs (remote MRU with disk persistence), TransferService (download+reupload for CopyRemoteFile), SFTPService, FileService ports, ServerService.AddServer

---

## Part 1: Feature 1 — Persistent Local Path History

### Background: How Terminal File Managers Handle Path History

| Manager | Persistence | Storage | Scope | Key |
|---------|-------------|---------|-------|-----|
| **lf** | Yes | `~/.local/share/lf/history` | Global (all directories) | `j` |
| **Midnight Commander** | Yes | `~/.mc/history` | Per-panel directory history | `Alt+Enter` |
| **ranger** | No (in-memory only) | N/A | Session only | `history_go -1` |
| **yazi** | Session only (built-in) | N/A | Session | `j`; persistent via plugins |
| **lazyssh (remote)** | Yes | `~/.lazyssh/recent-dirs/{user@host}.json` | Per-server, 10 entries | `r` |

**Key insight:** The existing lazyssh remote directory history (RecentDirs) already implements per-server persistence. The local path history feature is a symmetric extension: record local paths used for upload/download, persisted to `~/.lazyssh/`.

### What Exists in lazyssh Already

The `RecentDirs` component (`recent_dirs.go`) already provides:
- MRU list with move-to-front deduplication
- JSON persistence to `~/.lazyssh/recent-dirs/{user@host}.json`
- `r` key popup with `j`/`k` navigation and Enter selection
- Overlay rendering on top of FileBrowser
- `Record()` method called after successful transfer

**What's missing for local path history:**
- No `Record()` call for local paths (currently only `fb.remotePane.GetCurrentPath()` is recorded)
- No local-path MRU list (only remote MRU exists)
- No UI for browsing local path history (the `r` key is bound to remote only)
- Local path history should be **global** (not per-server) since local paths are local to the machine

### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Record local upload paths** | After upload (local->remote), the local source directory should be remembered | LOW | One-liner: call `localPaths.Record(fb.localPane.GetCurrentPath())` after upload success |
| **Record local download paths** | After download (remote->local), the local target directory should be remembered | LOW | Same as upload, call after download success |
| **Persistent storage** | Local paths should survive app restart | LOW | JSON file at `~/.lazyssh/local-path-history.json`, mirrors RecentDirs pattern |
| **MRU popup for local paths** | Users expect to quickly jump to previously used local directories | LOW | Reuse RecentDirs overlay component with different data source |
| **Max 10 entries** | Consistent with remote MRU limit | LOW | Same `maxRecentDirs = 10` cap |

### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Global local path history** | Unlike remote paths (per-server), local paths are shared across all servers — one history covers all transfers regardless of which server you're connected to | LOW | Single `~/.lazyssh/local-path-history.json` file |
| **Symmetric UX with remote MRU** | Same `r` key behavior on local pane as on remote pane — users don't need to remember different keys per pane | LOW | Bind `r` on local pane (pane 0) to show local path history popup |

### Anti-Features

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **Named bookmarks for local paths** | Adds complexity (name management, collision handling, search) for minimal value over MRU. MRU automatically surfaces the paths you actually use most. | MRU-only list. Named bookmarks deferred to v2+. |
| **Per-server local path history** | Local paths have nothing to do with which remote server you're connected to. A path used for server A's uploads is equally relevant for server B. | Single global local history file. |
| **Path frecency (frequency+recency) scoring** | Over-engineering for a list of 10 items. MRU is simpler and works well enough — you always want the most recently used, not the most frequently used. | Simple move-to-front MRU. |

### Feature Dependencies

```
[Persistent Local Path History]
    └──requires──> [LocalPathHistory data structure]
                      └──new──> mirrors RecentDirs but for local paths (no serverKey)
                      └──reuse──> JSON persistence pattern from RecentDirs.loadFromDisk/saveToDisk
    └──requires──> [UI popup for local paths]
                      └──reuse──> RecentDirs overlay component (Draw/HandleKey/Show/Hide)
                      └──or──> new LocalPathHistory component using same pattern
    └──requires──> [Record calls in transfer success paths]
                      └──modify──> initiateTransfer: after successful upload, record local path
                      └──modify──> initiateTransfer: after successful download, record local path
                      └──modify──> initiateDirTransfer: same pattern for directory transfers
    └──requires──> [Key binding for local pane]
                      └──modify──> handleGlobalKeys: bind 'r' when activePane == 0
```

### Dependency Notes

1. **LocalPathHistory can reuse the RecentDirs component directly.** RecentDirs already has all the needed behavior (MRU, persistence, popup). The only difference is: (a) no `serverKey`, (b) different file path (`~/.lazyssh/local-path-history.json`), (c) no `currentPath` highlighting needed (local path is always known).

2. **Alternatively, refactor RecentDirs into a generic PathHistory component** with serverKey as optional. This is cleaner but more refactoring. For v1.3, creating a separate `LocalPathHistory` struct that mirrors `RecentDirs` is faster and lower risk.

3. **The `r` key currently only works on remote pane** (`activePane == 1`). Extending to local pane (`activePane == 0`) requires adding a branch in `handleGlobalKeys` in `file_browser_handlers.go`.

### Recommended Key Binding

| Key | Context | Action |
|-----|---------|--------|
| `r` | Local pane (activePane == 0) | Show local path history popup |
| `r` | Remote pane (activePane == 1) | Show remote directory history popup (existing) |

**Conflict analysis:** `r` is already consumed on remote pane. Adding it for local pane is a natural extension — the user sees the same behavior on both sides.

### Implementation Sketch

```go
// local_path_history.go — mirrors recent_dirs.go
type LocalPathHistory struct {
    *tview.Box
    paths         []string
    visible       bool
    selectedIndex int
    onSelect      func(path string)
    log           *zap.SugaredLogger
    filePath      string // ~/.lazyssh/local-path-history.json
}

func NewLocalPathHistory(log *zap.SugaredLogger) *LocalPathHistory { ... }
func (lph *LocalPathHistory) Record(path string) { ... } // identical to RecentDirs.Record
func (lph *LocalPathHistory) Draw(screen tcell.Screen) { ... } // identical to RecentDirs.Draw
func (lph *LocalPathHistory) HandleKey(event *tcell.EventKey) *tcell.EventKey { ... }
```

Recording in `initiateTransfer()`:
```go
// After successful upload
fb.localPathHistory.Record(fb.localPane.GetCurrentPath())
fb.recentDirs.Record(fb.remotePane.GetCurrentPath()) // existing

// After successful download
fb.localPathHistory.Record(fb.localPane.GetCurrentPath())
fb.recentDirs.Record(fb.remotePane.GetCurrentPath()) // existing
```

---

## Part 2: Feature 2 — Duplicate SSH Connection (Dup)

### Background: How SSH Managers Handle Entry Duplication

Duplication (cloning a server entry to create a new one with a different name) is a common pattern in configuration management tools. In the SSH config context, it's valuable when:

- Creating a new server that's similar to an existing one (same user, port, identity file, proxy settings)
- Testing a modified config without losing the original
- Creating multiple aliases for the same server with different options (e.g., different ports, different proxy jumps)

**Tools that support this:**
- **Termius** — Right-click > Duplicate on any server entry
- **MobaXterm** — Clone session feature
- **SSH config editing** — Manually copy-pasting Host blocks (the baseline behavior users fall back to)

In the terminal SSH manager space, this feature is less common because most tools (lazyssh included) are simple config editors. Adding a `d` key to duplicate is a significant quality-of-life improvement over manual config editing.

### Key Conflict: `d` Key is Already Used

**Critical finding:** In the current `handleGlobalKeys` in `handlers.go`, `d` is bound to `handleServerDelete()` (line 61). This means we **cannot** use `d` for duplicate on the server list.

The PROJECT.md specifies `d` key for duplication, but this conflicts with the existing delete binding. We need an alternative key.

**Available options:**
- `D` (Shift+D) — different from `d` (delete), easy to remember ("D for Duplicate")
- `y` — free on server list (currently unused in global keys; `y` in file browser would conflict with future yank)
- `w` — free, mnemonic "copy/clone"
- `C` (Shift+C) — mnemonic "Copy/Clone server config"

**Recommendation:** `D` (Shift+D). It's visually related to `d` (delete) but distinct, and the mnemonic "D for Duplicate" is natural. The status bar can show `[white]d[-] Delete  [white]D[-] Duplicate` to make both visible.

### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Duplicate server config** | Create a new entry with all fields copied from selected server | LOW | `ServerService.AddServer()` already exists; just copy the struct |
| **Prompt for new alias** | User must specify a unique alias for the duplicated entry | LOW | Reuse InputDialog pattern or use tview.Modal with InputField |
| **Unique alias validation** | Prevent creating duplicate aliases (SSH config uniqueness) | LOW | `validateServer()` already checks alias format; need alias uniqueness check |
| **List refresh after dup** | New entry should appear in server list immediately | LOW | Call `refreshServerList()` after successful dup |
| **Cursor on new entry** | Scroll to and select the newly created entry | LOW | `serverList.SetCurrentItem(index)` |

### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **One-key duplication** | Most users currently edit SSH config manually or use Add+fill-every-field. Dup reduces setup from 20+ fields to just an alias. | LOW | Single keypress + type alias + Enter |
| **Preserves all SSH config fields** | Dup copies everything: proxy settings, forwarding rules, authentication config, identity files, etc. Users only need to change what's different. | LOW | `domain.Server` struct is the full config; copy = deep copy |

### Anti-Features

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **Multi-field diff editor after dup** | Too complex for v1.3. After dup, user can `e` (edit) to change specific fields. | Dup creates entry, user edits separately if needed |
| **Bulk duplication** | Selecting multiple servers and duplicating all is an edge case with no clear use pattern | Single server duplication only |
| **Dup with modifications prompt** | "Which fields do you want to change?" adds significant UI complexity | Create exact copy, then `e` to edit |

### Feature Dependencies

```
[Dup SSH Connection]
    └──requires──> [Unique alias validation]
                      └──exists──> validateServer() checks format, need alias uniqueness
                      └──extend──> check against existing server aliases via ListServers
    └──requires──> [InputDialog for alias]
                      └──reuse──> file_browser.InputDialog or tview.Modal + InputField
    └──requires──> [ServerService.AddServer()]
                      └──exists──> already implemented in server_service.go
    └──requires──> [Deep copy of domain.Server]
                      └──trivial──> Go struct assignment is value copy; slices need manual copy
    └──requires──> [Key binding on server list]
                      └──modify──> handleGlobalKeys in handlers.go, add 'D' case
```

### Dependency Notes

1. **Deep copy concern:** `domain.Server` contains slices (`Aliases`, `IdentityFiles`, `Tags`, `LocalForward`, `RemoteForward`, `DynamicForward`). Simple `serverCopy := server` will share slice backing arrays. Need manual slice copies or `deepCopyServer()` helper.

2. **Alias uniqueness:** `validateServer()` checks format but not uniqueness. Need to check if alias already exists in `~/.ssh/config` before adding. This can be done by calling `ListServers("")` and checking for name collision.

3. **InputDialog availability:** The `file_browser.InputDialog` is a package-private component. For use in `handlers.go` (different package), we either: (a) extract it to a shared package, (b) create a similar modal directly in handlers.go using `tview.Modal` + `tview.InputField`, or (c) add the dup flow as a method on TUI. Option (b) is simplest and avoids cross-package dependencies.

### Implementation Sketch

```go
func (t *tui) handleServerDuplicate() {
    server, ok := t.serverList.GetSelectedServer()
    if !ok {
        t.showStatusTempColor("No server selected", "#FF6B6B")
        return
    }

    // Create modal with input field for new alias
    input := tview.NewInputField().SetLabel("New alias: ").SetFieldWidth(30)
    modal := tview.NewForm().
        AddFormItem(input).
        AddButton("Create", func() {
            newAlias := strings.TrimSpace(input.GetText())
            if newAlias == "" {
                t.showStatusTempColor("Alias cannot be empty", "#FF6B6B")
                return
            }
            if newAlias == server.Alias {
                t.showStatusTempColor("Alias must differ from original", "#FF6B6B")
                return
            }
            // Check uniqueness
            servers, _ := t.serverService.ListServers("")
            for _, s := range servers {
                if s.Alias == newAlias {
                    t.showStatusTempColor("Alias already exists", "#FF6B6B")
                    return
                }
            }
            // Deep copy and set new alias
            dup := deepCopyServer(server)
            dup.Alias = newAlias
            dup.PinnedAt = time.Time{}  // clear pinned state
            dup.SSHCount = 0            // reset connection count
            if err := t.serverService.AddServer(dup); err != nil {
                t.showStatusTempColor("Dup failed: "+err.Error(), "#FF6B6B")
                return
            }
            t.refreshServerList()
            // Scroll to new entry
            // ...
            t.returnToMain()
            t.showStatusTemp("Duplicated: " + newAlias)
        }).
        AddButton("Cancel", func() { t.returnToMain() })
    t.app.SetRoot(modal, true)
    t.app.SetFocus(input)
}
```

---

## Part 3: Feature 3 — Dual-Remote File Transfer (Local Relay)

### Background: How Dual-Remote Transfer Works

Dual-remote transfer means transferring files between two different remote servers, using the local machine as a relay. This is fundamentally different from the current local<->remote transfer model.

**Why local relay?** SFTP protocol has no native server-to-server transfer capability. The two practical approaches are:

| Approach | Mechanism | Progress Tracking | Complexity |
|----------|-----------|-------------------|------------|
| **`scp -3`** | `scp -3 user1@host1:/file user2@host2:/dest` — data flows host1→local→host2 | **No built-in progress bar** in relay mode | LOW (single command) |
| **Download + Re-upload** | Download from server A to temp, upload temp to server B | Full progress tracking per phase | MEDIUM (two transfers) |
| **SSH agent forwarding** | `ssh -A -t user1@host1 scp file user2@host2:/dest` — data flows host1→host2 directly | Limited (only remote-side progress) | HIGH (requires agent setup on both servers) |
| **MC VFS approach** | Opens two SFTP sessions and orchestrates copy between them | Depends on implementation | HIGH (in-app SSH library needed) |

**Critical technical finding: `scp -3` has no progress bar.**
When using `scp -3` in relay mode, the standard `-v` (verbose) flag does not produce byte-level progress. The `--progress-bar` flag also does not work in relay mode because the data flows through stdin/stdout of the local scp process, not through a direct file descriptor. This is a known limitation of OpenSSH's scp implementation. (Confidence: MEDIUM — based on manual testing experience and community reports; could not find authoritative documentation confirming this explicitly, but multiple StackOverflow threads discuss the lack of progress in `scp -3` mode.)

**Recommendation:** Use the **download + re-upload** approach (two-phase transfer). This is exactly what `CopyRemoteFile`/`CopyRemoteDir` already do for same-server remote-to-remote copies. For cross-server, the pattern is identical — the only difference is using two different SFTPService instances.

### How Midnight Commander Handles This

MC supports dual-remote via its VFS layer:
1. Left panel connects to server A via `sftp://` or `sh://` (Shell link/FISH)
2. Right panel connects to server B via `sftp://` or `sh://`
3. User selects files and presses F5 (Copy) — MC orchestrates the transfer internally
4. MC uses its VFS abstraction to read from one SFTP session and write to another, buffering in memory or temp files

**MC's approach requires an in-app SSH library** (libssh2 or similar). lazyssh deliberately avoids this per its security constraints ("reuse system scp/sftp commands"). So lazyssh must use the two-phase relay approach.

### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Select two different servers** | User picks source server and destination server from the server list | MEDIUM | New UI flow: server list selection mode |
| **Dual-remote file browser** | Left panel = server A, Right panel = server B (both remote) | HIGH | Requires two SFTP connections simultaneously |
| **File transfer between servers** | Download from A to local temp, upload from temp to B | HIGH | Reuses existing download+upload pattern from CopyRemoteFile/CopyRemoteDir |
| **Staged progress display** | Phase 1: "Downloading from server A...", Phase 2: "Uploading to server B..." | MEDIUM | TransferModal already supports phase labels (see remote paste dir) |
| **Cancel support** | User can cancel at any point during either phase | LOW | context.Context cancellation already propagates through TransferService |
| **Temp file cleanup** | Temp files are deleted after upload completes (or on cancel/error) | LOW | `defer os.RemoveAll(tmpDir)` pattern already used |
| **Directory transfer** | Recursive directory transfer between servers | HIGH | DownloadDir + UploadDir, same pattern as CopyRemoteDir |
| **Conflict handling** | Skip/overwrite/rename on the destination server | MEDIUM | Reuse buildConflictHandler pattern |

### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Keyboard-driven server selection** | Most tools require mouse clicks or complex multi-step flows. lazyssh selects servers from the familiar server list. | MEDIUM | `D` key on server list enters dual-select mode; j/k to pick source, Enter, j/k to pick dest, Enter |
| **Staged progress with speed/ETA** | Unlike `scp -3` which has no progress, lazyssh shows full progress for both phases | MEDIUM | Already supported by TransferModal; just need phase labels |
| **No additional security risk** | Still uses system scp/sftp, no in-app SSH library | LOW | Core constraint maintained |

### Anti-Features

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **Same-server dual-remote transfer** | Copying files within the same server already works (CopyRemoteFile/CopyRemoteDir in v1.2). Dual-remote is specifically for **different** servers. | Use existing remote copy/move (c/x + p on remote pane) for same-server operations |
| **Parallel download+upload (streaming)** | Would require piping download output directly to upload input without temp storage. This is fragile (what if upload fails mid-stream? partial data on dest). The two-phase approach with temp storage is more robust. | Download fully to temp, then upload. Temp is cleaned up afterward. |
| **`scp -3` command approach** | No progress bar, no per-file tracking for directories, hard to integrate with existing TransferModal. The two-phase approach gives full control. | Download + re-upload via existing TransferService |
| **Direct server-to-server via SSH hop** | Requires SSH agent forwarding to be configured on both servers, plus password-less auth between them. Too many prerequisites. | Local relay (works with existing SSH config) |
| **Resume/checkpoint for interrupted transfers** | Significant complexity; would need to track which files succeeded in each phase | Re-transfer from scratch on retry (consistent with existing v1.x behavior) |

### Feature Dependencies

```
[Dual-Remote File Transfer]
    └──requires──> [Two simultaneous SFTP connections]
                      └──new──> second SFTPService instance for destination server
                      └──architectural concern──> current TUI has single sftpService
    └──requires──> [Server selection UI flow]
                      └──new──> dual-select mode on server list (pick source + dest)
    └──requires──> [DualRemoteFileBrowser component]
                      └──new──> variant of FileBrowser with both panes remote
                      └──or──> extend FileBrowser to accept two SFTPService instances
    └──requires──> [DownloadDir + UploadDir orchestration]
                      └──exists──> TransferService.CopyRemoteDir pattern
                      └──adapt──> use different SFTPService for upload vs download
    └──requires──> [TransferModal phase labels]
                      └──exists──> already supports fileLabel override (see handleRemotePaste)
    └──requires──> [Temp directory management]
                      └──exists──> os.MkdirTemp + defer os.RemoveAll pattern
```

### Dependency Notes

1. **Two SFTP connections is the biggest architectural challenge.** Currently, `tui` has a single `sftpService` field and `FileBrowser` receives one `SFTPService`. For dual-remote, we need two independent SFTP connections. This means:
   - Either create a new `SFTPService` instance at the adapter level for the second server
   - Or modify `FileBrowser` to optionally accept two SFTP services
   - The `transferService` currently holds a reference to one `SFTPService` — for dual-remote, it needs to coordinate between two

2. **Server selection UI needs careful design.** The user needs to:
   - Pick source server from server list
   - Pick destination server from server list
   - Both must be different servers
   - The UI should make the two-step selection obvious

   **Proposed flow:** On the server list, press `D` (if not used for dup) or a new key (e.g., `T` for Transfer) to enter dual-select mode:
   - Status bar: "Select SOURCE server (Enter to confirm)"
   - User navigates with j/k, presses Enter
   - Status bar: "Select DESTINATION server (Enter to confirm)"
   - User navigates with j/k, presses Enter
   - File browser opens with source on left, destination on right

   **Key conflict with dup:** Both dup and dual-remote need a key on the server list. See key binding analysis below.

3. **TransferService extension.** The current `CopyRemoteFile`/`CopyRemoteDir` methods use `ts.sftp` for both download and upload. For dual-remote, we need to download from one SFTPService and upload to a different one. Options:
   - Add new methods `RelayFile(ctx, srcSFTP, dstSFTP, ...)` and `RelayDir(ctx, srcSFTP, dstSFTP, ...)`
   - Or create a new `DualRemoteTransferService` that takes two SFTP instances
   - Recommendation: Add methods to existing TransferService that accept explicit SFTP service parameters

### Staged Progress Display Design

The transfer should show clear phase separation:

```
Phase 1 — Download from server A:
┌──────────────────────────────────────────┐
│ Downloading from server-a (user@a.com)   │
│                                          │
│ file.txt                                 │
│ [████████████░░░░░░░░░░░] 45%  2.3 MB/s  │
│ ETA: 0:12                                 │
└──────────────────────────────────────────┘

Phase 2 — Upload to server B:
┌──────────────────────────────────────────┐
│ Uploading to server-b (user@b.com)       │
│                                          │
│ file.txt                                 │
│ [████████████████░░░░░░] 75%  1.8 MB/s  │
│ ETA: 0:05                                 │
└──────────────────────────────────────────┘

Summary:
┌──────────────────────────────────────────┐
│ Transfer complete                         │
│ server-a → server-b                      │
│ 3 files transferred, 0 failed            │
│ Total: 15.2 MB in 8.3s (1.83 MB/s)      │
│                                          │
│ Press any key to close                   │
└──────────────────────────────────────────┘
```

### Key Binding Analysis for Server List

Currently used keys on server list (from `handlers.go`):
- `q` — quit
- `/` — search
- `a` — add server
- `e` — edit server
- `d` — **delete server** (CONFLICT with dup if `d` is used)
- `p` — pin server
- `s`/`S` — sort
- `c` — copy SSH command
- `g` — ping
- `r` — refresh
- `t` — tags
- `f` — port forward
- `F` — file browser
- `x` — stop forwarding
- `j`/`k` — navigate
- `Enter` — connect

**Available keys for dup and dual-remote:**
- `D` (Shift+D) — Dup server entry
- `T` — Transfer (dual-remote)
- `y` — free
- `w` — free
- `b` — free

**Recommendation:**
- `D` for dup (mnemonic: Duplicate)
- `T` for dual-remote transfer (mnemonic: Transfer)

---

## Combined Feature Dependencies

```
[Feature 1: Persistent Local Path History]
    └──no dependency on Feature 2 or 3
    └──extends──> existing RecentDirs pattern
    └──modifies──> initiateTransfer, initiateDirTransfer success paths

[Feature 2: Dup SSH Connection]
    └──no dependency on Feature 1 or 3
    └──modifies──> handleGlobalKeys in handlers.go
    └──uses──> ServerService.AddServer (existing)
    └──uses──> validateServer (existing, needs uniqueness check)

[Feature 3: Dual-Remote Transfer]
    └──no dependency on Feature 1 or 2
    └──requires──> two SFTPService instances (architectural)
    └──requires──> server selection UI (new)
    └──extends──> TransferService with relay methods
    └──extends──> FileBrowser or new DualRemoteFileBrowser

All three features are INDEPENDENT — they can be built in any order.
```

---

## MVP Definition (v1.3)

### Launch With (P1)

- [ ] **Local path history persistence** — Record upload/download local paths, JSON at `~/.lazyssh/local-path-history.json`, MRU popup with `r` on local pane
- [ ] **Dup SSH connection** — `D` key on server list, InputField for alias, deep copy of server config, uniqueness validation
- [ ] **Dual-remote file browser** — Select two servers from list, dual-remote pane layout, file/directory listing on both
- [ ] **Dual-remote file transfer** — Download from server A to temp, upload to server B, staged progress display

### Add After Validation (v1.x)

- [ ] **Dual-remote file management** — delete/rename/mkdir between two remote servers
- [ ] **Dual-remote copy/move** — clipboard operations across two remote servers
- [ ] **Path history for dual-remote** — record remote paths used in dual-remote context
- [ ] **Dual-remote conflict handling** — overwrite/skip/rename on destination server

### Future Consideration (v2+)

- [ ] **Same-server dual-directory view** — two panes for the same server showing different directories (out of scope per PROJECT.md)
- [ ] **Persistent bookmarks** — named bookmarks beyond MRU
- [ ] **Dup with field diff** — show which fields differ between original and dup, allow selective modification before saving
- [ ] **Streaming relay** — pipe download directly to upload without temp storage (for very large files)

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority | Phase Suggestion |
|---------|------------|---------------------|----------|------------------|
| Local path history persistence | MEDIUM | LOW | P1 | Phase 1 (simplest, extends existing pattern) |
| Dup SSH connection | MEDIUM | LOW | P1 | Phase 2 (simple, self-contained) |
| Dual-remote file browser UI | HIGH | HIGH | P1 | Phase 3 (architectural complexity) |
| Dual-remote file transfer | HIGH | HIGH | P1 | Phase 4 (depends on Phase 3) |
| Dual-remote file management | MEDIUM | HIGH | P2 | v1.x |
| Dual-remote copy/move | MEDIUM | HIGH | P2 | v1.x |

**Phase ordering rationale:**
1. **Local path history first** — lowest risk, extends existing RecentDirs pattern, no architectural changes
2. **Dup second** — low risk, self-contained in server list, no dependency on file browser
3. **Dual-remote browser third** — highest architectural impact (two SFTP connections), needs careful design
4. **Dual-remote transfer fourth** — depends on Phase 3's browser, but transfers reuse existing TransferService patterns

---

## Competitor Feature Analysis

| Feature | Midnight Commander | Termius | lf | lazyssh (planned) |
|---------|-------------------|---------|----|-------------------|
| **Local path history** | Yes (per-panel) | N/A (GUI) | Yes (persistent) | Yes (global MRU, `r` key) |
| **Remote path history** | Yes (per-panel) | Yes (per-host) | No | Yes (per-server MRU, `r` key) |
| **Dup server entry** | No (edit config) | Yes (right-click) | N/A | Yes (`D` key) |
| **Dual-remote panels** | Yes (VFS SFTP link) | Yes (dual SFTP) | No (local only) | Yes (dual SFTP) |
| **Dual-remote transfer** | Yes (F5 between remotes) | Yes (drag-drop) | N/A | Yes (local relay, staged progress) |
| **Transfer progress** | Basic | Detailed | N/A | Detailed (bar, speed, ETA, phases) |
| **Cancel mid-transfer** | Yes (Ctrl+C) | Yes | N/A | Yes (context cancellation) |

**Key insight:** lazyssh's dual-remote transfer via local relay is less efficient than MC's in-app VFS approach or Termius's direct transfer. However, it maintains the "zero new dependencies" constraint and provides better progress feedback than `scp -3`. The tradeoff is speed (data passes through local machine) vs. simplicity and security.

---

## Architectural Concerns for Dual-Remote

### Two SFTP Connections

Currently, the TUI and FileBrowser each hold a single SFTPService reference. For dual-remote:

```
Current:
  TUI.sftpService ──> single SFTP connection
  FileBrowser.sftpService ──> same connection
  TransferService.sftp ──> same connection

Dual-remote needed:
  DualRemoteFileBrowser.srcSFTP ──> SFTP connection to server A
  DualRemoteFileBrowser.dstSFTP ──> SFTP connection to server B
  TransferService.RelayFile(srcSFTP, dstSFTP, ...) ──> coordinates between both
```

**Options:**
1. **New DualRemoteFileBrowser component** — separate from FileBrowser, takes two SFTPService instances. Cleaner separation but more code.
2. **Extend FileBrowser** — add optional `srcSFTP` and `dstSFTP` fields, change pane behavior based on mode. More code reuse but more complex.
3. **TransferService methods with explicit SFTP parameters** — `RelayFile(ctx, srcSFTP, dstSFTP, ...)` bypasses the struct's single `sftp` field.

**Recommendation:** Option 1 (new component). The dual-remote browser has fundamentally different semantics (both panes are remote, no local pane). A separate component avoids adding mode-switching complexity to the existing FileBrowser.

### SFTPService Factory

Currently, SFTPService is created once in `cmd/main.go` and injected everywhere. For dual-remote, we need to create SFTPService instances on-demand:

```go
// New factory function needed
func NewSFTPService(log *zap.SugaredLogger) ports.SFTPService {
    return sftp_client.New(log)
}
```

The TUI would create two SFTPService instances when entering dual-remote mode, and close both when exiting.

### Disk Space Consideration

The download-to-temp approach uses local disk space equal to the size of the transferred data. For large transfers, this could be significant. The status bar or transfer modal should show a warning for large directories.

---

## Sources

- [Midnight Commander Remote Connect via Shell Link (4sysops)](https://4sysops.com/archives/midnight-commander-remote-connect-via-shell-link-copy-files-over-ssh-and-sftp-link-using-fish-and-public-key-authentication/)
- [SCP Between Two Remote Hosts From Third PC (SuperUser)](https://superuser.com/questions/686394/scp-between-two-remote-hosts-from-my-third-pc)
- [Transfer Files Between Two Remote SSH Servers (AskUbuntu)](https://askubuntu.com/questions/1116153/transfer-files-between-two-remote-ssh-servers)
- [lf Documentation — History File](https://github.com/gokcehan/lf/blob/master/doc.md)
- [lf History File Overwrite Bug (GitHub Issue #1450)](https://github.com/gokcehan/lf/issues/1450)
- [Ranger Persistent History Request (GitHub Issue #1741)](https://github.com/ranger/ranger/issues/1741)
- [Midnight Commander History Concurrency (GitHub Issue #4818)](https://github.com/MidnightCommander/mc/issues/4818)
- [Yazi Bookmarks Plugin (yamb.yazi)](https://github.com/h-hg/yamb.yazi)
- [Yazi Resources Page](https://yazi-rs.github.io/docs/resources/)
- [Termius SFTP Documentation](https://termius.com/documentation/connect-with-sftp)
- [Copy Files Between Two Remote SFTP Servers (SuperUser)](https://superuser.com/questions/204899/copy-files-between-two-remote-servers-with-sftp)
- [Red Hat — Secure File Transfer SCP/SFTP](https://www.redhat.com/en/blog/secure-file-transfer-scp-sftp)
- Existing codebase: `internal/adapters/ui/file_browser/recent_dirs.go` (RecentDirs pattern)
- Existing codebase: `internal/adapters/data/transfer/transfer_service.go` (CopyRemoteFile/CopyRemoteDir pattern)
- Existing codebase: `internal/adapters/ui/handlers.go` (server list key bindings)
- Existing codebase: `internal/core/services/server_service.go` (AddServer, validateServer)
- Existing codebase: `internal/core/domain/server.go` (Server struct with slices)

---
*Features research: 2026-04-15 — v1.3 Enhanced File Browser focus*
