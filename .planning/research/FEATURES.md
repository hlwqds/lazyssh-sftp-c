# Feature Research

**Analysis Date:** 2026-04-14
**Domain:** TUI File Transfer — Recent Remote Directories (v1.1)
**Confidence:** HIGH

## Research Sources

- Midnight Commander (mc) — Alt+u directory history, Ctrl+\ Hotlist (canonical dual-pane TUI)
- ranger — H/L directory history stack (session-based MRU)
- nnn — B key symlink bookmarks (persistent), `-` previous directory
- lf — marks/bookmarks system, special `'` mark for last directory
- FileZilla — QuickConnect history (10-entry MRU, recentservers.xml)
- Windows Explorer — Quick Access (separates "Recent" vs "Frequent")
- lazyssh existing codebase — RemotePane navigation, TransferModal overlay pattern

---

## Part 1: Recent Remote Directories (v1.1 New Feature)

### Table Stakes (Users Expect These)

Features users assume exist in a "recent directories" feature. Missing = feels broken.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| 自动记录访问的目录 | 所有主流文件管理器（MC、ranger、nnn、Windows Explorer）都在用户进入新目录时自动记录。用户不会手动"添加到收藏"，而是期望系统静默记录。 | LOW | 在 `RemotePane.NavigateInto()` 和 `NavigateToParent()` 的 `onPathChange` 回调中追加路径。只需在现有回调链上加一行调用。 |
| MRU 排序（最近访问排最前） | 用户按 `r` 打开列表时期望看到的第一个就是刚才离开的目录。这是 MRU（Most Recently Used）的标准行为，所有 IDE 和文件管理器都如此。 | LOW | `[]string` 切片即可。新路径去重后 prepend 到头部。10 条上限意味着 O(n) 去重完全足够。 |
| 重复路径折叠（去重） | 同一目录访问 5 次不应出现 5 条。用户期望每个路径只出现一次，但每次访问都更新其位置到最前面。 | LOW | 每次插入前从 slice 中移除已有相同路径（如果存在），再 prepend。Go slice 操作即可。 |
| 选择后导航到该目录 | 用户从列表中选中一条路径并按 Enter，远程面板应直接跳转到该目录。这是"最近目录"功能的核心价值——快速跳转。 | LOW | 调用已有的 `RemotePane.NavigateInto()` 或设置 `currentPath` 后 `Refresh()`。基础设施已存在。 |
| Esc 关闭弹窗 | 所有弹窗式 UI 组件的标准行为。 | LOW | 与现有 TransferModal 的 Esc 模式一致。 |
| 列表为空时显示提示 | 用户第一次使用时按 `r` 应看到"暂无记录"而不是空白弹窗。 | LOW | 简单的 len check。 |

### Differentiators (Competitive Advantage)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| 按服务器自动隔离 | FileBrowser 实例的生命周期绑定到单个服务器连接。最近目录列表存在 FileBrowser 实例中，天然按服务器隔离。不需要额外实现分组逻辑。 | LOW (架构天然支持) | PROJECT.md 提到"记录粒度为本机目录 + 服务器组合"。当前架构自动满足。 |
| 仅内存保存，退出清空 | 避免持久化带来的隐私顾虑（记录了用户访问过哪些服务器目录），避免缓存失效（远程目录可能被删除/重命名）。符合"不引入新安全风险"约束。 | LOW | 零存储代码。FileBrowser 被 GC 回收时列表自动消失。 |
| j/k 键导航弹窗列表 | 与远程面板的 j/k 导航保持一致的交互模型。用户不需要学习新按键。 | LOW | 复用 tview.Table 内置 j/k 支持或手动 `selectedIdx` 管理。 |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| 持久化到磁盘（跨会话保存） | 用户可能想重启后还能看到之前的远程目录 | 1. 安全风险：记录了用户访问过哪些服务器的哪些目录<br>2. 缓存失效：远程目录可能已被删除/重命名<br>3. 需要文件存储，违反"零外部依赖"约束 | 当前会话内有效。如未来需要可做 opt-in 配置 |
| 频率加权排序（Frequent） | Windows Quick Access 有 "Frequent folders" | 1. 实现复杂度高：需计数器 + 时间衰减<br>2. 用户心智不匹配：按 `r` 是"最近"不是"最常"<br>3. 10 条上限下频率排序意义不大 | 纯 MRU 排序，简单可预测 |
| 书签/收藏夹功能 | MC 的 Hotlist 是手动收藏的目录 | 1. 需手动管理（增删改），增加 UI 复杂度<br>2. 需持久化才有价值，引入安全风险<br>3. 超出 v1.1 范围 | v1.1 仅做自动记录。书签可作为 v2 特性 |
| 跨服务器目录列表 | 在服务器 A 的列表中显示服务器 B 的最近目录 | 1. 需全局状态，破坏实例隔离<br>2. 服务器 A 的 SFTP 无法访问服务器 B 的路径<br>3. 用户混淆风险 | 每个服务器连接独立维护 |
| 目录预览 | 列表中预览每个目录里有什么 | 1. 需 SFTP 预取每个 listing，网络开销大<br>2. 弹窗变宽，小终端体验差<br>3. 实现复杂度高 | 仅显示路径字符串 |

### Feature Dependencies

```
[Recent Directories Popup (v1.1)]
    └──requires──> [RemotePane.onPathChange callback]
                       └──exists──> v1.0 (used for terminal Sync)

[Recent Directories Popup (v1.1)]
    └──requires──> [RemotePane.NavigateInto / currentPath]
                       └──exists──> v1.0

[Popup List UI Component]
    └──follows──> [TransferModal overlay pattern]
                     └──exists──> v1.0 (Box embed + Draw + HandleKey + visible flag)

[Persistent Bookmarks (v2)]
    └──requires──> [Recent Directories (v1.1)]
    └──requires──> [Disk storage]
    └──conflicts──> [Security constraint: no sensitive data storage]
```

### Dependency Notes

- **Recent Directories requires RemotePane.onPathChange:** 回调已在 v1.0 实现（用于 Sync 终端防残影）。只需在回调中追加路径到历史列表，零新基础设施。
- **Popup List follows TransferModal pattern:** v1.0 的 TransferModal 展示了完整弹窗 overlay 模式：`*tview.Box` 嵌入 + 手动 `Draw()` + `HandleKey()` + `visible` 标志。新弹窗应遵循相同模式。
- **Persistent Bookmarks conflicts with security constraint:** 书签需持久化目录路径（含服务器信息），违反"不引入新安全风险"原则。如要实现需 opt-in + 加密存储，远超 v1.1 范围。

### MVP Definition (v1.1)

#### Launch With

- [ ] **自动记录远程目录导航** — NavigateInto/NavigateToParent 时自动追加路径
- [ ] **`r` 键弹出最近目录列表** — 远程面板焦点时按 `r`，居中弹窗，最多 10 条 MRU
- [ ] **j/k + Enter + Esc 交互** — 与文件面板一致的导航键
- [ ] **重复路径自动去重** — 同一路径只保留最新位置
- [ ] **空列表占位提示** — "暂无最近目录"

#### Add After Validation (v1.x)

- [ ] **高亮当前目录** — 列表中与当前路径相同的条目用不同颜色标记
- [ ] **路径缩写显示** — 过长路径缩写中间部分，保持弹窗宽度可控
- [ ] **数字键快速选择** — 按 1-9 直接跳转到对应序号目录

#### Future Consideration (v2+)

- [ ] **持久化书签** — 手动收藏目录，跨会话保存
- [ ] **按本地目录分组** — 如果未来支持多标签页，需设计分组键
- [ ] **目录访问计数** — MRU 基础上显示访问次数

### Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| 自动记录目录导航 | HIGH | LOW | P1 |
| `r` 键弹出列表 | HIGH | LOW | P1 |
| j/k + Enter + Esc 交互 | HIGH | LOW | P1 |
| 重复路径去重 | MEDIUM | LOW | P1 |
| 空列表提示 | MEDIUM | LOW | P1 |
| 高亮当前目录 | MEDIUM | LOW | P2 |
| 路径缩写显示 | LOW | LOW | P2 |
| 数字键快速选择 | LOW | MEDIUM | P3 |
| 持久化书签 | MEDIUM | HIGH | P3 |
| 频率加权排序 | LOW | HIGH | P3 |

### Competitor Feature Analysis (Recent Directories)

| Feature | Midnight Commander | Ranger | nnn | FileZilla | lazyssh (planned) |
|---------|-------------------|--------|-----|-----------|-------------------|
| 目录历史 | Alt+u 前后导航, Alt+Shift+h 下拉 | H/L 前后导航（栈式） | `-` 返回上一目录 | 无（仅 QuickConnect 服务器历史） | `r` 弹出 MRU 列表 |
| 书签/收藏 | Hotlist (Ctrl+\), 持久化 | 书签系统, 可持久化 | B 键书签, symlink 持久化 | Site Manager, 持久化 XML | v1.1 不做 |
| 持久化 | Hotlist 持久化; 历史仅会话内 | 历史仅会话内 | 书签 symlink 持久化 | 服务器列表持久化 | 仅会话内（内存） |
| 上限 | 无明确上限 | 无明确上限 | 书签单字符 (0-9, a-z) | QuickConnect 最多 10 条 | 最多 10 条 |
| 去重 | 历史栈自动去重 | 历史栈自动去重 | 书记天然去重 | QuickConnect 自动去重 | 自动去重 |

**关键洞察：** 主流终端文件管理器都将"目录历史"和"书签"分为两个独立功能。目录历史是自动的、会话级的、MRU 排序的；书签是手动的、持久化的、用户自定义排序的。lazyssh v1.1 只做前者。

**FileZilla 的教训：** QuickConnect 限制 10 条，超出后永久删除旧条目——用户多有抱怨。lazyssh 同样 10 条上限，但因内存存储，不存在数据丢失问题。

### How "Recent Directories" Works in Practice

Based on research across multiple file managers, the expected behavior is:

1. **Auto-record on navigation:** 每次用户通过 Enter 进入子目录或 h/Backspace 返回上级时，新路径自动追加到列表头部。不需要用户手动操作。
2. **MRU ordering (most recent first):** 列表严格按访问时间倒序排列。最近访问的在顶部。这是所有文件管理器的标准行为。
3. **Duplicate collapsing:** 同一路径多次访问只保留一条记录。每次访问将该路径移到列表头部，而非插入新条目。
4. **Selection navigates into directory:** 用户按 Enter 选中一条路径后，远程面板直接跳转到该目录并刷新文件列表。这与用户在面板中手动导航的效果完全一致。
5. **Per-server grouping (natural isolation):** 每个服务器连接（FileBrowser 实例）维护独立的最近目录列表。切换服务器时列表自然隔离。这不需要额外代码——列表存在 FileBrowser 实例的内存中。

---

## Part 2: File Transfer (v1.0 — Already Built)

### Table Stakes (Must Have)

Users expect these in any file transfer tool. Without them, the feature feels incomplete.

#### Navigation & Browsing

| Feature | Complexity | Dependency |
|---------|-----------|------------|
| Local directory browsing | LOW | None |
| Remote directory browsing (SFTP) | MEDIUM | SFTP connection |
| Parent directory navigation (../) | LOW | Both browsers |
| Hidden file toggle | LOW | Both browsers |
| Current path display | LOW | Both browsers |
| Sort by name/size/date | LOW | Both browsers |

#### Transfer Operations

| Feature | Complexity | Dependency |
|---------|-----------|------------|
| Single file upload | LOW | SFTP connection |
| Single file download | LOW | SFTP connection |
| Directory upload (recursive) | MEDIUM | SFTP walk |
| Directory download (recursive) | MEDIUM | SFTP walk |
| Transfer progress indication | MEDIUM | Progress tracking |
| Transfer cancel | MEDIUM | Process management |

#### UX Essentials

| Feature | Complexity | Dependency |
|---------|-----------|------------|
| Keyboard navigation (arrows/j/k) | LOW | tview |
| Quick transfer shortcut (Enter or specific key) | LOW | UI |
| File selection (space to mark) | LOW | UI |
| Status bar with connection info | LOW | UI |
| Error display | LOW | UI |

### Differentiators

| Feature | Complexity | Why It Matters |
|---------|-----------|----------------|
| Zero-config remote access | LOW | Server list already has SSH config |
| Seamless server switching | LOW | Switch servers without re-entering credentials |
| Integrated with SSH management | MEDIUM | One tool for connection + transfer + config |
| Quick-open from server list | LOW | Press `F` on any server for instant file browser |

### Anti-Features (Deliberately NOT Built)

| Feature | Reason |
|---------|--------|
| File preview (F3 in mc) | Adds significant complexity; lazyssh scope is transfer, not viewing |
| Drag-and-drop emulation | TUI tools are keyboard-driven |
| Archive VFS (browse zip/tar) | Orthogonal to SSH file transfer |
| File editing | Out of scope per PROJECT.md |
| Shell link / fish protocol | mc-specific VFS, adds protocol handling complexity |
| Resume/partial transfer | scp/sftp don't natively support |
| Multi-threaded transfer | v1 single-threaded for simplicity |
| Transfer queue | Over-engineering for v1 |
| Bookmark management | Server list already serves this purpose |

### Key UX Patterns from Research

#### Midnight Commander Key Bindings (Reference)
- F5 = Copy (transfer)
- F6 = Move
- F7 = Mkdir
- F8 = Delete
- F3 = View
- Tab = Switch panels
- Insert = Select file

#### FileZilla UX Patterns
- Drag between panes = copy (not move)
- Double-click = transfer
- Non-blocking background transfers
- Non-blocking confirmation dialogs

#### lazyssh Adaptation
- `Enter` on file = transfer to other pane
- `Tab` = switch pane focus
- `Space` = select/deselect file
- `F5` = transfer current directory
- `Backspace` or `h` = go to parent directory
- `s`/`S` = sort field/direction
- `r` = recent directories popup (v1.1)

## Sources

- [Midnight Commander man page](https://source.midnight-commander.org/man/mc.html)
- [MC directory hotlist — Unix.SE](https://unix.stackexchange.com/questions/14483/does-mc-midnight-commander-have-favourites-for-directories)
- [ranger GitHub](https://github.com/ranger/ranger)
- [nnn ArchWiki](https://wiki.archlinux.org/title/Nnn)
- [lf documentation](https://github.com/gokcehan/lf/blob/master/doc.md)
- [FileZilla Forum — QuickConnect history](https://forum.filezilla-project.org/viewtopic.php?t=26927)
- [Windows Quick Access — SuperUser](https://superuser.com/questions/1669420/how-does-the-windows-file-explorer-quick-access-recent-items-feature-work)
- [UX StackExchange — Recent file list placement](https://ux.stackexchange.com/questions/135146/in-a-desktop-application-should-the-recent-file-list-placed-directly-in-the-fil)
- 现有代码库: `internal/adapters/ui/file_browser/remote_pane.go`
- 现有代码库: `internal/adapters/ui/file_browser/transfer_modal.go`
- 现有代码库: `internal/adapters/ui/file_browser/file_browser_handlers.go`
- `.planning/PROJECT.md` — v1.1 milestone 定义和约束

---
*Features research: 2026-04-14 — v1.1 Recent Remote Directories focus*
