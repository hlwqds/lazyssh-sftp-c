# Technology Stack Research

**Analysis Date:** 2026-04-14
**Domain:** TUI File Transfer for Go SSH Manager

---

## v1.1 Recent Remote Directories — Stack Additions

**结论: 零新外部依赖。** 全部 UI 能力由已有 tview/tcell 提供，数据结构由 Go 标准库提供。

### UI 组件

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `tview.List` | current (pre-release 2025-06-25) | 弹出式目录列表 | 已在 `ServerList` 中验证，内置 j/k/Enter/Esc 键盘导航和选中高亮 |
| `app.SetRoot()` | current | 临时全屏覆盖弹窗 | 已在 `handlers.go:272-276`、`server_form.go:1925` 中用于确认弹窗 |
| `tview.List.SetBorder(true)` | current | 弹窗边框 + 标题 | 无需额外容器组件 |

### 数据结构

| Package | Purpose | Why |
|---------|---------|-----|
| `container/list` (stdlib) | 有序去重目录历史 | 双向链表，O(1) 头部插入 + O(1) 删除已有条目，语义匹配"最近使用" |

### 弹出层实现方案

**推荐方案: `tview.List` + `app.SetRoot()` 临时覆盖**

原理: 按 `r` 键时创建 `tview.List`，填充历史目录，调用 `app.SetRoot(list, true)` 临时替换根组件。关闭时调用 `app.SetRoot(fileBrowser, true)` 恢复。

代码模式参考 `handlers.go:272-276`:
```go
// 已有模式: 创建弹窗 -> 设置回调 -> SetRoot
modal := tview.NewModal().
    SetText("...").
    AddButtons([]string{"Delete", "Cancel"}).
    SetDoneFunc(func(buttonIndex int, buttonLabel string) { ... })
t.app.SetRoot(modal, true)
```

近期目录弹窗:
```go
list := tview.NewList().
    ShowSecondaryText(false).
    SetBorder(true).
    SetTitle(" Recent Directories ").
    SetSelectedBackgroundColor(tcell.Color24).
    SetSelectedTextColor(tcell.Color255).
    SetHighlightFullLine(true)

for _, dir := range recentDirs {
    list.AddItem(dir, "", 0, nil)
}

list.SetSelectedFunc(func(index int, mainText string, _, _ rune) {
    // 跳转到选中目录
    rp.NavigateToPath(mainText)
    app.SetRoot(fb, true)
    app.SetFocus(rp)
})

list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
    if event.Key() == tcell.KeyEscape {
        app.SetRoot(fb, true)
        app.SetFocus(rp)
        return nil
    }
    return event // j/k 由 tview.List 内置处理
})

app.SetRoot(list, true)
```

### Keyboard Interaction

| Key | Behavior | 需要自定义? |
|-----|----------|------------|
| `j` / ArrowDown | 下一项 | 否（tview.List 内置） |
| `k` / ArrowUp | 上一项 | 否（tview.List 内置） |
| `Enter` | 选中跳转 | 否（配置 `SetSelectedFunc`） |
| `Esc` | 关闭弹窗 | 是（`SetInputCapture` 中处理） |

### 快捷键冲突分析

| Key | 当前绑定位置 | 冲突? |
|-----|-------------|-------|
| `r` | `RemotePane.SetInputCapture` — 未使用 | 无冲突 |
| `r` | `TransferModal.HandleKey` — `case 'r': // Rename` | 无冲突（TransferModal 仅在传输中激活，且有自己的 HandleKey 链） |
| `r` | `handleGlobalKeys` — 未使用 | 无冲突 |

### 集成点

1. **键盘入口:** `file_browser_handlers.go` 的 `handleGlobalKeys` 添加 `case 'r'`
2. **目录记录:** `RemotePane.NavigateInto()` 的 `onPathChange` 回调（已存在于 `file_browser.go:131`）
3. **弹窗管理:** `FileBrowser.showRecentDirs()` 新方法

### What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `tview.Modal` | 仅支持按钮(AddButtons)，不支持列表项选择 | `tview.List` + `SetBorder(true)` |
| `tview.Table` | 多列组件，对此场景过度设计 | `tview.List` |
| `tview.Form` | 用于表单输入，不适合纯选择列表 | `tview.List` |
| `tview.DropDown` | 下拉框嵌入布局中，不适合全屏弹窗 | `tview.List` + `app.SetRoot()` |
| 自定义 `*tview.Box` + `Draw()` | TransferModal 的 Draw() 未被任何布局树调用，模式不完整 | `tview.List`（标准原语，自动参与 Draw 周期） |
| `tview.Pages` | 代码库中未使用，需重构 FileBrowser 布局 | `app.SetRoot()` 临时覆盖 |

---

## v1.0 File Transfer — Existing Stack (Preserved)

| Component | Technology | Version |
|-----------|-----------|---------|
| Language | Go | 1.24.6 |
| TUI Framework | tview/tcell | tview latest, tcell/v2 2.9.0 |
| CLI | Cobra | 1.9.1 |
| Logging | Zap | 1.27.0 |
| SSH Config | ssh_config (forked) | 1.4.0 |
| SFTP Client | pkg/sftp | latest (v1.13.x) |

## Sources

- **项目代码库（HIGH confidence）:**
  - `internal/adapters/ui/server_list.go` -- `*tview.List` 嵌入模式
  - `internal/adapters/ui/handlers.go:272-276` -- `tview.NewModal()` + `app.SetRoot()` 弹窗模式
  - `internal/adapters/ui/file_browser/transfer_modal.go` -- 自定义 overlay 模式
  - `internal/adapters/ui/file_browser/file_browser.go` -- FileBrowser 布局结构
  - `internal/adapters/ui/file_browser/remote_pane.go` -- 目录导航 + onPathChange 回调
  - `internal/adapters/ui/file_browser/file_browser_handlers.go` -- 全局键盘处理入口
  - `go.mod` -- tview v0.0.0-20250625164341, tcell/v2 v2.9.0

---
*Stack research: 2026-04-14 (v1.1 Recent Remote Directories)*
*Original: 2026-04-13 (v1.0 File Transfer)*
