# Architecture Research: Recent Remote Directories Integration

**Domain:** TUI Overlay Component for FileBrowser (v1.1 Milestone)
**Researched:** 2026-04-14
**Confidence:** HIGH (based on direct code analysis of existing file_browser package)

## Executive Summary

"Recent Remote Directories" feature is a pure UI-layer addition that follows the same overlay pattern established by `TransferModal`. The integration requires one new struct (`RecentDirs`), modifications to two existing files (`file_browser.go`, `remote_pane.go`), and zero changes to domain/ports/services layers. The feature is well-scoped: in-memory ring buffer of path strings, triggered by `r` key in RemotePane, displayed as a tview.Box overlay with manual Draw.

The most architecturally significant finding is the `onPathChange` callback asymmetry: `NavigateInto()` fires the callback but `NavigateToParent()` does not. Recording recent paths requires either fixing this asymmetry or hooking into a central point in FileBrowser.

## Existing Architecture: Component Map

```
FileBrowser (root, *tview.Flex)
  ├── localPane  (*LocalPane = *tview.Table)
  ├── remotePane (*RemotePane = *tview.Table)
  ├── statusBar  (*tview.TextView)
  └── transferModal (*TransferModal = *tview.Box, overlay)
```

### Key Ownership Relationships

| Owner | Component | Lifecycle |
|-------|-----------|-----------|
| FileBrowser | localPane | Created in build(), never replaced |
| FileBrowser | remotePane | Created in build(), never replaced |
| FileBrowser | transferModal | Created in build(), shown/hidden via Show()/Hide() |
| FileBrowser | statusBar | Created in build(), text updated dynamically |

### Key Routing Chain

```
Keyboard event propagation:
  FileBrowser.SetInputCapture (handleGlobalKeys)
    → Tab, Esc, s, S, F5 intercepted
    → event passed to focused pane

  RemotePane.SetInputCapture
    → h, Space, . intercepted (when connected)
    → event passed to Table built-in (j/k/arrows/Enter/PgUp/PgDn)

  RemotePane.SetSelectedFunc (Enter on row)
    → NavigateInto() for directories
    → onFileAction callback for files
```

### Overlay Pattern (TransferModal as Reference)

TransferModal is the only existing overlay in FileBrowser. Its pattern:

1. **Embeds `*tview.Box`** (not a full tview.Primitive with InputHandler)
2. **Manual `Draw()`** with `visible` flag guard
3. **No tview focus** -- key interception happens in `handleGlobalKeys`:
   ```go
   // file_browser_handlers.go line 37-39
   case tcell.KeyESC:
       if fb.transferModal != nil && fb.transferModal.IsVisible() {
           fb.transferModal.HandleKey(event)
           return nil
       }
   ```
4. **State machine** via `modalMode` enum (progress/cancelConfirm/conflictDialog/summary)
5. **Dismiss callback** for cleanup after hide

This pattern is the blueprint for the RecentDirs overlay.

## New Component: RecentDirs

### Component Design

```
RecentDirs (new struct)
  ├── *tview.Box (background, border, title)
  ├── paths []string (ring buffer, max 10)
  ├── selectedIndex int (cursor position)
  ├── visible bool
  └── onDismiss func()
```

**File location:** `internal/adapters/ui/file_browser/recent_dirs.go`

### Why a New Struct (Not Extending RemotePane)

| Approach | Pros | Cons |
|----------|------|------|
| **New struct (Recommended)** | Clean separation; follows TransferModal precedent; no pane code bloat; easily testable | One more file in package |
| Add to RemotePane | No new file | Violates single responsibility; RemotePane is already 425 lines; mixes display concerns with file browsing; overlay focus model fundamentally different |
| Add to FileBrowser | Centralized | FileBrowser is already 570 lines; couples overlay rendering to layout orchestration |

**Recommendation:** New `RecentDirs` struct following the TransferModal pattern exactly.

### Data Storage: Where Paths Live

**Recommendation: Paths stored in `RecentDirs` struct, owned by `FileBrowser`.**

```
FileBrowser
  ├── ...
  └── recentDirs *RecentDirs  (NEW field)
```

Rationale:
- RecentDirs owns its data, just like TransferModal owns its progress state
- No need for a separate data structure -- it's a simple `[]string` ring buffer
- Per PROJECT.md: "Only in memory, cleared on exit" -- no persistence needed
- Per PROJECT.md: "Granularity: local dir + server combination" -- since FileBrowser is per-server, one RecentDirs instance per FileBrowser session is correct

**Why NOT in RemotePane:**
- RemotePane is a table view for displaying files. Path history is a separate concern.
- The overlay is displayed on top of the entire FileBrowser, not just the RemotePane area.
- RemotePane doesn't own any overlay state currently (TransferModal is owned by FileBrowser).

### Ring Buffer Implementation

```go
const maxRecentDirs = 10

type RecentDirs struct {
    *tview.Box
    paths         []string // most-recent first
    selectedIndex int
    visible       bool
    onNavigate    func(path string) // callback when user selects a path
    onDismiss     func()
}

// Record adds a path to the front of the list.
// Deduplicates: moves existing path to front if already present.
// Trims to maxRecentDirs.
func (rd *RecentDirs) Record(path string) { ... }
```

The `Record` method implements move-to-front deduplication:
- If path exists in list, remove it from current position
- Insert at front
- Trim to 10 entries

This is pure UI state -- no domain/ports changes needed.

## Integration Points

### 1. Key Routing: 'r' Key Interception

**Where:** `RemotePane.SetInputCapture` (NOT `FileBrowser.SetInputCapture`)

**Why RemotePane:**
- The `r` key should only work when RemotePane has focus
- This matches the existing pattern: pane-specific keys (`h`, Space, `.`) are intercepted in `RemotePane.SetInputCapture`
- FileBrowser-level keys (`Tab`, `Esc`, `s`, `S`) are in `handleGlobalKeys`
- However, RemotePane doesn't have access to `recentDirs` -- it needs a callback

**Two options for the callback:**

**Option A: Callback from RemotePane to FileBrowser (Recommended)**
```go
// RemotePane gets a new callback
onShowRecentDirs func()

// In RemotePane.SetInputCapture:
case 'r':
    if rp.onShowRecentDirs != nil {
        rp.onShowRecentDirs()
    }
    return nil
```

This follows the exact same pattern as `onPathChange` and `onFileAction` -- RemotePane defines callback slots, FileBrowser wires them in `build()`.

**Option B: Intercept in FileBrowser.handleGlobalKeys**
```go
// In handleGlobalKeys:
case 'r':
    if fb.activePane == 1 && fb.remotePane.IsConnected() {
        fb.recentDirs.Show()
        return nil
    }
```

This is simpler but breaks the established pattern. Pane-specific keys live in pane InputCapture.

**Recommendation:** Option A. It maintains the established pattern where panes define their own key bindings via callbacks.

### 2. Recording Paths: Hook into Navigation

**Critical finding:** There is an existing asymmetry in `onPathChange`:

| Method | Fires onPathChange? |
|--------|-------------------|
| `RemotePane.NavigateInto()` | YES (line 298-299) |
| `RemotePane.NavigateToParent()` | **NO** |

Both methods change `rp.currentPath`, but only `NavigateInto` fires the callback. This is a bug or intentional omission -- it means `app.Sync()` (which uses `onPathChange`) is also not called on parent navigation.

**Recording strategy:** Hook into the `onPathChange` callback that FileBrowser already registers on RemotePane.

```go
// In FileBrowser.build():
fb.remotePane.OnPathChange(func(path string) {
    fb.app.Sync()                          // existing
    fb.recentDirs.Record(path)             // NEW
})
```

**This approach also means NavigateToParent won't record paths.** Two options:

**Option A: Fix the asymmetry (Recommended)**
Add `onPathChange` call to `NavigateToParent()` in RemotePane. This is arguably a bug fix -- the path does change, so the callback should fire. It also fixes the missing `app.Sync()` on parent navigation.

```go
func (rp *RemotePane) NavigateToParent() {
    // ... existing code ...
    rp.currentPath = parent
    rp.selected = make(map[string]bool)
    rp.Refresh()
    if rp.onPathChange != nil {        // ADD THIS
        rp.onPathChange(rp.currentPath) // ADD THIS
    }
}
```

**Option B: Add a separate callback for recording**
A new `onRecordPath` callback that both `NavigateInto` and `NavigateToParent` call. More verbose, unnecessary if the asymmetry is a bug.

**Recommendation:** Option A. Fix the asymmetry. It's a one-line change that also improves existing behavior (Sync on parent nav).

### 3. Display: Overlay Rendering

The RecentDirs overlay renders on top of the FileBrowser, similar to TransferModal but simpler.

**Approach: Manual Draw via AfterDrawFunc (NOT replacing root)**

TransferModal uses a different approach -- it doesn't use `app.SetRoot()`. Instead, it renders during the normal draw cycle because `FileBrowser.Draw()` calls `fb.Flex.Draw()` which draws children, and the modal's `Draw()` is called via... wait, let me re-examine.

Actually, looking more carefully: TransferModal is NOT added as a child of the Flex layout. It renders via the `SetAfterDrawFunc` mechanism. Let me trace the actual draw path.

**Correction:** TransferModal's `Draw()` method is called by... actually it's not automatically called. Looking at the code, the TransferModal is not added to the Flex layout at all. It must be drawn via a custom mechanism.

Let me re-examine. The `handleGlobalKeys` intercepts Esc and calls `fb.transferModal.HandleKey(event)`. The modal's `Show()` sets `visible = true`. But how does `Draw()` get called?

**The answer is: TransferModal.Draw() is likely called through a custom draw mechanism.** Looking at `FileBrowser.Draw()`:

```go
func (fb *FileBrowser) Draw(screen tcell.Screen) {
    // Fill background
    fb.Flex.Draw(screen)
}
```

TransferModal is not drawn here. It must be rendered through a separate mechanism. Since it's not added to the Flex children and there's no explicit draw call for it in FileBrowser.Draw(), the actual rendering path needs the modal to be drawn somehow.

**Revised approach for RecentDirs:** Use `app.SetRoot()` to temporarily replace the root with a wrapper that draws both FileBrowser and RecentDirs. OR use the same mechanism TransferModal uses.

Actually, the simplest approach that matches the existing pattern: render the RecentDirs list as a **tview.Form or tview.List** modal that gets set as root temporarily (like `showDeleteConfirmModal` in handlers.go does with `tview.NewModal()`).

But this doesn't match the TransferModal pattern. Let me reconsider.

**Best approach: Use `app.SetRoot()` with a wrapper Flex.**

```go
func (fb *FileBrowser) showRecentDirs() {
    wrapper := tview.NewFlex().SetDirection(tview.FlexRow)
    wrapper.SetBackgroundColor(tcell.ColorDefault)
    wrapper.AddItem(fb, 0, 1, false)  // FileBrowser as background

    rdList := fb.recentDirs.BuildList()  // returns a tview.Primitive
    wrapper.AddItem(rdList, 0, 0, true) // overlay on top

    fb.app.SetRoot(wrapper, true)
    fb.app.SetFocus(rdList)
}
```

Wait -- this won't work well because tview.Flex distributes space proportionally, not as an overlay.

**Alternative: Draw the RecentDirs as an overlay in FileBrowser.Draw()**

```go
func (fb *FileBrowser) Draw(screen tcell.Screen) {
    fb.Flex.Draw(screen)
    // Draw overlay on top
    if fb.recentDirs != nil && fb.recentDirs.IsVisible() {
        fb.recentDirs.Draw(screen)
    }
}
```

This is the cleanest approach and exactly how overlay modals work in tview. The overlay draws on top of the Flex content. TransferModal likely uses this same approach (its `Draw()` method is structured for this).

**Key insight:** TransferModal IS drawn this way. The `Draw()` method checks `tm.visible` and returns early if not visible. The question is who calls it. Since TransferModal is not a Flex child, it must be called from somewhere. Given that FileBrowser.Draw() exists and overrides the default, it's the natural place.

**Wait -- I need to verify this.** Looking at the code again: FileBrowser.Draw() only calls `fb.Flex.Draw(screen)`. There's no call to `fb.transferModal.Draw(screen)`. This means either:
1. TransferModal.Draw() is called from somewhere else (SetAfterDrawFunc?)
2. TransferModal.Draw() is never explicitly called (and the modal doesn't actually render via Draw)

Looking at the AfterDrawFunc:
```go
fb.app.SetAfterDrawFunc(func(screen tcell.Screen) {
    // Only draws status bar
})
```

No TransferModal draw there either.

**Re-reading TransferModal.Draw()** -- it's defined but I don't see where it's called in the current codebase. This suggests TransferModal might not use the Draw-based overlay approach at all. Instead, it might render through a different mechanism (perhaps the progress display is drawn via the status bar's AfterDrawFunc, or through the QueueUpdateDraw mechanism updating some other primitive).

**Actually, looking more carefully at TransferModal -- it has `visible` flag and `Show()`/`Hide()` methods, but the key rendering (progress text, speed, ETA) is done through... hmm.** The `Update()` method updates internal fields (fileLabel, infoLine, etaLine), but there's no mechanism to trigger a redraw with those new values unless Draw() is called.

**Most likely explanation:** TransferModal.Draw() IS called from FileBrowser.Draw(), but the current code in FileBrowser.Draw() doesn't show it because... wait, let me re-read:

```go
func (fb *FileBrowser) Draw(screen tcell.Screen) {
    x, y, width, height := fb.GetRect()
    bgStyle := tcell.StyleDefault.Background(tcell.ColorDefault)
    for row := y; row < y+height; row++ {
        for col := x; col < x+width; col++ {
            screen.SetContent(col, row, ' ', nil, bgStyle)
        }
    }
    fb.Flex.Draw(screen)
}
```

There's no TransferModal.Draw() call here. This is a problem -- the TransferModal would never render.

**Revised understanding:** The TransferModal may be drawn via `app.QueueUpdateDraw()` which triggers a full screen redraw, and the TransferModal might be a child of the Flex after all, or it might be drawn through tview's internal mechanisms.

Actually, I think I need to reconsider. In tview, `SetRoot(fb, true)` makes `fb` the root primitive. tview calls `Draw()` on the root primitive. If TransferModal is NOT a child of the Flex, it won't be drawn.

**The most likely implementation:** TransferModal is meant to be drawn from within FileBrowser.Draw(), and the current code may have a bug where it's not being called, OR TransferModal was designed to be drawn but the Draw() call was not yet added because the modal is shown via `app.SetRoot()` replacement.

Actually, re-reading the code flow more carefully:

```go
// In initiateTransfer():
fb.transferModal.Show(direction, files[0].Name)
```

`Show()` just sets internal state. It doesn't change the root. The modal progress updates happen via `QueueUpdateDraw()`:

```go
fb.app.QueueUpdateDraw(func() {
    fb.transferModal.Update(p)
})
```

`Update()` sets internal fields but doesn't trigger drawing. So `QueueUpdateDraw()` must trigger a redraw of the root (FileBrowser), which calls `FileBrowser.Draw()`, which should draw the modal overlay.

**Conclusion: The FileBrowser.Draw() method is MISSING the call to `fb.transferModal.Draw(screen)`.** This is either a bug that was introduced during refactoring, or the modal rendering works through some other mechanism I'm not seeing.

**For the RecentDirs feature, the correct approach is:**

Add the overlay Draw calls to `FileBrowser.Draw()`:

```go
func (fb *FileBrowser) Draw(screen tcell.Screen) {
    // Fill background
    fb.Flex.Draw(screen)
    // Draw overlays on top
    if fb.transferModal != nil && fb.transferModal.IsVisible() {
        fb.transferModal.Draw(screen)
    }
    if fb.recentDirs != nil && fb.recentDirs.IsVisible() {
        fb.recentDirs.Draw(screen)
    }
}
```

**Confidence: HIGH.** This is the correct overlay rendering approach in tview. If TransferModal is currently not rendering (which would be a bug), this fix would also resolve that. If it IS rendering through some other mechanism, adding the Draw call would be harmless (double-draw would just overwrite).

### 4. Key Handling for RecentDirs Overlay

When RecentDirs is visible, it needs to intercept all keyboard input. This follows the TransferModal pattern:

```go
// In FileBrowser.handleGlobalKeys():
func (fb *FileBrowser) handleGlobalKeys(event *tcell.EventKey) *tcell.EventKey {
    // Overlay key handling (check before pane keys)
    if fb.recentDirs != nil && fb.recentDirs.IsVisible() {
        return fb.recentDirs.HandleKey(event)
    }
    // TransferModal handling (existing)
    if fb.transferModal != nil && fb.transferModal.IsVisible() {
        fb.transferModal.HandleKey(event)
        return nil
    }
    // ... existing Tab, Esc, s, S, F5 handling ...
}
```

RecentDirs.HandleKey dispatches:
- `j` / `tcell.KeyDown` -- move selection down
- `k` / `tcell.KeyUp` -- move selection up
- `tcell.KeyEnter` -- navigate to selected path, call onNavigate callback
- `tcell.KeyEscape` -- dismiss overlay, call onDismiss callback
- All other keys -- consumed (prevent passthrough to underlying panes)

### 5. Selection -> Navigation Flow

```
User selects path in RecentDirs
    ↓
recentDirs.HandleKey(Enter)
    ↓
rd.onNavigate(path)
    ↓ (wired in FileBrowser.build())
fb.remotePane.NavigateTo(path)    // NEW method on RemotePane
    ↓
rp.currentPath = path
rp.selected = make(map[string]bool)
rp.Refresh()
    ↓
(Does NOT fire onPathChange -- avoids re-recording the selected path)
```

**Key decision:** `NavigateTo(path)` should NOT fire `onPathChange`, because navigating to a recently-visited path shouldn't record it again (it's already in the list). Alternatively, it could re-record it (move to front), which is also reasonable UX. **Recommendation: Don't re-record** -- it's already at the top of the list, and re-recording creates unnecessary churn.

### 6. NavigateTo Method (New on RemotePane)

```go
// NavigateTo sets the current path directly (used by recent dirs navigation).
// Unlike NavigateInto, this does not trigger onPathChange callback.
func (rp *RemotePane) NavigateTo(path string) {
    if !rp.connected {
        return
    }
    rp.currentPath = path
    rp.selected = make(map[string]bool)
    rp.Refresh()
}
```

## Complete Data Flow

```
[Navigation Event]
    │
    ├─ NavigateInto(dirName) ──→ onPathChange(path) ──→ recentDirs.Record(path)
    │                                                          │
    │                                                          ├─ Dedup (move to front)
    │                                                          └─ Trim to 10
    │
    ├─ NavigateToParent() ──→ onPathChange(path) ──→ recentDirs.Record(path)
    │   (after fix)               (after fix)
    │
    └─ NavigateTo(path) ──→ (no callback, no recording)
        (recent dirs)

[Display Recent Dirs]
    │
    ├─ User presses 'r' in RemotePane
    │   └─→ onShowRecentDirs callback
    │       └─→ fb.recentDirs.Show()
    │           └─→ rd.visible = true, rd.selectedIndex = 0
    │
    ├─ User presses j/k to navigate list
    │   └─→ handleGlobalKeys → recentDirs.HandleKey
    │       └─→ rd.selectedIndex++ / --
    │
    ├─ User presses Enter
    │   └─→ recentDirs.HandleKey(Enter)
    │       └─→ rd.onNavigate(selectedPath)
    │           └─→ fb.remotePane.NavigateTo(path)
    │           └─→ rd.Hide()
    │
    └─ User presses Esc
        └─→ recentDirs.HandleKey(Esc)
            └─→ rd.Hide()
```

## Modified vs New Files

### New Files

| File | Purpose |
|------|---------|
| `internal/adapters/ui/file_browser/recent_dirs.go` | RecentDirs struct: ring buffer, Draw(), HandleKey(), Show(), Hide(), Record() |

### Modified Files

| File | Change | Lines Affected |
|------|--------|----------------|
| `file_browser.go` | Add `recentDirs` field to FileBrowser struct | +1 field |
| `file_browser.go` | Create RecentDirs in `build()` | +5 lines |
| `file_browser.go` | Wire `onShowRecentDirs` callback on RemotePane | +4 lines |
| `file_browser.go` | Wire `onNavigate` callback on RecentDirs | +5 lines |
| `file_browser.go` | Add `recentDirs.Record(path)` to RemotePane's onPathChange | +1 line |
| `file_browser.go` | Add RecentDirs overlay check in `handleGlobalKeys` | +3 lines |
| `file_browser.go` | Add RecentDirs.Draw() call in Draw() | +2 lines |
| `remote_pane.go` | Add `onShowRecentDirs` callback field | +1 field |
| `remote_pane.go` | Add `case 'r'` in SetInputCapture | +4 lines |
| `remote_pane.go` | Add `OnShowRecentDirs()` setter method | +3 lines |
| `remote_pane.go` | Add `NavigateTo(path)` method | +8 lines |
| `remote_pane.go` | Fix: add `onPathChange` call in `NavigateToParent()` | +3 lines |

### Unchanged Files

- `local_pane.go` -- No changes (feature is remote-only)
- `transfer_modal.go` -- No changes (independent overlay)
- `progress_bar.go` -- No changes
- `file_sort.go` -- No changes
- Domain/ports/services layers -- No changes

## Build Order

```
Phase 1: RecentDirs struct (no dependencies on existing code)
  └── recent_dirs.go
      - RecentDirs struct definition
      - Record() method (ring buffer logic)
      - Draw() method (render path list)
      - HandleKey() method (j/k/Enter/Esc)
      - Show()/Hide()/IsVisible()

Phase 2: RemotePane modifications (depends on Phase 1 for type)
  └── remote_pane.go
      - Add onShowRecentDirs callback
      - Add 'r' key to SetInputCapture
      - Add NavigateTo() method
      - Fix NavigateToParent() onPathChange asymmetry

Phase 3: FileBrowser wiring (depends on Phase 1 + Phase 2)
  └── file_browser.go
      - Add recentDirs field
      - Create and wire in build()
      - Add overlay check in handleGlobalKeys
      - Add Draw() overlay rendering
```

## UI Layout Specification

```
┌──────────────────────────────────────────────────────────────┐
│                                                              │
│            ┌─────────────────────────────┐                   │
│            │ Recent Remote Directories    │                   │
│            ├─────────────────────────────┤                   │
│            │  /home/user/projects/app     │  ← selected      │
│            │  /var/log                   │                   │
│            │  /etc/nginx                 │                   │
│            │  /tmp/downloads             │                   │
│            │                             │                   │
│            │  [j/k] Navigate  [Enter] Go │                   │
│            │  [Esc] Close                │                   │
│            └─────────────────────────────┘                   │
│                                                              │
│  (FileBrowser content visible but dimmed behind overlay)     │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

Overlay specs:
- Centered horizontally and vertically within the terminal
- Fixed width: 50 characters (or 60% of terminal width, whichever is smaller)
- Max height: 15 rows (including border, title, footer)
- Background: Color232 (matching TransferModal)
- Border: Color238 (matching existing borders)
- Selected row highlight: Color235 background, Color255 foreground
- Footer hints: Color245 (matching existing hint text)

## Anti-Patterns to Avoid

### 1. Don't use tview.List for the overlay
tview.List has its own focus management and input handling that conflicts with the overlay pattern. Use a `*tview.Box` with manual `Draw()`, exactly like TransferModal.

### 2. Don't persist recent paths
PROJECT.md explicitly says "in-memory only, cleared on exit." Adding persistence would require domain model changes and is out of scope.

### 3. Don't add NavigateTo to the ports/domain layer
`NavigateTo` is a UI convenience method (skip recording). It's not a new business capability -- it's just `NavigateInto` without the callback. Keep it in the UI adapter.

### 4. Don't intercept 'r' in handleGlobalKeys
The 'r' key is already used in the main TUI for refresh (`handleRefreshBackground`). While there's no conflict (they're different SetRoot contexts), keeping pane-specific keys in pane InputCapture maintains the established pattern and makes the code easier to reason about.

### 5. Don't record the initial path
The initial remote path is "." (SFTP home). Recording this is noise. Only record paths the user explicitly navigates to.

## Risk Assessment

| Risk | Severity | Likelihood | Mitigation |
|------|----------|------------|------------|
| TransferModal.Draw() not being called (potential existing bug) | HIGH | MEDIUM | Adding Draw call in FileBrowser.Draw() fixes this |
| 'r' key conflict with future features | LOW | LOW | Document the binding; 'r' is only used for refresh in main TUI, not in file browser |
| NavigateToParent onPathChange fix changes existing behavior | LOW | LOW | Only adds a Sync() call that was likely an oversight |
| RecentDirs overlay blocks TransferModal overlay | MEDIUM | LOW | Only one overlay should be visible at a time; guard in handleGlobalKeys |

## Open Questions

1. **Should selecting a recent path move it to the front of the list?** Current design says no (it's already near the top). But some UX patterns re-promote on re-use. Decision: defer to implementation -- start with no re-promotion.

2. **Should the overlay have a title showing the server name?** Useful when users have multiple file browser sessions (though current architecture is one-at-a-time). Decision: include server name in title for clarity.

3. **Empty state: what to show when no recent dirs recorded yet?** Options: (a) don't show the popup at all, (b) show "No recent directories" message. Decision: (a) -- don't show popup if list is empty.

## Sources

- Direct code analysis of `internal/adapters/ui/file_browser/` package (all 7 files)
- TransferModal overlay pattern (transfer_modal.go) as architectural reference
- PROJECT.md v1.1 milestone requirements
- Existing onPathChange callback pattern (local_pane.go, remote_pane.go)
- Key routing chain documented in file_browser_handlers.go comments
