# Phase 11: T Key Marking - Research

**Researched:** 2026-04-15
**Domain:** tview TUI 状态管理与键盘事件处理
**Confidence:** HIGH

## Summary

Phase 11 是一个纯 UI 交互层改动，在服务器列表中添加 T 键标记功能。核心实现涉及三个层面：(1) 在 `tui` struct 上添加标记状态字段（`markSource`/`markTarget`），(2) 在 `handleGlobalKeys` 中新增 `case 'T'` 分发到 `handleServerMark`，(3) 修改 `formatServerLine` 以渲染 `[S]`/`[T]` 前缀。此外需要修改 ServerList 的 Esc 拦截逻辑，使其在标记模式下优先清除标记状态。

该 Phase 的复杂度主要在于 `formatServerLine` 的参数传递——当前函数签名 `formatServerLine(s domain.Server)` 是纯函数（仅依赖包级变量 `IsForwarding`），需要决定如何将标记状态传入。推荐方案是增加参数（`markSource`/`markTarget` 指针），在 `ServerList.UpdateServers` 中通过闭包或新方法传入。

**Primary recommendation:** 遵循现有 `case 'D'` (dup) 的 handler 模式，新增 `case 'T'` 调用 `handleServerMark`，标记状态存储在 `tui` struct 上，`formatServerLine` 增加标记参数。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** 使用 `T` (Shift+t) 触发标记，`t` (lowercase) 已被 handleTagsEdit() 占用。在 handleGlobalKeys switch 中新增 `case 'T'`，与 `case 'D'` (dup) 模式一致。
- **D-02:** 标记前缀使用文本 `[S]`（绿色 tcell.ColorGreen，源端）和 `[T]`（蓝色 tcell.ColorBlue，目标端），在 formatServerLine() 的 primary 文本最前面添加。与现有 emoji 前缀（📌/📡）和转发标记（Ⓕ）并排显示。
- **D-03:** Esc 优先清除标记状态（如有标记，清除并刷新列表、显示提示；再次按 Esc 返回搜索栏）。需要修改 ServerList 的 InputCapture 或在 handleGlobalKeys 中拦截 Esc。
- **D-04:** 标记同一服务器两次时显示红色错误提示 "Cannot mark same server twice"，标记状态不变，用户需选择另一台服务器。
- **D-05:** 两个服务器标记完成后立即自动调用双远端浏览器。Phase 11 先用占位函数 `handleDualRemoteBrowser(source, target domain.Server)` + TODO 注释，Phase 12 填充实现。状态栏提示 "Opening dual remote browser..." 后清除标记。
- **D-06:** 标记状态存储在 `tui` struct 上（而非 ServerList），因为需要跨组件访问（handlers.go 写入，formatServerLine 读取）。结构建议：`markSource *domain.Server` + `markTarget *domain.Server`，nil 表示未标记。

### Claude's Discretion
- 标记状态的具体字段命名（markSource/markTarget 或其他）
- formatServerLine() 接收标记状态的参数传递方式（函数参数 vs 全局/闭包访问）
- 状态栏提示文本的具体措辞
- 标记完成后清除标记的时机（打开浏览器前清除 vs 浏览器关闭后清除）

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| MARK-01 | 用户可以在服务器列表按 T 键标记第一个服务器为源端（Shift+t） | handleGlobalKeys switch 新增 `case 'T'`，复用 `GetSelectedServer()` + `showStatusTemp()` 模式 |
| MARK-02 | 再按 T 键标记第二个服务器为目标端，自动打开双远端文件浏览器 | `handleServerMark` 内部状态机：idle → source_marked → target_marked → open_browser → idle |
| MARK-03 | 标记状态下按 Esc 清除所有标记，恢复普通选择状态 | 修改 ServerList InputCapture 或 handleGlobalKeys 拦截 Esc，优先检查标记状态 |
| MARK-04 | 防止标记同一服务器两次（显示错误提示或忽略） | 比较 `server.Alias`，调用 `showStatusTempColor(..., "#FF6B6B")` |
| MARK-05 | 已标记的服务器在列表中有视觉提示（[S]/[T] 前缀） | 修改 `formatServerLine` 增加 tview 颜色标签 `[green][S][-]` / `[blue][T][-]` |
</phase_requirements>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| tview | v0.0.0-20250625 | TUI 框架，提供 List、InputCapture、颜色标签 | 项目已依赖，不可引入其他 UI 框架 |
| tcell/v2 | v2.9.0 | 终端单元格操作，颜色定义 | 项目已依赖 |
| domain.Server | — | 服务器实体，标记状态引用此类型 | 现有领域模型 |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| go.uber.org/zap | v1.27.0 | 结构化日志 | 标记操作日志记录（可选） |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| tview 颜色标签 `[green]` | tcell.ColorGreen + SetTextColor | 颜色标签更简洁，与现有 `fCol` 渲染方式一致 |
| 参数传递标记状态 | 包级变量 | 包级变量破坏封装性，参数传递更清晰 |

**Installation:** 无需安装新依赖，完全复用现有 stack。

**Version verification:** 现有依赖已锁定在 go.sum 中，无需额外版本检查。

## Architecture Patterns

### Recommended State Machine

```
idle → (T pressed) → source_marked → (T pressed, different server) → target_marked → auto open browser → idle
                    → (Esc pressed) → idle
                    → (T pressed, same server) → source_marked (error message)
target_marked → (auto) → open browser → idle
source_marked → (Esc pressed) → idle (clear marks, refresh)
```

### Pattern 1: Handler 函数模式

**What:** 在 `handleGlobalKeys` switch 中新增 case，调用私有方法 `handleServerMark`。遵循现有 `handleServerDup`、`handleServerPin` 等模式。

**When to use:** 所有全局键盘事件处理

**Example:**
```go
// handlers.go — handleGlobalKeys switch 中新增
case 'T':
    t.handleServerMark()
    return nil
```

### Pattern 2: formatServerLine 参数扩展

**What:** 修改 `formatServerLine` 签名，增加标记状态参数。在函数内根据标记状态渲染 `[S]`/`[T]` 前缀。

**When to use:** 需要在列表行渲染中显示动态状态时

**推荐方案:** 增加参数 `markSource, markTarget *domain.Server`，通过 `server.Alias` 匹配判断是否为标记服务器。

```go
// utils.go — 修改函数签名
func formatServerLine(s domain.Server, markSource, markTarget *domain.Server) (primary, secondary string) {
    // 标记前缀
    markPrefix := ""
    switch {
    case markSource != nil && s.Alias == markSource.Alias:
        markPrefix = "[green][S][-] "
    case markTarget != nil && s.Alias == markTarget.Alias:
        markPrefix = "[blue][T][-] "
    }

    icon := cellPad(pinnedIcon(s.PinnedAt), 2)
    // ... 其余不变，primary 开头加入 markPrefix
    primary = fmt.Sprintf("%s%s [white::b]%-12s[-] ...", markPrefix, icon, s.Alias, ...)
    return
}
```

### Pattern 3: ServerList 标记状态传递

**What:** `ServerList.UpdateServers` 需要获取标记状态并传递给 `formatServerLine`。推荐在 `ServerList` 上添加一个回调或存储标记状态的指针。

**When to use:** 子组件需要访问父组件（tui）的状态时

**推荐方案:** 在 `ServerList` 上添加 `markStateGetter` 回调函数，由 `tui.buildComponents()` 时设置。

```go
// server_list.go
type MarkStateGetter func() (*domain.Server, *domain.Server)

type ServerList struct {
    *tview.List
    servers           []domain.Server
    onSelection       func(domain.Server)
    onSelectionChange func(domain.Server)
    onReturnToSearch  func()
    markStateGetter   MarkStateGetter  // 新增
}

func (sl *ServerList) OnMarkState(fn MarkStateGetter) *ServerList {
    sl.markStateGetter = fn
    return sl
}

func (sl *ServerList) UpdateServers(servers []domain.Server) {
    var markSource, markTarget *domain.Server
    if sl.markStateGetter != nil {
        markSource, markTarget = sl.markStateGetter()
    }
    // ...
    for i := range servers {
        primary, secondary := formatServerLine(servers[i], markSource, markTarget)
        // ...
    }
}
```

### Pattern 4: Esc 拦截修改

**What:** 修改 ServerList 的 InputCapture，在标记模式下 Esc 优先清除标记而非返回搜索栏。

**When to use:** 需要在现有按键拦截中添加优先级逻辑时

**推荐方案:** 在 ServerList 上添加 `markClearer` 回调，返回 true 表示已消费 Esc。

```go
// server_list.go — 修改 InputCapture
sl.List.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
    switch event.Key() {
    case tcell.KeyESC:
        if sl.markClearer != nil && sl.markClearer() {
            return nil  // 标记已清除，消费 Esc
        }
        if sl.onReturnToSearch != nil {
            sl.onReturnToSearch()
        }
        return nil
    // ... 其余 key
    }
    return event
})
```

### Anti-Patterns to Avoid

- **直接修改 formatServerLine 为方法:** 不应将纯函数改为 tui 方法，会破坏 utils.go 的可测试性。保持为函数，通过参数传入状态。
- **标记状态存储在 ServerList:** CONTEXT.md D-06 明确决定存储在 tui struct 上。ServerList 不应持有业务状态。
- **用包级变量传递标记状态:** 与 `IsForwarding` 类似的包级变量虽然存在（历史原因），新代码不应再添加。
- **标记完成后不清除状态:** 必须在打开浏览器前清除标记，否则列表渲染会残留旧标记。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 状态栏临时消息 | 自定义 timer + text update | `showStatusTemp()` / `showStatusTempColor()` | 已有完善的 2 秒自动恢复机制 |
| 错误提示 | 自定义红色文字渲染 | `showStatusTempColor(msg, "#FF6B6B")` | 与项目其他错误提示一致 |
| 列表刷新 | 手动 Clear + AddItem | `refreshServerList()` | 已封装搜索过滤 + 排序 + 更新 |
| 服务器获取 | 直接访问 servers slice | `t.serverList.GetSelectedServer()` | 封装了边界检查 |

**Key insight:** Phase 11 的所有 UI 反馈机制都已存在，无需构建任何新的 UI 组件或状态显示基础设施。

## Common Pitfalls

### Pitfall 1: formatServerLine 签名变更的级联影响

**What goes wrong:** 修改 `formatServerLine` 签名后，所有调用点都需要更新。如果遗漏某个调用点，编译失败。

**Why it happens:** `formatServerLine` 是包内函数，目前只有 `ServerList.UpdateServers` 调用它（utils_test.go 中也有测试调用）。

**How to avoid:** 修改签名后立即编译验证，使用 `go build ./...` 确认所有调用点已更新。

**Warning signs:** 编译错误明确指出签名不匹配。

### Pitfall 2: tview 颜色标签格式错误

**What goes wrong:** 使用了错误的颜色标签格式（如 `[green]` 而非 `[#00FF00]`），导致颜色不显示或显示为原始标签文本。

**Why it happens:** tview 支持两种颜色格式——命名颜色（`[red]`、`[green]`）和十六进制（`[#FF6B6B]`）。命名颜色是 tcell 预定义的。

**How to avoid:** 现有代码使用十六进制格式 `[#A0FFA0]`、`[#FF6B6B]` 等。但 CONTEXT.md D-02 指定使用 `tcell.ColorGreen` 和 `tcell.ColorBlue`，对应的 tview 命名颜色标签为 `[green]` 和 `[blue]`。需要验证 tview 是否支持 `green`/`blue` 命名颜色。

**Warning signs:** 颜色标签显示为原始文本而非着色。

### Pitfall 3: Esc 键冲突

**What goes wrong:** 标记模式下按 Esc，同时触发了标记清除和搜索栏聚焦，导致 UI 状态混乱。

**Why it happens:** ServerList 的 InputCapture 和 tui 的 handleGlobalKeys 都可能处理 Esc 事件。如果两层都处理，行为不可预测。

**How to avoid:** 只在一个层级处理 Esc。推荐在 ServerList InputCapture 中处理（因为它已经有 Esc 拦截），通过 `markClearer` 回调与 tui 通信。handleGlobalKeys 不需要额外处理 Esc。

**Warning signs:** 按 Esc 后焦点跳动或列表未正确刷新。

### Pitfall 4: 标记状态与服务器列表不同步

**What goes wrong:** 用户标记服务器 A 为源端，然后搜索过滤导致服务器 A 不在列表中，但标记状态仍然存在。之后取消搜索，列表重新显示服务器 A 带有标记，但用户可能已忘记标记状态。

**Why it happens:** `refreshServerList()` 会重新调用 `ListServers(query)` + `UpdateServers()`，标记状态存储在 tui 上不受搜索过滤影响。这是正确行为——标记应跨搜索持久化。

**How to avoid:** 不需要避免，这是期望行为。但需确保 `refreshServerList` 正确传递标记状态给 `formatServerLine`。

**Warning signs:** 搜索后标记前缀消失。

### Pitfall 5: handleServerMark 中 GetSelectedServer 返回 false

**What goes wrong:** 当列表为空时按 T 键，`GetSelectedServer()` 返回 `false`，如果未处理会导致空操作或 panic。

**Why it happens:** 遵循现有 handler 模式，必须检查 `ok` 返回值。

**How to avoid:** `handleServerMark` 入口处 `if !ok { return }` 提前退出，与 `handleServerPin` 等一致。

## Code Examples

### 已有 handler 模式参考

```go
// handlers.go:140 — handleServerPin（最简单的标记类操作）
func (t *tui) handleServerPin() {
    if server, ok := t.serverList.GetSelectedServer(); ok {
        pinned := server.PinnedAt.IsZero()
        _ = t.serverService.SetPinned(server.Alias, pinned)
        t.refreshServerList()
    }
}
```

### formatServerLine 现有实现

```go
// utils.go:84-100 — 当前 formatServerLine 实现
func formatServerLine(s domain.Server) (primary, secondary string) {
    icon := cellPad(pinnedIcon(s.PinnedAt), 2)
    fGlyph := ""
    isFwd := IsForwarding != nil && IsForwarding(s.Alias)
    if isFwd {
        fGlyph = "Ⓕ"
    }
    fCol := cellPad(fGlyph, 2)
    if isFwd {
        fCol = "[#A0FFA0]" + fCol + "[-]"
    }
    primary = fmt.Sprintf("%s [white::b]%-12s[-] [#AAAAAA]%-18s[-] %s [#888888]Last SSH: %s[-]  %s",
        icon, s.Alias, s.Host, fCol, humanizeDuration(s.LastSeen), renderTagBadgesForList(s.Tags))
    secondary = ""
    return
}
```

### tview 颜色标签格式参考

```go
// 现有代码中的颜色标签用法
fCol = "[#A0FFA0]" + fCol + "[-]"           // 十六进制绿色
t.showStatusTempColor(msg, "#FF6B6B")        // 错误红色
parts = append(parts, fmt.Sprintf("[black:#5FAFFF] %s [-:-:-]", t))  // 标签 badge
```

### ServerList InputCapture 现有实现

```go
// server_list.go:57-67
sl.List.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
    switch event.Key() {
    case tcell.KeyLeft, tcell.KeyRight, tcell.KeyBackspace, tcell.KeyBackspace2, tcell.KeyESC:
        if sl.onReturnToSearch != nil {
            sl.onReturnToSearch()
        }
        return nil
    }
    return event
})
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| N/A — 这是新功能 | tview 颜色标签 `[#RRGGBB]` | 项目建立时 | Phase 11 使用相同格式 |

**关于 tview 命名颜色:** tview 支持命名颜色标签（如 `[red]`、`[green]`、`[blue]`），但项目现有代码统一使用十六进制格式。为保持一致性，推荐：
- `[S]` 源端：`[#00FF00]` (纯绿) 或 `[#A0FFA0]` (柔绿，与转发标记一致)
- `[T]` 目标端：`[#5FAFFF]` (蓝色，与标签 badge 一致)

**Deprecated/outdated:**
- 无

## Open Questions

1. **tview 命名颜色 vs 十六进制颜色**
   - What we know: CONTEXT.md D-02 指定 `tcell.ColorGreen` 和 `tcell.ColorBlue`，但项目现有代码使用十六进制格式
   - What's unclear: 是否应遵循 CONTEXT.md 的命名颜色还是项目的十六进制惯例
   - Recommendation: 使用十六进制格式保持一致性。`[S]` 用 `[#A0FFA0]`，`[T]` 用 `[#5FAFFF]`，与现有转发标记和标签 badge 风格统一

2. **标记完成后清除标记的时机**
   - What we know: D-05 说"状态栏提示后清除标记"
   - What's unclear: 是在调用 handleDualRemoteBrowser 前清除，还是在浏览器关闭后清除
   - Recommendation: 在调用 handleDualRemoteBrowser 前（即打开浏览器前）清除标记并刷新列表。浏览器关闭后 `returnToMain()` 会重新加载列表，此时不应有残留标记

3. **状态栏默认文本是否需要更新**
   - What we know: `DefaultStatusText()` 列出所有快捷键
   - What's unclear: 是否需要在默认状态栏文本中添加 T 键说明
   - Recommendation: 是的，在 DefaultStatusText 中添加 `[white]T[-] Mark` 说明

## Environment Availability

Step 2.6: SKIPPED (no external dependencies identified — Phase 11 is purely code/config changes within the existing Go codebase)

## Validation Architecture

> SKIPPED — `workflow.nyquist_validation` is explicitly set to `false` in `.planning/config.json`.

## Sources

### Primary (HIGH confidence)
- 项目源码直接分析 — handlers.go, tui.go, server_list.go, utils.go, status_bar.go
- CONTEXT.md 用户决策 — D-01 到 D-06 已锁定
- REQUIREMENTS.md — MARK-01 到 MARK-05 需求定义

### Secondary (MEDIUM confidence)
- tview 颜色标签格式 — 从现有代码推断（`[#A0FFA0]`, `[#FF6B6B]` 等）

### Tertiary (LOW confidence)
- 无

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — 完全复用现有依赖，无新引入
- Architecture: HIGH — 基于现有代码模式（handler switch、formatServerLine、InputCapture）直接推导
- Pitfalls: HIGH — 所有可能的问题点已在代码审查中识别

**Research date:** 2026-04-15
**Valid until:** 90 days — 纯 UI 层改动，不依赖外部 API 或快速变化的库
