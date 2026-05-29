---
phase: quick
plan: 260414-ucr
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/adapters/ui/file_browser/local_pane.go
  - internal/adapters/ui/file_browser/remote_pane.go
autonomous: true
requirements: [ucr-kitty-header-transparency]
user_setup: []
must_haves:
  truths:
    - "Header row is visible in kitty with background_opacity < 1"
    - "Header row background color (Color235) is rendered opaque, not blending with terminal background"
    - "Data rows remain unaffected by the change"
  artifacts:
    - path: "internal/adapters/ui/file_browser/local_pane.go"
      provides: "Local pane header cells with SetTransparency(false)"
      contains: "SetTransparency(false)"
    - path: "internal/adapters/ui/file_browser/remote_pane.go"
      provides: "Remote pane header cells with SetTransparency(false)"
      contains: "SetTransparency(false)"
  key_links:
    - from: "local_pane.go populateTable()"
      to: "tview.TableCell"
      via: "SetTransparency(false) on header cells"
      pattern: "SetTransparency\\(false\\)"
    - from: "remote_pane.go populateTable()"
      to: "tview.TableCell"
      via: "SetTransparency(false) on header cells"
      pattern: "SetTransparency\\(false\\)"
---

<objective>
Fix table header row invisible in kitty with background_opacity < 1.

Purpose: tview.NewTableCell defaults to Transparent=true, which causes the Background(tcell.Color235) set via headerStyle to be ignored when kitty composites a semi-transparent terminal background. The header text renders but blends into the transparent background, making the header row visually invisible.

Output: Header cells in both LocalPane and RemotePane render with an opaque Color235 background.
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
  <name>Task 1: Add SetTransparency(false) to header cells in both panes</name>
  <files>internal/adapters/ui/file_browser/local_pane.go, internal/adapters/ui/file_browser/remote_pane.go</files>
  <action>
    In both local_pane.go and remote_pane.go, locate the header cell creation in populateTable().

    **local_pane.go** (line ~145): Add `.SetTransparency(false)` to the header cell chain:
    ```go
    cell := tview.NewTableCell(h.text).
        SetStyle(headerStyle).
        SetAlign(h.align).
        SetMaxWidth(h.maxWidth).
        SetExpansion(h.expansion).
        SetSelectable(false).
        SetTransparency(false)  // <-- add this
    ```

    **remote_pane.go** (line ~200): Same change:
    ```go
    cell := tview.NewTableCell(h.text).
        SetStyle(headerStyle).
        SetAlign(h.align).
        SetMaxWidth(h.maxWidth).
        SetExpansion(h.expansion).
        SetSelectable(false).
        SetTransparency(false)  // <-- add this
    ```

    Design rationale: SetTransparency(false) tells tview to render the cell's background color as opaque, preventing kitty's background_opacity from compositing through the cell. This is the minimal fix -- only header cells need it because they use a non-default background (Color235). Data cells use tcell.ColorDefault which should blend with the terminal background (per the existing SetBackgroundColor(tcell.ColorDefault) on the Table itself).

    Do NOT modify any other cells, styles, or transparency settings. Do NOT add SetTransparency(false) to data rows or clearAndShowEmpty cells.
  </action>
  <verify>
    <automated>cd /home/huanglin/code/lazyssh && go build ./... && grep -n "SetTransparency(false)" internal/adapters/ui/file_browser/local_pane.go internal/adapters/ui/file_browser/remote_pane.go</automated>
  </verify>
  <done>
    - `go build ./...` succeeds
    - Both files contain exactly one `SetTransparency(false)` call, each in the header cell creation chain within populateTable()
    - No other cells have SetTransparency calls
  </done>
</task>

</tasks>

<verification>
- `go build ./...` compiles without errors
- `grep -c "SetTransparency(false)"` returns 1 for each file (local_pane.go, remote_pane.go)
- Header cells are the only cells with SetTransparency(false)
</verification>

<success_criteria>
Header row renders with opaque Color235 background in kitty with background_opacity < 1, making the header text clearly visible. Data rows remain unaffected.
</success_criteria>

<output>
After completion, create `.planning/quick/260414-ucr-fix-table-header-row-invisible-in-kitty-/260414-ucr-SUMMARY.md`
</output>
