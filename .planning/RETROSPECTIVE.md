# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.0 — File Transfer

**Shipped:** 2026-04-13
**Phases:** 3 | **Plans:** 9 | **Tasks:** 24

### What Was Built
- Dual-pane file browser (tview.Table, keyboard navigation, 4-column display)
- SFTP transfer engine (32KB buffered copy, progress callbacks, directory recursive transfer)
- Progress UI (Unicode progress bar, sliding-window speed/ETA, TransferModal overlay)
- Transfer cancellation (context.Context propagation, double-Esc confirmation, D-04 partial file cleanup)
- File conflict resolution (Overwrite/Skip/Rename, buffered channel goroutine sync)
- Cross-platform compatibility (build tags for Unix/Windows permissions, 3-platform compilation)

### What Worked
- Clean Architecture layering kept UI and business logic decoupled — TransferService changes (Phase 3) didn't require UI restructuring
- Port/Adapter pattern made SFTP mock testing straightforward — unit tests for cancellation and conflict logic were easy to write
- Plan iteration (checker feedback on Phase 3 plans) caught missing details before execution
- Worktree isolation for parallel agents prevented merge conflicts

### What Was Inefficient
- Phase 2 plan revision after initial planning — verifier feedback required re-planning, adding overhead
- tcell API incompatibility (ColorRGBTo256 doesn't exist in v2) — discovered during execution, not planning
- REQUIREMENTS.md TRAN-06 checkbox was missed during execution — verification caught it but it shouldn't have been missed
- Worktree merge needed manual STATE.md/ROADMAP.md syncing — orchestrator overhead for worktree cleanup

### Patterns Established
- onFileAction callback for pane-to-orchestrator event flow
- context.WithCancel lifecycle in TUI goroutines
- TransferModal multi-mode state machine (progress/cancelConfirm/conflictDialog/summary)
- Build tags for platform-specific permission handling
- ConflictHandler callback + buffered channel for goroutine-UI synchronization

### Key Lessons
1. Verify requirement checkboxes match implementation status before milestone completion
2. Check API compatibility (tcell/v2) during research phase, not execution
3. Worktree isolation adds cleanup overhead — consider inline execution for sequential phases

### Cost Observations
- Model mix: 100% sonnet (executor agents), 100% opus (orchestrator/verifier)
- Sessions: ~1 (single-day milestone)
- Notable: Entire milestone completed in 1 day with 3 sequential wave executions

## Milestone: v1.1 — Recent Remote Directories

**Shipped:** 2026-04-14
**Phases:** 2 | **Plans:** 3 | **Tasks:** 5

### What Was Built
- In-memory MRU directory list with move-to-front dedup, relative path filtering, 10-entry cap
- NavigateToParent onPathChange callback fix (symmetric navigation)
- NavigateTo silent navigation method (popup selection without re-recording)
- Centered popup overlay with j/k navigation, current-path yellow highlighting
- Empty state ("暂无最近目录") with minimum height safeguard
- TransferModal.Draw() pre-existing rendering bug fix via overlay draw chain

### What Worked
- 2-phase split (data + UI) kept concerns cleanly separated — Phase 4 data layer was testable in isolation
- TransferModal overlay pattern provided a proven reference implementation for RecentDirs
- RESEARCH.md pitfall analysis caught the TransferModal.Draw() never-called bug before execution
- UI-SPEC design contract eliminated ambiguity in color/layout decisions

### What Was Inefficient
- TransferModal.Draw() not-called bug was a pre-existing issue from v1.0 — should have been caught earlier
- RecentDirs needed SetCurrentPath decoupling (not discovered during research) — added during execution

### Patterns Established
- SetCurrentPath() string injection for overlay-to-pane decoupling
- Full key interception pattern (HandleKey returns nil for ALL keys when popup visible)
- Overlay draw chain in FileBrowser.Draw() (multiple overlays, priority order)

### Key Lessons
1. Pre-existing bugs in overlay rendering may go unnoticed without dedicated audit — check all overlay Draw() calls after adding new ones
2. SetCurrentPath() decoupling avoids tight coupling between overlay components and panes
3. Empty state minimum height (5 cells) prevents zero-size popup rendering artifacts

### Cost Observations
- Model mix: sonnet (executor), opus (orchestrator/verifier)
- Sessions: 1 (single-day milestone)
- Notable: Small milestone (2 phases) completed efficiently with research + UI-spec front-loading

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Sessions | Phases | Key Change |
|-----------|----------|--------|------------|
| v1.0 | 1 | 3 | Initial project, established patterns |
| v1.1 | 1 | 2 | Small focused milestone, overlay pattern reuse |

### Cumulative Quality

| Milestone | Tests | Key Quality Metric |
|-----------|-------|-------------------|
| v1.0 | 23 | go vet clean, all platforms compile |
| v1.1 | 28 | +5 tests, TransferModal.Draw() bug fix |

### Top Lessons (Verified Across Milestones)

1. Port/Adapter decoupling enables fast feature iteration without cross-layer regressions
2. Plan checking before execution prevents scope gaps
3. Context cancellation should be designed in from the start, not bolted on later
