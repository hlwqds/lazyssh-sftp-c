# Phase 9: Dup SSH Connection - Context

**Gathered:** 2026-04-15
**Status:** Ready for planning

<domain>
## Phase Boundary

在服务器列表中按 D 键快速复制当前选中服务器的配置，自动生成唯一别名后打开编辑表单，用户可修改字段后保存为新条目。不修改任何现有接口或架构。

</domain>

<decisions>
## Implementation Decisions

### 快捷键
- **D-01:** 使用 `D` (Shift+d) 触发复制，`d` 已被 delete 占用
- **D-02:** 在 `handleGlobalKeys` switch 中新增 `case 'D'`，与现有 `case 'd'` 并列

### 别名生成
- **D-03:** 后缀格式 `-copy`，冲突时递增：`原名-copy`、`原名-copy-2`、`原名-copy-3`
- **D-04:** 生成别名时需检查现有服务器列表确保唯一性

### 复制范围
- **D-05:** 复制全部 SSH 配置字段（Host/User/Port/IdentityFiles/ProxyJump 等所有字段）
- **D-06:** 清除运行时元数据：`PinnedAt`（置零值）、`SSHCount`（归零）、`LastSeen`（置零值）

### 表单与保存
- **D-07:** 复制后以 Add 模式打开 ServerForm（`NewServerForm(ServerFormAdd, &dupServer)`），用户可修改任意字段
- **D-08:** 保存通过现有 `handleServerSave` → `AddServer` 路径，无需新增保存逻辑

### 保存后行为
- **D-09:** 保存成功后光标自动定位到新创建的条目（滚动到可见位置）

### Claude's Discretion
- 别名唯一性检查的具体实现（遍历列表 vs service 层方法）
- handleServerDup 函数内部的代码组织

</decisions>

<specifics>
## Specific Ideas

- 类似 macOS Finder 的文件复制行为：选中 → 复制 → 自动打开编辑 → 保存
- 别名后缀风格参考 macOS：`-copy` 而非 `(2)`

</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Server entity and service
- `internal/core/domain/server.go` — Server struct，含 PinnedAt/SSHCount/LastSeen 等需清除的元数据字段
- `internal/core/services/server_service.go` — AddServer 方法，validateServer 校验逻辑
- `internal/core/ports/services.go` — ServerService 接口定义

### UI handlers and form
- `internal/adapters/ui/handlers.go` — handleGlobalKeys switch（D 键插入位置）、handleServerAdd/handleServerEdit/handleServerSave 模式
- `internal/adapters/ui/server_form.go` — NewServerForm 构造函数、ServerFormAdd/Edit 模式

### Conventions
- `.planning/codebase/CONVENTIONS.md` — 命名、代码风格、错误处理模式
- `.planning/codebase/STRUCTURE.md` — 项目结构和文件组织

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `handleServerAdd()` — Add 模式打开 ServerForm 的完整模式，Dup 直接复用此模式
- `handleServerSave()` — 统一 Add/Edit 保存，`original == nil` 时调用 AddServer
- `NewServerForm(ServerFormAdd, &server)` — 传入 server 预填充表单字段
- `t.serverList.GetSelectedServer()` — 获取当前选中服务器
- `t.serverService.AddServer()` — 添加新服务器到仓库

### Established Patterns
- handler 函数模式：`handleServerXxx()` 私有方法，获取选中服务器 → 操作 → 刷新列表
- ServerForm 模式：构造函数 + SetApp + OnSave + OnCancel 链式调用 → SetRoot
- Go struct 值拷贝：`domain.Server` 是值类型，直接赋值即为深拷贝（注意 slice 字段需手动拷贝）

### Integration Points
- `handleGlobalKeys` switch（handlers.go:47）— 新增 `case 'D'` 调用 handleServerDup
- 新增 `handleServerDup()` 方法（handlers.go）— 复制逻辑
- `handleServerSave` 可能需要扩展以支持保存后定位到新条目（通过 alias 匹配）

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 09-dup-ssh-connection*
*Context gathered: 2026-04-15*
