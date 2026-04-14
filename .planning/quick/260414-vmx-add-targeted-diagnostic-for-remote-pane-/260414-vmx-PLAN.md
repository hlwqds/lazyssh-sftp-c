---
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/adapters/ui/file_browser/remote_pane.go
  - internal/adapters/ui/file_browser/file_browser.go
autonomous: true

must_haves:
  truths:
    - "Remote pane table rowOffset is logged on every populateTable() call"
    - "Defensive SetOffset(0, 0) is called after Clear() in populateTable() to prevent stale scroll"
    - "Existing diagnostic drawCount/screen-scanning code in file_browser.go remains intact"
  artifacts:
    - path: "internal/adapters/ui/file_browser/remote_pane.go"
      provides: "rowOffset diagnostic + defensive reset in populateTable()"
  key_links:
    - from: "RemotePane.populateTable()"
      to: "tview.Table.SetOffset(0, 0)"
      via: "called after Clear() to reset scroll position"
---

<objective>
Add a targeted diagnostic log for the remote pane's tview.Table rowOffset in populateTable(), and add a defensive SetOffset(0, 0) reset after Clear() to prevent stale scroll offset from causing ghost content.

Purpose: The tview.Table internally tracks rowOffset for scrolling. When populateTable() calls Clear(), the cell content resets but rowOffset may retain a value from a previous (larger) directory listing. This can cause the table to render with an offset, showing stale cells or blank rows. A diagnostic log helps confirm this theory; the defensive reset fixes it.

Output: Remote pane with rowOffset logging and safe scroll reset on every table refresh.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@internal/adapters/ui/file_browser/remote_pane.go
@internal/adapters/ui/file_browser/file_browser.go

## Key Interface: tview.Table offset API

From tview source (github.com/rivo/tview):
```go
// SetOffset sets how many rows and columns should be skipped when drawing the table.
func (t *Table) SetOffset(row, column int) *Table

// GetOffset returns the current row and column offset.
func (t *Table) GetOffset() (row, column int)
```

The Table.Draw() method uses `rowOffset` to skip rows:
```go
row += t.rowOffset  // line 809 in table.go
```

The Select() method auto-adjusts rowOffset for scroll-into-view, and Clear() sets rowOffset=0 (line 841), but this may race with deferred draws or app.Sync() timing.
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add rowOffset diagnostic and defensive reset in RemotePane.populateTable()</name>
  <files>internal/adapters/ui/file_browser/remote_pane.go</files>
  <action>
    In `populateTable()` (line 182), after the `rp.Clear()` call on line 183, add two things:

    1. **Diagnostic log**: Before Clear(), capture and log the current rowOffset via `rp.GetOffset()`. This tells us if a stale offset existed before the table was rebuilt:
       ```go
       // Diagnostic: log rowOffset before clearing to detect stale scroll state
       if _, rowOff := rp.GetOffset(); rowOff > 0 {
           rp.log.Infow("populateTable: non-zero rowOffset before Clear",
               "rowOffset", rowOff,
               "path", rp.currentPath)
       }
       ```

    2. **Defensive reset**: After `rp.Clear()`, explicitly reset the offset to (0, 0) as a safety net. Even though tview.Table.Clear() already sets rowOffset=0, the explicit call is defensive against any timing edge cases with deferred draws:
       ```go
       rp.SetOffset(0, 0) // defensive: ensure scroll resets with content
       ```

    The resulting populateTable() start should look like:
    ```go
    func (rp *RemotePane) populateTable(entries []domain.FileInfo) {
        // Diagnostic: log rowOffset before clearing to detect stale scroll state
        if _, rowOff := rp.GetOffset(); rowOff > 0 {
            rp.log.Infow("populateTable: non-zero rowOffset before Clear",
                "rowOffset", rowOff,
                "path", rp.currentPath)
        }
        rp.Clear()
        rp.SetOffset(0, 0) // defensive: ensure scroll resets with content
        // ... rest of function unchanged
    ```

    Do NOT modify any other logic in populateTable(). Do NOT modify file_browser.go -- the existing diagnostic code there (drawCount, screen scanning) serves a different purpose and should remain as-is.
  </action>
  <verify>
    <automated>cd /home/huanglin/code/lazyssh && go build ./...</automated>
  </verify>
  <done>
    - populateTable() logs rowOffset when non-zero before Clear()
    - SetOffset(0, 0) is called after Clear() as defensive reset
    - go build passes with no errors
  </done>
</task>

</tasks>

<verification>
- `go build ./...` passes
- `go vet ./...` passes
- No changes to file_browser.go or any other files
</verification>

<success_criteria>
- Remote pane's populateTable() defensively resets scroll offset on every refresh
- Non-zero rowOffset is logged as a diagnostic signal for ghost content investigation
- Build passes cleanly
</success_criteria>

<output>
After completion, create `.planning/quick/260414-vmx-add-targeted-diagnostic-for-remote-pane-/260414-vmx-SUMMARY.md`
</output>
