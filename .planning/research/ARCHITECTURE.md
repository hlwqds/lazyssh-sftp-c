# Architecture Research

**Analysis Date:** 2026-04-13
**Domain:** File Transfer Integration with Existing Clean Architecture

## Existing Architecture Summary

```
cmd/main.go (Entry)
  ↓
internal/core/domain/server.go (Domain Entity)
internal/core/ports/ (Interfaces)
internal/core/services/server_service.go (Business Logic)
  ↓
internal/adapters/data/ (SSH Config persistence)
internal/adapters/ui/ (TUI presentation)
```

## New Components Needed

### Domain Layer

**No changes needed.** File transfer operations are application services, not domain entities. The `Server` domain entity already contains all SSH connection info needed.

### Ports Layer (New Interfaces)

```
internal/core/ports/file_transfer.go
  - FileTransferService interface
    - ListLocalDir(path) → []FileInfo
    - ListRemoteDir(server, path) → []FileInfo
    - Upload(server, localPath, remotePath) → TransferTask
    - Download(server, remotePath, localPath) → TransferTask
    - CancelTransfer(taskID) error
```

```
internal/core/ports/file_browser.go
  - FileInfo struct (name, size, mode, modTime, isDir)
  - TransferProgress struct (bytesTotal, bytesDone, speed, eta)
  - TransferCallback func(TransferProgress)
```

### Services Layer (New Service)

```
internal/core/services/file_transfer_service.go
  - fileTransferService struct
    - Uses pkg/sftp for remote operations
    - Uses os for local operations
    - Manages transfer lifecycle
    - Tracks active transfers for cancel support
```

### Adapter Layer - Data (New)

```
internal/adapters/data/sftp_client/
  - sftpClient.go — Wraps pkg/sftp connection
  - Uses system SSH binary via NewClientPipe()
  - Connection pooling per server
  - Automatic cleanup on disconnect
```

```
internal/adapters/data/local_fs/
  - localFileSystem.go — Local file system operations
  - Directory listing, file info
  - Uses os package directly
```

### Adapter Layer - UI (New Components)

```
internal/adapters/ui/file_browser/
  - file_browser.go — Main dual-pane layout (tview.Flex)
  - local_pane.go — Left pane for local files (tview.Table)
  - remote_pane.go — Right pane for remote files (tview.Table)
  - transfer_bar.go — Progress display at bottom
  - transfer_dialog.go — Conflict resolution modal
  - file_browser_handlers.go — Keyboard input handling
```

## Integration Points with Existing Code

**Minimal changes required (4 files):**

| File | Change |
|------|--------|
| `internal/adapters/ui/handlers.go` | Add `case 'F'` for file transfer entry |
| `internal/adapters/ui/app.go` | Wire file browser component |
| `cmd/main.go` | Inject FileTransferService dependency |
| `internal/adapters/ui/status_bar.go` | Add transfer status display |

**All other changes are new files only.**

## Data Flow

### Opening File Browser
1. User selects server in list, presses `F`
2. Handler calls `FileTransferService.OpenBrowser(server)`
3. Service creates SFTP connection via system SSH pipe
4. UI renders dual-pane layout
5. Left pane: local home dir, Right pane: remote home dir

### Browsing Remote Directory
1. User navigates in remote pane (Enter/arrow keys)
2. Pane requests `ListRemoteDir(server, path)` from service
3. Service calls SFTP client ReadDir()
4. Results rendered in tview.Table

### Uploading File
1. User selects local file, presses Enter (or transfer key)
2. Handler calls `FileTransferService.Upload(server, src, dst)`
3. Service checks destination existence → conflict dialog if needed
4. Service initiates SFTP Put with progress callback
5. UI updates progress bar via callback
6. On complete, remote pane refreshes

### Canceling Transfer
1. User presses Esc/Ctrl+C during transfer
2. Handler calls `FileTransferService.Cancel(taskID)`
3. Service signals cancel to SFTP operation
4. UI shows "Canceled" status, partial file cleaned up

## Build Order (Dependencies)

1. **Ports & Types** — FileInfo, TransferProgress interfaces (no dependencies)
2. **Local FS Adapter** — Simple os wrapper (no dependencies)
3. **SFTP Client Adapter** — pkg/sftp wrapper (needs pkg/sftp dependency)
4. **File Transfer Service** — Orchestrates adapters (needs ports + adapters)
5. **UI Local Pane** — tview.Table for local files (needs ports)
6. **UI Remote Pane** — tview.Table for remote files (needs service)
7. **UI Integration** — Dual-pane layout, handlers, progress, conflict dialog

## Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| pkg/sftp pipe connection fails | MEDIUM | Fallback to sftp batch mode |
| Progress callback blocks UI | HIGH | Use goroutine + tview.Application.QueueUpdateDraw() |
| Large directory listing slow | MEDIUM | Pagination/lazy loading |
| SFTP connection timeout | MEDIUM | Configurable timeout, retry logic |

---
*Architecture research: 2026-04-13*
