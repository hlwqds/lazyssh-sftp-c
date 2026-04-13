# Technology Stack

**Analysis Date:** 2025-04-13

## Languages

**Primary:**
- Go 1.24.6 - Core application

## Runtime

**Environment:**
- Go runtime 1.24.6
- Cross-platform compilation (Linux, Windows, Darwin)
- Native binary execution

**Package Manager:**
- Go modules (go.mod)
- Cargo.lock present (committed as go.sum)

## Frameworks

**Core:**
- TUI (Terminal User Interface) - Built with tview and tcell
- Cobra CLI framework - Command-line argument parsing
- Custom SSH config parser - Forked from kevinburke/ssh_config

**Testing:**
- Go testing framework - Built-in unit tests
- Go benchmarking - Performance testing

**Build/Dev:**
- GoReleaser - Cross-platform binary packaging
- golangci-lint - Code linting and quality checks
- staticcheck - Static analysis
- gofumpt - Code formatter

## Key Dependencies

**Critical:**
- tview v0.0.0 - TUI framework for terminal UI
- tcell/v2 v2.9.0 - Terminal cell manipulation library
- cobra v1.9.1 - CLI argument parsing and command structure
- zap v1.27.0 - High-performance structured logging
- ssh_config v1.4.0 (forked) - SSH configuration parsing

**Infrastructure:**
- atotto/clipboard v0.1.4 - Clipboard integration
- mattn/go-runewidth v0.0.16 - Text width calculation for Unicode
- rivo/uniseg v0.4.7 - Unicode text segmentation

## Configuration

**Environment:**
- No environment variables required
- Configuration loaded from `~/.ssh/config`
- User home directory resolved at runtime

**Build:**
- Go build tags - None specified
- LDFLAGS for version injection
- Cross-compilation for multiple platforms (amd64, arm, arm64, 386)

## Platform Requirements

**Development:**
- Go 1.22+ (minimum for GitHub Actions)
- Go 1.24.6 (current version)
- golangci-lint, gofumpt, staticcheck (development tools)

**Production:**
- Any platform supporting compiled Go binaries
- No runtime dependencies (static binaries with minimal libc requirements)
- OpenSSH client required for SSH functionality
- Standard terminal environment

---

*Stack analysis: 2025-04-13*
```