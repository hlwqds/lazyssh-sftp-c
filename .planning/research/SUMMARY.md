# Project Research Summary

**Project:** lazyssh v1.1 -- Recent Remote Directories
**Domain:** TUI File Transfer Overlay (Go/tview)
**Researched:** 2026-04-14
**Confidence:** HIGH

## Executive Summary

本里程碑是在已有双栏文件传输功能（v1.0）基础上，为远程面板添加一个 "最近目录" 弹出列表。用户按 `r` 键即可在内存 MRU 列表中选择最近访问的远程目录快速跳转。这是一个纯 UI 层增强，零外部依赖（仅用已有 tview/tcell + Go 标准库），零跨层变更（不涉及 domain/ports/services），且遵循 PROJECT.md "不引入新安全风险" 的原则——仅内存存储，退出清空。

研究结论非常明确：实现路径清晰且风险可控。架构上直接复用 TransferModal 的 overlay 模式（`*tview.Box` + 手动 `Draw()` + `HandleKey()` + `visible` 标志），新建一个 `RecentDirs` 结构体，通过 `onPathChange` 回调自动记录导航路径，通过 `onShowRecentDirs` 回调从 RemotePane 触发弹窗。主要风险集中在 tview 按键路由（焦点管理、按键泄漏、Esc 穿透）和绘制层叠（overlay 渲染顺序、kitty 透明背景残影），但均有明确的前置解决方案。研究还发现了一个现有代码的 bug：`NavigateToParent()` 未触发 `onPathChange` 回调（导致 `app.Sync()` 在返回上级时缺失），修复此不对称性是本功能的必要前提。

## Key Findings

### Recommended Stack

**零新增外部依赖。** 全部能力由现有技术栈提供。

**Core technologies:**
- **tview `*tview.Box`** -- overlay 容器，手动 Draw 实现精确控制绘制位置和层叠顺序
- **Go `[]string` slice** -- MRU 路径存储，O(n) 去重+移动到头部，10 条上限下性能完全足够
- **tview 按键路由链** -- `Application.InputCapture` -> root `SetInputCapture` -> focused `SetInputCapture` -> `InputHandler`，弹窗需在此链中正确插入

### Expected Features

**Must have (table stakes):**
- 自动记录远程目录导航（NavigateInto/NavigateToParent 时静默追加）-- 所有主流文件管理器的标准行为
- `r` 键弹出最近目录列表（最多 10 条 MRU 排序）-- 功能的唯一入口
- j/k + Enter + Esc 交互 -- 与文件面板一致的导航键
- 重复路径自动去重（同路径只保留最新位置）
- 空列表时显示状态栏提示（不弹出空白框）

**Should have (competitive):**
- 按服务器自动隔离（FileBrowser 实例生命周期天然满足，零额外代码）
- 仅内存保存，退出清空（避免安全风险和缓存失效）
- 高亮当前目录（列表中与当前路径相同的条目用不同颜色标记）

**Defer (v2+):**
- 持久化书签/收藏夹 -- 需持久化，违反安全约束，远超 v1.1 范围
- 频率加权排序 -- 10 条上限下无实际意义
- 目录预览 -- 需 SFTP 预取，网络开销大
- 跨服务器目录列表 -- 需全局状态，破坏实例隔离

### Architecture Approach

本功能遵循现有 Clean Architecture 模式，仅在 `internal/adapters/ui/file_browser/` 包内新增和修改文件，不涉及 domain/ports/services 层。

**Major components:**
1. **RecentDirs（新结构体）** -- `recent_dirs.go`，MRU 路径环形缓冲区 + overlay 绘制 + 键盘处理，嵌入 `*tview.Box`，实现 `Draw()`/`HandleKey()`/`Show()`/`Hide()`/`Record()` 方法
2. **RemotePane（修改）** -- `remote_pane.go`，添加 `onShowRecentDirs` 回调和 `case 'r'` 按键绑定，新增 `NavigateTo(path)` 方法（不触发 onPathChange），修复 `NavigateToParent()` 的 onPathChange 不对称性
3. **FileBrowser（修改）** -- `file_browser.go`，持有 `recentDirs` 字段，在 `build()` 中创建并绑定回调，在 `handleGlobalKeys` 中添加 overlay 可见性检查，在 `Draw()` 中添加 overlay 渲染

**Key pattern:** 复用 TransferModal overlay 模式。TransferModal 是代码库中唯一的 overlay 先例，RecentDirs 应严格遵循其架构：自定义 primitive + visible 标志 + handleGlobalKeys 委托 + FileBrowser.Draw() 中手动绘制。

### Critical Pitfalls

1. **`r` 键与 TransferModal 冲突对话框冲突** -- TransferModal 的 `modeConflictDialog` 中 `r` 已绑定为 Rename。在 `handleGlobalKeys` 中添加 `case 'r'` 时，必须先检查所有 overlay 的可见性状态，modal 可见时不处理 `r`。

2. **j/k 键泄漏到背景 Table** -- 弹窗显示时若未正确消费按键，j/k 会传播到 RemotePane Table 导致同步滚动。弹窗的 HandleKey 必须消费所有按键（返回 nil），且 handleGlobalKeys 中弹窗检查必须在 pane 按键处理之前。

3. **焦点未恢复** -- tview 无内置焦点栈，弹窗关闭后焦点可能丢失。需在 Show() 前记录 `previousFocus`，在所有关闭路径（Enter/Esc/空列表）中统一恢复。Enter 选择后应将焦点保持在 RemotePane。

4. **Overlay 绘制被遮挡或关闭后残留** -- 弹窗不在 Flex 布局树中，需在 `FileBrowser.Draw()` 中手动调用 `recentDirs.Draw(screen)`。关闭时需强制重绘清除残留。kitty 透明背景下需用 `tcell.ColorDefault` 填充。

5. **OnPathChange 数据污染** -- 弹窗内通过 Enter 选择目录跳转时，不应触发路径记录（路径已在列表中）。使用单独的 `NavigateTo()` 方法（不触发回调），将记录逻辑与导航逻辑解耦。

## Implications for Roadmap

Based on research, suggested phase structure:

### Phase 1: RecentDirs Core Struct
**Rationale:** 零依赖，可独立开发和测试。包含所有核心数据结构和渲染逻辑。
**Delivers:** `recent_dirs.go` -- RecentDirs struct、Record() 环形缓冲区、Draw() 渲染、HandleKey() 键盘处理、Show()/Hide()/IsVisible()
**Addresses:** 自动记录目录导航、MRU 排序、去重、j/k/Enter/Esc 交互
**Avoids:** P8（空列表/单条目边界 -- 在 HandleKey 中添加边界检查）、P9（路径去重排序 -- Record() 中实现 move-to-front）

### Phase 2: RemotePane Integration
**Rationale:** 依赖 Phase 1 的 RecentDirs 类型。将键盘入口和导航方法接入现有面板。
**Delivers:** `remote_pane.go` 修改 -- `onShowRecentDirs` 回调、`case 'r'` 按键、`NavigateTo()` 方法、`NavigateToParent()` onPathChange 修复
**Uses:** RecentDirs struct (Phase 1)
**Implements:** `r` 键弹窗触发、选择后目录跳转、路径记录（通过 onPathChange）
**Avoids:** P5（数据污染 -- NavigateTo 不触发回调）、P6（未连接状态 -- 在 `case 'r'` 中添加 IsConnected 检查）、P10（Esc 双重含义 -- 按键守卫条件）

### Phase 3: FileBrowser Wiring & Polish
**Rationale:** 依赖 Phase 1 + Phase 2。将所有组件在 FileBrowser 层面组装，处理 overlay 层叠和全局按键路由。
**Delivers:** `file_browser.go` 修改 -- recentDirs 字段、build() 绑定、handleGlobalKeys overlay 检查、Draw() overlay 渲染、焦点恢复
**Avoids:** P1（`r` 键冲突 -- overlay 可见性守卫）、P2（j/k 泄漏 -- handleGlobalKeys 优先检查弹窗）、P3（焦点恢复 -- previousFocus 记录/恢复）、P4（绘制残留 -- FileBrowser.Draw() 中手动绘制）、P11（resize 位置 -- Draw() 中动态计算位置）

### Phase Ordering Rationale

- Phase 1 是纯新建文件，无任何现有代码依赖，可以独立实现和单元测试 Record() 的去重逻辑
- Phase 2 修改 RemotePane，引入 Phase 1 类型，同时修复 NavigateToParent 的 onPathChange 不对称性（这个修复本身是一个 bug fix，应该在路径记录逻辑就绪后立即实施）
- Phase 3 是最终组装层，处理最复杂的交互问题（按键路由、焦点管理、overlay 绘制），放在最后可以基于已验证的组件进行集成
- 这个顺序确保每个 Phase 都有明确的输入依赖和可验证的输出

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 3:** overlay 绘制机制存在不确定性。ARCHITECTURE.md 发现 FileBrowser.Draw() 中当前没有调用 TransferModal.Draw()，这意味着要么 TransferModal 的渲染通过其他未发现的机制工作，要么这是一个现有 bug。Phase 3 实施时需要先验证 TransferModal 的实际渲染路径，再决定 RecentDirs 的绘制方式。

Phases with standard patterns (skip research-phase):
- **Phase 1:** 环形缓冲区是经典数据结构，MRU move-to-front 有成熟的实现模式
- **Phase 2:** 回调模式和按键绑定遵循已有代码惯例（onPathChange、onFileAction），无新概念

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | 零新增依赖，全部基于代码库已验证的 tview/tcell 能力 |
| Features | HIGH | 竞品分析充分（mc/ranger/nnn/lf/FileZilla），用户期望明确 |
| Architecture | HIGH | 基于直接代码分析，TransferModal overlay 模式已验证，集成点精确到行号 |
| Pitfalls | HIGH | 基于代码审查和 tview 源码分析，11 个 pitfall 均有具体代码位置引用 |

**Overall confidence:** HIGH

### Gaps to Address

- **TransferModal 实际渲染路径未确认:** ARCHITECTURE.md 深入分析发现 FileBrowser.Draw() 中未调用 TransferModal.Draw()，渲染机制存在不确定性。Phase 3 实施前必须验证 TransferModal 的实际渲染方式（可能通过 SetAfterDrawFunc、QueueUpdateDraw 间接触发、或存在未发现的 Draw 调用点）。这不阻塞 Phase 1 和 Phase 2，但 Phase 3 的绘制策略取决于此发现。

- **NavigateToParent onPathChange 不对称性影响范围:** 虽然研究判断为 bug，但修复后 `app.Sync()` 在返回上级时也会被调用，可能影响现有行为。需要确认是否曾因缺少此 Sync 导致可见问题（如终端标题未更新）。

## Sources

### Primary (HIGH confidence)
- 项目源码 `internal/adapters/ui/file_browser/` 全部 7 个文件 -- overlay 模式、按键路由链、回调机制
- PROJECT.md v1.1 里程碑定义 -- 功能约束和验收标准

### Secondary (MEDIUM confidence)
- [tview Concurrency Wiki](https://github.com/rivo/tview/wiki/Concurrency) -- QueueUpdateDraw 线程安全模型
- [tview Issue #715](https://github.com/rivo/tview/issues/715) -- 按键路由链权威说明
- [tview Primitives Wiki](https://github.com/rivo/tview/wiki/Primitives) -- 自定义 primitive 实现模式
- Midnight Commander、ranger、nnn、lf、FileZilla -- 竞品功能分析

### Tertiary (LOW confidence)
- [tview Issue #104](https://github.com/rivo/tview/issues/104) -- popup 组件需求讨论（确认 tview 无内建 popup）

---
*Research completed: 2026-04-14*
*Ready for roadmap: yes*
