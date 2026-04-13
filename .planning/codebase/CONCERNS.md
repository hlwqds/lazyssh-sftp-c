# Codebase Concerns

**Analysis Date:** 2025-04-13

## Tech Debt

### Large UI Components
**Issue:** Single large components with excessive responsibilities
- Files: `[internal/adapters/ui/server_form.go]`
- Impact: 2287 lines in one file makes maintenance and testing difficult
- Fix approach: Split into smaller, focused components - one per form tab (Basic, Connection, Forwarding, Authentication)

### Duplicated SSH Package
**Issue:** Forked ssh_config package with custom modifications
- Files: `go.mod` line 5 (replace directive), go.sum
- Impact: Maintenance burden, security patches need manual integration
- Fix approach: Contribute changes upstream to original package or move to different approach

### Process Management Race Condition
**Issue:** Potential race condition in forward tracking
- Files: `[internal/core/services/server_service.go]` lines 240-242
- Impact: Multiple goroutines accessing `s.forwards` map without proper synchronization
- Fix approach: Use channels for process tracking or implement proper locking pattern

### Error Messages Too Technical
**Issue:** Raw error messages exposed to users
- Files: `[internal/adapters/ui/handlers.go]` lines 613-616
- Impact: Poor user experience with system-level error messages
- Fix approach: Implement user-friendly error messages and logging system errors

## Known Bugs

### Process Cleanup Not Guaranteed
**Issue:** Forwards may not be properly cleaned up on application exit
- Files: `[internal/core/services/server_service.go]` lines 244-248
- Symptoms: SSH processes left running if app crashes or force killed
- Workaround: Manual process cleanup required
- Trigger: Application crash or SIGKILL signal

### SSH Command Injection Risk
**Issue:** Potential command injection in SSH arguments
- Files: `[internal/core/services/server_service.go]` lines 208-209
- Symptoms: If alias contains shell metacharacters, could execute arbitrary commands
- Workaround: Sanitize input before passing to exec.Command

## Security Considerations

### Exec.Command Without Validation
**Issue:** User input passed directly to SSH command
- Files: `[internal/core/services/server_service.go]` lines 158, 182, 208, 347`
- Risk: Command injection if input contains shell metacharacters
- Current mitigation: Relies on SSH config parsing
- Recommendations: Implement proper input sanitization or use SSH config approach

### File Permissions Not Checked
**Issue:** SSH config file permissions not verified
- Files: `[internal/adapters/data/ssh_config_file/crud.go]`
- Risk: Could modify world-readable config files
- Recommendations: Check file permissions and warn if too permissive

## Performance Bottlenecks

### Large Slice Operations
**Issue:** Frequent slice reallocations
- Files: `[internal/core/services/server_service.go]` lines 179, 205, 241
- Problem: Multiple append operations on slices
- Cause: No capacity pre-allocation
- Improvement path: Pre-allocate slices with known capacity

### UI Blocking Operations
**Issue:** Synchronous ping operations block UI
- Files: `[internal/adapters/ui/handlers.go]` lines 596-612
- Problem: UI freezes during ping operations
- Cause: No goroutine for async operations
- Improvement path: Use go routines for background operations

## Fragile Areas

### SSH Config Parser
**Issue:** Custom SSH config parser with complex logic
- Files: `[internal/adapters/data/ssh_config_file/crud.go]`, `[internal/adapters/data/ssh_config_file/mapper.go]`
- Why fragile: Complex string parsing and regex patterns
- Safe modification: Test thoroughly with various config formats
- Test coverage: Gaps in edge cases

### TUI State Management
**Issue:** Complex state in TUI components
- Files: `[internal/adapters/ui/server_form.go]`, `[internal/adapters/ui/tui.go]`
- Why fragile: Multiple mutable state variables
- Safe modification: Immutable patterns or state management library
- Test coverage: No comprehensive test suite for TUI interactions

## Scaling Limits

### Memory Usage
**Issue: Loading large SSH config files
- Current capacity: Limited by in-memory parsing
- Limit: Performance degrades with configs >1000 entries
- Scaling path: Lazy loading or pagination for large configs

### Concurrent SSH Connections
**Issue: Multiple forward operations
- Current capacity: Limited by process tracking implementation
- Limit: Race conditions in process map
- Scaling path: Proper process management with context cancellation

## Dependencies at Risk

### tview Package
**Issue:** Using development version of tview
- Risk: Breaking changes in unreleased versions
- Impact: UI could break with updates
- Migration plan: Pin to stable version or switch to alternative

### go.mod Replace Directive
**Issue: Custom fork of ssh_config package
- Risk: Divergence from upstream
- Impact: Security patches delayed
- Migration plan: Track upstream or use standard library

## Missing Critical Features

### Input Validation
**Problem: Limited input validation before SSH command execution
- Blocks: Secure operation with potentially malicious inputs
- Priority: High

### Recovery Mechanism
**Problem: No recovery from corrupted SSH config
- Blocks: Reliable operation after config corruption
- Priority: Medium

## Test Coverage Gaps

### Integration Tests Missing
**What's not tested: End-to-end workflows
- Files: No integration test files found
- Risk: UI interaction bugs not caught
- Priority: High

### Error Path Testing
**What's not tested: Error handling and recovery
- Files: Only unit tests for validation
- Risk: Error states not properly handled
- Priority: Medium

---

*Concerns audit: 2025-04-13*