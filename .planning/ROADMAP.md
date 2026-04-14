# Roadmap: LazySSH File Transfer

## Milestones

- **v1.0 File Transfer** - Phases 1-3 (shipped 2026-04-13)
- **v1.1 Recent Remote Directories** - Phases 4-5 (in progress)

## Phases

<details>
<summary>v1.0 File Transfer (Phases 1-3) - SHIPPED 2026-04-13</summary>

- [x] Phase 1: Foundation (3/3 plans) - completed 2026-04-13
- [x] Phase 2: Core Transfer (3/3 plans) - completed 2026-04-13
- [x] Phase 3: Polish (3/3 plans) - completed 2026-04-13

</details>

### v1.1 Recent Remote Directories (In Progress)

**Milestone Goal:** 在文件浏览器的远程面板中，记录并快速重新访问最近浏览过的远程目录。

- [ ] **Phase 4: Directory History Core** - 构建内存 MRU 目录列表数据结构，自动记录远程面板的每次目录导航，修复 NavigateToParent 回调缺失 bug
- [ ] **Phase 5: Recent Directories Popup** - 按 `r` 键弹出最近目录列表，支持 j/k 导航、Enter 跳转、Esc 关闭，高亮当前目录

## Phase Details

### Phase 4: Directory History Core
**Goal**: 每次远程面板的目录导航都被静默记录到内存 MRU 列表中，NavigateToParent 的 onPathChange 不对称性 bug 被修复
**Depends on**: Phase 3 (v1.0 shipped)
**Requirements**: HIST-01, HIST-02, HIST-03, HIST-04, AUX-02
**Success Criteria** (what must be TRUE):
  1. 用户在远程面板中进入子目录或返回上级目录后，该目录路径被记录到内部最近目录列表
  2. 用户多次导航到同一目录时，该路径在列表中仅出现一次，且位于最前
  3. 最近目录列表始终不超过 10 条，超出时最旧条目被移除
  4. 用户按 `h` 返回上级目录时，父目录路径被正确记录，且终端标题同步更新（AUX-02 修复验证）
**Plans**: 2 plans

Plans:
- [x] 04-01-PLAN.md — 创建 RecentDirs MRU 数据结构 + 单元测试
- [x] 04-02-PLAN.md — 修复 NavigateToParent bug + 添加 NavigateTo + 接入 Record 调用

**UI hint**: yes

### Phase 5: Recent Directories Popup
**Goal**: 用户按 `r` 键即可查看并快速跳转到最近访问过的远程目录
**Depends on**: Phase 4
**Requirements**: POPUP-01, POPUP-02, POPUP-03, POPUP-04, POPUP-05, AUX-01
**Success Criteria** (what must be TRUE):
  1. 用户在远程面板获得焦点时按 `r` 键，屏幕中央弹出一个显示最近目录路径的列表
  2. 用户可以在弹窗列表中用 `j`/`k`/上下方向键移动选中项，按 `Enter` 跳转到该目录，按 `Esc` 关闭弹窗
  3. 用户按 `Enter` 选择目录后，远程面板直接跳转到该路径并刷新文件列表，弹窗关闭
  4. 当还没有访问过任何目录时，按 `r` 显示"暂无最近目录"提示文本
  5. 弹窗列表中，与当前远程面板路径相同的条目用不同颜色高亮显示
**Plans**: 1 plan

Plans:
- [ ] 05-01-PLAN.md — 补全 RecentDirs Draw()/HandleKey() + 接入 FileBrowser 按键路由和 overlay 渲染链

**UI hint**: yes

## Progress

**Execution Order:**
Phases execute in numeric order: 4 -> 5

| Phase | Milestone | Plans | Status | Completed |
|-------|-----------|-------|--------|-----------|
| 1. Foundation | v1.0 | 3/3 | Complete | 2026-04-13 |
| 2. Core Transfer | v1.0 | 3/3 | Complete | 2026-04-13 |
| 3. Polish | v1.0 | 3/3 | Complete | 2026-04-13 |
| 4. Directory History Core | v1.1 | 0/2 | Not started | - |
| 5. Recent Directories Popup | v1.1 | 0/1 | Not started | - |

Full details (v1.0): .planning/milestones/v1.0-ROADMAP.md
