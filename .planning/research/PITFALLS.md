# Pitfalls Research: Recent Remote Directories Popup (v1.1)

**Domain:** tview TUI 弹出列表叠加层集成
**Researched:** 2026-04-14
**Confidence:** HIGH -- 基于代码审查和 tview 源码分析

## Critical Pitfalls

### Pitfall 1: 'r' 键在 TransferModal 冲突对话框中已被占用

**What goes wrong:**
用户打开最近目录弹出列表，按下 `r` 键，本意是触发弹出功能。但如果此时 TransferModal 正处于冲突解决模式（`modeConflictDialog`），`r` 会被冲突对话框的 `HandleKey` 拦截并执行 "Rename" 操作。两个功能使用同一个按键但处于不同上下文中，如果不正确处理层级关系，会导致不可预测的行为。

**Why it happens:**
当前代码中，`TransferModal.HandleKey` 在 `modeConflictDialog` 模式下消费所有按键（第 375 行 `return nil`），包括 `r` 键（第 368-373 行）。FileBrowser 的 `handleGlobalKeys` 在 `case tcell.KeyESC` 中会检查 `fb.transferModal.IsVisible()` 并委托给 modal。但 `handleGlobalKeys` 的 rune switch 中并没有 `case 'r'`。

如果未来在 `handleGlobalKeys` 中添加 `case 'r'` 来弹出最近目录列表，需要确保当 TransferModal 可见时该按键不被处理。否则，在传输冲突对话框中按 `r` 会同时触发重命名和弹出列表。

**How to avoid:**
1. 在 `handleGlobalKeys` 中添加 `case 'r'` 时，必须**先检查 `fb.transferModal != nil && fb.transferModal.IsVisible()`**，如果 modal 可见则返回 event（让 modal 处理）或直接 return nil
2. 更好的做法：将弹出列表的按键改为仅在非 modal 状态下可用，在 handleGlobalKeys 中添加守卫条件
3. 记录按键绑定冲突矩阵：`r` = Rename（冲突对话框内）vs `r` = Recent dirs（全局快捷键）

**Warning signs:**
- 在冲突对话框中按 `r` 时出现异常行为（弹出列表闪烁或重命名+列表同时触发）
- Transfer 的 conflictHandler channel 收到意外的事件

**Phase to address:**
Phase 1（键盘路由实现）-- 在编写 handleGlobalKeys 的 `case 'r'` 时立即处理

---

### Pitfall 2: 弹出列表打开时，'j'/'k' 键泄漏到背景 Table

**What goes wrong:**
弹出最近目录列表后，用户按 `j` 或 `k` 试图在列表中上下移动选择。但由于 tview 的按键路由机制，如果弹出列表没有正确获取焦点，这些按键会泄漏到背景的 `RemotePane`（tview.Table），导致 Table 的行选择也跟着移动。

**Why it happens:**
tview 的按键传播链为：`Application.InputCapture` -> root `SetInputCapture` -> focused primitive `SetInputCapture` -> focused primitive `InputHandler`。如果弹出列表组件没有成为焦点（`app.GetFocus()` 返回的不是弹出列表），按键就会传给之前的焦点持有者（RemotePane Table）。

当前代码的事件传播链（file_browser_handlers.go 第 22-26 行注释）：
```
1. FileBrowser.SetInputCapture -> handles Tab, Esc, s, S
2. FocusedPane.SetInputCapture -> handles h, Backspace, Space, .
3. Table.InputHandler -> handles j/k/arrow/Enter/PgUp/PgDn (built-in)
```

弹出列表必须插入到这条链中，成为新的焦点持有者，使 step 2 和 step 3 被弹出列表的 InputCapture 替代。

**How to avoid:**
1. 弹出列表显示时必须调用 `fb.app.SetFocus(popup)` 使其成为焦点
2. 弹出列表的 `SetInputCapture` 必须消费 `j`/`k`/`Enter`/`Esc`（返回 nil），不返回 event
3. 在 FileBrowser 的 `handleGlobalKeys` 中添加弹出列表可见性检查：如果弹出列表可见，所有按键先委托给弹出列表处理
4. 参考 TransferModal 的模式：modal 不通过 `SetFocus` 获取焦点，而是通过 `handleGlobalKeys` 中的 `case tcell.KeyESC` 委托。但弹出列表需要处理更多按键（j/k/Enter），所以应使用 `SetFocus` 模式

**Warning signs:**
- 弹出列表显示时，背景 Table 的选择行在移动
- 按 `j`/`k` 时弹出列表和 Table 同时滚动

**Phase to address:**
Phase 1（键盘路由和焦点管理）

---

### Pitfall 3: 弹出列表关闭后焦点未恢复到之前的 Pane

**What goes wrong:**
用户在 RemotePane 中按 `r` 打开弹出列表，选择一个目录后跳转，但弹出列表关闭后焦点丢失。按 Tab 不再在 local/remote pane 间切换，或者焦点跑到错误的 pane 上。

**Why it happens:**
tview 没有内置的焦点栈（focus stack）。调用 `app.SetFocus(popup)` 后，之前持有焦点的 primitive 信息丢失。如果不显式记录并恢复之前的焦点，关闭弹出列表后焦点可能停留在弹出列表组件本身（如果它仍在 primitive 树中），或者回退到默认的第一个可聚焦子元素。

当前 TransferModal 使用了 `fb.currentPane()` 来恢复焦点（file_browser.go 第 115 行），这是一个可行的模式。但弹出列表的关闭路径更多：Enter 选择跳转、Esc 取消关闭、列表为空时自动关闭。

**How to avoid:**
1. 在弹出列表显示前，记录当前焦点：`previousFocus := fb.app.GetFocus()`
2. 在所有关闭路径（选择跳转、Esc 取消、空列表关闭）中，统一恢复焦点：`fb.app.SetFocus(previousFocus)`
3. 注意：如果弹出列表是通过 `Enter` 选择了一个目录并跳转，跳转后应该将焦点保持在 RemotePane（因为用户是在远程面板中操作），而不是回到之前可能不同的 pane
4. 实现一个 `dismiss()` 方法统一处理所有关闭逻辑，避免遗漏某个关闭路径

**Warning signs:**
- 关闭弹出列表后，Tab 切换行为异常
- 焦点视觉指示（边框颜色）与实际按键接收者不一致
- `fb.activePane` 状态与 `fb.app.GetFocus()` 返回值不一致

**Phase to address:**
Phase 1（焦点管理实现）

---

### Pitfall 4: 弹出列表绘制被背景组件遮挡或残留在关闭后

**What goes wrong:**
弹出列表显示时看不到它（被 Table 或 StatusBar 遮挡），或者关闭弹出列表后，列表的文本残留在屏幕上（"ghost" artifacts）。

**Why it happens:**
tview 没有 z-index 系统。绘制顺序由 primitive 在容器中的添加顺序决定。如果弹出列表作为一个独立的 primitive（不作为 FileBrowser Flex 的子元素），它不会被 Flex.Draw() 绘制。

当前 TransferModal 使用了 `SetAfterDrawFunc` 方式：FileBrowser 自己重写了 `Draw()` 方法（file_browser.go 第 209-218 行），TransferModal 则依赖 FileBrowser 的 `Draw()` 被调用后，再通过 `SetAfterDrawFunc` 绘制 StatusBar。

但如果弹出列表也是一个自定义 primitive（嵌入 `*tview.Box`），它需要：
1. 在 FileBrowser 的 `Draw()` 方法中被调用，或者
2. 通过 `SetAfterDrawFunc` 绘制（但 FileBrowser 已经用了这个来绘制 StatusBar），或者
3. 作为 FileBrowser Flex 的子元素添加（但这样会影响 Flex 的布局比例）

**How to avoid:**
1. **推荐方案**：在 FileBrowser.Draw() 中，在调用 `fb.Flex.Draw(screen)` 之后，检查弹出列表是否可见，如果可见则调用 `popup.Draw(screen)`。这与 TransferModal 的绘制方式一致（虽然 TransferModal 实际上是通过 `SetAfterDrawFunc` 间接绘制的）
2. **替代方案**：将弹出列表作为 FileBrowser Flex 的子元素，使用 `AddItem(popup, 0, 0, false)` 添加（proportion=0 表示不占 Flex 空间），然后在 Draw 中手动设置其 rect
3. 弹出列表的 `Draw()` 方法必须在 `visible == false` 时直接 return（参考 TransferModal 第 121-123 行）
4. 关闭时调用 `fb.app.Draw()` 或 `fb.app.QueueUpdateDraw(func(){})` 强制重绘，清除残留
5. 注意 kitty 透明度背景问题：FileBrowser.Draw() 中已经用 `tcell.ColorDefault` 填充背景来避免 ghost artifacts，弹出列表也需要遵循同样的模式

**Warning signs:**
- 弹出列表显示瞬间一闪然后消失
- 关闭弹出列表后列表文字残留在屏幕上
- 在 kitty 终端（透明背景）中出现叠加残影

**Phase to address:**
Phase 1（绘制和视觉集成）

---

## High Pitfalls

### Pitfall 5: OnPathChange 回调在弹出列表打开时记录路径，导致数据污染

**What goes wrong:**
用户打开弹出列表浏览最近目录时，如果远程导航（通过 Enter 选择目录跳转）触发了 `OnPathChange` 回调，该路径会被记录到最近目录列表中。但用户可能只是浏览后按 Esc 取消，不应该记录这次导航。更糟糕的是，如果弹出列表打开时后台有异步的 SFTP 操作触发路径变更回调，会记录错误的路径。

**Why it happens:**
当前代码中，`OnPathChange` 回调在 `NavigateInto()` 中被调用（remote_pane.go 第 298-299 行），FileBrowser 的 build() 方法中注册了两个 OnPathChange 回调（file_browser.go 第 128-133 行），目前只用于 `fb.app.Sync()`。v1.1 需要添加路径记录逻辑。

如果弹出列表的 "选择并跳转" 操作通过修改 `remotePane.currentPath` 并调用 `Refresh()` 实现，这会触发 `NavigateInto()` 或类似的路径变更方法，进而触发 `OnPathChange`。问题在于：记录应该只在用户确认跳转后发生，而不是在弹出列表内部预览时。

**How to avoid:**
1. 在记录路径的 OnPathChange 回调中添加守卫条件：如果弹出列表可见，不记录路径
2. 或者使用单独的 `RecordPath(path string)` 方法，只在弹出列表的 Enter 选择确认时显式调用
3. 不要依赖 NavigateInto 的副作用来记录路径 -- 将记录逻辑与导航逻辑解耦
4. 对于弹出列表内的预览导航（如果有），使用一个临时状态标志 `fb.popupNavigating = true` 来抑制记录

**Warning signs:**
- 取消弹出列表后，刚才浏览过的路径出现在最近列表中
- 最近目录列表中出现重复条目或顺序混乱

**Phase to address:**
Phase 2（路径记录逻辑）

---

### Pitfall 6: 弹出列表在未连接状态下触发导致空列表或崩溃

**What goes wrong:**
用户在 RemotePane 未连接 SFTP 时按 `r` 键，弹出空列表或显示 "No recent directories"。虽然功能上无害，但用户体验差。更严重的情况是，如果代码假设 SFTP 连接存在，可能在获取当前路径时产生 nil pointer dereference。

**Why it happens:**
RemotePane 有三种状态：Connecting、Connected、Error。在未连接状态下，`GetCurrentPath()` 返回的是初始值 `"."`（remote_pane.go 第 51 行），这是一个无意义的路径。如果在未连接时记录路径，会产生无效条目。

**How to avoid:**
1. 在 `handleGlobalKeys` 的 `case 'r'` 中，检查 `fb.remotePane.IsConnected()`，如果未连接则显示状态栏提示（参考 initiateTransfer 的模式，file_browser.go 第 263-264 行）
2. 在路径记录逻辑中，也添加连接状态检查
3. 弹出列表的数据源应该只包含已验证的有效路径

**Warning signs:**
- 在 "Connecting..." 状态下按 `r` 弹出包含 "." 路径的列表
- 断线重连后，最近目录列表中混入了无效路径

**Phase to address:**
Phase 1（按键守卫条件）

---

## Medium Pitfalls

### Pitfall 7: tview Modal vs 自定义 Primitive 的架构选择不当

**What goes wrong:**
选择使用 `tview.Modal`（内建模态框）来实现弹出列表，发现无法满足定制需求（自定义绘制、键盘绑定、位置控制）。或者选择纯自定义 primitive，但花费大量时间重复实现 Modal 已有的功能（背景遮罩、按键拦截）。

**Why it happens:**
tview 的 `Modal` 是一个全屏居中的对话框组件，自带背景遮罩和按键处理。但它被设计用于简单的 "Yes/No" 确认对话框，不适合用作可滚动的目录列表。当前代码中的 TransferModal 是一个完全自定义的 primitive（嵌入 `*tview.Box`，手动实现 Draw），这说明项目已经选择了自定义 primitive 路线。

**How to avoid:**
1. **使用自定义 primitive**，与 TransferModal 保持架构一致。嵌入 `*tview.Box`，手动实现 `Draw()`、`HandleKey()`
2. 弹出列表应该定位在 RemotePane 的区域内（类似 IDE 的下拉列表），而不是全屏居中
3. 如果需要背景遮罩效果，在 Draw() 中先绘制半透明背景
4. 参考 TransferModal 的模式：visible 标志、Show/Hide 方法、HandleKey 统一入口

**Why custom primitive is correct here:**
- 需要精确定位（锚定到 RemotePane 附近）
- 需要自定义绘制（目录路径列表、滚动指示器）
- 需要自定义键盘处理（j/k 导航、Enter 选择、Esc 关闭）
- 项目已有 TransferModal 作为自定义 primitive 的先例

**Phase to address:**
Phase 1（架构决策）

---

### Pitfall 8: 弹出列表只有一个或零个条目时的边界情况

**What goes wrong:**
弹出列表为空时显示一个空白框。只有一个条目时，用户按 `j`/`k` 导致选择越界或崩溃。

**Why it happens:**
tview.Table 的 `Select(row, col)` 在 row 超出范围时不会崩溃，但自定义的弹出列表如果用简单的 index 来跟踪选中项，`index+1` 或 `index-1` 可能越界。

**How to avoid:**
1. 空列表：不显示弹出列表，改为在状态栏显示 "No recent directories" 提示（参考 updateStatusBarTemp 模式）
2. 单条目列表：显示列表，j/k 不移动选择（已到首/末条），Enter 直接选择唯一项
3. 在 HandleKey 中对 j/k 做边界检查：`if selectedIndex > 0 { selectedIndex-- }`
4. 添加 `len(items) == 0` 的早期返回

**Warning signs:**
- 空列表时 panic: index out of range
- 单条目时按 `j` 后无法选择任何项

**Phase to address:**
Phase 1（边界处理）

---

### Pitfall 9: 路径去重和排序逻辑不正确

**What goes wrong:**
最近目录列表中出现重复路径（同一目录被多次记录），或者路径排序不符合用户预期（最新访问的不在最上面）。

**Why it happens:**
如果使用简单的 slice append 来记录路径，每次导航都会添加新条目。如果使用 map 去重但保留了旧的时间戳，最新访问的目录可能排在中间。

**How it happens (具体到代码):**
- NavigateInto 在每次进入目录时触发 OnPathChange
- 用户在同一目录间反复导航（进入 -> 返回 -> 再进入）会产生重复条目
- 如果去重策略是 "已存在则不添加"，则时间戳不会更新，排序会错乱

**How to avoid:**
1. 使用 `[]string`（有序 slice）而不是 `map[string]bool`（无序 set）来维护列表
2. 添加路径时：先检查是否已存在，如果存在则移除旧条目，然后在头部插入新条目
3. 保持最大长度为 10：插入后如果超过 10 条，截断尾部
4. 路径比较应该使用规范化的绝对路径（去除尾部 `/`，解析 `.` 和 `..`）

```go
// 伪代码：正确的去重+排序
func (r *RecentDirs) Record(path string) {
    normalized := normalizePath(path)
    // 移除已存在的条目
    for i, p := range r.entries {
        if p == normalized {
            r.entries = append(r.entries[:i], r.entries[i+1:]...)
            break
        }
    }
    // 在头部插入
    r.entries = append([]string{normalized}, r.entries...)
    // 截断到最大长度
    if len(r.entries) > 10 {
        r.entries = r.entries[:10]
    }
}
```

**Phase to address:**
Phase 2（路径记录逻辑）

---

## Low Pitfalls

### Pitfall 10: Esc 键的双重含义（关闭弹出列表 vs 关闭文件浏览器）

**What goes wrong:**
弹出列表打开时，用户按 Esc 期望关闭弹出列表回到文件浏览器。但如果 Esc 处理不当，可能会同时关闭弹出列表和文件浏览器。

**Why it happens:**
当前 FileBrowser 的 `handleGlobalKeys` 中，`case tcell.KeyESC` 先检查 TransferModal 是否可见（第 37-40 行），如果不可见则调用 `fb.close()` 关闭整个文件浏览器。如果在弹出列表可见时没有添加类似的检查，Esc 会直接关闭文件浏览器。

**How to avoid:**
1. 在 `handleGlobalKeys` 的 `case tcell.KeyESC` 中，优先检查弹出列表可见性
2. 事件处理顺序：TransferModal 可见 -> 弹出列表可见 -> 关闭文件浏览器
3. 弹出列表的 Esc 处理返回 nil（消费按键），确保不会传播到 FileBrowser.close()

**Phase to address:**
Phase 1（按键路由）

---

### Pitfall 11: 弹出列表位置在窗口大小改变后不正确

**What goes wrong:**
用户调整终端窗口大小后，弹出列表的位置偏移，部分超出屏幕或遮挡错误的区域。

**Why it happens:**
弹出列表的位置通常基于 RemotePane 的 `GetRect()` 计算。窗口 resize 后 tview 会重新布局所有 primitive 并重绘，但弹出列表的 rect 如果是在 Show() 时一次性计算的，resize 后不会自动更新。

**How to avoid:**
1. 弹出列表的 `Draw()` 方法中动态计算位置（基于 `fb.remotePane.GetRect()`），而不是在 Show() 时缓存
2. 或者监听 resize 事件并重新计算位置
3. 最简方案：在 Draw() 中每次根据当前 RemotePane 的 rect 重新定位

**Phase to address:**
Phase 1（绘制逻辑）或 Phase 3（polish）

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| 将弹出列表作为 FileBrowser Flex 的子元素而非独立 overlay | 不需要修改 Draw() 方法 | 弹出列表影响 Flex 布局计算，resize 时可能出问题 | Never -- 使用 Draw() 中手动绘制更可靠 |
| 用 tview.List 替代自定义绘制 | 快速实现，List 自带选中高亮 | 无法精确定位到 RemotePane 附近，全屏显示不符合设计 | Never -- 需要 pane-anchored 定位 |
| 在 NavigateInto 中直接记录路径（不使用单独的 RecordPath） | 代码更少，不需要额外的记录调用 | 耦合度高，弹出列表预览时会污染记录 | Never -- 记录和导航必须解耦 |
| 使用全局变量存储最近目录列表 | 不需要传递引用 | 违反 Clean Architecture，多服务器场景下数据混乱 | Never -- 数据应属于 FileBrowser 实例 |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| handleGlobalKeys + 弹出列表 | 在 `switch event.Rune()` 中添加 `case 'r'` 但没有检查弹出列表/TransferModal 状态 | 在 `case 'r'` 之前先检查所有 overlay 的可见性状态 |
| OnPathChange + 路径记录 | 直接在 OnPathChange 回调中 append 路径 | 使用守卫条件或单独的 RecordPath 方法 |
| RemotePane.NavigateInto + 弹出列表选择 | 复用 NavigateInto 来实现弹出列表的目录跳转，导致路径被记录两次 | 弹出列表确认后直接修改 currentPath + Refresh，不经过 NavigateInto |
| SetAfterDrawFunc + 弹出列表绘制 | 试图用 SetAfterDrawFunc 绘制弹出列表，但 FileBrowser 已用它绘制 StatusBar | 在 FileBrowser.Draw() 中手动绘制弹出列表，或修改 SetAfterDrawFunc 同时处理两者 |
| app.SetFocus + Tab 切换 | 弹出列表打开后，Tab 仍然触发 switchFocus，导致焦点混乱 | handleGlobalKeys 中 Tab 检查弹出列表可见性，可见时不切换 |

---

## "Looks Done But Isn't" Checklist

- [ ] **按键守卫**: `case 'r'` 在 TransferModal 可见和弹出列表可见时都被正确拦截 -- verify: 在冲突对话框中按 `r` 只触发重命名
- [ ] **焦点恢复**: 弹出列表关闭后 Tab 仍然正确切换 pane -- verify: 打开弹出列表 -> Esc 关闭 -> Tab 切换
- [ ] **空列表**: 没有最近目录时按 `r` 不弹出空白框 -- verify: 首次打开文件浏览器时按 `r`
- [ ] **单条目**: 只有一条记录时 j/k 不崩溃 -- verify: 记录一个目录后按 `r`，按 `j` 和 `k`
- [ ] **Esc 不穿透**: 弹出列表中按 Esc 只关闭弹出列表，不关闭文件浏览器 -- verify: 打开弹出列表 -> Esc -> 确认仍在文件浏览器中
- [ ] **未连接**: SFTP 未连接时按 `r` 有友好提示 -- verify: 在 Connecting 状态下按 `r`
- [ ] **路径去重**: 同一目录多次访问不产生重复条目 -- verify: 进入目录 -> 返回 -> 再进入 -> 按 `r` 检查列表
- [ ] **绘制清除**: 关闭弹出列表后无屏幕残留 -- verify: 打开弹出列表 -> Esc -> 检查屏幕
- [ ] **kitty 透明背景**: 在 kitty（透明背景）下无 ghost artifacts -- verify: 在 kitty 中打开/关闭弹出列表
- [ ] **resize**: 调整终端窗口后弹出列表位置正确 -- verify: 打开弹出列表 -> resize 窗口 -> 检查位置

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| 按键路由错误（P1, P2） | LOW | 在 handleGlobalKeys 中添加 overlay 可见性检查，测试所有按键组合 |
| 焦点丢失（P3） | LOW | 保存 previousFocus 并在所有关闭路径中恢复 |
| 绘制残留（P4） | MEDIUM | 在 Draw() 中添加 visible 检查，调用 app.Sync() 强制重绘 |
| 路径污染（P5） | LOW | 添加守卫条件，清除无效数据 |
| 空列表崩溃（P8） | LOW | 添加 len == 0 早期返回和边界检查 |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| P1: 'r' 键冲突 | Phase 1 | 在冲突对话框中按 `r` 只触发 Rename |
| P2: j/k 泄漏到 Table | Phase 1 | 弹出列表可见时背景 Table 不响应 j/k |
| P3: 焦点未恢复 | Phase 1 | 关闭弹出列表后 Tab 切换正常 |
| P4: 绘制遮挡/残留 | Phase 1 | 打开/关闭弹出列表无视觉异常 |
| P5: OnPathChange 数据污染 | Phase 2 | 取消选择后路径不被记录 |
| P6: 未连接状态 | Phase 1 | Connecting 状态按 `r` 显示提示 |
| P7: Modal vs 自定义选择 | Phase 1 | 架构评审确认使用自定义 primitive |
| P8: 空列表/单条目边界 | Phase 1 | 边界测试不崩溃 |
| P9: 路径去重排序 | Phase 2 | 重复导航不产生重复条目 |
| P10: Esc 双重含义 | Phase 1 | Esc 只关闭弹出列表 |
| P11: resize 位置 | Phase 3 | resize 后弹出列表位置正确 |

---

## Sources

- [tview Concurrency Wiki](https://github.com/rivo/tview/wiki/Concurrency) -- HIGH confidence: tview 的线程安全模型，QueueUpdateDraw 用法
- [tview Issue #715: App keys not overridable by widgets](https://github.com/rivo/tview/issues/715) -- HIGH confidence: 按键路由链的权威说明
- [tview Issue #104: Popup/docked menu feature request](https://github.com/rivo/tview/issues/104) -- MEDIUM confidence: tview 没有内建 popup 组件
- [tview SetAfterDrawFunc Issue #65](https://github.com/rivo/tview/issues/65) -- HIGH confidence: SetAfterDrawFunc 的正确用法
- [tview Primitives Wiki](https://github.com/rivo/tview/wiki/Primitives) -- HIGH confidence: 自定义 primitive 的实现模式
- 项目源码分析 -- HIGH confidence: TransferModal 模式、handleGlobalKeys 路由链、OnPathChange 回调链

---
*Pitfalls research for: lazyssh v1.1 Recent Remote Directories*
*Researched: 2026-04-14*
