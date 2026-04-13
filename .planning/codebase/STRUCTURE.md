# Codebase Structure

**Analysis Date:** 2025-04-13

## Directory Layout

```
lazyssh/
├── cmd/                          # Application entry point
│   └── main.go                   # Cobra CLI setup and dependency injection
├── internal/                     # Internal application code
│   ├── adapters/                # Implementation of ports interfaces
│   │   ├── data/                # Data persistence and external integrations
│   │   │   └── ssh_config_file/ # SSH config file operations
│   │   │       ├── backup.go           # Backup management logic
│   │   │       ├── config_io.go        # Config file I/O operations
│   │   │       ├── crud.go             # CRUD operations (2287 lines)
│   │   │       ├── file_system.go      # File system abstraction
│   │   │       ├── mapper.go           # SSH config to domain mapping
│   │   │       ├── metadata_manager.go # Usage tracking metadata
│   │   │       └── ssh_config_file_repo.go # Repository implementation
│   │   └── ui/                  # Terminal User Interface components
│   │       ├── const.go          # UI constants and defaults
│   │       ├── defaults.go       # Default values for form fields
│   │       ├── field_help.go     # Field help documentation (722 lines)
│   │       ├── handlers.go       # Event handling logic (631 lines)
│   │       ├── header.go         # Application header
│   │       ├── network_interfaces.go # Network operations for ping
│   │       ├── search_bar.go     # Search functionality
│   │       ├── server_details.go # Server details display
│   │       ├── server_form.go    # Add/edit server form (2287 lines)
│   │       ├── server_list.go    # Server list component
│   │       ├── sort.go          # Sorting logic
│   │       ├── status_bar.go     # Status message display
│   │       ├── tui.go           # Main TUI application
│   │       ├── utils.go         # UI utility functions
│   │       ├── validation.go    # Form validation logic
│   │       └── validation_test.go # Validation tests
│   ├── core/                    # Domain layer
│   │   ├── domain/              # Business entities
│   │   │   └── server.go        # Server entity (117 fields, all SSH options)
│   │   ├── ports/               # Interface definitions
│   │   │   ├── repositories.go # Repository interface
│   │   │   └── services.go      # Service interface
│   │   └── services/            # Business logic implementation
│   │       ├── server_service.go # Main service implementation
│   │       ├── sysprocattr_unix.go  # Unix-specific process attributes
│   │       └── sysprocattr_windows.go # Windows-specific process attributes
│   └── logger/                 # Logging infrastructure
│       └── logger.go            # Zap logger setup
├── docs/                       # Documentation
│   └── logo.png                # Application logo
├── .github/                    # GitHub workflows and templates
│   ├── ISSUE_TEMPLATE/         # Issue templates
│   └── workflows/              # CI/CD workflows
├── bin/                        # Build outputs
├── makefile                   # Build automation
├── go.mod                     # Go module definition
├── go.sum                     # Dependency checksums
├── .gitignore                 # Git ignore rules
├── .golangci.yml              # Linting configuration
├── .goreleaser.yaml           # Release automation
├── README.md                  # Project documentation
└── LICENSE                    # Apache 2.0 license
```

## Directory Purposes

**cmd/:**
- Purpose: Application entry point and setup
- Contains: Main application initialization, CLI argument parsing, dependency injection
- Key files: `main.go` (72 lines) - sets up logger, repository, service, and TUI

**internal/core/domain/:**
- Purpose: Core business entities and data structures
- Contains: `Server` struct representing SSH configuration with all possible options
- Key files: `server.go` (117 fields covering authentication, forwarding, security, etc.)

**internal/core/ports/:**
- Purpose: Interface definitions for dependency injection
- Contains: Service and repository interfaces defining contracts
- Key files: `services.go` (ServerService interface), `repositories.go` (ServerRepository interface)

**internal/core/services/:**
- Purpose: Business logic implementation
- Contains: Server service with all domain operations
- Key files: `server_service.go` (business logic, validation, SSH execution)

**internal/adapters/data/ssh_config_file/:**
- Purpose: SSH configuration file persistence
- Contains: SSH config parsing, backup management, CRUD operations
- Key files: `crud.go` (2287 lines, main operations), `ssh_config_file_repo.go` (repository interface)

**internal/adapters/ui/:**
- Purpose: Terminal user interface
- Contains: TUI components, forms, event handling
- Key files: `server_form.go` (2287 lines, add/edit form), `server_list.go` (server listing)

**internal/logger/:**
- Purpose: Application logging
- Contains: Zap logger initialization
- Key files: `logger.go` (logger setup and configuration)

## Key File Locations

**Entry Points:**
- `cmd/main.go`: Application startup and dependency injection
- `internal/adapters/ui/tui.go`: Main TUI application coordinator

**Configuration:**
- `go.mod`: Module dependencies and version constraints
- `makefile`: Build automation targets
- `.golangci.yml`: Linting rules
- `.goreleaser.yaml`: Release automation configuration

**Core Logic:**
- `internal/core/services/server_service.go`: Business logic orchestrator
- `internal/adapters/data/ssh_config_file/crud.go`: SSH config CRUD operations
- `internal/core/domain/server.go`: Domain entity with all SSH options

**Testing:**
- `internal/adapters/ui/validation_test.go`: UI validation tests
- `internal/adapters/data/ssh_config_file/crud_test.go`: CRUD operations tests

## Naming Conventions

**Files:**
- Pattern: `snake_case.go`
- Examples: `server_service.go`, `ssh_config_file.go`, `server_form.go`

**Packages:**
- Pattern: `snake_case`
- Examples: `server_service`, `ssh_config_file`, `server_form`

**Types:**
- Pattern: `PascalCase` for structs and interfaces
- Examples: `ServerService`, `ServerRepository`, `Server`

**Methods:**
- Pattern: `PascalCase` (Go convention)
- Examples: `ListServers`, `AddServer`, `UpdateServer`

**Variables:**
- Pattern: `snake_case` for local variables
- Pattern: `PascalCase` for exported types
- Examples: `serverRepo`, `serverService`, `Server`

**Constants:**
- Pattern: `SCREAMING_SNAKE_CASE`
- Examples: `AppName`, `DefaultPort`

## Where to Add New Code

**New SSH Feature:**
- Domain: Add to `internal/core/domain/server.go`
- Repository: Implement in `internal/adapters/data/ssh_config_file/`
- Service: Add method to `internal/core/services/server_service.go`
- UI: Add to `internal/adapters/ui/server_form.go`

**New UI Component:**
- Implementation: `internal/adapters/ui/[component_name].go`
- Constants: `internal/adapters/ui/const.go`
- Validation: `internal/adapters/ui/validation.go`

**New External Integration:**
- Repository: Add new adapter in `internal/adapters/data/`
- Interface: Define in `internal/core/ports/`
- Service: Implement in service layer

**New Business Logic:**
- Service: Add to existing `server_service.go` or create new service
- Domain: Update `Server` entity if needed
- Tests: Add alongside implementation

## Special Directories

**internal/:**
- Purpose: Application internal code, not part of public API
- Generated: No
- Committed: Yes - contains all source code

**cmd/:**
- Purpose: Application entry point
- Generated: No
- Committed: Yes

**.github/:**
- Purpose: CI/CD workflows and issue templates
- Generated: Partially (workflows generated)
- Committed: Yes

**docs/:**
- Purpose: Documentation and assets
- Generated: No
- Committed: Yes

**bin/:**
- Purpose: Build outputs (added by build process)
- Generated: Yes
- Committed: No (excluded by .gitignore)

---

*Structure analysis: 2025-04-13*