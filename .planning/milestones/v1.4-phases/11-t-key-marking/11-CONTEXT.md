# Phase 11: T Key Marking - Context

**Gathered:** 2026-04-15
**Status:** Ready for planning

<domain>
## Phase Boundary

在服务器列表中按 T 键（Shift+t）依次标记两个服务器为源端和目标端，标记完成后自动打开双远端文件浏览器。包含标记状态管理、视觉反馈、Esc 清除、同一服务器防护。不包含双远端浏览器本身的实现（Phase 12）。

</domain>

<decisions>
## Implementation Decisions

### 快捷键
- **D-01:** 使用 `T` (Shift+t) 触发标记，`t` (lowercase) 已被 handleTagsEdit() 占用。在 handleGlobalKeys switch 中新增 `case 'T'`，与 `case 'D'` (dup) 模式一致。

### 视觉呈现
- **D-02:** 标记前缀使用文本 `[S]`（绿色 tcell.ColorGreen，源端）和 `[T]`（蓝色 tcell.ColorBlue，目标端），在 formatServerLine() 的 primary 文本最前面添加。与现有 emoji 前缀（📌/📡）和转发标记（Ⓕ）并排显示。

### 标记流程
- **D-03:** Esc 优先清除标记状态（如有标记，清除并刷新列表、显示提示；再次按 Esc 返回搜索栏）。需要修改 ServerList 的 InputCapture 或在 handleGlobalKeys 中拦截 Esc。
- **D-04:** 标记同一服务器两次时显示红色错误提示 "Cannot mark same server twice"，标记状态不变，用户需选择另一台服务器。

### 自动打开浏览器
- **D-05:** 两个服务器标记完成后立即自动调用双远端浏览器。Phase 11 先用占位函数 `handleDualRemoteBrowser(source, target domain.Server)` + TODO 注释，Phase 12 填充实现。状态栏提示 "Opening dual remote browser..." 后清除标记。

### 标记状态存储
- **D-06:** 标记状态存储在 `tui` struct 上（而非 ServerList），因为需要跨组件访问（handlers.go 写入，formatServerLine 读取）。结构建议：`markSource *domain.Server` + `markTarget *domain.Server`，nil 表示未标记。

### Claude's Discretion
- 标记状态的具体字段命名（markSource/markTarget 或其他）
- formatServerLine() 接收标记状态的参数传递方式（函数参数 vs 全局/闭包访问）
- 状态栏提示文本的具体措辞
- 标记完成后清除标记的时机（打开浏览器前清除 vs 浏览器关闭后清除）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` — MARK-01~MARK-05 需求定义

### Server entity and service
- `internal/core/domain/server.go` — Server struct，标记状态需引用 Server 实例
- `internal/core/services/server_service.go` — ServerService 接口
- `internal/core/ports/services.go` — ServerService/ServerRepository 接口定义

### UI handlers and key routing
- `internal/adapters/ui/handlers.go` — handleGlobalKeys switch（T 键插入位置，第 71 行 switch）
- `internal/adapters/ui/handlers.go:108` — `case 't': handleTagsEdit()`（已占用，不可覆盖）
- `internal/adapters/ui/tui.go:29` — tui struct 定义（标记状态字段添加位置）

### Server list rendering
- `internal/adapters/ui/server_list.go` — ServerList 组件，UpdateServers() 调用 formatServerLine()
- `internal/adapters/ui/utils.go:84` — formatServerLine() 函数（标记前缀渲染位置）
- `internal/adapters/ui/utils.go:76` — pinnedIcon() 参考（emoji 前缀模式）

### Esc handling
- `internal/adapters/ui/server_list.go:57-67` — ServerList InputCapture 拦截 Esc（返回搜索栏），标记模式下需优先清除标记

### Conventions
- `.planning/codebase/CONVENTIONS.md` — 命名、代码风格、错误处理模式
- `.planning/codebase/STRUCTURE.md` — 项目结构和文件组织

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `handleGlobalKeys()` switch — 已有 `case 'D'` (dup) 模式，`case 'T'` 完全复用相同模式
- `showStatusTemp()` / `showStatusError()` — 状态栏提示，标记反馈直接复用
- `t.serverList.GetSelectedServer()` — 获取当前选中服务器
- `t.refreshServerList()` — 刷新列表（标记后需调用以重新渲染 formatServerLine）
- `formatServerLine()` — 已有 emoji 前缀（📌/📡）和转发标记（Ⓕ），标记前缀遵循相同模式

### Established Patterns
- handler 函数模式：`handleServerXxx()` 私有方法，获取选中服务器 → 操作 → 刷新列表
- 键盘路由：handleGlobalKeys 中 switch 分发，返回 nil 表示已消费
- 状态栏反馈：成功用 showStatusTemp（绿色），错误用 showStatusError（红色）
- Esc 在 ServerList InputCapture 中被拦截用于返回搜索栏

### Integration Points
- `handleGlobalKeys` switch（handlers.go:71）— 新增 `case 'T'` 调用 handleServerMark
- `tui struct`（tui.go:29）— 添加 markSource/markTarget 字段
- `formatServerLine()`（utils.go:84）— 需要接收标记状态参数以渲染 [S]/[T] 前缀
- `ServerList.UpdateServers()`（server_list.go:70）— 调用 formatServerLine，可能需要传递标记状态
- ServerList InputCapture（server_list.go:57）— Esc 拦截，标记模式下需修改行为
- 新增 `handleDualRemoteBrowser()` 占位函数（handlers.go）— Phase 12 实现

</code_context>

<specifics>
## Specific Ideas

- 标记状态存储在 tui struct 而非 ServerList，因需跨组件访问（handlers 写入，formatServerLine 读取）
- formatServerLine() 可通过增加参数或闭包方式接收标记状态
- 双远端浏览器占位函数：`func (t *tui) handleDualRemoteBrowser(source, target domain.Server) { /* TODO: Phase 12 */ }`
- 标记流程状态机：idle → source_marked → target_marked → open_browser → idle

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 11-t-key-marking*
*Context gathered: 2026-04-15*
