---
phase: 01-foundation
plan: 01
subsystem: data-access
tags: [sftp, filesystem, clean-architecture, domain, ports, adapters]

# Dependency graph
requires: []
provides:
  - FileInfo domain entity (single source of truth for file listing data)
  - FileService port interface (contract for local/remote file listing)
  - SFTPService port interface (contract for SFTP connection lifecycle)
  - LocalFS adapter (local filesystem listing with sorting/filtering)
  - SFTPClient adapter (remote SFTP listing via system SSH binary)
  - buildSSHArgs utility (reusable SSH argument construction)
affects: [01-02-file-browser-ui, 01-03-transfer]

# Tech tracking
tech-stack:
  added: [github.com/pkg/sftp v1.13.10]
  patterns:
    - "Port/Adapter pattern for file operations (FileService/SFTPService interfaces)"
    - "Directories-first sorting partition (dirs sorted separately from files)"
    - "SSH argument builder duplication to avoid circular imports"

key-files:
  created:
    - internal/core/domain/file_info.go
    - internal/core/domain/file_info_test.go
    - internal/core/ports/file_service.go
    - internal/core/ports/file_service_test.go
    - internal/adapters/data/local_fs/local_fs.go
    - internal/adapters/data/local_fs/local_fs_test.go
    - internal/adapters/data/sftp_client/sftp_client.go
    - internal/adapters/data/sftp_client/sftp_client_test.go
    - internal/adapters/data/sftp_client/ssh_args.go
  modified:
    - go.mod (added pkg/sftp dependency)
    - go.sum (added pkg/sftp checksums)

key-decisions:
  - "Duplicate SSH arg builders in sftp_client/ssh_args.go to avoid circular import with adapters/ui"
  - "Use io.WriteCloser field for stdin pipe to enable explicit close in cleanup"
  - "Return empty slice (not nil) from ListDir for empty directories"
  - "SFTP sort logic duplicated from LocalFS rather than extracted to shared package (no shared package needed yet)"

patterns-established:
  - "Pattern: FileInfo as universal file listing data type across local and remote"
  - "Pattern: FileService interface for local, SFTPService extends it with connection lifecycle"
  - "Pattern: dirs-first sort partition (split dirs/files, sort each, concatenate)"

requirements-completed: [BROW-01, BROW-03, BROW-04, BROW-05, BROW-06, INTG-01, INTG-02]

# Metrics
duration: 6min
completed: 2026-04-13
---

# Phase 1 Plan 1: Domain, Ports, and Data Adapters Summary

**FileInfo domain entity, FileService/SFTPService port interfaces, LocalFS and SFTP client adapters with sorting and hidden file filtering**

## Performance

- **Duration:** 6 min
- **Started:** 2026-04-13T03:21:12Z
- **Completed:** 2026-04-13T03:27:54Z
- **Tasks:** 3
- **Files modified:** 9

## Accomplishments
- FileInfo domain entity with 6 fields (Name, Size, Mode, ModTime, IsDir, IsSymlink) and FileSortField type
- FileService and SFTPService port interfaces defining clean contracts for UI layer dependency
- LocalFS adapter with 9 passing tests covering filtering, sorting, symlinks, and error handling
- SFTPClient adapter using pkg/sftp NewClientPipe per D-09, with mutex-protected state and proper cleanup
- buildSSHArgs extracted from BuildSSHCommand to return []string for exec.Command usage

## Task Commits

Each task was committed atomically:

1. **Task 1: Create FileInfo domain entity and FileService port interface** - `a8734a2` (feat)
2. **Task 2: Create LocalFS adapter with sorting and hidden file filtering** - `27020ac` (feat)
3. **Task 3: Create SFTP client adapter and extract buildSSHArgs** - `ac65ae8` (feat)

**Plan metadata:** (pending final docs commit)

_Note: TDD tasks had test+feat commits combined (test verified RED, implementation confirmed GREEN in same commit for efficiency)._

## Files Created/Modified
- `internal/core/domain/file_info.go` - FileInfo struct and FileSortField constants
- `internal/core/domain/file_info_test.go` - Domain entity verification tests
- `internal/core/ports/file_service.go` - FileService and SFTPService interfaces
- `internal/core/ports/file_service_test.go` - Interface compilation verification tests
- `internal/adapters/data/local_fs/local_fs.go` - Local filesystem adapter with sorting
- `internal/adapters/data/local_fs/local_fs_test.go` - 9 tests for local file operations
- `internal/adapters/data/sftp_client/sftp_client.go` - SFTP client via NewClientPipe
- `internal/adapters/data/sftp_client/sftp_client_test.go` - 12 tests for arg construction and state
- `internal/adapters/data/sftp_client/ssh_args.go` - SSH argument builders (duplicated from utils.go)
- `go.mod` - Added github.com/pkg/sftp v1.13.10 dependency
- `go.sum` - Updated dependency checksums

## Decisions Made
- **SSH arg builder duplication:** Duplicated add*Options functions from `adapters/ui/utils.go` into `sftp_client/ssh_args.go` with a file-level comment explaining the intentional duplication to avoid circular imports (`adapters/data` cannot import `adapters/ui`).
- **SFTP stdin stored as io.WriteCloser:** The stdin pipe is stored in the SFTPClient struct to enable explicit close during cleanup, preventing resource leaks.
- **Empty slice vs nil:** ListDir returns `make([]domain.FileInfo, 0, ...)` instead of nil for empty directories, ensuring callers never need nil checks.
- **Sorting logic duplication:** Sort functions are duplicated between LocalFS and SFTPClient rather than extracted to a shared package. This is acceptable since the logic is simple and there are only two consumers. If a third consumer appears, extraction would be warranted.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None - all tasks completed without issues.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Domain types (FileInfo) and port interfaces (FileService, SFTPService) are ready for UI layer consumption
- LocalFS adapter can be injected into the file browser UI for local file browsing
- SFTPClient adapter can be injected for remote file browsing via SFTP
- Plan 01-02 (file browser UI) can depend on these interfaces without circular dependencies
- No blockers identified

## Self-Check: PASSED

All created files verified present. All commits verified in git log.

---
*Phase: 01-foundation*
*Completed: 2026-04-13*
