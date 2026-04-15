# Project Research Summary

**Project:** LazySSH File Transfer -- v1.2 File Management Operations
**Domain:** TUI 双面板 SFTP 文件浏览器中的文件管理操作（删除/重命名/新建/复制/移动）
**Researched:** 2026-04-15
**Confidence:** HIGH

## Executive Summary

v1.2 为 LazySSH 的双栏文件浏览器添加五种文件管理操作：删除（d）、重命名（R）、新建目录（m）、复制（c+p）、移动（x+p）。研究结论非常明确：**零新外部依赖**。所有操作所需的原语已存在于 `pkg/sftp v1.13.10`（当前为 indirect 依赖）和 Go 标准库 `os` 包中。唯一需要做的是将 `pkg/sftp` 从 `go.mod` 的 indirect 改为 direct 依赖。

架构方面，v1.2 遵循已有的 Clean Architecture 层次扩展：(1) Port 层将 `Remove`/`Stat` 从 `SFTPService` 下沉到 `FileService`，使本地面板和远程面板共享统一操作接口；(2) 新增 `CopyService` 接口处理同面板内的文件复制（需要 context 取消、进度回调和冲突处理，与 FileService 的简单 CRUD 语义不同）；(3) UI 层新增两个 overlay 组件（`ConfirmDialog` 和 `InputDialog`），严格遵循 `TransferModal`/`RecentDirs` 已建立的 overlay 模式。复制/移动采用 mark-put 模型（ranger/vifm/lf 风格），剪贴板状态存储在 `FileBrowser` 中（跨面板可见性所需）。

关键风险集中在四个领域：(1) SFTP 协议没有原生 copy 操作，远程面板内复制必须 download+reupload，大文件会慢；(2) `SFTP Remove` 不能删除非空目录，需要使用 `RemoveAll`；(3) 所有递归操作（删除目录、复制目录）必须在 goroutine 中执行并显示进度，否则阻塞 UI；(4) 快捷键冲突 -- 新增的 d/R/m/c/x/p 键必须检查 overlay 可见性后再处理。这些风险都有明确的预防策略，且现有代码中已有正确的 goroutine + QueueUpdateDraw 模式可以复用。

## Key Findings

### Recommended Stack

**零新增外部依赖。** 所有文件管理操作的技术原语已存在于项目中或 Go 标准库中。

**核心技术（均已有）：**
- `pkg/sftp v1.13.10`: 远程文件操作原语 -- 提供 Remove、RemoveAll、Rename、Mkdir、Stat 等全部所需方法，仅需将 indirect 改为 direct 依赖
- `os` 标准库: 本地文件操作 -- Remove、RemoveAll、Rename、Mkdir、Stat，全部一行代理
- `tview.InputField`: 文本输入组件 -- 用于重命名和新建目录的输入框，已在 ServerForm 中使用
- `context.Context`: 异步操作取消 -- 用于递归删除和复制的 goroutine 控制

**新增接口：**
- `CopyService`: 同面板内文件复制接口 -- 独立于 FileService，因为需要 context、进度回调和冲突处理

### Expected Features

**Must have（table stakes）-- 缺少任何一个都会让文件浏览器感觉不完整：**
- 删除文件/目录（d 键）-- 单文件、递归目录、多选批量删除，带确认对话框
- 重命名（R 键）-- 内联编辑 InputField，预填当前文件名，选中文件名茎（不含扩展名）
- 新建目录（m 键）-- InputField 弹窗，支持嵌套路径创建
- 复制标记+粘贴（c+p）-- 同面板复制 + 跨面板传输（复用 TransferService）
- 移动标记+粘贴（x+p）-- 同面板移动 + 跨面板传输+删除源文件

**Should have（差异化）：**
- 统一的 c/x/p 模型 -- 将本地复制和跨面板传输统一在一个心智模型下，比 mc 的 F5/F6 更灵活
- 状态栏标记提示 -- 标记后显示 "3 file(s) marked for copy"
- 递归删除范围显示 -- 确认对话框显示文件数和总大小

**Defer（v2+）：**
- Undo 撤销 -- 需要操作日志和反向执行，SFTP 无事务支持，复杂度高
- 批量重命名（正则/编号）-- 需要模式语法，复杂度高
- 文件权限编辑 -- 跨平台差异大，远程 chmod 支持不确定
- Trash/回收站集成 -- `trash-cli` 非系统自带，远程 SFTP 无 trash 概念

### Architecture Approach

v1.2 通过三层扩展集成到现有 Clean Architecture 中。

**主要组件：**
1. **FileService 接口扩展** -- 将 Remove、RemoveAll、Rename、Mkdir、Stat 提升到共享接口，LocalFS 和 SFTPClient 都实现，UI 层无需类型判断
2. **CopyService 接口（新增）** -- 独立接口处理同面板文件复制，需要 context 取消、进度回调和冲突处理；两个实现：LocalCopyService 和 RemoteCopyService
3. **ConfirmDialog overlay（新增）** -- 可复用的确认对话框，用于删除确认和危险操作确认
4. **InputDialog overlay（新增）** -- 可复用的文本输入弹窗，嵌入 tview.InputField，用于重命名和新建目录
5. **Clipboard 状态（新增）** -- 纯状态结构体，存储在 FileBrowser 中，管理复制/移动的标记状态

**关键设计决策：**
- 剪贴板状态在 FileBrowser 而非面板中 -- 跨面板操作需要 FileBrowser 级别的可见性
- 同面板复制用 CopyService，跨面板复制复用 TransferService -- 最大化代码复用
- copyWithProgress 从 transfer_service.go 提取为共享工具函数 -- 避免三份相同代码
- 所有 overlay 遵循互斥原则 -- 同一时间只有一个 overlay 可见，避免按键路由歧义

### Critical Pitfalls

1. **SFTP 协议没有原生 copy** -- 远程面板内复制必须 download 到临时文件 + upload 到目标路径，大文件慢。预防：复用 TransferService 模式，在 UI 中提示用户远程复制需经过本地中转
2. **SFTP Remove 不能删除非空目录** -- 需要使用 `RemoveAll()` 递归删除。预防：在 SFTPService port 接口中暴露 RemoveAll 方法
3. **递归操作阻塞 UI** -- 大型目录删除/复制如果同步执行会冻结终端。预防：所有耗时操作在 goroutine 中执行，使用 QueueUpdateDraw 更新 UI
4. **快捷键冲突** -- 新键 d/R/m/c/x/p 必须在 overlay 可见时被拦截。预防：在 handleGlobalKeys 中添加守卫条件，检查所有 overlay 可见性
5. **剪贴板路径在导航后失效** -- 如果只存文件名，导航后路径会错误。预防：存储完整绝对路径，导航时不清除剪贴板

## Implications for Roadmap

基于四个研究维度的综合分析，建议将 v1.2 分为 3 个 phase，按依赖关系和技术风险递进排列。

### Phase 1: Port 接口扩展 + 删除/重命名/新建目录

**Rationale:** 这是所有后续功能的基础。Port 接口扩展后才能实现任何文件操作；删除功能建立确认对话框模式（ConfirmDialog），重命名和新建目录建立文本输入模式（InputDialog），这两个 overlay 组件是后续复制/移动功能的构建块。三者技术复杂度低，可以一起交付。

**Delivers:** 可用的删除、重命名、新建目录功能（本地+远程）

**Addresses:** FEATURES.md 中的 Delete、Rename、Mkdir 三个 table stakes 功能

**Avoids:** PITFALLS P2（Remove 不能删非空目录 -- 通过 RemoveAll 解决）、P3（TOCTOU -- 通过删除前 Stat 验证解决）、P10（快捷键冲突 -- 通过 overlay 互斥检查解决）、P12（目录名验证 -- 通过输入框验证解决）

**包含的工作：**
- FileService 接口添加 Remove、RemoveAll、Rename、Mkdir、Stat
- SFTPClient 添加 RemoveAll、Rename、Mkdir 方法
- LocalFS 添加 Remove、RemoveAll、Rename、Mkdir、Stat 方法
- ConfirmDialog overlay 组件
- InputDialog overlay 组件
- d/R/m 快捷键路由 + 处理逻辑
- pkg/sftp 从 indirect 改为 direct 依赖

### Phase 2: 复制功能（CopyService + 剪贴板 + 同面板/跨面板复制）

**Rationale:** 复制功能依赖 Phase 1 建立的 overlay 模式作为参考和 port 接口。复制是独立于删除/重命名的新功能维度，引入了剪贴板状态管理这个新的架构概念。同面板复制需要新的 CopyService 接口和两个实现；跨面板复制复用 TransferService。

**Delivers:** c 标记 + p 粘贴的完整复制功能（同面板 + 跨面板）

**Addresses:** FEATURES.md 中的 Copy 功能

**Avoids:** PITFALLS P1（SFTP 无 copy -- 通过 download+upload 解决）、P5（递归操作阻塞 UI -- 通过 goroutine+QueueUpdateDraw 解决）、P6（剪贴板路径失效 -- 通过完整路径存储解决）、P8（符号链接循环 -- 通过 IsSymlink 检查解决）

**包含的工作：**
- CopyService 接口定义
- copyWithProgress 提取为共享工具函数
- LocalCopyService 和 RemoteCopyService 实现
- Clipboard 状态结构体
- c/p 快捷键路由 + 处理逻辑
- 跨面板复制复用 TransferService 的进度显示

### Phase 3: 移动功能 + 集成完善

**Rationale:** 移动本质上 = 复制 + 删除源文件，强依赖 Phase 2 的复制实现和 Phase 1 的删除实现。移动引入了非原子性风险（copy 成功但 delete 失败），需要额外的错误恢复逻辑。放在最后是因为它组合了前面两个 phase 的所有基础设施。

**Delivers:** x 标记 + p 粘贴的完整移动功能（同面板 + 跨面板）

**Addresses:** FEATURES.md 中的 Move 功能

**Avoids:** PITFALLS P4（Rename 跨文件系统 -- 通过 PosixRename + fallback 解决）、P7（连接断开 -- 通过部分完成状态报告解决）、P11（移动非原子性 -- 通过 copy 成功后再 delete + 失败时提示用户解决）

**包含的工作：**
- 移动 = 复制 + 删除源文件的组合逻辑
- PosixRename 优先 + copy+delete fallback
- 移动失败时的错误恢复和用户提示
- DI 链更新（CopyService 注入到 FileBrowser）

### Phase Ordering Rationale

- **Phase 1 先行** -- Port 接口扩展是所有操作的编译时前提；ConfirmDialog/InputDialog 是 Phase 2 剪贴板 UI 模式的参考实现
- **Phase 2 在中间** -- 复制引入了全新的架构概念（剪贴板、CopyService），独立于删除/重命名，但被移动依赖
- **Phase 3 最后** -- 移动是组合操作，依赖前两个 phase 的所有基础设施，且涉及最多的错误恢复逻辑

这种排列确保每个 phase 都有明确的交付价值（用户可以在 Phase 1 后就使用删除/重命名/新建目录），且技术复杂度递进。

### Research Flags

需要研究的 phase：
- **Phase 2:** CopyService 的远程复制（download+reupload）性能优化和临时文件管理策略需要验证，特别是大文件场景下的磁盘空间和清理逻辑
- **Phase 3:** 移动操作的非原子性错误恢复流程需要仔细设计 -- copy 成功但 delete 源失败时的用户体验

有标准模式、无需额外研究的 phase：
- **Phase 1:** Port 接口扩展和 adapter 实现是机械性的薄封装；overlay 组件遵循已建立的 TransferModal/RecentDirs 模式；所有 SFTP 原语在 pkg/sftp 源码中已验证存在
- **Phase 2（部分）:** 同面板本地复制使用标准 io.Copy + filepath.WalkDir 模式；跨面板复制完全复用 TransferService

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | 所有技术原语在 pkg/sftp 源码和 Go 标准库中直接验证，零推断 |
| Features | HIGH | 基于 mc/ranger/vifm/lf 四个成熟竞品的功能矩阵交叉验证，加上 Nielsen Norman Group 的 UX 最佳实践 |
| Architecture | HIGH | 基于项目全部 file_browser 包源码的直接分析，所有接口变更和组件设计都有现有代码支撑 |
| Pitfalls | HIGH | 基于项目代码审查、pkg/sftp 库文档、SFTP 协议规范和 OWASP 安全指南的交叉验证 |

**Overall confidence:** HIGH

### Gaps to Address

- **远程复制性能:** download+reupload 对大文件/大目录的性能影响需要在实现后进行实际测试。临时文件的磁盘空间管理（特别是远程目录大于本地剩余空间时）需要在 Phase 2 规划时考虑
- **符号链接处理策略:** 当前研究建议递归操作时跳过符号链接，但未做用户调研确认这是否符合预期行为。v1.2 采用"跳过"策略是安全的保守选择，后续可根据用户反馈调整
- **InputDialog 的 tview.InputField 焦点管理:** 研究提出了通过 `InputHandler(event, func(tview.Primitive) {})` 绕过 tview 焦点系统的方案，但这一方案需要在实现时验证 Enter/Esc 是否正确触发 doneFunc

## Sources

### Primary (HIGH confidence)
- `internal/core/ports/file_service.go` -- FileService + SFTPService 接口定义
- `internal/adapters/data/sftp_client/sftp_client.go` -- SFTPClient 实现
- `internal/adapters/data/local_fs/local_fs.go` -- LocalFS 实现
- `internal/adapters/data/transfer/transfer_service.go` -- copyWithProgress 模式
- `internal/adapters/ui/file_browser/` -- 全部 UI 组件源码
- `pkg/sftp v1.13.10` client.go -- Remove, RemoveAll, Rename, PosixRename, Mkdir 源码验证
- Midnight Commander / ranger / vifm / lf -- 功能矩阵和键绑定交叉验证
- Nielsen Norman Group -- 确认对话框 UX 指南

### Secondary (MEDIUM confidence)
- SFTP 协议规范 (RFC 4254 + drafts) -- SFTP 无 copy 操作
- OpenSSH PROTOCOL file -- copy-data 扩展（非标准，pkg/sftp 不支持）
- UX StackExchange -- Confirm vs Undo 辩论
- OWASP Unicode Encoding / Path Traversal -- 编码安全和路径遍历防护

### Tertiary (LOW confidence)
- lf file manager documentation -- 终端文件管理器剪贴板模式参考（lf 使用 trash-cli，与 lazyssh 的永久删除策略不同）
- rclone forum -- SFTP 远程复制限制的社区讨论

---
*Research completed: 2026-04-15*
*Ready for roadmap: yes*
