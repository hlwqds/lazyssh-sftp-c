---
phase: 07-copy-clipboard
plan: 01
subsystem: data-layer
tags: [copy, copydir, fileservice, transferservice, ports, adapters, sftp]

# Dependency graph
requires:
  - phase: 06-file-operations
    provides: "FileService interface with Remove/RemoveAll/Rename/Mkdir/Stat, TransferService with Upload/Download"
provides:
  - "FileService.Copy/CopyDir for local filesystem copy with permission+mtime preservation"
  - "TransferService.CopyRemoteFile/CopyRemoteDir for remote copy via download+re-upload"
  - "SFTPClient Copy/CopyDir stubs (SFTP protocol limitation)"
affects: [07-02-clipboard-ui]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Remote copy via download+re-upload with temp file/directory cleanup"
    - "SFTP protocol stub methods for unsupported operations"

key-files:
  created: []
  modified:
    - internal/core/ports/file_service.go
    - internal/core/ports/transfer.go
    - internal/adapters/data/local_fs/local_fs.go
    - internal/adapters/data/transfer/transfer_service.go
    - internal/adapters/data/sftp_client/sftp_client.go
    - internal/core/ports/file_service_test.go
    - internal/adapters/data/transfer/transfer_service_test.go

key-decisions:
  - "SFTPClient Copy/CopyDir return errRemoteCopyNotSupported instead of panicking -- SFTP protocol has no native copy, callers must use TransferService"
  - "CopyRemoteFile uses os.CreateTemp + defer os.Remove for temp file cleanup (Pitfall 3)"
  - "CopyRemoteDir uses os.MkdirTemp + defer os.RemoveAll for temp directory cleanup"

patterns-established:
  - "Remote operations unsupported by SFTP protocol: return sentinel error, delegate to TransferService"

requirements-completed: [CPY-02, CPY-03]

# Metrics
duration: 6min
completed: 2026-04-15
---

# Phase 07 Plan 01: Copy/CopyDir Port & Adapter Summary

**Local Copy/CopyDir with permission+mtime preservation (D-07), remote CopyRemoteFile/CopyRemoteDir via download+re-upload (D-01)**

## Performance

- **Duration:** 6 min
- **Started:** 2026-04-15T03:35:36Z
- **Completed:** 2026-04-15T03:41:49Z
- **Tasks:** 2 planned + 1 deviation fix
- **Files modified:** 7

## Accomplishments
- FileService interface extended with Copy and CopyDir method signatures
- LocalFS.Copy implements single-file copy with os.Chmod and os.Chtimes for permission+mtime preservation
- LocalFS.CopyDir implements recursive directory copy reusing Copy method
- TransferService interface extended with CopyRemoteFile and CopyRemoteDir method signatures
- transferService.CopyRemoteFile implements remote file copy via download-to-temp + re-upload with defer cleanup
- transferService.CopyRemoteDir implements remote directory copy via download-to-temp-dir + re-upload with defer cleanup
- SFTPClient stubs added for Copy/CopyDir since SFTP protocol has no native server-side copy

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Copy/CopyDir to FileService port and LocalFS adapter** - `fb32b76` (feat)
2. **Task 2: Add CopyRemoteFile/CopyRemoteDir to TransferService port and adapter** - `fc7f8ce` (feat)
3. **Deviation fix: Add Copy/CopyDir stubs to test mocks** - `8ee13fc` (fix)

## Files Created/Modified
- `internal/core/ports/file_service.go` - Added Copy/CopyDir method signatures to FileService interface
- `internal/core/ports/transfer.go` - Added CopyRemoteFile/CopyRemoteDir method signatures to TransferService interface
- `internal/adapters/data/local_fs/local_fs.go` - Implemented Copy (single file, preserves permissions+mtime) and CopyDir (recursive)
- `internal/adapters/data/transfer/transfer_service.go` - Implemented CopyRemoteFile (download+re-upload with temp file) and CopyRemoteDir (download+re-upload with temp dir)
- `internal/adapters/data/sftp_client/sftp_client.go` - Added Copy/CopyDir stubs returning errRemoteCopyNotSupported
- `internal/core/ports/file_service_test.go` - Added Copy/CopyDir to mockFileService
- `internal/adapters/data/transfer/transfer_service_test.go` - Added Copy/CopyDir to mockSFTPService

## Decisions Made
- SFTPClient Copy/CopyDir return sentinel error `errRemoteCopyNotSupported` rather than attempting any server-side operation -- the SFTP protocol has no native copy command, so callers must route through TransferService
- Remote copy uses temp file/directory in OS temp space with defer cleanup, ensuring no resource leaks even on failure (Pitfall 3 from research)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] SFTPClient missing Copy/CopyDir implementations**
- **Found during:** Task 2 (final `go build ./...` verification)
- **Issue:** SFTPService embeds FileService. Adding Copy/CopyDir to FileService broke SFTPClient compile-time interface check (`var _ ports.SFTPService = (*SFTPClient)(nil)`)
- **Fix:** Added Copy/CopyDir stub methods returning `errRemoteCopyNotSupported` sentinel error. SFTP protocol has no native copy -- callers must use TransferService.CopyRemoteFile/CopyRemoteDir
- **Files modified:** internal/adapters/data/sftp_client/sftp_client.go
- **Verification:** `go build ./...` and `go vet ./...` pass
- **Committed in:** `fc7f8ce` (Task 2 commit)

**2. [Rule 2 - Missing Critical] Test mocks missing Copy/CopyDir implementations**
- **Found during:** Post-task `go vet ./...` verification
- **Issue:** mockFileService and mockSFTPService in test files did not implement new Copy/CopyDir methods, causing vet failures
- **Fix:** Added no-op Copy/CopyDir stubs to both mock structs
- **Files modified:** internal/core/ports/file_service_test.go, internal/adapters/data/transfer/transfer_service_test.go
- **Verification:** `go vet ./...` and `go test ./...` all pass
- **Committed in:** `8ee13fc` (separate fix commit)

---

**Total deviations:** 2 auto-fixed (2 missing critical -- interface satisfaction)
**Impact on plan:** Both auto-fixes were necessary consequences of extending the FileService interface. Adding methods to an interface requires all implementations (including mocks) to provide those methods. No scope creep.

## Issues Encountered
None -- all issues were straightforward interface satisfaction fixes.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Port and adapter layer complete for copy operations
- Plan 02 (clipboard UI) can consume Copy/CopyDir and CopyRemoteFile/CopyRemoteDir through their interfaces
- No blockers for next phase

---
*Phase: 07-copy-clipboard*
*Completed: 2026-04-15*

## Self-Check: PASSED

All files exist, all commits verified, all 14 acceptance criteria met.
