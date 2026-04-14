---
phase: quick
plan: 260414-tpo
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/adapters/ui/file_browser/local_pane.go
  - internal/adapters/ui/file_browser/remote_pane.go
autonomous: true
requirements: []
must_haves:
  truths:
    - "Table header row no longer overlaps with first data row in LocalPane"
    - "Table header row no longer overlaps with first data row in RemotePane"
    - "j/k navigation still skips the header row (lands on first data row)"
  artifacts:
    - path: "internal/adapters/ui/file_browser/local_pane.go"
      provides: "Local file browser pane"
      change: "Remove SetFixed(1, 0) call"
    - path: "internal/adapters/ui/file_browser/remote_pane.go"
      provides: "Remote file browser pane"
      change: "Remove SetFixed(1, 0) call"
  key_links:
    - from: "local_pane.go build()"
      to: "tview.Table"
      via: "SetFixed(1, 0) removed; SetSelectable(false) on header cells still prevents selection"
    - from: "remote_pane.go build()"
      to: "tview.Table"
      via: "SetFixed(1, 0) removed; SetSelectable(false) on header cells still prevents selection"
---

<objective>
Fix table header/data row visual overlap in both LocalPane and RemotePane.

Purpose: `SetFixed(1, 0)` causes the header row to render on top of the first data row in the current tview version. Removing it eliminates the overlap. Header cells already have `SetSelectable(false)` which ensures j/k navigation skips the header, so `SetFixed` is redundant for selection behavior.

Output: Two files modified with a single line removed from each.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@internal/adapters/ui/file_browser/local_pane.go
@internal/adapters/ui/file_browser/remote_pane.go
</context>

<tasks>

<task type="auto">
  <name>Task 1: Remove SetFixed(1, 0) from both pane build() methods</name>
  <files>internal/adapters/ui/file_browser/local_pane.go, internal/adapters/ui/file_browser/remote_pane.go</files>
  <action>
    Remove the `SetFixed(1, 0)` call from both files:

    1. **local_pane.go line 61**: Delete `lp.SetFixed(1, 0)             // fixed header row` (the entire line including the comment).

    2. **remote_pane.go line 65**: Delete `rp.SetFixed(1, 0)             // fixed header row` (the entire line including the comment).

    No other changes needed. The header cells in both `populateTable()` methods already call `.SetSelectable(false)` on every header cell (local_pane.go:151, remote_pane.go:206), which is what prevents j/k from selecting the header row. `SetFixed` was only for visual pinning and is causing the overlap bug in this tview version.

    Do NOT add any replacement call. Do NOT modify SetSelectable or any other configuration.
  </action>
  <verify>
    <automated>cd /home/huanglin/code/lazyssh && grep -n "SetFixed" internal/adapters/ui/file_browser/local_pane.go internal/adapters/ui/file_browser/remote_pane.go</automated>
  </verify>
  <done>
    `grep -n "SetFixed" ...` returns no output (zero matches in both files).
    Both files compile cleanly: `go build ./...`
  </done>
</task>

</tasks>

<verification>
1. `grep -rn "SetFixed" internal/adapters/ui/file_browser/` returns no matches
2. `go build ./...` compiles without errors
3. Visual: header row and first data row are on separate lines in both panes
</verification>

<success_criteria>
- `SetFixed` call removed from both local_pane.go and remote_pane.go
- Both files compile cleanly
- Header row renders without overlapping first data row
- j/k navigation still lands on first data row (not header)
</success_criteria>

<output>
After completion, create `.planning/quick/260414-tpo-fix-table-header-data-row-overlap-in-bot/260414-tpo-SUMMARY.md`
</output>
