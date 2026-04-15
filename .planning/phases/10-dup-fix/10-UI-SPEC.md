---
phase: 10
slug: dup-fix
status: draft
shadcn_initialized: false
preset: none
created: 2026-04-15
---

# Phase 10 -- UI Design Contract

> Visual and interaction contract for the Dup Fix phase. This is a Go/tview terminal application -- no web design system. All visual tokens are defined by tcell 256-color palette and tview Styles, already configured in the codebase.

---

## Design System

| Property | Value |
|----------|-------|
| Tool | tview + tcell/v2 (terminal TUI) |
| Preset | not applicable (not a web project) |
| Component library | tview (tview.List, tview.TextView, tview.Flex) |
| Icon library | none (text-based TUI) |
| Font | terminal monospace (user-configured) |

**Theme source:** `internal/adapters/ui/tui.go:87-96` -- `initializeTheme()` sets all tview.Styles values. No modifications needed for this phase.

---

## Spacing Scale

Not applicable. This is a terminal TUI -- spacing is controlled by tview's built-in layout engine (Flex with row/column directions). No explicit pixel spacing tokens exist or are needed for this phase.

**Existing layout pattern:** The server list uses `tview.NewFlex()` with `SetDirection(tview.FlexColumn)` for horizontal layout and `tview.FlexRow` for vertical layout. This phase does not change layout structure.

---

## Typography

Not applicable in the web-design sense. Terminal text rendering is controlled by:

| Role | tview Mechanism | Value |
|------|----------------|-------|
| Status bar text | `tview.TextView.SetDynamicColors(true)` + tview color tags | Default: `[white]...[-]` |
| Status bar temp message | `showStatusTempColor(msg, "#A0FFA0")` -- green | `#A0FFA0` (success), `#FF6B6B` (error) |
| List items | `tview.List` default text color | `tcell.Color252` (via `PrimaryTextColor`) |
| Selected item | `tview.List` selected colors | `tcell.Color24` bg, `tcell.Color255` fg |
| List title | `tview.List` title color | `tcell.Color250` |

**No typography changes for this phase.** The dup fix only changes behavior (what happens after D key), not visual rendering.

---

## Color

All colors are tcell 256-color values, already defined in `initializeTheme()`:

| Role | tcell Value | Usage |
|------|-------------|-------|
| Dominant (60%) | `tcell.Color232` | `PrimitiveBackgroundColor` -- main background |
| Secondary (30%) | `tcell.Color235` | `ContrastBackgroundColor` -- status bar, search bar bg |
| Accent (10%) | `tcell.Color24` | `SelectedBackgroundColor` -- selected list item highlight |
| Destructive | `#FF6B6B` | Error status messages via `showStatusTempColor` |
| Success | `#A0FFA0` | Success status messages via `showStatusTemp` |
| Border | `tcell.Color238` | `BorderColor` -- all panel borders |
| Title text | `tcell.Color250` | `TitleColor` -- panel titles |
| Primary text | `tcell.Color252` | `PrimaryTextColor` -- main content text |
| Muted text | `tcell.Color245` | `SecondaryTextColor` / `TertiaryTextColor` |

**Accent reserved for:** Selected list item background (`SelectedBackgroundColor`). Not used for status messages -- those use explicit hex colors via tview color tags.

**No color changes for this phase.** The dup fix reuses existing `showStatusTemp()` (green) and `showStatusTempColor()` (red for errors) patterns.

---

## Copywriting Contract

| Element | Copy | Source |
|---------|------|--------|
| Primary action feedback | `Server duplicated: {alias}` | RESEARCH.md recommended pattern; matches existing `showStatusTemp` usage (e.g., "Copied: {cmd}", "Sort: {mode}") |
| Error state | `Dup failed: {error}` | RESEARCH.md recommended pattern; matches existing error pattern (e.g., "Ping {alias}: DOWN ({error})") |
| Empty state | not applicable | D key only works on a selected server; `GetSelectedServer()` returns false if list is empty (handler returns silently) |
| Destructive confirmation | not applicable | Dup is a non-destructive operation -- it only adds a new entry |

**Status bar display rules:**
- Success: `showStatusTemp("Server duplicated: " + alias)` -- green text, auto-restores to `DefaultStatusText()` after 2 seconds
- Error: `showStatusTempColor(fmt.Sprintf("Dup failed: %v", err), "#FF6B6B")` -- red text, auto-restores after 2 seconds

---

## Interaction Contract

### D Key Behavior (handleServerDup)

**Before (current -- Phase 9):**
1. Deep copy selected server
2. Generate unique alias
3. Open ServerForm with pre-filled data
4. User edits and saves via form
5. handleServerSave() -> AddServer() -> refresh -> scroll to new entry

**After (this phase):**
1. Deep copy selected server (unchanged)
2. Generate unique alias (unchanged)
3. Call `t.serverService.AddServer(dup)` directly (no form)
4. On success: `refreshServerList()` -> find new entry by alias -> `SetCurrentItem(index)` -> `showStatusTemp("Server duplicated: {alias}")`
5. On error: `showStatusTempColor("Dup failed: {error}", "#FF6B6B")` -- stay on current list position
6. User never leaves the server list view

### List Positioning After Dup

- New entry is selected immediately after duplication
- List auto-scrolls to make new entry visible (tview.List.SetCurrentItem handles this)
- If a search filter is active, clear the search filter before refreshing so the new entry is guaranteed visible (open question from RESEARCH.md -- recommended resolution: clear search)

### Dead Code Cleanup

The following existing code becomes unreachable after this change and must be removed:
- `tui.dupPendingAlias` field (`internal/adapters/ui/tui.go:53`)
- `dupPendingAlias` assignment in `handleServerDup()` (`handlers.go:340`)
- `dupPendingAlias` reference in `handleServerSave()` (`handlers.go:354-355, 374-382`)

---

## Registry Safety

Not applicable. This is a Go terminal application using tview/tcell. No component registries exist.

| Registry | Blocks Used | Safety Gate |
|----------|-------------|-------------|
| N/A | N/A | N/A |

---

## Component Inventory

This phase does not create new components. It modifies existing handler behavior:

| Component | File | Change Type |
|-----------|------|-------------|
| `handleServerDup()` | `internal/adapters/ui/handlers.go:288-349` | Modify -- remove form creation, add direct save + scroll |
| `handleServerSave()` | `internal/adapters/ui/handlers.go:351-386` | Modify -- remove dupPendingAlias branch |
| `tui` struct | `internal/adapters/ui/tui.go:53` | Modify -- remove dupPendingAlias field |
| `showStatusTemp()` | `internal/adapters/ui/handlers.go:727` | No change -- reused as-is |
| `showStatusTempColor()` | `internal/adapters/ui/handlers.go:735` | No change -- reused as-is |
| `refreshServerList()` | `internal/adapters/ui/handlers.go` | No change -- reused as-is |
| `ServerList.SetCurrentItem()` | `internal/adapters/ui/server_list.go` | No change -- reused as-is |

---

## Checker Sign-Off

- [ ] Dimension 1 Copywriting: PASS -- status messages defined, match existing patterns
- [ ] Dimension 2 Visuals: PASS -- no visual changes, existing tview theme unchanged
- [ ] Dimension 3 Color: PASS -- no color changes, reuses existing status bar colors
- [ ] Dimension 4 Typography: PASS -- no typography changes, terminal-rendered text
- [ ] Dimension 5 Spacing: PASS -- no layout changes, tview Flex layout unchanged
- [ ] Dimension 6 Registry Safety: PASS -- not applicable (Go TUI project)

**Approval:** pending
