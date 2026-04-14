# Phase 5: Recent Directories Popup - Research

**Researched:** 2026-04-14
**Domain:** tview TUI overlay component integration (Draw/HandleKey/key routing)
**Confidence:** HIGH (based on direct code analysis of all file_browser package files)

## Summary

Phase 5 为 Phase 4 创建的 `RecentDirs` 数据结构补全 UI 弹出层。核心工作分为三块：(1) 补全 `RecentDirs.Draw()` 手动渲染——标题、路径列表、选中项高亮、当前路径标记、空状态；(2) 新增 `RecentDirs.HandleKey()` 处理 j/k/上下方向键/Enter/Esc；(3) 在 `FileBrowser` 中接入按键路由（`r` 键拦截）和 overlay 渲染调用。

研究过程中发现一个**关键预存问题**：`TransferModal.Draw()` 方法已定义但从未被 `FileBrowser.Draw()` 调用。这意味着 TransferModal 的视觉渲染实际上从未生效。Phase 5 必须在 `FileBrowser.Draw()` 中同时添加 `transferModal.Draw(screen)` 和 `recentDirs.Draw(screen)` 调用，一并修复此问题。

**Primary recommendation:** 遵循 TransferModal 的 overlay 模式（`*tview.Box` embed + `visible` flag + 手动 `Draw` + `HandleKey`），在 `FileBrowser.Draw()` 中添加 overlay 绘制调用，在 `handleGlobalKeys` 中添加 `r` 键路由。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** 弹窗居中显示，宽度 = 终端宽度的 60%（最大 80 列），高度 = min(路径数量 + 2, 15) 行。+2 为标题行 + 内边距。
- **D-02:** 列表每行显示完整绝对路径（如 `/home/user/projects/my-app`），不缩写。
- **D-03:** 列表最多 10 行（与 `maxRecentDirs` 一致），不滚动。路径少于 10 条时弹窗高度自适应缩小。
- **D-04:** 选中项使用 `tcell.Color236` 深灰背景 + `tcell.Color250` 白色文字。未选中行为默认黑底白字。
- **D-05:** 当前路径条目（AUX-01）使用 `tcell.ColorYellow` 黄色文字显示。如果当前路径恰好是选中项，同时应用选中背景色 + 黄色文字。
- **D-06:** 空状态时居中显示灰色文字「暂无最近目录」，使用 `tcell.Color240` 灰色。
- **D-07:** `r` 键仅在远程面板获得焦点（`fb.activePane == 1`）时触发弹窗。本地面板按 `r` 无效。
- **D-08:** 弹窗可见时，完全拦截所有按键——j/k/上下方向键/Enter/Esc 全部由弹窗的 HandleKey 处理，不传递到下层组件。
- **D-09:** 按 Esc 关闭弹窗，焦点恢复到远程面板。不额外调用 app.Sync()（关闭弹窗后 FileBrowser.Draw 会自然重绘）。
- **D-10:** 按 Enter 选中路径后，调用 `RemotePane.NavigateTo(path)`（Phase 4 已添加，不触发 onPathChange），然后关闭弹窗。选中路径同时调用 `Record(path)` 将其提升到 MRU 列表头部。
- **D-11:** 使用手动渲染方式——在 Draw() 中用 `tview.Print()` 逐行渲染文本，与 TransferModal 的渲染模式一致。
- **D-12:** 选中项仅通过背景色区分，不额外显示 `>` 或 `▸` 符号。
- **D-13:** 弹窗顶部显示「 Recent Directories 」标题，使用 `rd.SetTitle(" Recent Directories ")`。

### Claude's Discretion
- Draw() 中边框内边距（上下左右 padding）的具体像素值
- HandleKey 中 j/k 与上下方向键的具体实现细节（直接修改 selectedIndex 还是用 tview 内置机制）
- Show() 时是否需要调用 app.ForceDraw() 确保立即渲染

### Deferred Ideas (OUT OF SCOPE)
- 路径缩写显示（过长路径缩写中间部分）— v1.x
- 数字键快速选择（按 1-9 直接跳转）— v1.x
- 持久化书签/收藏夹 — v2+
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| POPUP-01 | 用户在远程面板获得焦点时按 `r` 键，弹出居中目录列表 | handleGlobalKeys 中添加 `case 'r'` 拦截，检查 `activePane == 1`；TransferModal overlay 路由模式作为参考 |
| POPUP-02 | 弹窗中 j/k/上下方向键移动选中项 | HandleKey 方法处理 event.Rune() 'j'/'k' 和 event.Key() KeyDown/KeyUp；selectedIndex 边界检查 |
| POPUP-03 | Enter 选中后跳转目录并刷新，弹窗关闭 | HandleKey 中 Enter 调用 NavigateTo(path) + Record(path) + Hide()；NavigateTo 已在 Phase 4 实现 |
| POPUP-04 | Esc 关闭弹窗，焦点恢复远程面板 | HandleKey 中 Esc 调用 Hide()；FileBrowser.Draw() 自然重绘清除残留 |
| POPUP-05 | 列表为空时显示"暂无最近目录" | Draw() 中 len(paths) == 0 分支渲染灰色文本 |
| AUX-01 | 当前路径条目用不同颜色高亮 | Draw() 中比较 path 与 remotePane.GetCurrentPath()，匹配时使用 tcell.ColorYellow |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| tview | v0.0.0 (git) | TUI framework, overlay rendering via tview.Print() | 项目已有依赖，提供 tview.Print/Box/Screen API |
| tcell/v2 | v2.9.0 | Terminal cell manipulation, color constants, key events | 项目已有依赖，tcell.Color* 和 tcell.EventKey 必需 |

### Supporting
无新增依赖。Phase 5 完全使用现有 tview/tcell API。

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| 手动 Draw() + tview.Print() | tview.List 内建组件 | List 有自己的焦点管理和 InputHandler，与 overlay 模式冲突；无法精确控制位置和颜色 |
| handleGlobalKeys 中 `case 'r'` | RemotePane.SetInputCapture 回调 | CONTEXT.md D-07 锁定为 handleGlobalKeys 方式（检查 activePane == 1），更简单直接 |

**Installation:** 无需安装新依赖。

**Version verification:** 使用项目 go.mod 中已锁定的 tview 和 tcell/v2 版本。

## Architecture Patterns

### Recommended Project Structure

```
internal/adapters/ui/file_browser/
├── recent_dirs.go          # Phase 5 补全 Draw() 和 HandleKey()
├── recent_dirs_test.go     # Phase 5 新增 HandleKey 和 Draw 单元测试
├── file_browser.go         # 修改: Draw() 添加 overlay 调用, build() 中无需改动（Record 已接入）
├── file_browser_handlers.go # 修改: handleGlobalKeys 添加 'r' 键拦截和 overlay 可见性检查
├── remote_pane.go          # 不修改（Phase 4 已添加 NavigateTo 和 GetCurrentPath）
└── transfer_modal.go       # 不修改
```

### Pattern 1: Overlay Rendering via FileBrowser.Draw()

**What:** 在 FileBrowser 的自定义 Draw() 方法中，`fb.Flex.Draw(screen)` 之后调用所有 overlay 组件的 Draw()。这确保 overlay 总是绘制在内容之上。

**When to use:** 所有需要叠加在 FileBrowser 内容上方的组件（TransferModal、RecentDirs）。

**Example:**
```go
// Source: 项目代码 file_browser.go, 模式扩展
func (fb *FileBrowser) Draw(screen tcell.Screen) {
    // 1. Fill background (existing)
    x, y, width, height := fb.GetRect()
    bgStyle := tcell.StyleDefault.Background(tcell.ColorDefault)
    for row := y; row < y+height; row++ {
        for col := x; col < x+width; col++ {
            screen.SetContent(col, row, ' ', nil, bgStyle)
        }
    }
    // 2. Draw main content (existing)
    fb.Flex.Draw(screen)
    // 3. Draw overlays on top (NEW)
    if fb.transferModal != nil && fb.transferModal.IsVisible() {
        fb.transferModal.Draw(screen)
    }
    if fb.recentDirs != nil && fb.recentDirs.IsVisible() {
        fb.recentDirs.Draw(screen)
    }
}
```

**CRITICAL FINDING:** 当前代码中 `TransferModal.Draw()` 从未被调用。这是一个预存 bug。Phase 5 必须同时修复 TransferModal 的渲染，添加 `fb.transferModal.Draw(screen)` 调用。详情见 Common Pitfalls 章节。

### Pattern 2: Overlay Key Routing in handleGlobalKeys

**What:** 在 `handleGlobalKeys` 方法最前面添加 overlay 可见性检查。当任何 overlay 可见时，按键事件优先委托给 overlay 处理，不传递到下层组件。

**When to use:** FileBrowser 中所有 overlay 组件的按键拦截。

**Example:**
```go
// Source: 项目代码 file_browser_handlers.go, 模式扩展
func (fb *FileBrowser) handleGlobalKeys(event *tcell.EventKey) *tcell.EventKey {
    // Overlay key interception (check BEFORE pane keys)
    if fb.recentDirs != nil && fb.recentDirs.IsVisible() {
        return fb.recentDirs.HandleKey(event)
    }
    // TransferModal handling (existing pattern)
    if fb.transferModal != nil && fb.transferModal.IsVisible() {
        fb.transferModal.HandleKey(event)
        return nil
    }
    // ... existing Tab, Esc, s, S, F5 handling ...
    switch event.Rune() {
    case 'r':
        if fb.activePane == 1 && fb.remotePane.IsConnected() {
            fb.recentDirs.Show()
            return nil
        }
    }
    return event // pass to focused pane's InputCapture
}
```

**Key insight:** `r` 键拦截放在 overlay 检查之后、pane 按键之前。这样当 RecentDirs 可见时，所有按键（包括 `r`）都被 RecentDirs.HandleKey 消费。当 TransferModal 可见时，所有按键被 TransferModal.HandleKey 消费。`r` 只在无 overlay 可见且远程面板有焦点时触发。

### Pattern 3: Manual Draw with tview.Print()

**What:** overlay 组件的 Draw() 方法使用 `tview.Print()` 逐行渲染文本到 screen。`tview.Print()` 自动处理文本截断、对齐和颜色标签。

**When to use:** 所有自定义 overlay 组件（与 TransferModal 一致）。

**Example:**
```go
// Source: transfer_modal.go drawProgress 模式
func (rd *RecentDirs) drawList(screen tcell.Screen, x, y, width, height int, currentPath string) {
    paths := rd.GetPaths()
    for i, path := range paths {
        row := y + i
        if row >= y+height {
            break
        }
        fgColor := tcell.Color250 // white
        bgColor := tcell.ColorDefault
        if path == currentPath {
            fgColor = tcell.ColorYellow // AUX-01
        }
        if i == rd.selectedIndex {
            bgColor = tcell.Color236 // selected
        }
        tview.Print(screen, path, x+1, row, width-2, tview.AlignLeft, fgColor)
        // Fill row background for selected item
        if bgColor != tcell.ColorDefault {
            for col := x; col < x+width; col++ {
                _, _, style, _ := screen.GetContent(col, row)
                screen.SetContent(col, row, ' ', nil, style.Background(bgColor))
            }
            tview.Print(screen, path, x+1, row, width-2, tview.AlignLeft, fgColor)
        }
    }
}
```

### Anti-Patterns to Avoid

- **Don't use tview.List for the overlay:** tview.List 有自己的焦点管理和 InputHandler，与 overlay 模式冲突。使用 `*tview.Box` + 手动 `Draw` + `HandleKey`（与 TransferModal 一致）。
- **Don't add RecentDirs as a Flex child:** Flex 会分配布局空间给子元素，overlay 不应影响 Flex 布局。使用 Draw() 中手动绘制。
- **Don't intercept `r` in RemotePane.SetInputCapture:** 虽然 pane-specific keys 通常在 pane 的 InputCapture 中处理，但 CONTEXT.md D-07 明确锁定使用 handleGlobalKeys 方式（检查 `activePane == 1`），更简单直接且与 TransferModal 的 Esc 拦截模式一致。
- **Don't call NavigateTo() for popup selection without Record():** D-10 明确要求选中时同时调用 Record(path) 将路径提升到 MRU 列表头部。
- **Don't render selected item background AFTER text:** tview.Print 设置的字符颜色会被后续 SetContent 覆盖。必须先填充背景，再打印文本。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 文本渲染（截断、对齐、颜色） | 手动 screen.SetContent 逐字符 | `tview.Print()` | 自动处理宽度截断、对齐、颜色标签，一行搞定 |
| 弹窗边框和背景 | 手动绘制边框线 | `tview.Box.DrawForSubclass(screen, rd)` | Box 已处理边框、标题、背景填充，与 TransferModal 一致 |
| 终端尺寸获取 | 手动存储屏幕宽高 | `screen.Size()` 在 Draw() 中每次动态获取 | 终端可能 resize，Draw() 中动态计算保证位置正确 |
| 按键事件匹配 | 手动 rune/switch 嵌套 | `event.Rune()` + `event.Key()` 双路 switch | tview 标准模式，同时处理 rune 和 special key |

**Key insight:** tview.Print() 是渲染文本的唯一正确方式。TransferModal 中所有文本渲染都使用它，RecentDirs 应保持一致。

## Common Pitfalls

### Pitfall 1: TransferModal.Draw() 从未被调用（预存 bug）

**What goes wrong:** FileBrowser.Draw() 只调用 `fb.Flex.Draw(screen)`，TransferModal 不是 Flex 的子元素，所以其 Draw() 从未被执行。TransferModal 的视觉渲染（进度条、速度、ETA）实际上从未显示在屏幕上。

**Why it happens:** 在 Phase 2 的 commit 288a3b6 中，TransferModal 被创建并接入 handleGlobalKeys（Esc 拦截）和 initiateTransfer（Show/Update/Hide），但其 Draw() 从未被添加到 FileBrowser.Draw() 的调用链中。后续 Phase 3 添加了 FileBrowser.Draw() 覆写（用于 kitty 透明度修复），但也没有添加 TransferModal.Draw() 调用。

**How to avoid:** Phase 5 必须在 FileBrowser.Draw() 中同时添加 `fb.transferModal.Draw(screen)` 和 `fb.recentDirs.Draw(screen)`。这同时修复了 TransferModal 的预存渲染 bug。

**Warning signs:** 如果 TransferModal 的进度条、速度、ETA 从未在屏幕上显示过，说明此 bug 一直存在。

### Pitfall 2: 'r' 键与 TransferModal modeConflictDialog 的 Rename 冲突

**What goes wrong:** 用户打开 TransferModal 的冲突对话框后按 `r`，期望触发 Rename。但如果 handleGlobalKeys 中的 `case 'r'` 没有正确检查 TransferModal 可见性，会同时触发弹出最近目录列表和 Rename。

**Why it happens:** TransferModal.HandleKey 在 modeConflictDialog 模式下消费 `r` 键（transfer_modal.go 第 368-373 行）。如果 handleGlobalKeys 中 `case 'r'` 在 TransferModal 可见性检查之前执行，会绕过 TransferModal 的按键处理。

**How to avoid:** 在 handleGlobalKeys 中，overlay 可见性检查必须在所有其他按键处理之前。`r` 键拦截必须在 `fb.transferModal.IsVisible()` 检查之后。

**Warning signs:** 在冲突对话框中按 `r` 时出现意外行为。

### Pitfall 3: 选中项背景渲染顺序错误

**What goes wrong:** 先用 tview.Print() 打印文本，再用 SetContent 填充背景色，导致文本被空格覆盖而消失。

**Why it happens:** tview.Print() 内部调用 screen.SetContent() 设置字符和样式。如果之后再用 screen.SetContent() 设置同一位置的背景，会覆盖之前设置的字符。

**How to avoid:** 渲染顺序必须是：(1) 填充整行背景色（如有选中），(2) 在填充后的背景上用 tview.Print() 打印文本。

**Warning signs:** 选中项显示为空白行而非带背景的路径文本。

### Pitfall 4: Draw() 中 SetRect 未调用导致 Box 渲染到错误位置

**What goes wrong:** RecentDirs 的 Box 没有被设置 rect（x, y, width, height），导致 DrawForSubclass 在错误位置渲染边框，或者渲染到 (0,0) 位置。

**Why it happens:** tview.Box 需要知道自己的 rect 才能正确渲染边框和背景。如果 RecentDirs 没有被添加到 Flex 布局中（它不应该被添加），则需要手动调用 SetRect 设置位置和大小。

**How to avoid:** 在 RecentDirs.Draw() 中，调用 `rd.Box.DrawForSubclass(screen, rd)` 之前，先调用 `rd.SetRect(x, y, width, height)` 设置弹窗位置和大小。由于 Draw() 在每次重绘时都会被调用，这自然处理了窗口 resize。

**Warning signs:** 弹窗边框出现在屏幕左上角 (0,0) 位置。

### Pitfall 5: Esc 在弹窗可见时穿透到 FileBrowser.close()

**What goes wrong:** 弹窗打开时按 Esc，同时关闭弹窗和整个文件浏览器。

**Why it happens:** 当前 handleGlobalKeys 的 Esc 处理只检查 TransferModal 可见性（第 36-41 行），不检查 RecentDirs 可见性。如果 RecentDirs.HandleKey 没有消费 Esc 事件，事件会传播到 FileBrowser.close()。

**How to avoid:** handleGlobalKeys 最前面的 overlay 检查确保 RecentDirs 可见时所有按键都被消费（返回 nil）。RecentDirs.HandleKey 对 Esc 返回 nil（消费事件并调用 Hide()）。

### Pitfall 6: 空列表时弹窗尺寸为 0 导致异常

**What goes wrong:** 路径列表为空时，高度计算为 `0 + 2 = 2`，减去边框后内部高度为 0，无法渲染"暂无最近目录"文本。

**Why it happens:** CONTEXT.md D-01 规定高度 = min(路径数量 + 2, 15)。当路径数量为 0 时，高度为 2（只有标题行 + 边框），没有空间渲染空状态文本。

**How to avoid:** 空列表时设置最小高度为 5（标题行 + 空状态文本行 + 上下边框内边距），确保有足够空间渲染提示文本。

### Pitfall 7: kitty 透明背景下的 ghost artifacts

**What goes wrong:** 在 kitty 终端（透明背景）中，弹窗关闭后残留文本或背景色块。

**Why it happens:** kitty 的 composited 背景与特定颜色不匹配时，残留内容不会自动清除。FileBrowser.Draw() 已通过 `tcell.ColorDefault` 填充解决此问题，但 overlay 关闭后的第一帧可能残留。

**How to avoid:** FileBrowser.Draw() 中的 `tcell.ColorDefault` 背景填充会自然覆盖残留。弹窗关闭后 QueueUpdateDraw 触发重绘即可清除。D-09 正确指出不需要额外 app.Sync()。

## Code Examples

Verified patterns from existing codebase:

### handleGlobalKeys overlay 检查模式（现有代码）
```go
// Source: file_browser_handlers.go line 31-56
func (fb *FileBrowser) handleGlobalKeys(event *tcell.EventKey) *tcell.EventKey {
    switch event.Key() {
    case tcell.KeyTab:
        fb.switchFocus()
        return nil
    case tcell.KeyESC:
        if fb.transferModal != nil && fb.transferModal.IsVisible() {
            fb.transferModal.HandleKey(event)
            return nil
        }
        fb.close()
        return nil
    // ...
    }
    return event
}
```

### TransferModal HandleKey 模式（参考实现）
```go
// Source: transfer_modal.go line 348-414
func (tm *TransferModal) HandleKey(event *tcell.EventKey) *tcell.EventKey {
    if !tm.visible {
        return event
    }
    switch tm.mode {
    // ... mode-specific handling ...
    case modeSummary:
        tm.Hide()
        return nil
    }
    return event
}
```

### tview.Print 渲染模式（参考实现）
```go
// Source: transfer_modal.go line 141-167
func (tm *TransferModal) drawProgress(screen tcell.Screen, x, y, width, _ int) {
    row1 := y + 1
    tview.Print(screen, tm.fileLabel, x, row1, width, tview.AlignCenter, tcell.Color255)
    // ...
}
```

### Box.DrawForSubclass + SetRect 模式
```go
// Source: transfer_modal.go line 120-137
func (tm *TransferModal) Draw(screen tcell.Screen) {
    if !tm.visible {
        return
    }
    tm.Box.DrawForSubclass(screen, tm)
    x, y, width, height := tm.GetInnerRect()
    // ... render content within inner rect ...
}
```

### RecentDirs 现有结构（Phase 4 已创建）
```go
// Source: recent_dirs.go line 30-47
type RecentDirs struct {
    *tview.Box
    paths   []string
    visible bool
}

func NewRecentDirs() *RecentDirs {
    rd := &RecentDirs{
        Box:     tview.NewBox(),
        paths:   make([]string, 0, maxRecentDirs),
        visible: false,
    }
    rd.SetBorder(true).
        SetBorderColor(tcell.Color238).
        SetTitleColor(tcell.Color250).
        SetBackgroundColor(tcell.Color232)
    return rd
}
```

### NavigateTo 方法（Phase 4 已添加）
```go
// Source: remote_pane.go line 306-317
func (rp *RemotePane) NavigateTo(path string) {
    if !rp.connected {
        return
    }
    rp.currentPath = path
    rp.selected = make(map[string]bool)
    rp.Refresh()
}
```

### 弹窗尺寸计算（CONTEXT.md D-01 伪代码）
```go
// CONTEXT.md 中给定的计算公式
width := termWidth * 60 / 100
if width > 80 { width = 80 }
height := len(paths) + 2
if height > 15 { height = 15 }
if height < 3 { height = 3 }
x := (termWidth - width) / 2
y := (termHeight - height) / 2
```

### Enter 选中后的完整流程（CONTEXT.md）
```go
// CONTEXT.md 中给定的流程
fb.recentDirs.Hide()
fb.remotePane.NavigateTo(selectedPath)
fb.recentDirs.Record(selectedPath)
fb.app.SetFocus(fb.remotePane)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| N/A | overlay via FileBrowser.Draw() + manual tview.Print() | 项目建立时（TransferModal） | Phase 5 必须修复 Draw() 调用缺失问题 |

**Deprecated/outdated:**
- 无。Phase 5 遵循项目已有的 overlay 模式，无弃用 API。

## Open Questions

1. **TransferModal.Draw() 是否真的从未被调用？**
   - What we know: 搜索全代码库，`transferModal.Draw` 和 `recentDirs.Draw` 均无调用。FileBrowser.Draw() 只调用 `fb.Flex.Draw(screen)`。TransferModal 不是 Flex 子元素。
   - What's unclear: 应用是否通过某种未发现的 tview 内部机制绘制 TransferModal。可能在某些终端环境下 Draw 确实被调用，但在其他环境下不工作。
   - Recommendation: Phase 5 在 FileBrowser.Draw() 中同时添加两个 overlay 的 Draw 调用。即使 TransferModal 当前能渲染，添加调用也是无害的（重复 Draw 只是覆盖同一内容）。如果不能渲染，这修复了一个预存 bug。

2. **Show() 时是否需要 ForceDraw？**
   - What we know: TransferModal.Show() 只设置 internal state，不触发重绘。`r` 键返回 nil 后 tview 的正常 draw cycle 会触发重绘。
   - What's unclear: 是否存在 Show() 后不触发重绘的边界情况。
   - Recommendation: 不调用 ForceDraw。让 tview 的正常 draw cycle 处理。如果有问题再添加。

3. **Record() 在 Enter 选中时是否应该触发？**
   - What we know: CONTEXT.md D-10 明确要求调用 Record(path)。Phase 4 CONTEXT.md D-07 也指出"Phase 5 中用户从最近列表选择路径跳转后，该路径重新提升到列表头部"。
   - What's unclear: NavigateTo 不触发 onPathChange，所以 Record 不会自动被调用，必须手动调用。
   - Recommendation: 在 HandleKey 的 Enter 处理中显式调用 Record(path)。

## Environment Availability

Step 2.6: SKIPPED (no external dependencies identified -- Phase 5 is pure Go code changes within the existing codebase, using only tview/tcell which are already in go.mod)

## Validation Architecture

> SKIPPED: `workflow.nyquist_validation` is explicitly set to false in .planning/config.json

## Sources

### Primary (HIGH confidence)
- 项目源码 `internal/adapters/ui/file_browser/` 全部 9 个文件 -- 直接代码分析
- `05-CONTEXT.md` -- 用户锁定决策 D-01 到 D-13
- `.planning/research/ARCHITECTURE.md` -- overlay 渲染分析、数据流、集成点
- `.planning/research/PITFALLS.md` -- 11 个 pitfall 及预防策略
- `.planning/phases/04-directory-history-core/04-CONTEXT.md` -- Phase 4 锁定决策（D-03 overlay pattern, D-07 re-record, D-09 NavigateTo）

### Secondary (MEDIUM confidence)
- Git 历史分析（commit 288a3b6, 371e9d0, 9deebc5, cd0a58b）-- 确认 TransferModal.Draw() 从未被调用
- `.planning/REQUIREMENTS.md` -- POPUP-01 到 POPUP-05, AUX-01 需求定义

### Tertiary (LOW confidence)
- 无。所有发现均基于直接代码分析和用户决策。

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - 零新依赖，使用已有 tview/tcell
- Architecture: HIGH - 基于直接代码分析和 TransferModal 模式参考
- Pitfalls: HIGH - 7 个 pitfall 均基于代码审查确认，包含 TransferModal.Draw() 缺失的 git 历史验证

**Research date:** 2026-04-14
**Valid until:** 30 days（tview/tcell API 稳定，项目内部代码变更不影响研究结论）
