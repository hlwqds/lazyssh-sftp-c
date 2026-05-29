# Phase 4: Directory History Core - Research

**Researched:** 2026-04-14
**Domain:** Go TUI 内存数据结构 + 回调 bug 修复 (tview/tcell 生态)
**Confidence:** HIGH (基于完整源码审查和已有研究文档验证)

## Summary

Phase 4 是 v1.1 里程碑的数据层基础阶段。核心工作是创建 `RecentDirs` 内存 MRU（Most Recently Used）数据结构，在远程面板的目录导航事件中自动记录路径，同时修复 `NavigateToParent()` 缺少 `onPathChange` 回调的预存 bug。

本阶段零新依赖、零 UI 渲染变更。所有改动集中在 `file_browser` 包内：新建一个文件（`recent_dirs.go`），修改两个文件（`remote_pane.go`、`file_browser.go`）。架构决策已在 CONTEXT.md 中锁定，本研究验证这些决策的技术可行性并补充实现细节。

**Primary recommendation:** 严格遵循 CONTEXT.md 锁定的 9 项决策实现，以 TransferModal 为结构模板，Record 方法使用 move-to-front 去重模式。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** 使用 `[]string` 有序 slice（非 map）维护 MRU 列表，最多 10 条。10 条上限下 O(n) 去重完全足够，无需 container/list。
- **D-02:** Record 方法实现 move-to-front 去重：先移除已有条目，再 prepend 到头部，最后截断到 10 条。
- **D-03:** RecentDirs 作为独立 struct，嵌入 `*tview.Box`（与 TransferModal 一致），存放在 `file_browser/recent_dirs.go`。数据存储在 RecentDirs 内部，由 FileBrowser 持有。
- **D-04:** 路径记录通过现有 `onPathChange` 回调实现——在 FileBrowser.build() 中为 RemotePane 的 onPathChange 添加 `fb.recentDirs.Record(path)` 调用。
- **D-05:** 不记录相对路径。如果路径以 `"."` 开头（如 `"."`, `"./docs"`），跳过记录。只有在 NavigateToParent 返回到绝对路径后才开始记录。
- **D-06:** 路径规范化仅去尾部斜杠（`strings.TrimRight(path, "/")`），不做完整路径解析（SFTP 远程路径通常是规范绝对路径）。
- **D-07:** Phase 5 中用户从最近列表选择路径跳转后，该路径重新提升到列表头部（调用 Record）。
- **D-08:** 修复 `RemotePane.NavigateToParent()` 缺少 `onPathChange` 回调——在方法末尾添加 `if rp.onPathChange != nil { rp.onPathChange(rp.currentPath) }`。这同时修复了返回上级时 `app.Sync()` 未调用的问题。
- **D-09:** 在 RemotePane 上添加 `NavigateTo(path string)` 方法——直接设置 currentPath 并 Refresh，不触发 onPathChange 回调。用于 Phase 5 的弹出列表选择跳转（避免通过 NavigateInto 间接触发）。

### Claude's Discretion
- RecentDirs 的 Draw() 方法实现细节（颜色、宽度、边距等）留给 Phase 5 规划
- NavigateTo 方法是否需要调用 UpdateTitle() —— 当前设计不调用，因为 Refresh() 已内部调用

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| HIST-01 | 用户导航到新目录时自动记录到最近目录列表 | D-04: 通过 onPathChange 回调实现，D-08 修复确保 NavigateToParent 也触发记录 |
| HIST-02 | 列表按 MRU 排序 | D-02: Record 方法 prepend 到头部实现 MRU 排序 |
| HIST-03 | 同一路径自动去重，仅保留最新位置 | D-02: move-to-front 去重逻辑 |
| HIST-04 | 最多保留 10 条记录 | D-01: `[]string` slice + 截断到 10 条 |
| AUX-02 | 修复 NavigateToParent 缺少 onPathChange 回调 | D-08: 一行修复，在 NavigateToParent 末尾添加回调调用 |
</phase_requirements>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `strings` | 1.24.6 | 路径规范化 (TrimRight) | 零依赖，标准库即可满足 |
| tview.Box | v0.0.0 | RecentDirs 嵌入基础 | 与 TransferModal 架构一致 |

### Supporting

本阶段无需额外依赖。所有操作均为内存中的字符串 slice 操作和回调注册。

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `[]string` slice (D-01) | `container/list` 双向链表 | 10 条上限下 slice 的 O(n) 去重完全足够，container/list 引入额外导入且代码更冗长 |
| 嵌入 `*tview.Box` (D-03) | 嵌入 `*tview.Table` | Box 更轻量，Draw() 完全自定义控制；Table 自带选中状态但与我们的 overlay 模式不匹配 |
| `onPathChange` 回调记录 (D-04) | 独立 `RecordPath()` 调用点 | 回调方式只需改 build() 一处，独立调用需要在 NavigateInto 和 NavigateToParent 中各加一行 |

**Installation:**
无需安装任何新包。本阶段零新依赖。

## Architecture Patterns

### Recommended Project Structure

```
internal/adapters/ui/file_browser/
├── recent_dirs.go      # NEW — RecentDirs struct, Record(), GetPaths(), Draw() stub
├── remote_pane.go      # MODIFY — NavigateToParent fix, NavigateTo method
├── file_browser.go     # MODIFY — RecentDocs field, build() wiring, Draw() overlay
├── file_browser_handlers.go  # MODIFY — handleGlobalKeys overlay guard
├── transfer_modal.go   # REFERENCE ONLY — overlay pattern template
├── local_pane.go       # UNCHANGED
├── progress_bar.go     # UNCHANGED
└── file_sort.go        # UNCHANGED
```

### Pattern 1: Overlay Component (TransferModal Template)

**What:** 嵌入 `*tview.Box` 的自定义组件，通过 `visible` 标志控制绘制，手动实现 `Draw()` 方法。
**When to use:** 需要在 FileBrowser 上方叠加渲染的组件（modal、popup、dropdown）。
**Example (from transfer_modal.go lines 69-95):**
```go
type TransferModal struct {
    *tview.Box
    app        *tview.Application
    visible    bool
    onDismiss  func()
    // ... other fields
}

func (tm *TransferModal) Draw(screen tcell.Screen) {
    if !tm.visible {
        return
    }
    tm.Box.DrawForSubclass(screen, tm)
    // ... render content
}

func (tm *TransferModal) Show(...) {
    tm.visible = true
    // ... set state
}

func (tm *TransferModal) Hide() {
    tm.visible = false
    if tm.onDismiss != nil {
        tm.onDismiss()
    }
}

func (tm *TransferModal) IsVisible() bool {
    return tm.visible
}
```

**Phase 4 的 RecentDirs 应遵循相同模式**，但 Phase 4 不需要实现 Draw() 的实际渲染逻辑——只需提供 struct 骨架和 `visible`/`Show()`/`Hide()`/`IsVisible()` 方法。

### Pattern 2: Move-to-Front Deduplication (MRU)

**What:** 在有序列表中维护唯一性，同时保证最新访问的条目在头部。
**When to use:** 需要去重且保持访问顺序的场景。
**原理:** Go 的 slice 是引用类型，但 `append` 创建新 slice header 时会复制元素引用。移除中间元素使用 `append(slice[:i], slice[i+1:]...)` 模式——这不会泄漏内存因为 string 值是不可变的，被移除的 string 会被 GC 回收。
**Example (from CONTEXT.md D-02 + PITFALLS.md P9):**
```go
const maxRecentDirs = 10

func (rd *RecentDirs) Record(path string) {
    // D-06: 路径规范化仅去尾部斜杠
    normalized := strings.TrimRight(path, "/")

    // D-05: 不记录相对路径
    if strings.HasPrefix(normalized, ".") {
        return
    }

    // D-02: move-to-front 去重
    for i, p := range rd.paths {
        if p == normalized {
            rd.paths = append(rd.paths[:i], rd.paths[i+1:]...)
            break
        }
    }

    // prepend 到头部
    rd.paths = append([]string{normalized}, rd.paths...)

    // D-01: 截断到 10 条
    if len(rd.paths) > maxRecentDirs {
        rd.paths = rd.paths[:maxRecentDirs]
    }
}
```

### Pattern 3: Callback Registration in build()

**What:** FileBrowser.build() 中为子组件注册回调函数，实现松耦合的事件传递。
**When to use:** 子组件需要通知父组件状态变化，但不应直接持有父组件引用。
**Example (from file_browser.go lines 126-133, current code):**
```go
// 现有 onPathChange 回调注册
fb.localPane.OnPathChange(func(_ string) {
    fb.app.Sync()
})
fb.remotePane.OnPathChange(func(_ string) {
    fb.app.Sync()
})
```

**Phase 4 修改为：**
```go
fb.remotePane.OnPathChange(func(path string) {
    fb.app.Sync()                     // 保留现有功能
    fb.recentDirs.Record(path)        // D-04: 新增路径记录
})
```

### Pattern 4: NavigateToParent Bug Fix (D-08)

**What:** 在 `NavigateToParent()` 末尾补充缺失的 `onPathChange` 回调。
**原理:** `NavigateInto()` 在第 298-299 行调用了 `onPathChange`，但 `NavigateToParent()`（第 276-288 行）没有。这是一个不对称 bug——两个方法都改变了 `currentPath`，但只有一个通知了观察者。修复后，`app.Sync()` 也会在返回上级时被调用（解决 kitty 透明背景下的 ghost artifact 问题）。
**Example (remote_pane.go, current code at lines 276-288):**
```go
// 修复前：
func (rp *RemotePane) NavigateToParent() {
    if !rp.connected {
        return
    }
    parent := parentPath(rp.currentPath)
    if parent == rp.currentPath {
        return
    }
    rp.currentPath = parent
    rp.selected = make(map[string]bool)
    rp.Refresh()
    // BUG: 缺少 onPathChange 回调
}

// 修复后（D-08）：
func (rp *RemotePane) NavigateToParent() {
    if !rp.connected {
        return
    }
    parent := parentPath(rp.currentPath)
    if parent == rp.currentPath {
        return
    }
    rp.currentPath = parent
    rp.selected = make(map[string]bool)
    rp.Refresh()
    if rp.onPathChange != nil {
        rp.onPathChange(rp.currentPath)
    }
}
```

### Pattern 5: NavigateTo Method (D-09)

**What:** 在 RemotePane 上添加直接设置路径的方法，不触发 `onPathChange`。
**原理:** Phase 5 的弹出列表选择跳转时，不应该再次触发路径记录（路径已在列表中）。`NavigateTo` 与 `NavigateInto` 的区别在于不调用 `onPathChange` 回调。`Refresh()` 内部调用 `populateTable()`，`populateTable()` 末尾调用 `UpdateTitle()`（remote_pane.go 第 263 行），所以不需要单独调用 `UpdateTitle()`。
**Example:**
```go
// NavigateTo sets the current path directly without triggering onPathChange.
// Used by RecentDirs navigation (Phase 5) to avoid re-recording the path.
func (rp *RemotePane) NavigateTo(path string) {
    if !rp.connected {
        return
    }
    rp.currentPath = path
    rp.selected = make(map[string]bool)
    rp.Refresh()
}
```

### Anti-Patterns to Avoid

- **在 NavigateInto 和 NavigateToParent 中分别调用 Record():** 违反 D-04 决策。应通过 onPathChange 回调统一处理，避免调用点分散导致遗漏。
- **Record 方法使用 map 辅助去重:** 10 条上限下 O(n) 线性扫描足够快（微秒级），引入 map 增加代码复杂度且 map 遍历顺序不确定。
- **NavigateTo 调用 onPathChange:** 会导致弹出列表选择跳转时路径被重新记录并提升到头部（D-07 说 Phase 5 才需要此行为，且由 Phase 5 显式调用 Record 实现）。
- **在 RecentDirs 中存储 time.Time 时间戳:** MRU 语义已由 slice 顺序表达，额外时间戳是冗余数据。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| MRU 去重 | 自定义 hashmap + linked list | `[]string` + 线性扫描 (D-01) | 10 条上限下 O(n) 完全足够，线性代码更简单更易审查 |
| 路径规范化 | `filepath.Clean()` + `path.Clean()` | `strings.TrimRight(path, "/")` (D-06) | SFTP 远程路径已是规范绝对路径，filepath.Clean 会错误地处理 Windows 路径分隔符 |
| 组件可见性管理 | 自定义 flags + 状态机 | TransferModal 的 `visible` + `Show()`/`Hide()`/`IsVisible()` 模式 | 项目已有成熟模式，保持一致性 |
| slice 中间元素移除 | 自定义 copy loop | `append(slice[:i], slice[i+1:]...)` | Go 惯用法，编译器优化后性能等同 |

**Key insight:** 本阶段的数据结构极其简单（一个最多 10 个字符串的有序列表）。复杂度不在于数据操作本身，而在于正确地集成到现有的回调链中。不要过度设计数据层——把精力放在集成点的正确性上。

## Common Pitfalls

### Pitfall 1: 相对路径 "." 被记录到列表中 (P5 变体)

**What goes wrong:** RemotePane 初始 `currentPath` 为 `"."`（SFTP home 目录），如果 `NavigateToParent()` 修复后触发了 `onPathChange`，`"."` 会被传给 `Record()` 并记录到列表中。用户打开弹出列表时会看到无意义的 "." 路径。
**Why it happens:** SFTP 连接建立后，初始路径是 `"."`。第一次 `NavigateToParent()` 会将其解析为绝对路径（如 `/home/user`），但在此之前如果任何路径变更事件触发 `Record()`，相对路径会被记录。
**How to avoid:** D-05 已规定跳过以 `"."` 开头的路径。`Record()` 方法的第一道防线就是 `strings.HasPrefix(normalized, ".")` 检查。这同时过滤了 `"."`、`"./docs"` 等相对路径。
**Warning signs:** 弹出列表中出现 "." 或 "./" 开头的条目。

### Pitfall 2: NavigateToParent 修复改变了现有行为

**What goes wrong:** 修复 `NavigateToParent()` 添加 `onPathChange` 后，`app.Sync()` 在返回上级时也被调用。虽然这是正确行为（修复了 ghost artifact），但可能暴露之前被掩盖的渲染问题。
**Why it happens:** `onPathChange` 回调目前唯一的副作用是 `fb.app.Sync()`。修复后 Sync() 在所有导航操作中都被调用，而不仅仅是在 NavigateInto 中。
**How to avoid:** 这实际上是行为改善，不是退化。验证方法：在 kitty 透明背景下按 `h` 返回上级目录，确认无 ghost artifacts。如果出现新问题，说明 Sync() 的时序或频率需要调整——但这是独立于本阶段的渲染问题。
**Warning signs:** kitty 透明背景下返回上级时出现短暂闪烁（实际上应该是改善）。

### Pitfall 3: Record 方法在空 slice 上调用 append 的边界行为

**What goes wrong:** 第一次调用 `Record()` 时 `rd.paths` 为 nil。`append([]string{normalized}, rd.paths...)` 对 nil slice 的展开是安全的（等同于 append 到空 slice），但如果代码写成 `rd.paths = append(rd.paths[:0], rd.paths[0:]...)` 这样的错误形式会 panic。
**Why it happens:** Go 的 nil slice 支持 append 操作，但 slice 切片操作 `nil[:0]` 是安全的（返回 nil），而 `nil[0:]` 也是安全的。实际风险在于去重循环 `for i, p := range rd.paths` 对 nil slice 不会执行循环体，这是正确的。
**How to avoid:** Record 方法对 nil/empty slice 的行为天然正确，无需特殊处理。单元测试应覆盖空列表首次插入的场景。
**Warning signs:** 无——Go 的 nil slice 语义保证了正确性。

### Pitfall 4: D-09 NavigateTo 与 D-08 NavigateToParent 的交互

**What goes wrong:** Phase 5 实现弹出列表选择跳转时，如果使用 `NavigateTo()` 跳转到某个路径，然后用户按 `h` 返回上级，`NavigateToParent()` 现在会触发 `onPathChange`，导致父路径被记录。这是正确行为——用户确实导航到了父目录。
**Why it happens:** D-08 修复后，所有通过 `NavigateToParent()` 的导航都会触发记录。`NavigateTo()` 不触发记录（设计如此），但后续的 `NavigateToParent()` 会触发。
**How to avoid:** 这是预期行为，不是 bug。弹出列表选择跳转后（Phase 5），用户可以按 `r` 再次打开列表看到更新后的顺序（D-07 规定 Phase 5 在选择时调用 Record 重新提升）。
**Warning signs:** 无——行为符合预期。

## Code Examples

Verified patterns from source code:

### 1. RemotePane.NavigateInto — onPathChange 回调触发点
```go
// Source: remote_pane.go lines 291-301
func (rp *RemotePane) NavigateInto(dirName string) {
    if !rp.connected {
        return
    }
    rp.currentPath = joinPath(rp.currentPath, dirName)
    rp.selected = make(map[string]bool)
    rp.Refresh()
    if rp.onPathChange != nil {
        rp.onPathChange(rp.currentPath)
    }
}
```

### 2. RemotePane.NavigateToParent — 需要修复的位置
```go
// Source: remote_pane.go lines 276-288
func (rp *RemotePane) NavigateToParent() {
    if !rp.connected {
        return
    }
    parent := parentPath(rp.currentPath)
    if parent == rp.currentPath {
        return // already at root
    }
    rp.currentPath = parent
    rp.selected = make(map[string]bool)
    rp.Refresh()
    // BUG: onPathChange not called here (AUX-02)
}
```

### 3. FileBrowser.build() — onPathChange 回调注册点
```go
// Source: file_browser.go lines 126-133
fb.localPane.OnPathChange(func(_ string) {
    fb.app.Sync()
})
fb.remotePane.OnPathChange(func(_ string) {
    fb.app.Sync()
})
```

### 4. RemotePane.OnPathChange — setter 方法模式
```go
// Source: remote_pane.go lines 370-374
func (rp *RemotePane) OnPathChange(fn func(path string)) *RemotePane {
    rp.onPathChange = fn
    return rp
}
```

### 5. RemotePane.currentPath 初始值
```go
// Source: remote_pane.go line 51
currentPath: ".", // SFTP starts in user's home directory
```

### 6. parentPath — 现有路径处理函数
```go
// Source: remote_pane.go lines 406-417
func parentPath(p string) string {
    if p == "" || p == "/" || p == "~" {
        return p
    }
    p = strings.TrimRight(p, "/")
    idx := strings.LastIndex(p, "/")
    if idx <= 0 {
        return "/"
    }
    return p[:idx]
}
```

**关键观察:** `parentPath(".")` 返回 `"."`（因为 `strings.TrimRight(".", "/")` = `"."`，`strings.LastIndex(".", "/")` = -1，idx <= 0 返回 `"/"`）。等等，让我重新推算：`"."` -> TrimRight -> `"."` -> LastIndex(`"/"`) = -1 -> idx <= 0 -> return `"/"`。所以从 `"."` 调用 NavigateToParent 会设置 currentPath 为 `"/"`，然后触发 onPathChange(`"/"`)。Record 方法收到 `"/"` 后会记录它。但 `"/"` 是一个有效的绝对路径（root 目录），应该被记录。这是正确行为。

**再推算一步:** 从 `"/"` 调用 NavigateToParent -> `parentPath("/")` 返回 `"/"`，`parent == rp.currentPath` 为 true，return。不会触发 onPathChange。正确——已在 root，不重复记录。

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| container/list 双向链表 | `[]string` slice | D-01 (本次 Phase 4) | 更简单的代码，10 条上限下性能无差异 |
| 在 NavigateInto/NavigateToParent 中直接记录 | 通过 onPathChange 回调统一记录 | D-04 (本次 Phase 4) | 单一调用点，不会遗漏 |
| NavigateToParent 无回调 | NavigateToParent 补充 onPathChange | D-08 (本次 Phase 4, AUX-02) | 修复 ghost artifact，统一导航行为 |

**Deprecated/outdated:**
- 无——本阶段不涉及任何已废弃的 API 或模式。

## Open Questions

1. **Record 是否应该在 NavigateToParent 返回 root "/" 时也记录？**
   - What we know: D-05 仅过滤以 "." 开头的路径。"/" 是有效绝对路径，不满足 HasPrefix(".", ...) 条件，会被记录。
   - What's unclear: 用户频繁按 `h` 到达 root 后，"/" 是否应该在列表中占据一个位置。这在 10 条上限下可能是浪费。
   - Recommendation: 按照当前决策实现（不过滤 "/"）。如果 UX 测试发现 "/" 占位不合理，可以在 Phase 5 中添加特殊过滤。这是 Claude's Discretion 范围，不阻塞实现。

2. **NavigateTo 是否应该在 Phase 4 中实现还是推迟到 Phase 5？**
   - What we know: D-09 明确规定在 RemotePane 上添加 NavigateTo 方法。CONTEXT.md canonical_refs 列出这是 Phase 4 的范围。
   - What's unclear: 无——D-09 已锁定。
   - Recommendation: Phase 4 实现 NavigateTo 方法。虽然 Phase 5 才使用它，但它是 RemotePane 的公共 API，提前实现不增加风险，且保持 Phase 4 的完整性。

## Environment Availability

Step 2.6: SKIPPED (no external dependencies identified — 本阶段是纯代码变更，无外部工具/服务/运行时依赖)

## Sources

### Primary (HIGH confidence)
- 源码审查: `internal/adapters/ui/file_browser/` 全部 7 个文件 — 结构、模式、回调链、bug 位置
- `.planning/research/ARCHITECTURE.md` — 完整集成分析、数据流、构建顺序、NavigateToParent 不对称性发现
- `.planning/research/PITFALLS.md` — 11 个 pitfalls，Phase 4 涉及 P1/P5/P6/P9
- `.planning/phases/04-directory-history-core/04-CONTEXT.md` — 9 项锁定决策 (D-01 through D-09)
- Go 语言规范 — nil slice 语义、append 行为、strings 包 API

### Secondary (MEDIUM confidence)
- TransferModal overlay 模式 (transfer_modal.go) — 作为 RecentDirs 结构模板
- parentPath 函数行为推算 — 基于代码的静态分析，未经运行时验证

### Tertiary (LOW confidence)
- 无 — 所有研究结论均基于源码直接审查

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — 零新依赖，仅使用 Go 标准库和现有 tview.Box
- Architecture: HIGH — TransferModal 模式已在项目中验证，onPathChange 回调链已明确
- Pitfalls: HIGH — 所有 pitfalls 基于源码审查，边界条件已通过代码推算验证

**Research date:** 2026-04-14
**Valid until:** 30 天（数据结构和回调模式不会快速变化）
