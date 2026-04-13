# Testing Patterns

**Analysis Date:** 2025-04-13

## Test Framework

**Runner:**
- **Framework:** Go built-in `testing`
- **Command:** `go test`
- **Parallel:** Enabled by default
- **Coverage:** Enabled with `go test -cover`

**Assertion Library:**
- Standard library `testing` package
- No external assertion libraries detected

**Run Commands:**
```bash
go test                    # Run all tests
go test -v                # Verbose output
go test -cover            # Run with coverage
go test -coverprofile=coverage.out # Generate coverage report
go test -run TestValidateHost   # Run specific test
go test -bench=.          # Run benchmarks
```

## Test File Organization

**Location:**
- **Co-located:** Test files in same directory as source
- **Naming:** `snake_case_test.go`
- **Pattern:** Mirror source directory structure

**Structure:**
```
internal/adapters/data/ssh_config_file/
├── crud.go
├── crud_test.go
├── mapper.go
└── ssh_config_file_repo.go

internal/adapters/ui/
├── handlers.go
├── validation_test.go
├── utils_test.go
└── field_help_test.go
```

## Test Structure

**Suite Organization:**
```go
package ssh_config_file

import (
    "testing"
)

// Test function name convention: Test[FunctionName]
func TestConvertCLIForwardToConfigFormat(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        // Test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := r.convertCLIForwardToConfigFormat(tt.input)
            if result != tt.expected {
                t.Errorf("convertCLIForwardToConfigFormat(%q) = %q, want %q", tt.input, result, tt.expected)
            }
        })
    }
}
```

**Patterns:**
- **Table-driven tests:** Primary pattern for unit tests
- **Sub-tests:** Used for test organization
- **Setup/Teardown:** Manual setup in test functions

## Mocking

**Framework:** No external mocking framework detected
- Use interfaces for testability
- Manual mocking where needed

**Patterns:**
```go
// Interface-based dependency injection
type Repository struct {
    log          *zap.SugaredLogger
    configPath   string
    metadataPath string
    fs           FileSystem
}

// Test with real filesystem
func TestRead_ExistingFile(t *testing.T) {
    // Create temporary file
    // ...
    r := &Repository{
        fs: &osFileSystem{},
    }
}

// Test helper types for complex mocking
type testError struct {
    msg string
}

func (e *testError) Error() string {
    return e.msg
}
```

**What to Mock:**
- External dependencies (filesystem, network)
- Time-based operations
- Complex objects in tests

**What NOT to Mock:**
- Simple data structures
- Business logic
- Pure functions

## Fixtures and Factories

**Test Data:**
```go
// Test case structure
tests := []struct {
    name     string
    input    string
    expected string
}{
    {
        name:     "basic local forward",
        input:    "8080:localhost:80",
        expected: "8080 localhost:80",
    },
    {
        name:     "invalid format - only one colon",
        input:    "8080:localhost",
        expected: "8080:localhost", // returned as-is
    },
}

// Complex test data setup
func TestValidateKeyPaths(t *testing.T) {
    oldHome := os.Getenv("HOME")
    t.Cleanup(func() {
        _ = os.Setenv("HOME", oldHome)
    })

    tempHome := t.TempDir()
    sshDir := filepath.Join(tempHome, ".ssh")
    // ... setup test files
}
```

**Location:**
- Test data defined inline in test functions
- No separate test data directories detected
- Temporary directories created per test

## Coverage

**Requirements:** Not enforced at project level
- Manual coverage checking available
- Coverage profiles can be generated

**View Coverage:**
```bash
go test -coverprofile=coverage.out
go tool cover -html=coverage.out  # View in browser
```

## Test Types

**Unit Tests:**
- **Scope:** Individual functions and methods
- **Approach:** Isolated testing with mocks
- **Examples:** `TestValidateHost`, `TestConvertCLIForwardToConfigFormat`

**Integration Tests:**
- **Scope:** Component interactions
- **Approach:** Real filesystem operations
- **Examples:** File CRUD operations in `crud_test.go`

**E2E Tests:**
- **Framework:** Not detected
- **Coverage:** Limited E2E testing

## Common Patterns

**Async Testing:**
```go
// No async-specific patterns detected
// All tests appear to be synchronous
```

**Error Testing:**
```go
func TestValidateHost_InvalidHost(t *testing.T) {
    tests := []struct {
        name    string
        host    string
        wantErr bool
    }{
        {"Empty host", "", true},
        {"Host with spaces", "example .com", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validateHost(tt.host)
            if (err != nil) != tt.wantErr {
                t.Errorf("validateHost(%s) error = %v, wantErr %v", tt.host, err, tt.wantErr)
            }
        })
    }
}
```

**Table-driven Testing:**
```go
func TestBuildSSHCommand_PortForwarding(t *testing.T) {
    tests := []struct {
        name     string
        server   domain.Server
        expected []string // expected parts in the command
    }{
        {
            name: "local forward",
            server: domain.Server{
                Alias:        "test",
                Host:         "example.com",
                User:         "user",
                LocalForward: []string{"8080:localhost:80"},
            },
            expected: []string{"ssh", "-L", "8080:localhost:80", "user@example.com"},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := BuildSSHCommand(tt.server)
            // Verify expected parts are in result
            for _, part := range tt.expected {
                if !strings.Contains(result, part) {
                    t.Errorf("BuildSSHCommand() missing expected part %q", part)
                }
            }
        })
    }
}
```

**Test Cleanup:**
```go
func TestValidateKeyPaths(t *testing.T) {
    oldHome := os.Getenv("HOME")
    t.Cleanup(func() {
        _ = os.Setenv("HOME", oldHome)
    })
    // ... rest of test
}
```

**Validation Testing:**
```go
// State-based testing
func TestValidationState_MultipleErrors(t *testing.T) {
    state := NewValidationState()

    // Set multiple errors
    state.SetError("Alias", "Alias is required")
    state.SetError("Host", "Host is required")

    // Verify state
    if !state.HasErrors() {
        t.Error("Expected HasErrors to return true")
    }

    errors := state.GetAllErrors()
    // Verify error count and content
}
```

## Test Best Practices Observed

1. **Comprehensive edge cases:** Tests cover normal, edge, and error cases
2. **Clear test names:** Descriptive names indicating what's being tested
3. **Table-driven approach:** Efficient way to test multiple scenarios
4. **No global state:** Each test runs in isolation
5. **Manual cleanup:** Proper resource cleanup with `t.Cleanup`
6. **Structured assertions:** Clear error messages with expected vs actual
7. **Test organization:** Logical grouping with sub-tests

---

*Testing analysis: 2025-04-13*
```