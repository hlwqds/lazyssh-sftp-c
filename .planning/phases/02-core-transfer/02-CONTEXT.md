# Phase 2: Core Transfer - Context

**Gathered:** 2026-04-13
**Status:** Ready for planning

<domain>
## Phase Boundary

在 Phase 1 建立的双栏文件浏览器基础上，添加实际的文件传输能力：用户可以通过 SFTP 浏览远程文件，在本地和远程之间上传/下载单个文件和整个目录，并看到详细的传输进度。此阶段不包含传输取消（Phase 3）、文件冲突处理（Phase 3）和跨平台验证（Phase 3）。

</domain>

<decisions>
## Implementation Decisions

### Transfer Trigger
- **D-01:** `Enter` 键在文件上触发传输（不是目录）。方向由当前焦点 pane 决定：本地 pane 焦点 → 上传（local→remote），远程 pane 焦点 → 下载（remote→local）
- **D-02:** 多选文件（`Space` 标记）一起传输，方向规则同上
- **D-03:** `Enter` 在目录上不触发传输（目录递归传输使用专用快捷键）
- **D-04:** 目录递归传输使用 `F5` 键触发，方向规则与文件相同

### Progress Display
- **D-05:** 传输进度通过 `tview.Modal` 覆盖层显示（居中弹出），不使用 status bar
- **D-06:** Modal 显示完整信息：当前文件名、进度条（`tview.ProgressBar`）、传输速度、ETA、百分比
- **D-07:** 多文件传输显示逐文件进度（当前文件的进度），不是总体进度
- **D-08:** `Esc` 键在进度 modal 中取消传输（取消逻辑在 Phase 3 实现，此阶段先预留接口）

### Directory Transfer
- **D-09:** `F5` 在选中的目录上触发递归传输（上传或下载整个目录树）
- **D-10:** 目录传输遇到错误时跳过失败文件继续传输，不中止整个操作
- **D-11:** 目录传输完成后显示摘要（"Transferred 8/10 files, 2 failed"），列出失败文件

### Post-Transfer Behavior
- **D-12:** 传输完成后自动刷新目标 pane（上传后刷新远程 pane，下载后刷新本地 pane）
- **D-13:** 多文件传输完成后，目标 pane 滚动到第一个传输文件的位置

### Claude's Discretion
- 进度条样式和颜色方案（遵循现有 theme）
- 速度显示格式（MB/s vs KB/s 自动切换）
- ETA 计算方式（滑动平均 vs 瞬时速度）
- Modal 的具体布局和边框样式
- 目录传输时先计算总文件数还是边传输边计数

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 1 Artifacts (Foundation)
- `internal/adapters/ui/file_browser/file_browser.go` — FileBrowser root component, dual-pane layout, StatusBar integration
- `internal/adapters/ui/file_browser/local_pane.go` — LocalPane: file listing, SetSelectedFunc (Enter key), multi-select, Refresh()
- `internal/adapters/ui/file_browser/remote_pane.go` — RemotePane: SFTP connection lifecycle, same API as LocalPane
- `internal/adapters/ui/file_browser/file_browser_handlers.go` — Global keyboard handlers (Tab/Esc/s/S)
- `internal/adapters/ui/file_browser/file_sort.go` — FileSortMode enum, sortFileEntries utility
- `internal/core/ports/file_service.go` — FileService/SFTPService port interfaces
- `internal/core/domain/file_info.go` — FileInfo domain entity
- `internal/adapters/data/sftp_client/sftp_client.go` — SFTPClient adapter (Connect, Close, ListDir, HomeDir)
- `internal/adapters/data/sftp_client/ssh_args.go` — buildSSHArgs utility
- `internal/adapters/data/local_fs/local_fs.go` — LocalFS adapter (ListDir)

### Existing TUI Patterns
- `internal/adapters/ui/tui.go` — Main TUI struct, app.QueueUpdateDraw() pattern, SetRoot/returnToMain
- `internal/adapters/ui/handlers.go` — Global key handling pattern, showStatusTempColor()
- `internal/adapters/ui/status_bar.go` — StatusBar component pattern
- `cmd/main.go` — Dependency injection pattern

### Research
- `.planning/phases/01-foundation/01-RESEARCH.md` — pkg/sftp library usage, SFTP connection patterns, tview.Table navigation
- `.planning/phases/01-foundation/01-UI-SPEC.md` — Color tokens, layout patterns, copywriting, keyboard bindings

### Requirements
- `.planning/REQUIREMENTS.md` — Phase 2 requirements: BROW-02, UI-06, TRAN-01, TRAN-02, TRAN-03, TRAN-04, TRAN-05

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `RemotePane` — Already implements remote SFTP directory browsing with ListDir, navigation, sorting, filtering. Phase 2 extends it with transfer capability.
- `LocalPane.SetSelectedFunc()` — Currently handles Enter on directories (NavigateInto). Phase 2 extends it to handle Enter on files (initiate upload).
- `FileBrowser.statusBar` — Available for quick status messages, but progress display uses separate Modal per D-05.
- `app.QueueUpdateDraw()` — Essential pattern for all transfer progress updates from goroutines.
- `domain.FileInfo{IsDir}` — Already distinguishes files from directories, used to route Enter key behavior.

### Established Patterns
- Background operations: goroutine + `app.QueueUpdateDraw()` for thread-safe UI updates
- Modal dialogs: not yet used in file browser, but tview.Modal is available
- Error display: `showStatusTempColor(msg, "#FF6B6B")` for red error messages
- Theme: Dark theme with tcell.Color232-250 palette
- Keyboard handling: `SetInputCapture` for custom keys, pass-through to Table for built-in keys

### Integration Points
- `local_pane.go:88-102` — SetSelectedFunc: extend to handle `!fi.IsDir` case (file transfer)
- `remote_pane.go:91-100` — SetSelectedFunc: extend to handle `!fi.IsDir` case (file transfer)
- `file_browser_handlers.go` — Add F5 handler for directory transfer
- `sftp_client.go` — Add UploadFile, DownloadFile, UploadDir, DownloadDir methods
- `file_service.go` — Extend SFTPService port interface with transfer methods

</code_context>

<specifics>
## Specific Ideas

- 进度 Modal 参考 Midnight Commander 的传输对话框风格：文件名 + 进度条 + 速度/ETA
- 遵循 Phase 1 的颜色方案：传输中绿色，错误红色，完成蓝色
- F5 选择参考 Midnight Commander 的 F5 复制快捷键
- 目录传输先遍历计算总文件数，然后逐文件传输并更新进度

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 02-core-transfer*
*Context gathered: 2026-04-13*
