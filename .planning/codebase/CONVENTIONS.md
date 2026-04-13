# Coding Conventions

**Analysis Date:** 2025-04-13

## Language & Edition
- **Go Version:** 1.24.6
- **Standard:** Go 1.x compatible
- **Format:** Official Go formatting

## Naming Patterns

**Files:**
- **Source files:** `snake_case.go` (e.g., `crud.go`, `handlers.go`, `logger.go`)
- **Test files:** `snake_case_test.go` (e.g., `crud_test.go`, `validation_test.go`)
- **Package directories:** `lowercase` (e.g., `ssh_config_file`, `domain`)

**Functions:**
- **Public:** `PascalCase` (e.g., `NewRepository`, `BuildSSHCommand`, `validateHost`)
- **Private:** `camelCase` (e.g., `convertCLIForwardToConfigFormat`, `handleGlobalKeys`, `setFieldValues`)
- **Test functions:** `TestCamelCase` or `Test_underscore_case` (e.g., `TestValidateHost`, `TestBuildSSHCommand_PortForwarding`)

**Variables:**
- **Public:** `PascalCase` (e.g., `Server`, `Repository`, `TUI`)
- **Private:** `camelCase` (e.g., `serverRepo`, `log`, `fieldValidators`)
- **Constants:** `SCREAMING_SNAKE_CASE` (e.g., `ForwardTypeLocal`, `ForwardModeOnlyForward`, `AppName`)

**Types:**
- **Structs:** `PascalCase` (e.g., `Server`, `ValidationState`, `FieldValidator`)
- **Interfaces:** `PascalCase` with "er" suffix (e.g., `tview.Primitive`, `tcell.EventHandler`)
- **Custom errors:** `*PascalError` (e.g., `*testError` in tests)

## Code Style

**Formatting:**
- **Tool:** `gofmt` (used by `go fmt` command)
- **Import organization:** Standard Go import grouping
  ```go
  import (
      "fmt"
      "os"
      "path/filepath"
      "time"

      "github.com/atotto/clipboard"
      "github.com/gdamore/tcell/v2"
      "github.com/Adembc/lazyssh/internal/core/domain"
      "github.com/rivo/tview"
  )
  ```
- **Line length:** No hard limit but long lines wrapped appropriately
- **Braces:** Always on same line as statement (K&R style)

**Linting:**
- **Primary tool:** `golangci-lint`
- **Configuration:** `.golangci.yml`
- **Key rules enforced:**
  - `errcheck` - Check for unchecked errors
  - `gocyclo` - Cyclomatic complexity
  - `goconst` - Detect duplicate constants
  - `gosimple` - Simplify code
  - `govet` - Static analysis
  - `nolintlint` - Check nolint comments
  - `revive` - Alternative linter with rules
  - `staticcheck` - Advanced static analysis
  - `unparam` - Unused parameters
  - `unused` - Unused variables

## Import Organization

**Order:**
1. Standard library imports (alphabetical within groups)
2. Third-party imports (alphabetical)
3. Internal imports (alphabetical, with `internal/` prefix)

**Path Aliases:** None detected in current codebase

## Error Handling

**Patterns:**
- **Error return:** Always return `error` as last return parameter
- **Error checking:** Immediate after function call
  ```go
  home, err := os.UserHomeDir()
  if err != nil {
      log.Errorw("failed to get user home directory", "error", err)
      os.Exit(1)
  }
  ```
- **Error logging:** `log.Errorw()` with structured fields
- **Error propagation:** Return errors to caller with context

**Common Patterns:**
```go
// Pattern 1: Immediate exit on critical error
log, err := logger.New("LAZYSSH")
if err != nil {
    fmt.Println(err)
    os.Exit(1)
}

// Pattern 2: Return error to caller
func (r *Repository) Read() ([]Server, error) {
    data, err := os.ReadFile(r.configPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read config: %w", err)
    }
    // ...
}

// Pattern 3: Log and continue
_, err = clipboard.WriteAll(cmd)
if err != nil {
    t.log.Errorw("failed to copy to clipboard", "error", err)
}
```

**nolint Usage:**
- Used selectively with justification comments
- Common cases:
  - `//nolint:errcheck // Safe to ignore for defers`
  - `//nolint:gocritic // exitAfterDefer: ensure immediate exit`
  - `//nolint:exhaustive // We only handle specific keys`

## Logging

**Framework:** `zap` (via `go.uber.org/zap`)

**Patterns:**
- **Info logging:** `log.Infow()` with structured fields
- **Error logging:** `log.Errorw()` with structured fields
- **Debug logging:** Not explicitly configured but available
- **Sugar logger:** Used for convenience (`.Sugar()`)

**Example:**
```go
log.Infow("server connected",
    "alias", server.Alias,
    "host", server.Host,
    "port", server.Port,
)
```

## Comments

**Header Comments:** All files include Apache 2.0 license header

**Code Comments:**
- Function documentation through code structure
- Important logic explained in comments
- nolint comments include justification

**JSDoc/TSDoc:** Not used (Go uses different conventions)

## Function Design

**Size:** Typically focused, single responsibility
- Small functions for specific operations
- Medium functions for complex operations
- Large functions avoided through decomposition

**Parameters:**
- Typically 2-5 parameters
- Use structs for related parameters (e.g., `domain.Server`)
- Optional parameters use `string`/`int` with zero values

**Return Values:**
- Primary value + error pattern
- Multiple values with error as last parameter
- Options/flags use boolean returns

**Example patterns:**
```go
// Simple function with single return
func validateHost(host string) error

// Function with error return
func (r *Repository) Read() ([]Server, error)

// Function with multiple values
func NewTUI(log *zap.SugaredLogger, service *services.ServerService, version, gitCommit string) *TUI
```

## Module Design

**Exports:**
- Only necessary types marked `pub`
- Internal types private by default
- Interface-first design where appropriate

**Package Structure:**
- `internal/core/domain` - Domain models
- `internal/core/services` - Business logic
- `internal/adapters/data` - Data persistence
- `internal/adapters/ui` - User interface
- `cmd/` - Application entry point

**Dependencies:**
- Clear dependency hierarchy
- Domain layer independent of adapters
- Services depend on domain, not adapters
- Adapters implement interfaces defined in domain/services

## Serialization

**Framework:** `serde` via encoding/json patterns

**Patterns:**
- Struct tags for JSON field mapping
- Custom Marshal/Unmarshal methods for complex types
- Error handling for invalid data

**Example:**
```go
type Server struct {
    Alias string `json:"alias"`
    Host  string `json:"host"`
    Port  int    `json:"port"`
}
```

---

*Convention analysis: 2025-04-13*
```