---
status: complete
phase: 07-copy-clipboard
source: [07-VERIFICATION.md]
started: 2026-04-15T14:40:00Z
updated: 2026-04-15T14:55:18Z
---

## Current Test

[testing complete]

## Tests

### 1. Local copy workflow
expected: |
  Press `c` on a file in local pane. Verify green `[C]` prefix appears on the file name with dark background, visible without cursor being on it. Status bar shows clipboard feedback. Navigate to another directory and back — the `[C]` prefix should still be visible on the file.
result: pass

### 2. Local paste workflow
expected: |
  After `c` on a file, press `p` in the same pane. Verify file is copied with `.1` suffix. Original file unchanged.
result: pass

### 3. Remote paste (copy to remote)
expected: |
  After `c` on a local file, switch to remote pane with Tab, press `p`. Verify TransferModal appears with "Copying: filename" title, shows download then upload progress with percentage that updates. On completion, file appears in remote pane.
result: pass

### 4. Esc clipboard clearing
expected: |
  After `c` marks a file, press Esc. Verify clipboard is cleared (no `[C]` prefix), status bar shows "Clipboard cleared", cursor stays on the same file. Press Esc again — file browser closes.
result: pass

### 5. Cross-pane rejection
expected: |
  After `c` on a local file, switch to remote pane, press `p`. Verify "Cross-pane paste not supported (v1.3+)" error in status bar. No transfer starts.
result: pass

### 6. Conflict dialog on download
expected: |
  In remote pane, press Enter on a file whose name also exists in the local pane's current directory. Verify conflict dialog appears with "File already exists:" header, file info, and [o] Overwrite [s] Skip [r] Rename options. Press `s` to skip — file is not downloaded. Press Esc — also skips.
result: pass

### 7. Transfer progress percentage
expected: |
  Download a file (press Enter on remote file, or use F5 for directory). Verify progress bar fills and percentage number updates (e.g. 12%, 45%, 87%). Speed and ETA also display.
result: pass

### 8. Transfer cancel cleanup
expected: |
  Start a file transfer. Press Esc to show cancel confirm. Press Esc again to confirm cancel. Verify "Transfer canceled" summary appears. Verify no partial/temp file remains at the destination (no `.lazyssh.tmp` file).
result: pass

### 9. Empty clipboard paste
expected: |
  Without pressing `c` first, press `p`. Verify nothing happens (silent no-op).
result: [pending]

### 10. Status bar hints
expected: |
  Verify status bar shows `c Copy` and `p Paste` in default hints. During transfer, verify status bar is hidden (modal covers it).
result: pass

## Summary

total: 10
passed: 10
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

