# Phase 3: Polish - Context

**Gathered:** 2026-04-13
**Status:** Ready for planning

<domain>
## Phase Boundary

在 Phase 1 和 Phase 2 建立的双栏文件浏览器和文件传输基础上，添加三个关键的可靠性能力：(1) 传输取消支持 — 用户可以中断正在进行的传输；(2) 文件冲突处理 — 目标文件已存在时提供 overwrite/skip/rename 选择；(3) 跨平台兼容 — 确保 Windows/macOS/Linux 上正常工作。此阶段不添加新功能，只提升现有功能的健壮性。

</domain>

<decisions>
## Implementation Decisions

### Cancel Mechanism
- **D-01:** 使用 `context.Context` 传播取消信号 — 添加到所有 TransferService 方法签名（UploadFile, DownloadFile, UploadDir, DownloadDir）
- **D-02:** `copyWithProgress` 循环中检查 `ctx.Done()`，收到取消信号时中断 io.Copy
- **D-03:** Esc 键第一次按下显示 "Cancel transfer? (y/n)" 确认提示，第二次按下确认取消（防止误操作）
- **D-04:** 取消后总是删除目标侧的部分文件（不留 orphaned half-files）
- **D-05:** 取消操作不需要关闭 SFTP 连接 — 连接可以复用，只取消当前传输 goroutine

### Conflict Resolution
- **D-06:** 传输前检查目标文件是否存在（Stat），存在则暂停传输、弹出冲突对话框
- **D-07:** 冲突对话框提供三个选项：Overwrite / Skip / Rename（自动添加 .1, .2 后缀）
- **D-08:** 冲突对话框显示在 TransferModal 区域内（替换进度显示），不切换到单独的 view
- **D-09:** 目录传输中每个冲突文件单独提示（非 apply-all），用户可以为每个文件选择不同操作

### Cross-Platform Compatibility
- **D-10:** 使用 Go build tags（`file_windows.go`, `file_unix.go`）处理平台差异，不使用 runtime.GOOS 散弹枪检查
- **D-11:** 路径处理：本地路径统一使用 `filepath.Join`/`filepath.Clean`，远程路径使用 `path.Join`（Unix 风格，SFTP 标准）
- **D-12:** 符号链接：默认跟随符号链接（follow symlinks），不做符号链接保留（保留需要额外复杂度）
- **D-13:** 文件权限：传输时尝试设置权限（chmod），但如果目标系统不支持（如 Windows NTFS）则静默忽略错误
- **D-14:** 显示格式：文件大小使用 `humanize` 风格自动切换（B/KB/MB/GB），日期使用 locale-independent 格式

### Claude's Discretion
- context.Context 的 WithTimeout 值（是否需要超时保护）
- Rename 的具体后缀格式（.1, .2 vs _copy vs timestamp）
- 冲突对话框的精确布局和颜色
- 符号链接检测的具体实现方式
- Windows 上 filepath.ToSlash 的具体调用位置
- 文件权限失败时的日志级别（warn vs debug）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 2 Artifacts (Core Transfer — what we're making more robust)
- `internal/core/ports/transfer.go` — TransferService interface (needs context.Context added)
- `internal/adapters/data/transfer/transfer_service.go` — TransferService implementation (needs cancel support in copyWithProgress)
- `internal/adapters/ui/file_browser/transfer_modal.go` — TransferModal (needs conflict dialog mode)
- `internal/adapters/ui/file_browser/progress_bar.go` — ProgressBar renderer
- `internal/core/domain/transfer.go` — TransferProgress domain type
- `internal/adapters/data/sftp_client/sftp_client.go` — SFTPClient (remote I/O methods need context)
- `internal/adapters/ui/file_browser/file_browser.go` — FileBrowser (initiateTransfer, initiateDirTransfer need cancel wiring)

### Phase 1 Artifacts (Foundation — existing patterns)
- `internal/adapters/ui/file_browser/file_browser_handlers.go` — Keyboard handlers (Esc handling for cancel)
- `internal/adapters/ui/tui.go` — TUI struct, QueueUpdateDraw pattern
- `internal/adapters/ui/handlers.go` — Global key handling pattern
- `internal/adapters/ui/status_bar.go` — StatusBar component

### Requirements
- `.planning/REQUIREMENTS.md` — Phase 3 requirements: TRAN-06, TRAN-07, INTG-03

### Research
- `.planning/phases/01-foundation/01-RESEARCH.md` — pkg/sftp library usage, SFTP connection patterns
- `.planning/phases/02-core-transfer/02-RESEARCH.md` — Transfer service design, progress callback patterns

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `TransferModal.SetDismissCallback()` — Already exists, currently no-op for cancel. Extend to trigger context cancellation.
- `TransferModal.Show()/Update()/Hide()` — Modal lifecycle. Conflict dialog can reuse Show() with different mode.
- `app.QueueUpdateDraw()` — Thread-safe UI updates from goroutines. Essential for conflict prompts during transfer.
- `copyWithProgress()` — The 32KB buffered copy loop. Needs `ctx.Done()` check between chunks.
- `domain.TransferProgress{Failed, FailError}` — Already has failure reporting. Cancel can set these fields.

### Established Patterns
- Background operations: goroutine + `app.QueueUpdateDraw()`
- Modal dialogs: TransferModal overlay pattern from Phase 2
- Error display: `showStatusTempColor(msg, "#FF6B6B")` for red errors
- Theme: Dark theme with tcell.Color232-250 palette

### Integration Points
- `ports/transfer.go` — All 4 method signatures need `ctx context.Context` as first parameter
- `transfer_service.go:301` — `copyWithProgress()` needs ctx.Done() check in the copy loop
- `file_browser.go:200-290` — `initiateTransfer()` and `initiateDirTransfer()` need context creation and cancel wiring
- `file_browser_handlers.go:23` — Esc handler needs cancel confirmation flow
- `transfer_modal.go` — Needs new conflict dialog mode alongside existing progress mode
- `sftp_client.go` — `CreateRemoteFile`, `OpenRemoteFile` may need context for cancel
- `local_fs.go` — Local file operations may need context for cancel

### Key Constraints
- tview is single-threaded — all UI updates MUST go through `app.QueueUpdateDraw()`
- SFTP operations run in goroutines — cancel signal must cross goroutine boundaries
- TransferService is behind a port interface — interface change affects mock tests
- Build tags for platform code add files but shouldn't change public API

</code_context>

<specifics>
## Specific Ideas

- 参考 Midnight Commander 的传输取消行为：按 Esc 弹出确认对话框
- 冲突对话框类似 Midnight Commander 的 "Overwrite / Skip / Rename / Append" 选择
- Windows 路径处理参考 Go 标准库 filepath 包的设计哲学
- 符号链接跟随行为参考 scp 默认行为（recursive 模式下跟随）

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 03-polish*
*Context gathered: 2026-04-13*
