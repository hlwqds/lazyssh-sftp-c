# Milestones

## v1.1 Recent Remote Directories (Shipped: 2026-04-14)

**Phases completed:** 2 phases, 3 plans, 5 tasks

**Key accomplishments:**

- In-memory MRU directory list with move-to-front dedup, relative path filtering, and 10-entry cap -- zero new dependencies
- NavigateToParent onPathChange callback fix, NavigateTo silent navigation method, and RecentDirs.Record() wiring through OnPathChange callback chain
- Centered popup overlay with j/k navigation, current-path yellow highlighting, and TransferModal.Draw() rendering bug fix

---

## v1.0 File Transfer (Shipped: 2026-04-13)

**Phases completed:** 3 phases, 9 plans, 24 tasks

**Key accomplishments:**

- FileInfo domain entity, FileService/SFTPService port interfaces, LocalFS and SFTP client adapters with sorting and hidden file filtering
- Dual-pane file browser with tview.Table, keyboard navigation (Tab/Esc/h/Space/./s/S), SFTP connection lifecycle, and 4-column file display
- File browser integration via F key entry point with constructor-based dependency injection for LocalFS and SFTPClient
- TransferProgress domain type, TransferService port with Upload/Download methods, SFTPClient remote I/O extensions (CreateRemoteFile, OpenRemoteFile, MkdirAll, WalkDir), and TransferService implementation with 32KB buffered progress-tracked file copying
- ProgressBar with Unicode block characters and TransferModal overlay with sliding-window speed/ETA calculation
- Enter/F5 keyboard-driven file and directory transfers through dual-pane file browser with progress modal overlay
- context.Context cancellation propagation with double-Esc confirmation UI and TransferModal multi-mode state machine
- SFTPService Stat/Remove with per-file conflict detection (Overwrite/Skip/Rename), D-04 partial file cleanup on cancel, and buffered-channel goroutine synchronization for conflict dialog UI
- Platform-separated file permission setting via Go build tags (os.Chmod on Unix, no-op on Windows), called after successful downloads with 0o644 standard mode

---
