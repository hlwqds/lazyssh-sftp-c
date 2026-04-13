# Phase 3: Polish - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-13
**Phase:** 3-polish
**Areas discussed:** Cancel mechanism, Conflict resolution, Cross-platform scope, Partial file cleanup

---

## Cancel mechanism

| Option | Description | Selected |
|--------|-------------|----------|
| context.Context (Recommended) | Add context.Context to all TransferService methods. Standard Go pattern, clean propagation. | ✓ |
| Cancel channel | Pass a cancel channel to transfer methods. Simpler but less idiomatic. | |
| Connection close | Close the SFTP connection to force-stop. Most aggressive. | |

**User's choice:** context.Context
**Notes:** Standard Go cancellation pattern. Will need to update TransferService port interface and all implementations.

| Option | Description | Selected |
|--------|-------------|----------|
| Confirm on Esc (Recommended) | First Esc shows "Cancel transfer? (y/n)", second Esc confirms. | ✓ |
| Immediate cancel | Esc immediately cancels with no confirmation. | |
| Button in modal | Esc shows a cancel button in the modal that must be selected. | |

**User's choice:** Confirm on Esc
**Notes:** Prevents accidental cancels during intense use. Two-step: prompt then confirm.

---

## Conflict resolution

| Option | Description | Selected |
|--------|-------------|----------|
| Per-file prompt (Recommended) | Show a prompt before each conflicting file: Overwrite / Skip / Rename. | ✓ |
| Apply-all choice | Ask once at the start: Overwrite all / Skip all / Rename all. | |
| Auto-rename always | Default to rename with suffix, never overwrite. | |

**User's choice:** Per-file prompt
**Notes:** Most control. User decides per-file during directory transfers.

| Option | Description | Selected |
|--------|-------------|----------|
| Modal conflict dialog (Recommended) | Pause transfer, show conflict dialog in the modal area. | ✓ |
| Status bar prompt | Show conflict options as status bar message with key shortcuts. | |
| Separate conflict view | Pause transfer, switch to a separate full-screen conflict view. | |

**User's choice:** Modal conflict dialog
**Notes:** Reuses TransferModal area, doesn't create a jarring context switch.

---

## Cross-platform scope

All four aspects selected: Path handling, Symlink handling, File permissions, Display formatting.

| Option | Description | Selected |
|--------|-------------|----------|
| Build tags + stdlib (Recommended) | Platform-specific files with build tags (file_windows.go, file_unix.go) and filepath.Join throughout. | ✓ |
| Runtime checks | Use runtime.GOOS checks at each call site. | |
| Platform abstraction layer | Abstract platform differences behind an interface. | |

**User's choice:** Build tags + stdlib
**Notes:** Go-idiomatic approach. filepath package handles most path differences.

---

## Partial file cleanup

| Option | Description | Selected |
|--------|-------------|----------|
| Always clean up (Recommended) | Always delete partial files after cancel. | ✓ |
| Ask after cancel | Show prompt after cancel: "Delete partial file? (y/n)". | |
| Leave with marker | Leave partial files with recognizable name (.lazyssh-partial). | |

**User's choice:** Always clean up
**Notes:** Simplest and cleanest. User never has orphaned half-files.

---

## Claude's Discretion

Areas where user deferred to Claude:
- context.Context timeout values
- Rename suffix format (.1, .2 vs _copy vs timestamp)
- Conflict dialog exact layout and colors
- Symlink detection implementation details
- filepath.ToSlash call locations
- File permission failure log level (warn vs debug)

## Deferred Ideas

None — discussion stayed within phase scope
