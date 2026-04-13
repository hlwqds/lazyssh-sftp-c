# Technology Stack Research

**Analysis Date:** 2026-04-13
**Domain:** TUI File Transfer for Go SSH Manager

## Existing Stack (No Changes)

| Component | Technology | Version |
|-----------|-----------|---------|
| Language | Go | 1.24.6 |
| TUI Framework | tview/tcell | tview latest, tcell/v2 2.9.0 |
| CLI | Cobra | 1.9.1 |
| Logging | Zap | 1.27.0 |
| SSH Config | ssh_config (forked) | 1.4.0 |

## New Dependencies for File Transfer

### Remote File Operations

| Library | Purpose | Version | Confidence |
|---------|---------|---------|------------|
| `pkg/sftp` | SFTP client via SSH pipe — provides programmatic SFTP API using system SSH binary | latest (v1.13.x) | HIGH |

**Rationale:** PROJECT.md mandates "use system scp/sftp commands" and excludes "Go SSH library." However, research conclusively shows that `os/exec` with raw sftp/scp is fundamentally inadequate for:
- **Programmatic directory listing** — requires parsing unstructured text from sftp batch mode
- **Progress tracking** — scp in non-TTY mode produces zero progress output
- **Connection reuse** — each `sftp -b -` call spawns a new SSH connection (0.5-3s overhead)

**Recommended solution:** `pkg/sftp.NewClientPipe()` — this uses the **system `ssh` binary** via stdin/stdout pipe (preserving the security principle of respecting `~/.ssh/config`, keys, ssh-agent) but provides a proper Go-native SFTP client API for programmatic control. This is NOT introducing a new SSH library — it's a Go wrapper around the system's SSH connection.

**What NOT to use:**
- `golang.org/x/crypto/ssh` directly — introduces its own SSH transport, bypasses user's SSH config
- `schollz/progressbar` — writes directly to terminal, NOT embeddable in tview

### Progress Display

| Approach | Purpose | Confidence |
|----------|---------|------------|
| Custom tview primitive | Progress bar widget embedded in TUI layout | MEDIUM |
| `tview.TextView` with manual rendering | Simpler alternative using formatted text | HIGH |

**Rationale:** tview has no built-in ProgressBar widget. `schollz/progressbar` writes directly to terminal and is NOT embeddable in tview. A custom tview primitive using `tview.Box` as base provides the most control.

### File Transfer Execution

| Tool | Purpose | When to Use |
|------|---------|-------------|
| `pkg/sftp` (Go API) | Directory listing, single file transfer, progress tracking | Primary — browse + transfer |
| `scp` (system command) | Recursive directory transfer (scp -r) | Fallback for batch directory copies |
| `rsync` (system command, if available) | Directory sync with conflict handling | Optional enhancement |

**Rationale:** For v1, `pkg/sftp` handles most operations. `scp -r` can be used for recursive directory uploads/downloads where SFTP walk + put/get is too slow. `rsync` is optional for conflict resolution.

## Key Decision: pkg/sftp

| Aspect | `os/exec` sftp/scp | `pkg/sftp` |
|--------|-------------------|------------|
| Directory listing | Fragile text parsing | Native Go API |
| Progress tracking | Impossible (no TTY) | Built-in |
| Connection reuse | New connection per call | Single connection |
| SSH config respect | Yes (system binary) | Yes (system SSH pipe) |
| Security | System SSH only | System SSH only |
| New dependency | None | pkg/sftp (Go wrapper) |

**Recommendation:** Use `pkg/sftp` for remote operations. It preserves the security principle while providing programmatic control.

## Key Binding Conflict

**CRITICAL:** The `f` key is already bound to `handlePortForward()` in `internal/adapters/ui/handlers.go`.

**Resolution options:**
- `F` (Shift+f) — for file transfer, keeping `f` for port forwarding
- `t` — for transfer
- `Ctrl+F` — for file transfer

**Recommendation:** Use `F` (Shift+f) for file transfer entry point.

---
*Stack research: 2026-04-13*
