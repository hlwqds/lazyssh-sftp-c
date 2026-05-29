# Phase 10: Dup Fix - Research

**Researched:** 2026-04-15
**Domain:** tview TUI 事件处理、Go Clean Architecture 适配器层修改
**Confidence:** HIGH

## Summary

Phase 10 是一个精准的 bug fix 阶段：当前 `handleServerDup()` 在复制服务器后会打开 `ServerForm` 编辑表单（复用了 handleServerAdd 的流程），要求用户手动保存才能完成复制。需求是改为直接调用 `AddServer()` 保存并返回列表，省去表单编辑步骤。

这是一个纯行为变更，不涉及新组件或新依赖。核心修改集中在 `handlers.go` 的 `handleServerDup()` 函数：移除 ServerForm 创建和 SetRoot 切换，改为直接调用 `t.serverService.AddServer(dup)` 保存，然后 `refreshServerList()` + 定位到新条目 + 状态栏提示。现有的 `dupPendingAlias` + `handleServerSave` 中的滚动逻辑可以被简化——因为不再经过表单保存，直接在 dup handler 内完成定位即可。

**Primary recommendation:** 将 `handleServerDup()` 改为同步保存模式：深拷贝 -> 生成唯一 alias -> AddServer() -> refreshServerList() -> SetCurrentItem + showStatusTemp。移除与表单相关的所有代码路径。

## User Constraints

> CONTEXT.md 不存在，以下约束来自 REQUIREMENTS.md 和 STATE.md。

### Locked Decisions (from STATE.md)
- handleServerDup 移除 ServerForm 创建，直接调用 AddServer() 保存

### Claude's Discretion
- 无 — 本阶段需求明确，无需要裁量的设计决策

### Deferred Ideas (OUT OF SCOPE)
- 无 — 本阶段范围严格限定为 DUP-FIX-01 和 DUP-FIX-02

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DUP-FIX-01 | D 键复制后直接调用 AddServer() 添加新条目到列表，不自动打开 ServerForm 编辑表单 | 现有 `handleServerDup()` 第 342-348 行创建 ServerForm 并 SetRoot，需移除；`AddServer()` 在 server_service.go:126 已有实现 |
| DUP-FIX-02 | 复制后自动滚动列表到新条目（复用现有 dupPendingAlias 滚动逻辑） | 现有 `handleServerSave()` 第 374-382 行已有 dupPendingAlias 查找+SetCurrentItem 逻辑，但此逻辑依赖表单保存回调；改为在 dup handler 内直接实现 |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.24.6 | 核心语言 | 项目指定版本 |
| tview | v0.0.0 (local) | TUI 框架 | 项目已依赖，ServerList 基于 tview.List 构建 |
| tcell/v2 | v2.9.0 | 终端抽象 | tview 底层依赖 |

**无需安装新依赖。** 本阶段仅修改现有 Go 代码。

## Architecture Patterns

### 当前 Dup 流程（需要修改）

```
handleServerDup()                        handlers.go:288
  ├─ 深拷贝 server -> dup               handlers.go:295-337
  ├─ 生成唯一 alias                     handlers.go:303
  ├─ 设置 dupPendingAlias = dup.Alias   handlers.go:340
  └─ 创建 ServerForm + SetRoot(form)    handlers.go:343-348  ← 要移除
       └─ 用户编辑后点 Save
            └─ handleServerSave()        handlers.go:351
                 ├─ AddServer(server)    handlers.go:359
                 ├─ refreshServerList()  handlers.go:371
                 ├─ 查找 dupPendingAlias handlers.go:374-378
                 └─ SetCurrentItem(i)    handlers.go:378
```

### 目标 Dup 流程（修改后）

```
handleServerDup()                        handlers.go:288
  ├─ 深拷贝 server -> dup               handlers.go:295-337 (不变)
  ├─ 生成唯一 alias                     handlers.go:303 (不变)
  ├─ AddServer(dup) 直接保存            ← 新增
  ├─ refreshServerList()                ← 新增
  ├─ 查找 dup.Alias 在列表中的 index    ← 新增（内联 handleServerSave 的逻辑）
  ├─ SetCurrentItem(index)              ← 新增
  ├─ showStatusTemp("Server duplicated: alias")  ← 新增
  └─ 无表单、无 SetRoot 切换
```

### 模式 1: 直接保存 + 列表刷新 + 定位

**What:** 在 handler 中直接调用 service 方法保存，然后刷新列表并定位到新条目，全程不离开主界面。
**When to use:** 需要无弹窗、无模式切换的即时操作。
**Example:**

```go
func (t *tui) handleServerDup() {
    server, ok := t.serverList.GetSelectedServer()
    if !ok {
        return
    }

    // 深拷贝（现有逻辑，保留）
    dup := server
    dup.PinnedAt = time.Time{}
    dup.SSHCount = 0
    dup.LastSeen = time.Time{}
    dup.Alias = generateUniqueAlias(server.Alias, t.serverService)
    // ... slice 字段拷贝 ...

    // 直接保存
    if err := t.serverService.AddServer(dup); err != nil {
        t.showStatusTempColor(fmt.Sprintf("Dup failed: %v", err), "#FF6B6B")
        return
    }

    // 刷新列表并定位
    t.refreshServerList()
    if servers, _ := t.serverService.ListServers(""); servers != nil {
        for i, s := range servers {
            if s.Alias == dup.Alias {
                t.serverList.SetCurrentItem(i)
                break
            }
        }
    }
    t.showStatusTemp(fmt.Sprintf("Server duplicated: %s", dup.Alias))
}
```

### 模式 2: handleServerSave 中的 dupPendingAlias 清理

**What:** `handleServerSave()` 当前有 dup 相关逻辑（第 354-355 行、第 374-382 行）。dup 不再经过表单保存后，这些代码变成死代码。
**When to use:** 清理不再需要的条件分支。
**处理:** 移除 `handleServerSave()` 中的 `dupPendingAlias` 引用。同时移除 `tui` struct 中的 `dupPendingAlias` 字段（tui.go:53）。

### Anti-Patterns to Avoid
- **保留 dupPendingAlias 字段但不使用:** 即使不清理 tui struct 字段也不会出错，但违反项目代码整洁原则。应在本次修改中一并清理。
- **不刷新列表直接操作 List item:** `UpdateServers()` 会 `Clear()` 整个 List 再重建，必须先 refresh 再定位，不能对旧 index 做 SetCurrentItem。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 状态栏临时消息 | 自定义定时器 + Text() | `showStatusTemp()` | 项目已有实现，2秒后自动恢复默认状态栏文本 |
| 唯一 alias 生成 | 自行检查冲突 | `generateUniqueAlias()` | 项目已有实现，支持 -copy, -copy-2, -copy-3 格式 |
| 列表刷新 | 手动操作 tview.List | `refreshServerList()` | 项目已有实现，包含搜索过滤和排序 |
| 列表定位 | 遍历 servers slice 找 index | `serverList.SetCurrentItem(i)` | tview.List 内置方法，自动滚动到可见区域 |

## Common Pitfalls

### Pitfall 1: 刷新后定位使用旧 index
**What goes wrong:** 在 `AddServer()` 之后不调用 `refreshServerList()` 就尝试用旧 index 做 `SetCurrentItem`，导致选中错误的服务。
**Why it happens:** `UpdateServers()` 会 `Clear()` 整个 tview.List 并重建，旧 index 失效。
**How to avoid:** 必须先 `refreshServerList()`，再用 `ListServers()` 获取最新列表查找新条目的 index。
**Warning signs:** 复制后选中了错误的行、或 index out of range panic。

### Pitfall 2: 错误处理不一致
**What goes wrong:** `AddServer()` 失败时没有任何反馈，用户以为复制成功了。
**Why it happens:** 忘记处理 error 返回值。
**How to avoid:** 使用 `showStatusTempColor()` 显示红色错误消息，与项目其他操作（ping、copy ssh）的错误处理模式一致。

### Pitfall 3: 搜索过滤状态干扰
**What goes wrong:** 用户在搜索过滤状态下复制服务器，新条目不在过滤结果中导致找不到。
**Why it happens:** `refreshServerList()` 使用当前搜索词过滤，新复制的服务器 alias 可能不匹配当前搜索词。
**How to avoid:** 定位时使用 `ListServers("")`（无过滤）获取全量列表查找 index，然后在 serverList（已过滤的 List）中用 `SetCurrentItem` 定位。但需要注意：如果新条目被搜索过滤掉了，`SetCurrentItem` 的 index 对应的是过滤后的列表，而非全量列表。
**关键发现:** `refreshServerList()` 调用 `t.serverService.ListServers(query)` 获取过滤结果，然后 `sortServersForUI` + `UpdateServers`。如果当前有搜索词，新复制的服务器可能不在 `sl.servers` 中。需要在定位逻辑中处理这种情况：如果新 alias 不在当前过滤列表中，应该清除搜索或显示提示。

### Pitfall 4: SetCurrentItem 超出范围
**What goes wrong:** `SetCurrentItem(i)` 传入的 index 超出 List 的 item 数量。
**Why it happens:** `serverList.servers` 是过滤+排序后的列表，`ListServers("")` 返回的是全量列表，两者的 index 不对应。
**How to avoid:** 定位时应该使用 `t.serverList` 内部的 `servers` slice（过滤后），而非重新查询。可以在 `refreshServerList()` 之后直接遍历 `t.serverList` 的数据。但 `servers` 是私有字段。
**推荐方案:** 在 `refreshServerList()` 中使用过滤后的 query 查找，或者给 `ServerList` 添加一个 `FindIndexByAlias(alias string) int` 方法。或者更简单——在 `handleServerDup` 中，复制后先清除搜索（如果有的话），然后刷新+定位。

## Code Examples

### 现有 showStatusTemp 用法（handlers.go 中多处使用）

```go
// 来源: handlers.go:150 - 排序切换后提示
t.showStatusTemp("Sort: " + t.sortMode.String())

// 来源: handlers.go:166 - 复制 SSH 命令成功
t.showStatusTemp("Copied: " + cmd)

// 来源: handlers.go:407 - ping 失败（红色）
t.showStatusTempColor(fmt.Sprintf("Ping %s: DOWN (%v)", alias, err), "#FF6B6B")
```

### 现有 refreshServerList + SetCurrentItem 模式（handleServerSave 中）

```go
// 来源: handlers.go:371-383
t.refreshServerList()

if original == nil && t.dupPendingAlias != "" {
    servers, _ := t.serverService.ListServers("")
    for i, s := range servers {
        if s.Alias == t.dupPendingAlias {
            t.serverList.SetCurrentItem(i)
            break
        }
    }
    t.dupPendingAlias = ""
}
```

### 推荐的错误处理模式（与 ping 操作一致）

```go
// 来源: handlers.go:402-413
t.showStatusTemp(fmt.Sprintf("Pinging %s…", alias))
go func() {
    up, dur, err := t.serverService.Ping(server)
    t.app.QueueUpdateDraw(func() {
        if err != nil {
            t.showStatusTempColor(fmt.Sprintf("Ping %s: DOWN (%v)", alias, err), "#FF6B6B")
            return
        }
        // ...
    })
}()
```

注意：dup 操作不需要 goroutine，因为 `AddServer()` 是同步文件操作且很快完成。

## State of the Art

本阶段不涉及框架版本或技术选型变更。以下是代码库内部的状态分析：

| 旧流程 | 新流程 | 影响 |
|--------|--------|------|
| handleServerDup -> ServerForm -> handleServerSave -> AddServer | handleServerDup -> AddServer (直接) | 移除中间表单步骤，减少用户操作 |
| dupPendingAlias 跨 handler 传递状态 | 无需跨 handler 状态 | 简化 tui struct，减少状态管理复杂度 |
| 表单保存后刷新+定位 | dup 后立即刷新+定位 | 行为更直观，与 handleServerDelete 的模式一致（操作后留在列表） |

**可清理的死代码：**
- `tui.dupPendingAlias` 字段（tui.go:53）
- `handleServerSave()` 中 `dupPendingAlias` 的引用（handlers.go:354-355, 374-382）

## Open Questions

1. **搜索过滤状态下的行为**
   - What we know: `refreshServerList()` 会使用当前搜索词过滤列表
   - What's unclear: 如果用户在搜索过滤状态下按 D 复制，新条目可能被过滤掉，导致无法定位
   - Recommendation: 在 dup 成功后，如果当前有搜索词，检查新 alias 是否在过滤结果中。如果不在，清除搜索词再刷新。或者更简单的方案：dup 时总是清除搜索词，确保新条目可见。

2. **排序模式下的定位**
   - What we know: `refreshServerList()` 调用 `sortServersForUI()` 排序。默认排序是 `SortByAliasAsc`。新条目的 alias 是 `original-copy`，按字母排序会出现在原始条目附近。
   - What's unclear: 无 — 定位逻辑通过遍历查找 alias，不受排序影响。

## Environment Availability

Step 2.6: SKIPPED (no external dependencies — this phase is a pure code change in existing Go files)

## Validation Architecture

> SKIPPED — `workflow.nyquist_validation` is explicitly `false` in `.planning/config.json`

## Sources

### Primary (HIGH confidence)
- 项目源码 `internal/adapters/ui/handlers.go` — handleServerDup, handleServerSave, refreshServerList, showStatusTemp 实现
- 项目源码 `internal/adapters/ui/tui.go` — tui struct 定义, dupPendingAlias 字段
- 项目源码 `internal/adapters/ui/server_list.go` — ServerList.UpdateServers, SetCurrentItem 行为
- 项目源码 `internal/core/services/server_service.go` — AddServer() 方法签名和行为
- `.planning/REQUIREMENTS.md` — DUP-FIX-01, DUP-FIX-02 需求定义
- `.planning/STATE.md` — 已锁定决策

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — 无新依赖，仅使用现有项目代码
- Architecture: HIGH — 直接分析源码，完全理解现有流程和目标流程
- Pitfalls: HIGH — 基于源码分析识别出搜索过滤状态干扰的关键风险

**Research date:** 2026-04-15
**Valid until:** 90 days (纯代码变更，不依赖外部技术)
