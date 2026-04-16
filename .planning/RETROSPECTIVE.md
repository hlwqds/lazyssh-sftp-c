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

## Milestone: v1.2 — File Operations

**Shipped:** 2026-04-15
**Phases:** 3 | **Plans:** 7 | **Tasks:** 12

### What Was Built
- FileService interface with 5 file management methods (Remove/RemoveAll/Rename/Mkdir/Stat) implemented in both LocalFS and SFTPClient
- ConfirmDialog + InputDialog overlay components following RecentDirs/TransferModal pattern
- Dual-pane delete (single/multi-select/recursive), rename with conflict detection, mkdir with cursor positioning
- Local Copy/CopyDir with permission+mtime preservation, remote CopyRemoteFile/CopyRemoteDir via download+re-upload
- Clipboard copy/paste (c mark + p paste, [C] green prefix, TransferModal modeCopy)
- Move marking (x mark, [M] red prefix, modeMove, handlePaste refactor with conflict dialog for all operations)

### What Worked
- FileService as unified interface eliminated type-switching in UI layer — same code paths for local and remote
- clipboardProvider 4-tuple pattern made operation-aware rendering trivial across panes
- modeMove reusing drawProgress path from modeCopy — zero new rendering code for move progress
- TDD approach (RED+GREEN) for FileService extension caught interface satisfaction issues early

### What Was Inefficient
- SFTPClient missing Copy/CopyDir stubs broke compile-time interface check — discovered during build, not planning
- Test mocks needed updating across multiple test files when FileService interface extended
- sed-based editing for tab-indented code blocks that Edit tool couldn't match

### Patterns Established
- FileService as unified file operations interface for UI-layer uniformity
- clipboardProvider 4-tuple (bool, string, string, ClipboardOp) for operation-aware rendering
- Remote copy via download+re-upload with temp file/directory cleanup
- SFTP protocol stub methods returning sentinel errors for unsupported operations

### Key Lessons
1. Extending a Go interface requires updating ALL implementations including test mocks — plan for this
2. SFTP protocol limitations (no native copy) should be documented in research, not discovered during execution
3. Move and Copy share enough rendering infrastructure that modeMove should reuse modeCopy draw paths

### Cost Observations
- Model mix: sonnet (executor), opus (orchestrator/verifier)
- Sessions: ~2 (file operations + copy/clipboard + move split across sessions)
- Notable: 3-phase milestone with heavy interface extension work, mock updates were recurring overhead

---

## Milestone: v1.3 — Dup SSH Connection

**Shipped:** 2026-04-15
**Phases:** 1 | **Plans:** 1 | **Tasks:** 2

### What Was Built
- D key (Shift+d) server duplication with deep copy of all configuration fields
- generateUniqueAlias() with -copy, -copy-2, ... suffix logic avoiding conflicts
- dupPendingAlias pattern for post-save list auto-scroll to new entry
- Status bar and server details D key hints

### What Worked
- Single-plan milestone — minimal overhead, direct execution
- Reused existing ServerService.AddServer and ServerForm — no new components needed
- Deep copy of slice fields prevented shared reference bugs between original and duplicate

### What Was Inefficient
- None — plan executed exactly as written with zero deviations

### Patterns Established
- dupPendingAlias field on tui struct to bridge form open/save gap for auto-scroll

### Key Lessons
1. Small, well-scoped milestones (1 phase, 1 plan) can ship in minutes with zero friction
2. Pre-existing infrastructure (ServerService, ServerForm) makes new features trivial when architecture is clean

### Cost Observations
- Model mix: sonnet (executor), opus (orchestrator)
- Sessions: 1 (single session, 2 min execution)
- Notable: Fastest milestone yet — clean architecture and existing components made this trivial

---

## Milestone: v1.4 — Dup Fix & Dual Remote Transfer

**Shipped:** 2026-04-16
**Phases:** 4 | **Plans:** 5 | **Tasks:** ~13

### What Was Built
- Dup 修复：D 键从 3 步简化为 1 步直接保存，移除 ServerForm 中间步骤
- T 键标记状态机：源端 [S]/目标端 [T] 视觉前缀，Esc 清除，同服务器防护
- DualRemoteFileBrowser 独立组件：双 SFTP 并行连接，50:50 布局，文件操作
- RelayTransferService 端口+适配器：download→temp→upload 中继模式
- TransferModal modeCrossRemote：两阶段进度显示（下载→上传阶段切换）
- 跨远端剪贴板 c/x+p + F5 快速传输，冲突处理，移动回滚

### What Worked
- 独立组件策略（DualRemoteFileBrowser不复用FileBrowser）避免了15+ activePane二元假设
- 两个独立 SFTPClient 实例实现了真正的并行连接，错误隔离
- RelayTransferService 组合两个 transfer.New() 实例，零代码重复
- modeCrossRemote 复用 TransferModal 现有渲染，dlDone 标志切换阶段
- RESEARCH.md 的 D-01~D-05 pitfall 分析在执行前就预见了关键设计陷阱

### What Was Inefficient
- Plan 13-02 中 relaySvc 字段类型指定了不存在的导出类型（auto-fixed during execution）
- executeF5Transfer 中 srcSFTP 未使用变量（auto-fixed during execution）
- cmd.Stderr 重定向问题在 Phase 12 研究时就已识别但未在执行中验证

### Patterns Established
- 独立组件优于复用复杂组件（DualRemoteFileBrowser vs 扩展 FileBrowser）
- 中继传输模式：download(src, temp) → upload(temp, dst) + defer cleanup
- 两阶段进度回调：combinedProgress + dlDone bool 阶段检测
- 移动回滚模式：源删除失败→尝试清理目标→显示手动清理消息

### Key Lessons
1. 计划中引用不存在的导出类型会导致执行时 auto-fix——研究阶段应验证类型可访问性
2. 中继传输的临时文件清理必须在所有代码路径（成功/取消/错误）上用 defer 保证
3. 双远端场景中 clipboardProvider 需要在两个面板上都注册，而非仅在一个面板上

### Cost Observations
- Model mix: sonnet (executor), opus (orchestrator/verifier)
- Sessions: ~2 (Phase 10-11 + Phase 12-13 split)
- Notable: 4-phase milestone completed in ~1 day, 2 auto-fixed bugs in final plan

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Sessions | Phases | Key Change |
|-----------|----------|--------|------------|
| v1.0 | 1 | 3 | Initial project, established patterns |
| v1.1 | 1 | 2 | Small focused milestone, overlay pattern reuse |
| v1.2 | 2 | 3 | Heavy interface extension, file operations |
| v1.3 | 1 | 1 | Minimal milestone, clean architecture payoff |
| v1.4 | ~2 | 4 | Dup fix + dual remote transfer, relay pattern |

### Cumulative Quality

| Milestone | Tests | Key Quality Metric |
|-----------|-------|-------------------|
| v1.0 | 23 | go vet clean, all platforms compile |
| v1.1 | 28 | +5 tests, TransferModal.Draw() bug fix |
| v1.2 | ~35 | +7 tests, FileService unified interface |
| v1.3 | ~35 | Zero deviations, 2min execution |
| v1.4 | ~35 | +1532 LOC, relay pattern, 2 auto-fixed bugs |

### Top Lessons (Verified Across Milestones)

1. Port/Adapter decoupling enables fast feature iteration without cross-layer regressions
2. Plan checking before execution prevents scope gaps
3. Context cancellation should be designed in from the start, not bolted on later
4. Extending Go interfaces requires updating ALL implementations including test mocks
5. Clean architecture makes single-plan milestones trivial when reusing existing components
6. Small, well-scoped milestones reduce friction to near-zero
7. Independent components beat extending complex ones when activePane assumptions differ (v1.4)
8. Relay transfer pattern (download→temp→upload) enables cross-server operations without server-to-server SSH
