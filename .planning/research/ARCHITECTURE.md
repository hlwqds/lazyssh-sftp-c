# Architecture Research: File Operations Integration (v1.2)

**Domain:** TUI File Management Operations for FileBrowser
**Researched:** 2026-04-15
**Confidence:** HIGH (based on direct code analysis of all file_browser package files, ports, adapters, and domain layer)

## Executive Summary

v1.2 file operations (delete, rename, mkdir, copy, move) integrate into the existing Clean Architecture through a three-layer expansion: (1) port interface additions to `FileService`, (2) adapter implementations in `LocalFS` and `SFTPClient`, (3) new overlay UI components following the established `TransferModal`/`RecentDirs` pattern.

The most architecturally significant finding is the clipboard/marking state management problem. Copy (`c`) and move (`x`) are two-step operations (mark source, navigate to destination, paste) that cross pane boundaries. This state must live in `FileBrowser` -- the only component with visibility into both panes. A simple `Clipboard` struct with `SourcePane`, `SourcePath`, `Operation`, and `SourceFiles` fields is sufficient.

All file management operations should be **synchronous for simple operations** (delete single file, rename, mkdir) and **asynchronous with progress for bulk operations** (recursive delete, copy/move directories). The async pattern follows the existing `initiateTransfer()` goroutine + `QueueUpdateDraw()` model.

## Existing Architecture: Current Component Map

```
FileBrowser (root, *tview.Flex)
  ├── localPane  (*LocalPane = *tview.Table)
  ├── remotePane (*RemotePane = *tview.Table)
  ├── statusBar  (*tview.TextView)
  ├── transferModal (*TransferModal = *tview.Box, overlay)
  └── recentDirs    (*RecentDirs = *tview.Box, overlay)
```

### Existing Key Routing Chain

```
Keyboard event propagation (v1.1):
  FileBrowser.SetInputCapture (handleGlobalKeys)
    → Overlay visibility check: recentDirs intercepts all keys when visible
    → Esc: transferModal.HandleKey() or close
    → Tab: switchFocus()
    → F5: initiateDirTransfer()
    → r (remote pane): recentDirs.Show()
    → s/S: sort controls
    → event passed to focused pane

  Pane.SetInputCapture (local_pane.go / remote_pane.go)
    → h: NavigateToParent()
    → Space: ToggleSelection()
    → . : ToggleHidden()
    → Backspace: NavigateToParent()
    → event passed to Table built-in (j/k/arrows/Enter/PgUp/PgDn)

  Pane.SetSelectedFunc (Enter on row)
    → NavigateInto() for directories
    → onFileAction callback for files (triggers initiateTransfer)
```

### Existing Overlay Pattern (TransferModal + RecentDirs as Reference)

Both overlays follow the same pattern:

1. **Embed `*tview.Box`** (not a full tview.Primitive with InputHandler)
2. **Manual `Draw(screen tcell.Screen)`** with `visible` flag guard
3. **No tview focus** -- key interception in `FileBrowser.handleGlobalKeys()`:
   ```go
   // Overlay key interception: check BEFORE any other key handling
   if fb.recentDirs != nil && fb.recentDirs.IsVisible() {
       return fb.recentDirs.HandleKey(event)
   }
   ```
4. **Draw chain** in `FileBrowser.Draw()`:
   ```go
   func (fb *FileBrowser) Draw(screen tcell.Screen) {
       fb.Flex.Draw(screen)
       if fb.transferModal != nil && fb.transferModal.IsVisible() {
           fb.transferModal.Draw(screen)
       }
       if fb.recentDirs != nil && fb.recentDirs.IsVisible() {
           fb.recentDirs.Draw(screen)
       }
   }
   ```
5. **State machine** via enum (`modalMode` for TransferModal)
6. **Dismiss callback** for cleanup after hide

**v1.2 overlay components must follow this exact pattern.**

## Architecture for v1.2 File Operations

### Updated Component Map

```
FileBrowser (root, *tview.Flex)
  ├── localPane      (*LocalPane = *tview.Table)
  ├── remotePane     (*RemotePane = *tview.Table)
  ├── statusBar      (*tview.TextView)
  ├── transferModal  (*TransferModal = *tview.Box, overlay)
  ├── recentDirs     (*RecentDirs = *tview.Box, overlay)
  ├── confirmDialog  (*ConfirmDialog = *tview.Box, overlay)  ← NEW
  ├── inputDialog    (*InputDialog = *tview.Box, overlay)    ← NEW
  └── clipboard      (*Clipboard)                            ← NEW (pure state, no UI)
```

### Layer 1: Port Interface Changes

#### FileService Interface Expansion

The critical design decision is **promoting shared operations to `FileService`**. Currently, `Remove` and `Stat` are only in `SFTPService`. For v1.2, both local and remote panes need delete, rename, mkdir, and stat. Rather than doing `if local then os.Remove else sftp.Remove` in the UI layer, we promote these to the shared `FileService` interface:

```go
// FileService provides file listing and management operations for local and remote filesystems.
type FileService interface {
    // --- Existing (v1.0) ---
    ListDir(path string, showHidden bool, sortField domain.FileSortField, sortAsc bool) ([]domain.FileInfo, error)

    // --- v1.2 Additions ---
    Remove(path string) error
    RemoveAll(path string) error
    Rename(oldPath, newPath string) error
    Mkdir(path string) error
    Stat(path string) (os.FileInfo, error)
}
```

**Why `Remove` and `Stat` move from SFTPService to FileService:**
- Both local and remote panes need identical operations
- UI layer should not do type assertions or `if/else` based on pane identity
- `SFTPService` embeds `FileService`, so it still has these methods -- no behavior change
- Compile-time safety: both `LocalFS` and `SFTPClient` must implement all methods

**Why `Mkdir` is separate from `MkdirAll`:**
- `Mkdir` creates a single directory and fails if parent doesn't exist -- appropriate for "new directory" UI where we want to catch typos
- `MkdirAll` creates recursively -- appropriate for transfer service where intermediate directories must exist
- Different semantics for different use cases

**Why NOT `CopyFile`/`CopyDir` in FileService:**
- Copy is a heavy I/O operation needing `context.Context`, progress callbacks, and conflict handling
- These requirements match `TransferService`'s interface pattern (see CopyService below)
- Mixing simple CRUD (`Remove`, `Rename`, `Mkdir`) with streaming I/O (`CopyFile`) violates interface segregation

#### New CopyService Interface

```go
// CopyService provides copy operations within a single filesystem.
// Cross-pane copy (local-to-remote, remote-to-local) uses existing TransferService.
type CopyService interface {
    CopyFile(ctx context.Context, srcPath, dstPath string, onProgress func(domain.TransferProgress), onConflict domain.ConflictHandler) error
    CopyDir(ctx context.Context, srcPath, dstPath string, onProgress func(domain.TransferProgress), onConflict domain.ConflictHandler) ([]string, error)
}
```

**Why a separate CopyService rather than extending FileService:**
- `context.Context` for cancellation (not needed for `Remove`/`Rename`/`Mkdir`)
- `onProgress` callback (heavy I/O vs instant operations)
- `onConflict` callback (copy can conflict, rename/mkdir don't)
- Interface Segregation Principle: FileService callers don't need progress callbacks

#### SFTPService After Refactoring

```go
type SFTPService interface {
    FileService  // inherits ListDir, Remove, RemoveAll, Rename, Mkdir, Stat

    // Connection lifecycle (unchanged)
    Connect(server domain.Server) error
    Close() error
    IsConnected() bool
    HomeDir() string

    // Remote I/O (unchanged)
    CreateRemoteFile(path string) (io.WriteCloser, error)
    OpenRemoteFile(path string) (io.ReadCloser, error)
    WalkDir(path string) ([]string, error)
}
```

Note: `Remove` and `Stat` are no longer declared directly on `SFTPService` -- they come from `FileService`. Behavior is identical. Existing code that calls `sftpService.Remove()` or `sftpService.Stat()` continues to work unchanged.

### Layer 2: Adapter Implementations

#### SFTPClient New Methods

All methods follow the existing mutex pattern: `c.mu.Lock()` -> acquire `c.client` -> `c.mu.Unlock()`.

| Method | Implementation | pkg/sftp API |
|--------|---------------|-------------|
| `RemoveAll(path)` | `client.RemoveAll(path)` | SSH_FXP_REMOVE (recursive via library) |
| `Rename(old, new)` | `client.Rename(old, new)` | SSH_FXP_RENAME |
| `Mkdir(path)` | `client.Mkdir(path)` | SSH_FXP_MKDIR |
| `Stat(path)` | **already implemented** | SSH_FXP_LSTAT |
| `Remove(path)` | **already implemented** | SSH_FXP_REMOVE |

**Existing methods that need no changes:** `Remove`, `Stat`, `MkdirAll`, `CreateRemoteFile`, `OpenRemoteFile`, `WalkDir`.

#### LocalFS New Methods

| Method | Implementation |
|--------|---------------|
| `Remove(path)` | `os.Remove(path)` |
| `RemoveAll(path)` | `os.RemoveAll(path)` |
| `Rename(old, new)` | `os.Rename(old, new)` |
| `Mkdir(path)` | `os.Mkdir(path, 0o750)` |
| `Stat(path)` | `os.Stat(path)` |

All are one-line proxies. No error wrapping beyond `fmt.Errorf`.

#### CopyService Implementations

Two adapters needed:

**1. `LocalCopyService`** (`internal/adapters/data/local_fs/local_copy.go`)
- Uses `os.Open` + `os.Create` + 32KB buffer
- Reuses the `copyWithProgress` pattern from `transfer_service.go`
- For `CopyDir`: uses `filepath.WalkDir` to enumerate, creates directories via `os.MkdirAll`

**2. `RemoteCopyService`** (`internal/adapters/data/sftp_client/remote_copy.go`)
- Uses `sftpService.OpenRemoteFile` + `sftpService.CreateRemoteFile` + 32KB buffer
- For `CopyDir`: uses `sftpService.WalkDir` to enumerate, creates directories via `sftpService.MkdirAll`
- Depends on `ports.SFTPService` for I/O operations

**Design decision: Extract `copyWithProgress` as a shared utility.**

The `copyWithProgress` function in `transfer/transfer_service.go` (lines 436-485) is identical for local-to-local and remote-to-remote copies. Options:

| Approach | Pros | Cons |
|----------|------|------|
| **Extract to `internal/core/services/copy.go`** (Recommended) | Shared code, single implementation, tested once | Adds a file to core/services (currently only server_service.go) |
| Duplicate in each CopyService | No shared dependency | 3 copies of the same code |
| Keep in transfer package, import from copy | Minimal change | Circular dependency risk if copy imports transfer types |

**Recommendation:** Extract `copyWithProgress` to `internal/core/services/copy.go`. Both `transfer.TransferService` and the two `CopyService` implementations import it. The function is pure I/O with no domain coupling -- it belongs in services, not adapters.

### Layer 3: UI Components

#### New Overlay: ConfirmDialog

Used for delete confirmation and dangerous operations.

```
ConfirmDialog (new struct)
  ├── *tview.Box (background, border, title)
  ├── message string
  ├── visible bool
  ├── onConfirm func()
  ├── onCancel func()
  └── mode confirmMode (confirm/cancel)
```

**File location:** `internal/adapters/ui/file_browser/confirm_dialog.go`

**Draw layout:**
```
┌─────────────────────────────┐
│  Confirm Delete             │
│                             │
│  Delete "filename.txt"?     │
│  (directory: recursively)   │
│                             │
│  [y] Yes  [n] No            │
│  Press Esc to cancel        │
└─────────────────────────────┘
```

**HandleKey dispatch:**
- `y` / `Enter` -> call `onConfirm`, `Hide()`
- `n` / `Esc` -> call `onCancel`, `Hide()`
- All other keys -> consumed (return nil)

**Pattern reference:** Follows `TransferModal.modeCancelConfirm` but as a standalone component. The cancel-confirm in TransferModal is embedded in a multi-mode state machine; a standalone `ConfirmDialog` is cleaner for delete/rename confirmations.

#### New Overlay: InputDialog

Used for rename and mkdir (any operation needing user text input).

```
InputDialog (new struct)
  ├── *tview.Box (background, border, title)
  ├── inputField *tview.InputField (embedded tview widget)
  ├── visible bool
  ├── onSubmit func(value string)
  ├── onCancel func()
  └── mode inputMode (rename/mkdir)
```

**File location:** `internal/adapters/ui/file_browser/input_dialog.go`

**Draw layout:**
```
┌─────────────────────────────────┐
│  Rename                         │
│                                 │
│  New name: [filename_new.txt__] │
│                                 │
│  [Enter] Confirm  [Esc] Cancel  │
└─────────────────────────────────┘
```

**Key design decision: tview.InputField vs manual text editing.**

| Approach | Pros | Cons |
|----------|------|------|
| **tview.InputField embedded in Box** (Recommended) | Handles cursor movement, text selection, character input; battle-tested; used in ServerForm | Needs focus management within overlay |
| Manual text buffer + Draw | Full control over rendering | Re-implementing cursor, text editing, Unicode handling -- high effort for no benefit |

**Recommendation:** Embed `tview.InputField` in the overlay Box. The InputField is a proper tview.Primitive with its own InputHandler. Key routing:

1. When `InputDialog` is visible, `handleGlobalKeys` delegates to `InputDialog.HandleKey()`
2. `InputDialog.HandleKey()` routes to `inputField.InputHandler()` for text editing
3. `Enter` triggers `onSubmit`, `Esc` triggers `onCancel`

**Focus management challenge:** `tview.InputField` expects to receive focus via `app.SetFocus()`. But overlays use manual key interception, not tview focus. Solution: call `inputField.InputHandler(event, func(p tview.Primitive) { ... })` directly in `InputDialog.HandleKey()`, bypassing tview's focus system. The callback from InputHandler is ignored (we don't change focus).

```go
func (id *InputDialog) HandleKey(event *tcell.EventKey) *tcell.EventKey {
    if !id.visible {
        return event
    }
    // Route all keys to InputField for text editing
    id.inputField.InputHandler(event, func(tview.Primitive) {})
    // Check if Enter was pressed (InputField handles it)
    if event.Key() == tcell.KeyEnter {
        id.onSubmit(id.inputField.GetText())
        id.Hide()
        return nil
    }
    if event.Key() == tcell.KeyEscape {
        id.onCancel()
        id.Hide()
        return nil
    }
    return nil // consume all keys when visible
}
```

**Wait -- there's a subtlety.** `tview.InputField` handles `Enter` internally via `doneFunc`. If we set a `doneFunc` on the InputField, it fires on Enter. We should NOT also check for Enter in HandleKey -- that would double-fire. The correct approach:

```go
func NewInputDialog(app *tview.Application) *InputDialog {
    id := &InputDialog{Box: tview.NewBox(), app: app}
    id.inputField = tview.NewInputField()
    id.inputField.SetDoneFunc(func(key tcell.Key) {
        if key == tcell.KeyEnter {
            if id.onSubmit != nil {
                id.onSubmit(id.inputField.GetText())
            }
            id.Hide()
        } else if key == tcell.KeyEscape {
            if id.onCancel != nil {
                id.onCancel()
            }
            id.Hide()
        }
    })
    return id
}

func (id *InputDialog) HandleKey(event *tcell.EventKey) *tcell.EventKey {
    if !id.visible {
        return event
    }
    // Route to InputField which handles Enter/Esc via doneFunc
    id.inputField.InputHandler(event, func(tview.Primitive) {})
    return nil // consume all keys
}
```

This correctly delegates Enter/Esc handling to the InputField's `doneFunc`.

#### Clipboard State: Where Does It Live?

**Answer: `FileBrowser` owns the clipboard.**

```
Clipboard (pure state struct, no UI)
  ├── SourcePane int      // 0 = local, 1 = remote
  ├── SourceFiles []domain.FileInfo
  ├── SourceDir string     // absolute directory path
  ├── Operation ClipboardOp // copy or move
  └── Active bool
```

**File location:** `internal/adapters/ui/file_browser/clipboard.go`

**Why in FileBrowser, not in panes:**
- Clipboard spans two panes: mark in source pane, paste in target pane
- `FileBrowser` is the only component with access to both panes
- Panes are independent components -- they shouldn't know about each other
- Matches the existing pattern: `FileBrowser` owns cross-pane state (`activePane`, `transferring`, `transferCancel`)

**Why not in domain layer:**
- Clipboard is purely UI state (which file is selected for copy/move)
- It doesn't need persistence, business rules, or service-layer access
- Domain layer should stay clean of UI-specific state

**User flow for copy:**
```
1. User selects file in LocalPane
2. User presses 'c' -> LocalPane.SetInputCapture intercepts
3. LocalPane calls onMarkCopy callback (new callback)
4. FileBrowser.handleMarkCopy() stores: {SourcePane: 0, SourceFiles: [fi], Operation: copy}
5. FileBrowser updates status bar: "1 file(s) marked for copy"
6. User presses Tab -> switchFocus() to RemotePane
7. User navigates to destination directory
8. User presses 'p' -> handleGlobalKeys intercepts
9. FileBrowser.handlePaste() checks clipboard is active
10. FileBrowser executes copy: LocalCopyService or TransferService
11. FileBrowser clears clipboard, refreshes target pane
```

**User flow for move:**
Same as copy, but:
- Step 2: User presses 'x' instead of 'c'
- Step 4: Operation stored as `move`
- Step 10: After copy completes, delete source files
- Move = copy + delete. For cross-pane move, use TransferService (which already handles upload/download) + delete source.

**Cross-pane copy/move decision tree:**

| Source | Target | Copy Implementation | Move Implementation |
|--------|--------|-------------------|-------------------|
| Local | Local | `LocalCopyService.CopyFile/CopyDir` | `LocalCopyService` + `LocalFS.Remove/RemoveAll` |
| Remote | Remote | `RemoteCopyService.CopyFile/CopyDir` | `RemoteCopyService` + `SFTPClient.Remove/RemoveAll` |
| Local | Remote | `TransferService.UploadFile/UploadDir` | `TransferService.Upload` + `LocalFS.Remove/RemoveAll` |
| Remote | Local | `TransferService.DownloadFile/DownloadDir` | `TransferService.Download` + `SFTPClient.Remove/RemoveAll` |

**Same-pane operations** (copy within local, copy within remote) use the new `CopyService`. **Cross-pane operations** use the existing `TransferService`. This maximizes code reuse.

### Key Routing Integration

Updated `handleGlobalKeys` with v1.2 keys:

```
FileBrowser.handleGlobalKeys(event):
  1. Overlay interception chain (order matters):
     a. inputDialog visible? -> inputDialog.HandleKey(event)
     b. confirmDialog visible? -> confirmDialog.HandleKey(event)
     c. recentDirs visible? -> recentDirs.HandleKey(event)
     d. transferModal visible? -> transferModal.HandleKey(event)
  2. Global keys:
     Tab -> switchFocus()
     Esc -> close()
     F5 -> initiateDirTransfer()
     p -> handlePaste()  ← NEW
  3. Pane-specific global keys:
     r (remote only) -> recentDirs.Show()
     s/S -> sort controls
  4. Pass to focused pane
```

Updated pane `SetInputCapture`:

```
Pane.SetInputCapture(event):
  h -> NavigateToParent()
  Space -> ToggleSelection()
  . -> ToggleHidden()
  d -> onDelete callback  ← NEW (if file selected)
  R -> onRename callback  ← NEW (if file selected)
  c -> onMarkCopy callback  ← NEW (if file selected)
  x -> onMarkMove callback  ← NEW (if file selected)
  m -> onMkdir callback  ← NEW
  Backspace -> NavigateToParent()
  Pass to Table (j/k/arrows/Enter/PgUp/PgDn)
```

**Why `d`, `R`, `c`, `x` go in Pane InputCapture (not handleGlobalKeys):**
- These keys operate on the **currently selected item** in the focused pane
- Panes own selection state (`selected map[string]bool`, `GetSelection()`)
- Pane InputCapture is the established location for item-specific actions (like Space for toggle selection)
- The callback pattern (`onDelete`, `onRename`, etc.) follows the existing `onFileAction` pattern

**Why `m` (mkdir) goes in Pane InputCapture:**
- mkdir creates a directory in the **current pane's path**
- Pane owns `currentPath`
- The callback passes `currentPath` to FileBrowser, which shows the InputDialog

**Why `p` (paste) goes in handleGlobalKeys:**
- Paste operates on the **clipboard** (owned by FileBrowser) and the **target pane's path**
- FileBrowser is the only component that knows both the clipboard state and which pane is focused
- Paste doesn't depend on which specific row is selected in the target pane -- it uses the directory path

### Overlay Priority and Mutual Exclusion

**Only one overlay visible at a time.** This is a hard constraint to avoid key routing ambiguity.

| Overlay | Trigger | Blocks Until |
|---------|---------|-------------|
| `transferModal` | F5 or Enter on file | Transfer complete/canceled |
| `recentDirs` | `r` key | Selection or Esc |
| `confirmDialog` | `d` key | Confirm or cancel |
| `inputDialog` | `R` or `m` key | Submit or cancel |

**Enforcement:** Before showing any overlay, check that no other overlay is visible:

```go
func (fb *FileBrowser) anyOverlayVisible() bool {
    return (fb.transferModal != nil && fb.transferModal.IsVisible()) ||
        (fb.recentDirs != nil && fb.recentDirs.IsVisible()) ||
        (fb.confirmDialog != nil && fb.confirmDialog.IsVisible()) ||
        (fb.inputDialog != nil && fb.inputDialog.IsVisible())
}
```

The key routing chain naturally enforces this because the first visible overlay intercepts all keys.

### Synchronous vs Asynchronous Operations

| Operation | Sync/Async | Rationale |
|-----------|-----------|-----------|
| Delete single file | **Sync** | `os.Remove` / `client.Remove` -- instant |
| Delete directory (recursive) | **Async** | `os.RemoveAll` / `client.RemoveAll` may take time for large directories; show progress |
| Rename | **Sync** | `os.Rename` / `client.Rename` -- instant (same filesystem) |
| Mkdir | **Sync** | `os.Mkdir` / `client.Mkdir` -- instant |
| Copy file (same pane) | **Async** | Streaming I/O, needs progress display |
| Copy directory (same pane) | **Async** | Many files, needs progress + cancel |
| Copy/Move (cross-pane) | **Async** | Reuses TransferService which is async |
| Move (same pane) | **Async** | Copy + delete, needs progress |

**Sync operations** execute in the UI thread (inside `QueueUpdateDraw` or directly in the key handler). Since they're instant, no blocking concern.

**Async operations** follow the existing transfer pattern:
```go
func (fb *FileBrowser) handlePaste() {
    // ... validation ...
    ctx, cancel := context.WithCancel(context.Background())
    go func() {
        err := copyService.CopyFile(ctx, src, dst, func(p domain.TransferProgress) {
            fb.app.QueueUpdateDraw(func() {
                fb.transferModal.Update(p)
            })
        }, onConflict)
        fb.app.QueueUpdateDraw(func() {
            if err != nil { /* show error */ }
            else { /* refresh pane */ }
        })
    }()
}
```

For async delete of directories, we can show a simple status bar message ("Deleting directory...") rather than a full TransferModal. Only the progress-intensive operations (copy/move) need the modal.

### Draw Chain Update

```go
func (fb *FileBrowser) Draw(screen tcell.Screen) {
    fb.Flex.Draw(screen)
    // Draw overlays in priority order (last drawn = visually on top)
    if fb.transferModal != nil && fb.transferModal.IsVisible() {
        fb.transferModal.Draw(screen)
    }
    if fb.recentDirs != nil && fb.recentDirs.IsVisible() {
        fb.recentDirs.Draw(screen)
    }
    if fb.confirmDialog != nil && fb.confirmDialog.IsVisible() {  // NEW
        fb.confirmDialog.Draw(screen)
    }
    if fb.inputDialog != nil && fb.inputDialog.IsVisible() {      // NEW
        fb.inputDialog.Draw(screen)
    }
}
```

Since only one overlay is visible at a time, the draw order among overlays doesn't matter visually. But maintaining a consistent order is good practice.

### Data Flow Diagrams

#### Delete Flow

```
User presses 'd' on selected file
    |
    v
Pane.SetInputCapture -> onDelete callback
    |
    v
FileBrowser.handleDelete(fi domain.FileInfo)
    |
    ├─ Check: anyOverlayVisible() -> return
    ├─ Check: transferring -> return
    |
    v
Build message: "Delete \"filename.txt\"?"
    |
    v
confirmDialog.Show(message)
  confirmDialog.onConfirm = func() {
      fullPath := joinPath(pane.currentPath, fi.Name)
      if fi.IsDir {
          // Async: recursive delete
          go func() {
              err := fileService.RemoveAll(fullPath)
              fb.app.QueueUpdateDraw(func() {
                  if err != nil { showError(err) }
                  else { pane.Refresh() }
              })
          }()
      } else {
          // Sync: single file delete
          err := fileService.Remove(fullPath)
          if err != nil { showError(err) }
          else { pane.Refresh() }
      }
  }
```

#### Rename Flow

```
User presses 'R' on selected file
    |
    v
Pane.SetInputCapture -> onRename callback
    |
    v
FileBrowser.handleRename(fi domain.FileInfo)
    |
    ├─ Check: anyOverlayVisible() -> return
    |
    v
inputDialog.Show("Rename", fi.Name)
  inputDialog.onSubmit = func(newName string) {
      if newName == "" || newName == fi.Name { return }
      oldPath := joinPath(pane.currentPath, fi.Name)
      newPath := joinPath(pane.currentPath, newName)
      err := fileService.Rename(oldPath, newPath)
      if err != nil { showError(err) }
      else { pane.Refresh() }
  }
```

#### Mkdir Flow

```
User presses 'm'
    |
    v
Pane.SetInputCapture -> onMkdir callback
    |
    v
FileBrowser.handleMkdir()
    |
    ├─ Check: anyOverlayVisible() -> return
    |
    v
inputDialog.Show("New Directory", "")
  inputDialog.onSubmit = func(name string) {
      if name == "" { return }
      fullPath := joinPath(pane.currentPath, name)
      err := fileService.Mkdir(fullPath)
      if err != nil { showError(err) }
      else { pane.Refresh() }
  }
```

#### Copy/Move (Mark + Paste) Flow

```
User presses 'c' or 'x' on selected file(s)
    |
    v
Pane.SetInputCapture -> onMarkCopy/onMarkMove callback
    |
    v
FileBrowser.handleMark(op ClipboardOp)
    |
    ├─ Collect selected files (or current row if none selected)
    ├─ Store in clipboard: {SourcePane, SourceFiles, SourceDir, Operation}
    └─ Update status bar: "3 file(s) marked for copy"
    |
    ... user navigates to target pane/directory ...
    |
User presses 'p'
    |
    v
handleGlobalKeys -> handlePaste()
    |
    ├─ Check: clipboard.Active -> false? show error
    ├─ Determine source/target services based on SourcePane vs activePane
    |
    ├─ Same pane (SourcePane == activePane):
    |   └─ CopyService.CopyFile/CopyDir (async)
    |
    └─ Cross pane (SourcePane != activePane):
        └─ TransferService.Upload/Download (async)
    |
    v
If Operation == move:
    After copy completes, delete source files
    |
    v
Clear clipboard, refresh target pane, update status bar
```

### Dependency Injection Chain

The DI chain in `cmd/main.go` needs to add CopyService:

```go
// cmd/main.go (current)
fileService := local_fs.New(log)
sftpService := sftp_client.New(log)
transferService := transfer.New(log, sftpService)
tui := ui.NewTUI(log, serverService, fileService, sftpService, transferService, version, gitCommit)

// cmd/main.go (v1.2)
fileService := local_fs.New(log)
sftpService := sftp_client.New(log)
transferService := transfer.New(log, sftpService)
localCopyService := local_fs.NewCopyService(log)           // NEW
remoteCopyService := sftp_client.NewCopyService(log, sftpService) // NEW
tui := ui.NewTUI(log, serverService, fileService, sftpService, transferService,
    localCopyService, remoteCopyService, version, gitCommit)  // UPDATED
```

The `NewFileBrowser` constructor also needs the new services:

```go
func NewFileBrowser(
    app *tview.Application,
    log *zap.SugaredLogger,
    fs ports.FileService,
    sftp ports.SFTPService,
    ts ports.TransferService,
    lcs ports.CopyService,   // NEW: local copy service
    rcs ports.CopyService,   // NEW: remote copy service
    server domain.Server,
    onClose func(),
) *FileBrowser
```

**Design decision: Pass two CopyService instances or one with a "side" parameter?**

| Approach | Pros | Cons |
|----------|------|------|
| **Two separate instances** (Recommended) | Each is a simple adapter; no runtime dispatch; type-safe | One more constructor parameter |
| One CopyService with `side` parameter | Fewer parameters | Runtime type switch; less clean |

**Recommendation:** Two separate instances. Both implement the same `CopyService` interface. `FileBrowser` dispatches to the correct one based on source/target pane.

## Modified vs New Files

### New Files

| File | Purpose | Lines (est.) |
|------|---------|-------------|
| `internal/core/ports/copy.go` | CopyService interface definition | ~15 |
| `internal/core/services/copy.go` | Extracted copyWithProgress utility | ~50 |
| `internal/adapters/data/local_fs/local_copy.go` | LocalCopyService adapter | ~120 |
| `internal/adapters/data/sftp_client/remote_copy.go` | RemoteCopyService adapter | ~130 |
| `internal/adapters/ui/file_browser/confirm_dialog.go` | ConfirmDialog overlay | ~100 |
| `internal/adapters/ui/file_browser/input_dialog.go` | InputDialog overlay | ~120 |
| `internal/adapters/ui/file_browser/clipboard.go` | Clipboard state struct | ~50 |

### Modified Files

| File | Change | Lines (est.) |
|------|--------|-------------|
| `internal/core/ports/file_service.go` | Add Remove, RemoveAll, Rename, Mkdir, Stat to FileService | +10 |
| `internal/adapters/data/sftp_client/sftp_client.go` | Add RemoveAll, Rename, Mkdir methods | +40 |
| `internal/adapters/data/local_fs/local_fs.go` | Add Remove, RemoveAll, Rename, Mkdir, Stat methods | +40 |
| `internal/adapters/data/transfer/transfer_service.go` | Replace inline copyWithProgress with import from services | -50, +5 |
| `internal/adapters/ui/file_browser/file_browser.go` | Add clipboard, confirmDialog, inputDialog fields; add handler methods | +80 |
| `internal/adapters/ui/file_browser/file_browser_handlers.go` | Add overlay checks, paste key routing | +30 |
| `internal/adapters/ui/file_browser/local_pane.go` | Add d/R/c/x/m key bindings + callbacks | +30 |
| `internal/adapters/ui/file_browser/remote_pane.go` | Add d/R/c/x/m key bindings + callbacks | +30 |
| `cmd/main.go` | Add CopyService creation, pass to TUI | +5 |
| `internal/adapters/ui/handlers.go` | Pass CopyService to NewFileBrowser | +5 |
| `go.mod` | Change pkg/sftp from indirect to direct | 1 line |

### Unchanged Files

- `internal/core/domain/file_info.go` -- FileInfo struct needs no changes
- `internal/core/domain/transfer.go` -- TransferProgress and ConflictHandler reused as-is
- `internal/adapters/ui/file_browser/transfer_modal.go` -- Reused for copy/move progress, not modified
- `internal/adapters/ui/file_browser/recent_dirs.go` -- Independent feature, not modified
- `internal/adapters/ui/file_browser/progress_bar.go` -- Reused by TransferModal, not modified
- `internal/adapters/ui/file_browser/file_sort.go` -- Sort logic, not modified

## Build Order

Build order respects dependency chains and allows incremental testing.

```
Phase 1: Port interfaces + shared utility (no UI changes)
  ├── internal/core/ports/file_service.go     (add methods to FileService)
  ├── internal/core/ports/copy.go             (new CopyService interface)
  └── internal/core/services/copy.go          (extract copyWithProgress)

Phase 2: Adapter implementations (depends on Phase 1)
  ├── internal/adapters/data/local_fs/local_fs.go      (add new methods)
  ├── internal/adapters/data/sftp_client/sftp_client.go (add new methods)
  ├── internal/adapters/data/local_fs/local_copy.go     (new file)
  ├── internal/adapters/data/sftp_client/remote_copy.go (new file)
  └── internal/adapters/data/transfer/transfer_service.go (refactor copyWithProgress)

Phase 3: UI state + simple overlays (depends on Phase 2)
  ├── internal/adapters/ui/file_browser/clipboard.go      (pure state)
  ├── internal/adapters/ui/file_browser/confirm_dialog.go (overlay)
  └── internal/adapters/ui/file_browser/input_dialog.go   (overlay)

Phase 4: Key routing + handler wiring (depends on Phase 3)
  ├── internal/adapters/ui/file_browser/local_pane.go          (new keys + callbacks)
  ├── internal/adapters/ui/file_browser/remote_pane.go         (new keys + callbacks)
  ├── internal/adapters/ui/file_browser/file_browser.go        (handler methods)
  └── internal/adapters/ui/file_browser/file_browser_handlers.go (overlay chain + paste)

Phase 5: DI chain + integration (depends on Phase 4)
  ├── cmd/main.go              (create CopyServices, pass to TUI)
  └── internal/adapters/ui/handlers.go (pass CopyServices to NewFileBrowser)
```

## Anti-Patterns to Avoid

### 1. Don't add file operations to TransferService
TransferService is specifically for cross-pane transfers (local <-> remote). Adding same-pane copy would violate single responsibility. Use the separate CopyService.

### 2. Don't use tview.Modal for confirm/input dialogs
`tview.Modal` uses `app.SetRoot()` which replaces the entire view. This breaks the overlay draw chain and causes visual artifacts. Use the `*tview.Box` + manual `Draw()` pattern established by TransferModal and RecentDirs.

### 3. Don't store clipboard state in panes
Panes are independent components. Clipboard requires cross-pane visibility. Storing clipboard in LocalPane means RemotePane can't access it during paste.

### 4. Don't handle delete/rename in the pane's SetSelectedFunc
SetSelectedFunc fires on Enter key, which is already used for navigation (directories) and file transfer (files). Adding delete/rename there would conflict.

### 5. Don't make FileService methods async
`Remove`, `Rename`, `Mkdir`, `Stat` are instant operations. Adding `context.Context` or callbacks would over-engineer the interface. Only `CopyService` needs async semantics.

### 6. Don't use SFTP Rename for move-to-existing-target
`SSH_FXP_RENAME` behavior when the target exists is server-dependent (RFC undefined). For move operations where the target might exist, use the CopyService copy + delete pattern instead, which gives us conflict handling via `onConflict`.

### 7. Don't forget the overlay mutual exclusion check
If two overlays somehow become visible simultaneously, key routing becomes ambiguous. Always check `anyOverlayVisible()` before showing a new overlay.

## Scalability Considerations

| Concern | Current (v1.1) | v1.2 | Future |
|---------|---------------|------|--------|
| Overlay count | 2 (TransferModal, RecentDirs) | 4 (+ConfirmDialog, InputDialog) | Could grow; consider overlay manager |
| Key bindings in panes | 3 (h, Space, .) | 8 (+d, R, c, x, m) | Approaching keyboard crowding |
| FileBrowser fields | 7 | 10 (+clipboard, confirmDialog, inputDialog) | Manageable |
| Constructor params | 6 | 8 (+2 CopyServices) | Consider options struct if grows further |
| Services in DI chain | 4 (fileService, sftpService, transferService, serverService) | 6 (+2 CopyServices) | Consider service container |

**Keyboard crowding mitigation:** At 8 pane-level bindings, we're approaching the limit of memorable shortcuts. Future features should consider prefix modes (e.g., `g` as a leader key like vim's `g` prefix).

**Overlay manager:** With 4 overlays, the mutual exclusion check is still simple (4 boolean checks). If we reach 6+, consider an `OverlayManager` struct that tracks the active overlay and provides `Show(overlay)` / `IsActive()` methods.

## Sources

- **HIGH confidence (project source code):**
  - `internal/core/ports/file_service.go` -- FileService + SFTPService interfaces
  - `internal/core/ports/transfer.go` -- TransferService interface pattern
  - `internal/adapters/data/sftp_client/sftp_client.go` -- SFTPClient implementation
  - `internal/adapters/data/local_fs/local_fs.go` -- LocalFS implementation
  - `internal/adapters/data/transfer/transfer_service.go` -- copyWithProgress pattern (lines 436-485)
  - `internal/adapters/ui/file_browser/file_browser.go` -- FileBrowser orchestration
  - `internal/adapters/ui/file_browser/file_browser_handlers.go` -- Key routing chain
  - `internal/adapters/ui/file_browser/transfer_modal.go` -- Overlay pattern reference
  - `internal/adapters/ui/file_browser/recent_dirs.go` -- Overlay pattern reference
  - `internal/adapters/ui/file_browser/local_pane.go` -- Pane callback pattern
  - `internal/adapters/ui/file_browser/remote_pane.go` -- Pane callback pattern
  - `internal/core/domain/transfer.go` -- ConflictHandler, TransferProgress types
  - `internal/core/domain/file_info.go` -- FileInfo domain model
  - `cmd/main.go` -- DI chain

- **HIGH confidence (pkg/sftp library API):**
  - `github.com/pkg/sftp v1.13.10` -- Remove, RemoveAll, Rename, Mkdir, Stat (verified in go module source)

---
*Architecture research: 2026-04-15 (v1.2 File Management Operations)*
*Original: 2026-04-14 (v1.1 Recent Remote Directories overlay pattern)*
