<!-- GSD:project-start source:PROJECT.md -->
## Project

**LazySSH File Transfer**

为 lazyssh（终端 SSH 管理器）添加内置的双栏文件传输功能。用户在服务器列表中选中服务器后，按快捷键打开双栏文件浏览器（本地 vs 远程），支持上传/下载文件和目录，提供详细的传输进度显示。底层复用系统 SCP/SFTP 命令，保持 lazyssh "不引入新安全风险" 的原则。

**Core Value:** 在终端内完成 SSH 文件传输，无需切换到 FileZilla 或记忆 scp 命令——选中服务器、选文件、传输，全部键盘驱动。

### Constraints

- **安全原则**: 不引入新的安全风险，复用系统 scp/sftp 命令，不存储/传输/修改密钥
- **跨平台**: 必须在 Linux/Windows/Darwin 上正常工作
- **架构一致**: 遵循现有 Clean Architecture 模式，通过 Port/Adapter 解耦
- **UI 框架**: 基于 tview/tcell 构建，不可引入其他 UI 框架
- **零外部依赖**: 不引入需要额外安装的依赖，sc/sftp 必须是系统自带的
<!-- GSD:project-end -->

<!-- GSD:stack-start source:codebase/STACK.md -->
## Technology Stack

## Languages
- Go 1.24.6 - Core application
## Runtime
- Go runtime 1.24.6
- Cross-platform compilation (Linux, Windows, Darwin)
- Native binary execution
- Go modules (go.mod)
- Cargo.lock present (committed as go.sum)
## Frameworks
- TUI (Terminal User Interface) - Built with tview and tcell
- Cobra CLI framework - Command-line argument parsing
- Custom SSH config parser - Forked from kevinburke/ssh_config
- Go testing framework - Built-in unit tests
- Go benchmarking - Performance testing
- GoReleaser - Cross-platform binary packaging
- golangci-lint - Code linting and quality checks
- staticcheck - Static analysis
- gofumpt - Code formatter
## Key Dependencies
- tview v0.0.0 - TUI framework for terminal UI
- tcell/v2 v2.9.0 - Terminal cell manipulation library
- cobra v1.9.1 - CLI argument parsing and command structure
- zap v1.27.0 - High-performance structured logging
- ssh_config v1.4.0 (forked) - SSH configuration parsing
- atotto/clipboard v0.1.4 - Clipboard integration
- mattn/go-runewidth v0.0.16 - Text width calculation for Unicode
- rivo/uniseg v0.4.7 - Unicode text segmentation
## Configuration
- No environment variables required
- Configuration loaded from `~/.ssh/config`
- User home directory resolved at runtime
- Go build tags - None specified
- LDFLAGS for version injection
- Cross-compilation for multiple platforms (amd64, arm, arm64, 386)
## Platform Requirements
- Go 1.22+ (minimum for GitHub Actions)
- Go 1.24.6 (current version)
- golangci-lint, gofumpt, staticcheck (development tools)
- Any platform supporting compiled Go binaries
- No runtime dependencies (static binaries with minimal libc requirements)
- OpenSSH client required for SSH functionality
- Standard terminal environment
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

## Language & Edition
- **Go Version:** 1.24.6
- **Standard:** Go 1.x compatible
- **Format:** Official Go formatting
## Naming Patterns
- **Source files:** `snake_case.go` (e.g., `crud.go`, `handlers.go`, `logger.go`)
- **Test files:** `snake_case_test.go` (e.g., `crud_test.go`, `validation_test.go`)
- **Package directories:** `lowercase` (e.g., `ssh_config_file`, `domain`)
- **Public:** `PascalCase` (e.g., `NewRepository`, `BuildSSHCommand`, `validateHost`)
- **Private:** `camelCase` (e.g., `convertCLIForwardToConfigFormat`, `handleGlobalKeys`, `setFieldValues`)
- **Test functions:** `TestCamelCase` or `Test_underscore_case` (e.g., `TestValidateHost`, `TestBuildSSHCommand_PortForwarding`)
- **Public:** `PascalCase` (e.g., `Server`, `Repository`, `TUI`)
- **Private:** `camelCase` (e.g., `serverRepo`, `log`, `fieldValidators`)
- **Constants:** `SCREAMING_SNAKE_CASE` (e.g., `ForwardTypeLocal`, `ForwardModeOnlyForward`, `AppName`)
- **Structs:** `PascalCase` (e.g., `Server`, `ValidationState`, `FieldValidator`)
- **Interfaces:** `PascalCase` with "er" suffix (e.g., `tview.Primitive`, `tcell.EventHandler`)
- **Custom errors:** `*PascalError` (e.g., `*testError` in tests)
## Code Style
- **Tool:** `gofmt` (used by `go fmt` command)
- **Import organization:** Standard Go import grouping
- **Line length:** No hard limit but long lines wrapped appropriately
- **Braces:** Always on same line as statement (K&R style)
- **Primary tool:** `golangci-lint`
- **Configuration:** `.golangci.yml`
- **Key rules enforced:**
## Import Organization
## Error Handling
- **Error return:** Always return `error` as last return parameter
- **Error checking:** Immediate after function call
- **Error logging:** `log.Errorw()` with structured fields
- **Error propagation:** Return errors to caller with context
- Used selectively with justification comments
- Common cases:
## Logging
- **Info logging:** `log.Infow()` with structured fields
- **Error logging:** `log.Errorw()` with structured fields
- **Debug logging:** Not explicitly configured but available
- **Sugar logger:** Used for convenience (`.Sugar()`)
## Comments
- Function documentation through code structure
- Important logic explained in comments
- nolint comments include justification
## Function Design
- Small functions for specific operations
- Medium functions for complex operations
- Large functions avoided through decomposition
- Typically 2-5 parameters
- Use structs for related parameters (e.g., `domain.Server`)
- Optional parameters use `string`/`int` with zero values
- Primary value + error pattern
- Multiple values with error as last parameter
- Options/flags use boolean returns
## Module Design
- Only necessary types marked `pub`
- Internal types private by default
- Interface-first design where appropriate
- `internal/core/domain` - Domain models
- `internal/core/services` - Business logic
- `internal/adapters/data` - Data persistence
- `internal/adapters/ui` - User interface
- `cmd/` - Application entry point
- Clear dependency hierarchy
- Domain layer independent of adapters
- Services depend on domain, not adapters
- Adapters implement interfaces defined in domain/services
## Serialization
- Struct tags for JSON field mapping
- Custom Marshal/Unmarshal methods for complex types
- Error handling for invalid data
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

## Pattern Overview
- Domain-Driven Design layers (Domain, Ports, Adapters)
- Clean separation of concerns between business logic and external dependencies
- Dependency inversion through ports and adapters pattern
- Terminal User Interface (TUI) built on tview/tcell
- SSH configuration parsing and management as core domain
## Layers
- Purpose: Handle application startup and orchestrate all components
- Location: `cmd/main.go`
- Contains: Cobra CLI setup, logger initialization, dependency injection
- Depends on: UI layer, Services layer, Logging
- Used by: External execution (binary)
- Purpose: Core business logic and entities independent of external concerns
- Location: `internal/core/domain/`
- Contains: `Server` entity with all SSH configuration options
- Depends on: Nothing (pure domain)
- Used by: Services layer via interfaces
- Purpose: Define interfaces for communicating with external systems
- Location: `internal/core/ports/`
- Contains: `ServerService` and `ServerRepository` interfaces
- Depends on: Domain types
- Used by: Services layer (implements) and Adapters layer (uses)
- Purpose: Implement business logic between domain and adapters
- Location: `internal/core/services/`
- Contains: `serverService` with all business operations
- Depends on: Ports interfaces
- Used by: Entry point (via dependency injection) and UI layer
- Purpose: Handle external data persistence and SSH config operations
- Location: `internal/adapters/data/`
- Contains: SSH config file parsing, backup management, metadata tracking
- Depends on: Ports interfaces, External libraries
- Used by: Services layer
- Purpose: Handle terminal user interface presentation and interaction
- Location: `internal/adapters/ui/`
- Contains: TUI components, event handling, keyboard shortcuts
- Depends on: Ports interfaces, tview/tcell libraries
- Used by: Entry point and user interactions
## Data Flow
## Key Abstractions
- Purpose: Represent SSH configuration as a rich domain object
- Examples: `internal/core/domain/server.go` (117 fields covering all SSH options)
- Pattern: Comprehensive entity with all possible SSH configuration options
- Fields include: basic connection details, proxy settings, forwarding, authentication, security, debugging
- Purpose: Abstract file operations and persistence
- Examples: `internal/adapters/data/ssh_config_file/ssh_config_file_repo.go`
- Pattern: Repository pattern with interface separation
- Uses modified ssh_config library for parsing while preserving comments
- Purpose: Core business logic orchestrating all operations
- Examples: `internal/core/services/server_service.go`
- Pattern: Service layer with rich business logic
- Handles sorting, validation, SSH execution, port forwarding
- Purpose: Terminal UI with clear separation of concerns
- Examples: `internal/adapters/ui/server_list.go`, `server_form.go`, `server_details.go`
- Pattern: Component-based architecture with composition
- Each UI component manages its own state and events
## Entry Points
- Location: `cmd/main.go`
- Triggers: Binary execution via `lazyssh` command
- Responsibilities: Initialize all components, inject dependencies, start TUI
- Key flow: Logger → Repository → Service → TUI → Run
- Not applicable - this is a terminal-only application
- Single command: `lazyssh`
- Arguments: Currently none documented, supports CLI options via Cobra
## Error Handling
- Services return errors wrapped with context via `fmt.Errorf()`
- Repositories handle file system errors with descriptive messages
- UI layer displays user-friendly error messages in status bar
- All errors logged via Zap logger with structured fields
- Backup system prevents data loss on write failures
## Cross-Cutting Concerns
- Zap logger initialized at startup
- Structured logging with contextual fields
- Different log levels for different components
- SSH config path discovery via `~/.ssh/config`
- Metadata stored in `~/.lazyssh/metadata.json`
- Environment detection (Unix/Windows) for system behaviors
- Atomic writes with temporary files
- Original backup on first change
- Timestamped rolling backups (max 10)
- Non-destructive editing preserves comments and formatting
- Server entity validation in service layer
- Field-level validation in UI components
- SSH command argument validation before execution
<!-- GSD:architecture-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd:quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd:debug` for investigation and bug fixing
- `/gsd:execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->



<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd:profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
