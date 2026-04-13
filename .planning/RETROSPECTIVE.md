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

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Sessions | Phases | Key Change |
|-----------|----------|--------|------------|
| v1.0 | 1 | 3 | Initial project, established patterns |

### Cumulative Quality

| Milestone | Tests | Key Quality Metric |
|-----------|-------|-------------------|
| v1.0 | 23 | go vet clean, all platforms compile |

### Top Lessons (Verified Across Milestones)

1. Port/Adapter decoupling enables fast feature iteration without cross-layer regressions
2. Plan checking before execution prevents scope gaps
3. Context cancellation should be designed in from the start, not bolted on later
