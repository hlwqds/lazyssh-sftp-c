# Architecture Research: v1.3 Enhanced File Browser

**Domain:** TUI Enhanced File Browser -- Local Path Persistence, Dup SSH, Dual-Remote Transfer
**Researched:** 2026-04-15
**Confidence:** HIGH (based on direct code analysis of all relevant files in internal/core/ports, internal/core/services, internal/adapters/data, internal/adapters/ui/file_browser, and cmd/main.go)

## Executive Summary

v1.3 adds three features that touch every architectural layer. (1) Local path history persistence extends the existing `RecentDirs` pattern to the local pane, requiring a new port interface and adapter for JSON persistence in `~/.lazyssh/`. (2) Dup SSH is purely a server-list-level feature that reuses existing `ServerService.AddServer()` -- it requires zero port/adapter changes, only a new handler in the TUI layer. (3) Dual-remote file transfer is the most architecturally significant feature: it requires a new `RelayTransferService` that orchestrates two SFTP connections, a new `DualRemoteFileBrowser` UI component (or mode-switch on existing FileBrowser), and a modified transfer modal for 3-phase staged progress (download from A, upload to B, cleanup).

The critical architectural insight is that features (1) and (2) are simple extensions of existing patterns, while feature (3) introduces a fundamentally new component -- a second SFTP connection managed concurrently. The existing `TransferService` is hardcoded to a single SFTP connection (`ts.sftp`). Dual-remote requires either a new service that accepts two SFTP connections or a factory-based approach to create TransferService instances per-connection.

## Feature 1: Local Path History Persistence

### Problem

Currently, `RecentDirs` (v1.1) persists remote directory paths to `~/.lazyssh/recent-dirs/{user@host}.json`. The local pane has no path history -- users must manually navigate to upload/download directories each time.

### Architecture: Port + Adapter + UI Hook

#### Port Interface: PathHistoryService

```go
// internal/core/ports/path_history.go

// PathHistoryService manages MRU path history for local file browser panes.
type PathHistoryService interface {
    // Record adds a path to the MRU list. Deduplicates and truncates to maxEntries.
    Record(path string)
    // GetPaths returns the MRU list (most recent first).
    GetPaths() []string
    // Clear removes all persisted paths.
    Clear()
}
```

**Why a separate port instead of extending RecentDirs:**
- `RecentDirs` is a UI component (embeds `*tview.Box`, has `Draw()`, `HandleKey()`). Ports should be pure interfaces.
- `RecentDirs` is per-server (keyed by `user@host`). Local path history is global (not per-remote-server).
- Separation of concerns: persistence logic belongs in adapters, UI rendering belongs in the UI layer.

#### Adapter: LocalPathHistory

```go
// internal/adapters/data/local_path_history/local_path_history.go

type localPathHistory struct {
    filePath string // ~/.lazyssh/local-path-history.json
    paths    []string
    maxEntries int  // 20 (more than remote's 10 since local paths vary more)
    mu       sync.RWMutex
    log      *zap.SugaredLogger
}
```

**Storage format:** Same as `RecentDirs` -- `[]string` JSON at `~/.lazyssh/local-path-history.json`. Maximum 20 entries (vs 10 for remote dirs, since local paths span more contexts).

**Why reuse the JSON `[]string` format:**
- Already proven by RecentDirs
- Simple, human-readable, no schema migration needed
- Error-tolerant (RecentDirs logs errors but never fails the caller)

#### UI Integration: Extend LocalPane

The local pane needs the same `r` key behavior as the remote pane, but with a separate `LocalRecentDirs` overlay instance.

```
LocalRecentDirs (new struct, mirrors RecentDirs pattern)
  ├── *tview.Box
  ├── paths []string
  ├── visible bool
  ├── selectedIndex int
  ├── onSelect func(path string)
  └── currentPath string
```

**Key routing change in `handleGlobalKeys`:**
```
case 'r':
    if fb.activePane == 0 {
        // Local pane: show LocalRecentDirs
        fb.localRecentDirs.SetCurrentPath(fb.localPane.GetCurrentPath())
        fb.localRecentDirs.Show()
    } else if fb.activePane == 1 && fb.remotePane.IsConnected() {
        // Remote pane: show RecentDirs (existing)
        fb.recentDirs.SetCurrentPath(fb.remotePane.GetCurrentPath())
        fb.recentDirs.Show()
    }
```

**Recording hook:** After successful upload/download, record the local path:
```go
// In initiateTransfer() after success:
fb.localRecentDirs.Record(fb.localPane.GetCurrentPath())
```

**Why NOT unify LocalRecentDirs and RecentDirs into one component:**
- Different data sources (PathHistoryService vs RecentDirs' internal persistence)
- Different MRU limits (20 vs 10)
- Different title text ("Recent Local Directories" vs "Recent Directories")
- The cost of a separate ~100-line struct is negligible vs the coupling cost of unification

### Modified vs New Files

| File | Change | Lines (est.) |
|------|--------|-------------|
| `internal/core/ports/path_history.go` | New port interface | ~15 |
| `internal/adapters/data/local_path_history/local_path_history.go` | New adapter | ~100 |
| `internal/adapters/ui/file_browser/local_recent_dirs.go` | New overlay component | ~150 |
| `internal/adapters/ui/file_browser/file_browser.go` | Add localRecentDirs field, wire in build() | +15 |
| `internal/adapters/ui/file_browser/file_browser_handlers.go` | Extend 'r' key to local pane | +5 |
| `internal/adapters/ui/file_browser/file_browser.go` | Record local path after transfer | +3 |
| `internal/adapters/ui/file_browser/file_browser.go` | Draw chain: add localRecentDirs overlay | +3 |
| `cmd/main.go` | Create LocalPathHistory, pass to TUI/NewFileBrowser | +3 |

### Dependency: None (independent feature)

---

## Feature 2: Dup SSH Connection

### Problem

Users want to create a new server entry based on an existing one (e.g., same host, different port, or same config with minor tweaks). Currently requires manually entering all fields via `a` (add).

### Architecture: Pure TUI Handler, Zero Port Changes

The entire feature lives in the TUI layer. It reuses existing `ServerService.AddServer()` which already handles alias conflict detection, SSH config writing, and metadata creation.

#### Key Binding: `D` (Shift+d) in Server List

The `d` key is already taken (server delete). Use `D` (uppercase) for dup, following the same pattern as `s`/`S` (sort/sort-reverse).

```
case 'D':
    t.handleServerDup()
    return nil
```

#### Handler Flow

```
handleServerDup():
  1. Get selected server from serverList
  2. Copy server entity: dupServer = server
  3. Generate unique alias: dupServer.Alias = server.Alias + "-copy"
     - If "server-copy" exists, try "server-copy-2", "server-copy-3", etc.
  4. Clear non-copyable fields:
     - Tags: [] (new server starts without tags)
     - LastSeen, PinnedAt: zero values (fresh metadata)
     - SSHCount: 0
  5. Open ServerForm in ADD mode with dupServer pre-filled
  6. User edits and saves via existing handleServerSave()
```

**Why pre-fill into ServerForm (add mode) instead of directly adding:**
- User must see and confirm the new alias before it's written
- User may want to change other fields (port, user, identity file)
- Reuses existing validation (alias format, host format, duplicate check)
- Reuses existing save path (AddServer -> SSH config write + metadata)

**Alias generation strategy:**
```
base = server.Alias + "-copy"
if exists(base) -> base = server.Alias + "-copy-2"
if exists(base) -> base = server.Alias + "-copy-3"
...
up to 100 attempts, then fall back to server.Alias + "-copy-" + timestamp
```

**Implementation location:** `handleServerDup()` in `internal/adapters/ui/handlers.go`.

### Modified vs New Files

| File | Change | Lines (est.) |
|------|--------|-------------|
| `internal/adapters/ui/handlers.go` | Add handleServerDup(), 'D' key binding | +50 |

### Dependency: None (independent feature, simplest of the three)

---

## Feature 3: Dual-Remote File Transfer

### Problem

Transfer files between two different remote servers. The local machine acts as a relay because direct server-to-server SFTP is not possible (SFTP protocol requires a client connection, and lazyssh uses system ssh binary, not a Go SSH library that could multiplex).

### Architecture: New Component, New Service, Modified Transfer Modal

This is the most architecturally complex feature. Three sub-problems:

1. **Server selection UI**: How does the user pick two servers?
2. **Connection management**: How to manage two concurrent SFTP connections?
3. **Relay transfer logic**: How to orchestrate download-from-A + upload-to-B?
4. **Staged progress display**: How to show 3-phase progress?

#### Sub-problem 1: Server Selection UI

**Option A: New `DualRemoteFileBrowser` component (Recommended)**

Create a new top-level component that wraps two RemotePanes and a TransferModal. Entry from the server list via a new key (e.g., `M` for "Move between remotes").

```
DualRemoteFileBrowser (new root, *tview.Flex)
  ├── leftRemotePane  (*RemotePane, connected to Server A)
  ├── rightRemotePane (*RemotePane, connected to Server B)
  ├── statusBar       (*tview.TextView)
  ├── transferModal   (*TransferModal, overlay)
  └── confirmDialog   (*ConfirmDialog, overlay)
```

**Why a new component instead of mode-switching on FileBrowser:**
- FileBrowser has a hard dependency on one SFTP connection (fb.sftpService)
- Dual-remote needs two independent SFTP connections with independent connection lifecycle
- The pane type is different (both RemotePane, not LocalPane + RemotePane)
- Mode-switching would require conditional logic throughout FileBrowser (if dualRemote then... else...)
- A separate component keeps each mode's complexity isolated

**Server selection flow:**
```
User presses 'M' on server list
  -> If no server selected: show error
  -> Selected server = Server A (left pane)
  -> Show server picker overlay (reuse ServerList as a selection popup)
  -> User picks Server B (right pane)
  -> Create DualRemoteFileBrowser(leftServer, rightServer)
  -> app.SetRoot(dualBrowser, true)
```

**Server picker overlay:**
```
ServerPickerOverlay (new struct, follows overlay pattern)
  ├── *tview.Box
  ├── servers []domain.Server
  ├── visible bool
  ├── selectedIndex int
  ├── onSelect func(domain.Server)
  └── filter string
```

**Why not use tview.Form with DropDown:**
- Server list can be long (50+ servers)
- Need filtering/search capability
- Need keyboard navigation (j/k)
- DropDown in tview doesn't support filtering

#### Sub-problem 2: Connection Management

**The core problem:** `TransferService` holds a single `ports.SFTPService` reference. For dual-remote, we need two independent SFTP connections.

**Option A: Create two SFTPClient instances (Recommended)**

```go
// In DualRemoteFileBrowser
sftpA := sftp_client.New(log)  // connects to Server A
sftpB := sftp_client.New(log)  // connects to Server B

sftpA.Connect(serverA)
sftpB.Connect(serverB)
```

**Option B: SFTPClient factory**
Create a factory that produces SFTPClient instances. Over-engineering for this use case -- we only ever need 2.

**Option C: Modify TransferService to accept two SFTP connections**
Violates single responsibility. TransferService should remain focused on local<->remote transfers.

**Recommended approach: New `RelayTransferService`**

```go
// internal/core/ports/relay_transfer.go

// RelayTransferService transfers files between two remote servers via local relay.
// The local machine downloads from sourceServer, then uploads to targetServer.
type RelayTransferService interface {
    // RelayFile downloads a file from source, uploads to target.
    RelayFile(ctx context.Context, srcPath string, dstPath string,
        onProgress func(domain.TransferProgress),
        onConflict domain.ConflictHandler) error

    // RelayDir downloads a directory from source, uploads to target.
    // Returns list of failed file paths.
    RelayDir(ctx context.Context, srcPath string, dstPath string,
        onProgress func(domain.TransferProgress),
        onConflict domain.ConflictHandler) ([]string, error)
}
```

```go
// internal/adapters/data/transfer/relay_transfer_service.go

type relayTransferService struct {
    log        *zap.SugaredLogger
    srcSFTP    ports.SFTPService  // download source
    dstSFTP    ports.SFTPService  // upload target
    srcLabel   string             // "user@host" for progress display
    dstLabel   string             // "user@host" for progress display
}
```

**Why a separate service instead of reusing TransferService:**
- TransferService is hardcoded to one SFTP + local filesystem. Relay needs two SFTP connections.
- Relay has different progress semantics (3 phases: download, upload, cleanup)
- Relay has no local filesystem involvement (temp files only, managed internally)
- Interface Segregation: callers of TransferService don't need relay methods

**Implementation strategy for RelayFile:**

```
Phase 1: Download from sourceSFTP to temp file
  - srcSFTP.OpenRemoteFile(srcPath)
  - os.CreateTemp("", "lazyssh-relay-*")
  - copyWithProgress (32KB buffer, same pattern as transfer_service.go)
  - Progress label: "Downloading from {srcLabel}: filename"

Phase 2: Upload temp file to targetSFTP
  - dstSFTP.CreateRemoteFile(dstPath)
  - copyWithProgress
  - Progress label: "Uploading to {dstLabel}: filename"

Phase 3: Cleanup
  - os.Remove(tempPath)
  - Report summary
```

**Implementation strategy for RelayDir:**
```
Phase 1: Download entire directory from sourceSFTP to temp dir
  - srcSFTP.WalkDir(srcPath) to get file list
  - For each file: download to temp/{relativePath}
  - Progress: "Phase 1/2: Downloading from {srcLabel} (3/20 files)"

Phase 2: Upload temp directory to targetSFTP
  - filepath.WalkDir(tempDir) to enumerate downloaded files
  - For each file: upload to dstPath/{relativePath}
  - Progress: "Phase 2/2: Uploading to {dstLabel} (3/20 files)"

Phase 3: Cleanup
  - os.RemoveAll(tempDir)
  - Report summary
```

**Why not stream directly from source to target (pipe without temp file):**
- SFTP OpenRemoteFile returns `io.ReadCloser`, SFTP CreateRemoteFile returns `io.WriteCloser`
- Technically we could do `io.Copy(sftpB.Create(), sftpA.Open())`
- BUT: This provides no progress tracking (no total size for download phase)
- AND: If upload fails mid-way, we can't retry without re-downloading
- AND: The temp file approach matches the proven CopyRemoteFile pattern already in TransferService
- The disk space trade-off is acceptable: temp files are deleted immediately after upload

#### Sub-problem 3: Staged Progress Display

**Extend TransferModal with a new mode: `modeRelay`**

```
TransferModal modes (extended):
  modeProgress       -- single file local<->remote
  modeCancelConfirm  -- cancel confirmation
  modeConflictDialog -- conflict resolution
  modeSummary        -- transfer complete
  modeCopy           -- remote copy (download+re-upload)
  modeMove           -- remote move (download+re-upload+delete)
  modeRelay          -- DUAL-REMOTE: 3-phase staged progress  ← NEW
```

**Relay progress layout:**
```
┌─────────────────────────────────────────────┐
│         Relay Transfer: file.txt            │
│                                             │
│  Phase 1/2: Downloading from user@host-a    │
│  ████████████████████░░░░░░  67%  2.3 MB/s  │
│  ETA: 0m 12s                                │
│                                             │
│  Source: user@host-a:/path/to/file.txt       │
│  Target: user@host-b:/path/to/file.txt       │
│                                             │
│  [Esc] Cancel                               │
└─────────────────────────────────────────────┘
```

**Phase transition:**
```
Phase 1 progress bar fills to 100%
  -> Progress bar resets
  -> Phase label changes: "Phase 2/2: Uploading to user@host-b"
  -> Speed samples reset (new connection, different throughput)
```

**Key decision: Reset progress bar between phases or show overall?**

| Approach | Pros | Cons |
|----------|------|------|
| **Reset per phase** (Recommended) | Clear feedback for each phase; speed is accurate per connection; matches CopyRemoteFile UI pattern | User can't see overall progress |
| Overall (phase1_bytes + phase2_bytes / total) | Shows overall completion | Misleading speed (averages two different connections); total bytes only known after download completes |

**Recommendation:** Reset per phase. Show "Phase 1/2" and "Phase 2/2" labels. This matches how `CopyRemoteFile` already works (download progress then upload progress).

#### Sub-problem 4: DualRemoteFileBrowser Component Design

```
DualRemoteFileBrowser (new root, *tview.Flex)
  ├── *tview.Flex (root layout)
  ├── app *tview.Application
  ├── log *zap.SugaredLogger
  ├── sftpA, sftpB *sftp_client.SFTPClient
  ├── leftPane *RemotePane  (connected to sftpA)
  ├── rightPane *RemotePane (connected to sftpB)
  ├── statusBar *tview.TextView
  ├── transferModal *TransferModal
  ├── confirmDialog *ConfirmDialog
  ├── serverA, serverB domain.Server
  ├── activePane int  // 0=left, 1=right
  ├── transferring bool
  ├── transferCancel context.CancelFunc
  └── onClose func()
```

**Key differences from FileBrowser:**
- No LocalPane (both panes are RemotePane)
- Two independent SFTP connections (not one)
- Uses RelayTransferService instead of TransferService
- No RecentDirs (remote dirs differ per server, would need per-server MRU -- deferred to future)
- No clipboard copy/move (cross-server copy is the relay transfer itself)

**Constructor:**
```go
func NewDualRemoteFileBrowser(
    app *tview.Application,
    log *zap.SugaredLogger,
    serverA, serverB domain.Server,
    relaySvc ports.RelayTransferService,
    onClose func(),
) *DualRemoteFileBrowser
```

**Key routing (simplified):**
```
DualRemoteFileBrowser.handleGlobalKeys(event):
  1. Overlay interception (transferModal, confirmDialog)
  2. Tab -> switchFocus()
  3. Esc -> close (cleanup both SFTP connections)
  4. F5 -> initiateRelayDirTransfer()
  5. Enter on file -> initiateRelayFileTransfer()
  6. Pass to focused pane
```

**Relay transfer initiation:**
```go
func (d *DualRemoteFileBrowser) initiateRelayTransfer() {
    if d.activePane == 0 {
        // Left -> Right: download from sftpA, upload to sftpB
        srcPane, dstPane = d.leftPane, d.rightPane
    } else {
        // Right -> Left: download from sftpB, upload to sftpA
        srcPane, dstPane = d.rightPane, d.leftPane
    }

    // Get selected file, determine src/dst paths
    // Show transferModal in modeRelay
    // Start goroutine with relaySvc.RelayFile()
}
```

**Why Enter triggers relay transfer (not F5):**
- F5 is for directory transfer (consistent with FileBrowser)
- Enter on a file in the source pane is the natural trigger
- The target is always the OTHER pane (no need to select target)

### DI Chain Changes

```go
// cmd/main.go (current)
sftpService := sftp_client.New(log)
transferService := transfer.New(log, sftpService)

// cmd/main.go (v1.3) -- no changes needed!
// DualRemoteFileBrowser creates its own SFTPClient instances internally
// RelayTransferService is created inside DualRemoteFileBrowser, not in main.go
```

**Why RelayTransferService is NOT created in main.go:**
- It needs two SFTPClient instances that are specific to the chosen server pair
- The server pair is only known at runtime (user selection in UI)
- Creating it in main.go would require passing it through the entire TUI -> handler -> component chain
- Internal creation in DualRemoteFileBrowser keeps the DI chain clean

### Modified vs New Files

| File | Change | Lines (est.) |
|------|--------|-------------|
| `internal/core/ports/relay_transfer.go` | New port interface | ~25 |
| `internal/adapters/data/transfer/relay_transfer_service.go` | New adapter | ~200 |
| `internal/adapters/ui/file_browser/dual_remote_browser.go` | New root component | ~300 |
| `internal/adapters/ui/file_browser/server_picker.go` | New overlay for server selection | ~150 |
| `internal/adapters/ui/file_browser/transfer_modal.go` | Add modeRelay, ShowRelay(), relay-specific Draw | +80 |
| `internal/adapters/ui/handlers.go` | Add 'M' key binding, handleDualRemote() | +40 |

### Dependency: None technically, but build after features 1 and 2 because it's the most complex

---

## Integration Points Summary

### Shared Changes Across Features

| Component | Feature 1 (Path History) | Feature 2 (Dup SSH) | Feature 3 (Dual Remote) |
|-----------|------------------------|--------------------|-----------------------|
| `internal/core/ports/` | New file: `path_history.go` | No change | New file: `relay_transfer.go` |
| `internal/adapters/data/` | New package: `local_path_history/` | No change | New file in `transfer/`: `relay_transfer_service.go` |
| `internal/adapters/ui/file_browser/` | New: `local_recent_dirs.go`, modify `file_browser.go`, `file_browser_handlers.go` | No change | New: `dual_remote_browser.go`, `server_picker.go`, modify `transfer_modal.go` |
| `internal/adapters/ui/handlers.go` | No change | Add `handleServerDup()`, 'D' key | Add `handleDualRemote()`, 'M' key |
| `cmd/main.go` | Create PathHistoryService, pass to TUI | No change | No change (internal creation) |
| Domain layer | No change | No change | No change |

### Build Order

```
Phase A: Dup SSH (simplest, zero architectural risk)
  └── internal/adapters/ui/handlers.go  (add 'D' key + handleServerDup)

Phase B: Local Path History (extends existing pattern)
  ├── internal/core/ports/path_history.go           (new port)
  ├── internal/adapters/data/local_path_history/     (new adapter)
  ├── internal/adapters/ui/file_browser/local_recent_dirs.go  (new overlay)
  └── internal/adapters/ui/file_browser/file_browser*.go      (wire + key routing)

Phase C: Dual-Remote Transfer (most complex, depends on understanding Phase A/B patterns)
  ├── internal/core/ports/relay_transfer.go           (new port)
  ├── internal/adapters/data/transfer/relay_transfer_service.go  (new adapter)
  ├── internal/adapters/ui/file_browser/server_picker.go        (new overlay)
  ├── internal/adapters/ui/file_browser/dual_remote_browser.go  (new component)
  └── internal/adapters/ui/file_browser/transfer_modal.go       (add modeRelay)
```

**Phase ordering rationale:**
- Phase A is trivially simple (~50 lines, one file, zero risk) -- do it first for quick win
- Phase B extends the RecentDirs pattern (well-understood, medium complexity)
- Phase C is architecturally novel (new component, new service, new UI mode) -- do it last when patterns from A/B are fresh

### Updated Component Map (All Features)

```
TUI (main view)
  ├── ServerList
  │   ├── 'D' -> handleServerDup() [Phase A]
  │   └── 'M' -> handleDualRemote() [Phase C]
  │
  └── FileBrowser (F key from server list)
      ├── localPane
      │   └── 'r' -> LocalRecentDirs overlay [Phase B]
      ├── remotePane
      │   └── 'r' -> RecentDirs overlay (existing)
      └── localRecentDirs (*LocalRecentDirs) [Phase B]

DualRemoteFileBrowser (M key from server list) [Phase C]
  ├── leftRemotePane  (Server A)
  ├── rightRemotePane (Server B)
  ├── transferModal (modeRelay)
  └── confirmDialog
```

## Anti-Patterns to Avoid

### 1. Don't try to mode-switch FileBrowser for dual-remote
FileBrowser has hard-coded local+remote semantics throughout (activePane, pane type checks, clipboard source pane). Adding a "dual remote mode" would require `if/else` branches everywhere. A separate component is cleaner.

### 2. Don't share SFTPClient between TransferService and RelayTransferService
Each SFTPClient manages one SSH process. Sharing would cause concurrent access issues (mutex contention at best, data corruption at worst). Always create new instances.

### 3. Don't forget to close both SFTP connections on DualRemoteFileBrowser close
The `close()` method must close BOTH sftpA and sftpB. Use goroutines (like existing FileBrowser.close()) to avoid blocking UI.

### 4. Don't store local path history per-server
Local paths are independent of which remote server you're connected to. The user's upload directory is the same whether they're connected to server-a or server-b. Keep local path history global.

### 5. Don't add RelayTransferService to the DI chain in main.go
The relay service needs runtime-determined SFTP connections (user picks servers in UI). Create it inside DualRemoteFileBrowser when servers are known.

### 6. Don't stream relay transfer without temp files
While theoretically possible (pipe src reader to dst writer), it loses progress accuracy and retry capability. The temp file approach is proven (CopyRemoteFile uses it).

## Scalability Considerations

| Concern | Current (v1.2) | v1.3 | Future |
|---------|---------------|------|--------|
| Overlay count | 4 | 6 (+LocalRecentDirs, ServerPicker) | Consider overlay manager at 6+ |
| TUI key bindings | ~15 in server list | ~17 (+D, M) | Approaching limit |
| SFTP connections | 1 per FileBrowser | 2 per DualRemoteFileBrowser | Parallel relay? |
| Services in DI chain | 4 | 5 (+PathHistoryService) | Still manageable |
| New top-level components | 1 (FileBrowser) | 2 (+DualRemoteFileBrowser) | Consider component registry |

**Key binding crowding mitigation:** At 17 bindings in the server list, we're approaching the memorability limit. The `M` key for dual-remote is acceptable because it's a power-user feature. Future features should consider prefix modes or a command palette.

## Sources

- **HIGH confidence (project source code):**
  - `internal/core/ports/file_service.go` -- FileService + SFTPService interfaces
  - `internal/core/ports/transfer.go` -- TransferService interface (reference for RelayTransferService)
  - `internal/core/ports/services.go` -- ServerService.AddServer() (reused by Dup SSH)
  - `internal/core/ports/repositories.go` -- ServerRepository.AddServer()
  - `internal/adapters/data/transfer/transfer_service.go` -- copyWithProgress, CopyRemoteFile pattern
  - `internal/adapters/data/sftp_client/sftp_client.go` -- SFTPClient connection model
  - `internal/adapters/ui/file_browser/file_browser.go` -- FileBrowser component structure
  - `internal/adapters/ui/file_browser/file_browser_handlers.go` -- Key routing chain
  - `internal/adapters/ui/file_browser/recent_dirs.go` -- Persistence pattern (JSON MRU)
  - `internal/adapters/ui/file_browser/transfer_modal.go` -- Modal mode system
  - `internal/adapters/ui/file_browser/local_pane.go` -- LocalPane callbacks
  - `internal/adapters/ui/handlers.go` -- Server list key routing, handleFileBrowser()
  - `internal/adapters/ui/tui.go` -- TUI struct, DI chain
  - `cmd/main.go` -- Application bootstrap
  - `internal/adapters/data/ssh_config_file/ssh_config_file_repo.go` -- AddServer implementation
  - `internal/core/services/server_service.go` -- validateServer, AddServer

---
*Architecture research: 2026-04-15 (v1.3 Enhanced File Browser)*
