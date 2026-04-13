# Pitfalls Research

**Analysis Date:** 2026-04-13
**Domain:** TUI File Transfer for Go SSH Manager

## Critical Pitfalls

### P1: SCP Non-TTY Produces Zero Progress
- **Severity:** CRITICAL
- **Description:** `exec.Command` does not allocate a PTY, so OpenSSH's `scp` completely suppresses its progress bar. The "detailed progress display" requirement is impossible with raw scp in non-TTY mode.
- **Warning signs:** Progress bar shows 0% or no output during transfer
- **Prevention:** Use `pkg/sftp` which provides programmatic progress callbacks. If using scp, count files and show "file N of M" progress instead.
- **Phase:** Phase 2 (Transfer execution)

### P2: SFTP `ls` Output Parsing is Fundamentally Fragile
- **Severity:** HIGH
- **Description:** Filenames with spaces, Unicode, or newlines break all naive parsing approaches (`strings.Fields()`, `fields[8]`, `awk`).
- **Warning signs:** File names displayed incorrectly, crash on special characters
- **Prevention:** Use `pkg/sftp` ReadDir() which returns structured `os.FileInfo` — no parsing needed. This is the strongest argument for pkg/sftp over batch mode.
- **Phase:** Phase 2 (Remote browsing)

### P3: Each `sftp -b -` Call Creates New SSH Connection
- **Severity:** HIGH
- **Description:** Directory browsing will feel sluggish (0.5-3s per operation) unless the user has SSH ControlMaster enabled.
- **Warning signs:** Noticeable lag when navigating directories
- **Prevention:** Use `pkg/sftp` with a persistent connection. Cache directory listings. Lazy-load directory contents.
- **Phase:** Phase 2 (Remote browsing)

### P4: `f` Key Already Bound to Port Forwarding
- **Severity:** HIGH
- **Description:** `handlers.go` binds `f` to `handlePortForward()`. Cannot use `f` for file transfer without conflict.
- **Warning signs:** File transfer key triggers port forwarding instead
- **Prevention:** Use `F` (Shift+f) for file transfer. Update key bindings documentation.
- **Phase:** Phase 1 (UI foundation)

## High Pitfalls

### P5: Progress Callback Blocks UI Thread
- **Severity:** HIGH
- **Description:** tview is single-threaded. Progress callbacks from SFTP operations must use `tview.Application.QueueUpdateDraw()` to avoid freezing the UI.
- **Warning signs:** UI becomes unresponsive during transfer
- **Prevention:** All SFTP operations in goroutines. All UI updates via QueueUpdateDraw.
- **Phase:** Phase 2 (Transfer execution)

### P6: Windows OpenSSH Behavioral Differences
- **Severity:** MEDIUM
- **Description:** Windows OpenSSH has subtle differences from Linux/macOS: path separators, symlink handling, permissions.
- **Warning signs:** File transfer fails on Windows but works on Linux
- **Prevention:** Test all operations on Windows. Use `filepath` package for path handling. Abstract platform-specific behavior.
- **Phase:** Phase 3 (Cross-platform polish)

### P7: File Conflict Detection Race Condition
- **Severity:** MEDIUM
- **Description:** Between checking file existence and starting transfer, another process may create/delete the file.
- **Warning signs:** Unexpected overwrite or "file not found" errors
- **Prevention:** Accept TOCTOU as inherent limitation. Use SFTP atomic rename if available. Document limitation.
- **Phase:** Phase 3 (Conflict handling)

## Medium Pitfalls

### P8: Large Directory Listing Performance
- **Severity:** MEDIUM
- **Description:** Listing directories with thousands of files causes UI lag and high memory usage.
- **Warning signs:** Slow rendering when entering large directories
- **Prevention:** Paginate directory listings (show first 200 entries). Lazy-load on scroll. Show count of total entries.
- **Phase:** Phase 2 (Remote browsing)

### P9: SFTP Connection Not Properly Closed
- **Severity:** MEDIUM
- **Description:** Leaving SFTP connections open when closing file browser or switching servers leaks resources.
- **Warning signs:** Increasing number of SSH processes over time
- **Prevention:** Implement cleanup in file browser close handler. Use `defer` for connection lifecycle. Track active connections.
- **Phase:** Phase 1 (Connection management)

### P10: Transfer Cancel Leaves Partial Files
- **Severity:** MEDIUM
- **Description:** Canceling a transfer mid-way leaves partial files on destination.
- **Warning signs:** Partial files with wrong sizes after canceled transfer
- **Prevention:** Delete partial files on cancel. Show confirmation dialog. Log cleanup actions.
- **Phase:** Phase 3 (Cancel support)

## Low Pitfalls

### P11: Unicode Filename Display Width
- **Severity:** LOW
- **Description:** CJK characters, emoji, and combining characters have display widths different from their byte length.
- **Warning signs:** Column misalignment in file browser
- **Prevention:** Use `mattn/go-runewidth` (already a dependency) for display width calculation.
- **Phase:** Phase 1 (UI foundation)

### P12: Symlink Handling
- **Severity:** LOW
- **Description:** Symlinks to directories vs files, broken symlinks, symlink loops.
- **Warning signs:** Crash or infinite loop on symlink directories
- **Prevention:** Detect symlinks explicitly. Show symlink indicator (→). Don't follow symlink loops. Use `os.Lstat()` for local, SFTP `Lstat()` for remote.
- **Phase:** Phase 2 (Remote browsing)

### P13: Permission Denied on Remote
- **Severity:** LOW
- **Description:** User may not have write permission to remote directory.
- **Warning signs:** Transfer fails with permission error
- **Prevention:** Catch permission errors, display clear message suggesting checking permissions.
- **Phase:** Phase 2 (Transfer execution)

### P14: Disk Space Check Missing
- **Severity:** LOW
- **Description:** Transfer starts without checking available disk space.
- **Warning signs:** Transfer fails halfway due to disk full
- **Prevention:** Optional: check available space before transfer for large files. Not critical for v1.
- **Phase:** Phase 3 (Polish)

## Summary

| Severity | Count | Key Theme |
|----------|-------|-----------|
| CRITICAL | 1 | Progress tracking impossible with raw scp |
| HIGH | 3 | Text parsing fragile, connection overhead, key conflict |
| MEDIUM | 3 | UI threading, Windows compat, race conditions |
| LOW | 4 | Unicode, symlinks, permissions, disk space |

**Strongest recommendation:** Use `pkg/sftp` to eliminate P1, P2, and P3 simultaneously. This single decision prevents the three most critical pitfalls.

---
*Pitfalls research: 2026-04-13*
