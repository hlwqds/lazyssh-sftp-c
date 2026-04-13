# Architecture

**Analysis Date:** 2025-04-13

## Pattern Overview

**Overall:** Clean Architecture with Hexagonal Design

**Key Characteristics:**
- Domain-Driven Design layers (Domain, Ports, Adapters)
- Clean separation of concerns between business logic and external dependencies
- Dependency inversion through ports and adapters pattern
- Terminal User Interface (TUI) built on tview/tcell
- SSH configuration parsing and management as core domain

## Layers

**Entry Point Layer:**
- Purpose: Handle application startup and orchestrate all components
- Location: `cmd/main.go`
- Contains: Cobra CLI setup, logger initialization, dependency injection
- Depends on: UI layer, Services layer, Logging
- Used by: External execution (binary)

**Domain Layer:**
- Purpose: Core business logic and entities independent of external concerns
- Location: `internal/core/domain/`
- Contains: `Server` entity with all SSH configuration options
- Depends on: Nothing (pure domain)
- Used by: Services layer via interfaces

**Ports Layer:**
- Purpose: Define interfaces for communicating with external systems
- Location: `internal/core/ports/`
- Contains: `ServerService` and `ServerRepository` interfaces
- Depends on: Domain types
- Used by: Services layer (implements) and Adapters layer (uses)

**Services Layer:**
- Purpose: Implement business logic between domain and adapters
- Location: `internal/core/services/`
- Contains: `serverService` with all business operations
- Depends on: Ports interfaces
- Used by: Entry point (via dependency injection) and UI layer

**Adapter Layer - Data:**
- Purpose: Handle external data persistence and SSH config operations
- Location: `internal/adapters/data/`
- Contains: SSH config file parsing, backup management, metadata tracking
- Depends on: Ports interfaces, External libraries
- Used by: Services layer

**Adapter Layer - UI:**
- Purpose: Handle terminal user interface presentation and interaction
- Location: `internal/adapters/ui/`
- Contains: TUI components, event handling, keyboard shortcuts
- Depends on: Ports interfaces, tview/tcell libraries
- Used by: Entry point and user interactions

## Data Flow

**Application Startup:**
1. `cmd/main.go` initializes logger and finds config paths
2. Creates SSH config repository with metadata manager
3. Creates server service with injected repository
4. Creates TUI with injected service
5. Runs Cobra command to start TUI

**Server Listing:**
1. TUI requests server list via `ServerService.ListServers()`
2. Service calls repository `ListServers()` with query filter
3. Repository parses SSH config file, filters servers, and returns domain objects
4. Service sorts servers (pinned first, then by name)
5. TUI displays server list with status indicators

**Server Management:**
1. User selects server for edit/add
2. TUI displays form with validation
3. User submits changes
4. TUI calls appropriate service method (AddServer/UpdateServer)
5. Service validates and calls repository
6. Repository performs atomic file write with backup
7. Metadata updated (last seen, SSH count)

**SSH Connection:**
1. User presses Enter on server
2. TUI calls `ServerService.SSH()`
3. Service records SSH usage via repository
4. Service executes system SSH binary with proper arguments
5. Process forked to maintain TUI responsiveness

## Key Abstractions

**Server Domain Entity:**
- Purpose: Represent SSH configuration as a rich domain object
- Examples: `internal/core/domain/server.go` (117 fields covering all SSH options)
- Pattern: Comprehensive entity with all possible SSH configuration options
- Fields include: basic connection details, proxy settings, forwarding, authentication, security, debugging

**SSH Config Repository:**
- Purpose: Abstract file operations and persistence
- Examples: `internal/adapters/data/ssh_config_file/ssh_config_file_repo.go`
- Pattern: Repository pattern with interface separation
- Uses modified ssh_config library for parsing while preserving comments

**Server Service:**
- Purpose: Core business logic orchestrating all operations
- Examples: `internal/core/services/server_service.go`
- Pattern: Service layer with rich business logic
- Handles sorting, validation, SSH execution, port forwarding

**TUI Components:**
- Purpose: Terminal UI with clear separation of concerns
- Examples: `internal/adapters/ui/server_list.go`, `server_form.go`, `server_details.go`
- Pattern: Component-based architecture with composition
- Each UI component manages its own state and events

## Entry Points

**Main Application:**
- Location: `cmd/main.go`
- Triggers: Binary execution via `lazyssh` command
- Responsibilities: Initialize all components, inject dependencies, start TUI
- Key flow: Logger → Repository → Service → TUI → Run

**HTTP API Entry Point:**
- Not applicable - this is a terminal-only application

**CLI Commands:**
- Single command: `lazyssh`
- Arguments: Currently none documented, supports CLI options via Cobra

## Error Handling

**Strategy:** Comprehensive error handling at all layers with structured logging

**Patterns:**
- Services return errors wrapped with context via `fmt.Errorf()`
- Repositories handle file system errors with descriptive messages
- UI layer displays user-friendly error messages in status bar
- All errors logged via Zap logger with structured fields
- Backup system prevents data loss on write failures

## Cross-Cutting Concerns

**Logging:**
- Zap logger initialized at startup
- Structured logging with contextual fields
- Different log levels for different components

**Configuration:**
- SSH config path discovery via `~/.ssh/config`
- Metadata stored in `~/.lazyssh/metadata.json`
- Environment detection (Unix/Windows) for system behaviors

**Backup Management:**
- Atomic writes with temporary files
- Original backup on first change
- Timestamped rolling backups (max 10)
- Non-destructive editing preserves comments and formatting

**Validation:**
- Server entity validation in service layer
- Field-level validation in UI components
- SSH command argument validation before execution

---

*Architecture analysis: 2025-04-13*