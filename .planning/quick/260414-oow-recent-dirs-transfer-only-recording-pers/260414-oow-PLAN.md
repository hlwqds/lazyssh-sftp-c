---
phase: quick
plan: 260414-oow
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/adapters/ui/file_browser/recent_dirs.go
  - internal/adapters/ui/file_browser/file_browser.go
autonomous: true
requirements: [REQ-01, REQ-02]

must_haves:
  truths:
    - "远程目录导航不再记录到最近目录列表"
    - "文件/目录传输成功后才记录远程目录到最近目录列表"
    - "应用重启后最近目录列表从磁盘恢复"
    - "每个服务器的最近目录独立存储（user@host 维度）"
  artifacts:
    - path: "internal/adapters/ui/file_browser/recent_dirs.go"
      provides: "RecentDirs 组件，支持磁盘持久化和按服务器隔离"
      contains: "loadFromDisk, saveToDisk"
    - path: "internal/adapters/ui/file_browser/file_browser.go"
      provides: "FileBrowser 中移除导航记录，仅保留传输记录"
  key_links:
    - from: "recent_dirs.go Record()"
      to: "~/.lazyssh/recent-dirs/{user@host}.json"
      via: "saveToDisk after each Record()"
    - from: "recent_dirs.go NewRecentDirs()"
      to: "~/.lazyssh/recent-dirs/{user@host}.json"
      via: "loadFromDisk on construction"
---

<objective>
修改最近远程目录功能：移除导航记录，仅保留传输记录；添加磁盘持久化，按服务器隔离存储。

Purpose: 导航时记录目录噪音太大（用户浏览即污染列表），只有实际发生传输的目录才有价值。持久化让列表在应用重启后依然可用。

Output: 修改后的 RecentDirs 组件（纯内存 -> 持久化）和 FileBrowser（移除导航回调中的 Record 调用）
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
</execution_context>

<context>
@.planning/STATE.md

# 现有持久化模式（参考 metadata_manager.go）
- 存储路径: `~/.lazyssh/` 目录
- 目录权限: `0o750`
- 文件权限: `0o600`
- 格式: JSON (`json.MarshalIndent`)
- 读写: `os.ReadFile` / `os.WriteFile`
- 目录创建: `os.MkdirAll`

# RecentDirs 当前接口
```go
// recent_dirs.go
type RecentDirs struct {
    *tview.Box
    paths         []string
    visible       bool
    selectedIndex int
    onSelect      func(path string)
    currentPath   string
}
func NewRecentDirs() *RecentDirs
func (rd *RecentDirs) Record(path string)
func (rd *RecentDirs) GetPaths() []string
```

# FileBrowser 中的三处 Record 调用
1. file_browser.go:146 — `fb.remotePane.OnPathChange` 回调中的 `fb.recentDirs.Record(path)` (导航记录，需删除)
2. file_browser.go:398 — `initiateTransfer` 成功后 `fb.recentDirs.Record(fb.remotePane.GetCurrentPath())` (传输记录，保留)
3. file_browser.go:504 — `initiateDirTransfer` 成功后 `fb.recentDirs.Record(fb.remotePane.GetCurrentPath())` (传输记录，保留)

# 服务器标识
```go
// domain.Server
type Server struct {
    Host string
    User string
    // ...
}
```
组合键格式: `user@host`（与 SSH config 和状态栏显示一致）
</context>

<tasks>

<task type="auto">
  <name>Task 1: 添加磁盘持久化并按服务器隔离 RecentDirs</name>
  <files>internal/adapters/ui/file_browser/recent_dirs.go</files>
  <action>
修改 `RecentDirs` 组件，添加磁盘持久化能力：

1. **修改构造函数签名**：
   ```go
   func NewRecentDirs(log *zap.SugaredLogger, serverHost, serverUser string) *RecentDirs
   ```
   - 新增 `log *zap.SugaredLogger` 字段（用于持久化错误日志）
   - 新增 `serverKey string` 字段，值为 `user@host`（如 `root@192.168.1.1`）
   - 新增 `filePath string` 字段，值为 `~/.lazyssh/recent-dirs/{user@host}.json`

2. **添加 `loadFromDisk` 方法**：
   - 在 `NewRecentDirs` 中调用
   - 计算文件路径: `filepath.Join(os.UserHomeDir(), ".lazyssh", "recent-dirs", serverKey+".json")`
   - 确保目录存在 (`os.MkdirAll`, 权限 `0o750`)
   - 如果文件不存在，返回空切片（静默，不报错）
   - 读取文件，`json.Unmarshal` 到 `[]string`
   - 赋值给 `rd.paths`

3. **添加 `saveToDisk` 方法**：
   - 使用 `json.MarshalIndent(rd.paths, "", "  ")` 序列化
   - 写入文件，权限 `0o600`（与 metadata_manager.go 一致）
   - 错误时 `log.Errorw()` 但不返回错误（Record 调用不应因持久化失败而中断）

4. **修改 `Record` 方法**：
   - 保持现有去重和截断逻辑不变
   - 在方法末尾调用 `rd.saveToDisk()`

5. **import 新增**：
   - `encoding/json`
   - `os`
   - `path/filepath`
   - `go.uber.org/zap`

设计原理：
- 选择每个服务器独立文件（而非合并到一个 JSON）是因为：(a) 并发安全——同时操作两个服务器不会冲突；(b) 删除/清理简单——删文件即可；(c) 与 metadata.json 的 map[string]ServerMetadata 模式互补。
- Record 后立即保存（而非延迟/批量）是因为每次传输间隔至少数秒，I/O 开销可忽略。
- 新增 import 用 `go.uber.org/zap` 而非其他日志库，与项目现有 logger 一致。
  </action>
  <verify>
    <automated>cd /home/huanglin/code/lazyssh && go build ./...</automated>
  </verify>
  <done>
    - NewRecentDirs 接受 log, serverHost, serverUser 参数
    - 构造时从 `~/.lazyssh/recent-dirs/{user@host}.json` 加载历史数据
    - Record() 调用后自动持久化到磁盘
    - 编译通过（file_browser.go 暂时会报错因为 NewRecentDirs 签名变了，Task 2 修复）
  </done>
</task>

<task type="auto">
  <name>Task 2: 移除导航记录，更新 FileBrowser 调用</name>
  <files>internal/adapters/ui/file_browser/file_browser.go</files>
  <action>
修改 `FileBrowser`，移除导航时的 Record 调用，更新 NewRecentDirs 调用以匹配新签名：

1. **移除导航记录**（file_browser.go 第 144-147 行）：
   ```go
   // 修改前：
   fb.remotePane.OnPathChange(func(path string) {
       fb.app.Sync()
       fb.recentDirs.Record(path) // D-04: record path for recent dirs list
   })

   // 修改后：
   fb.remotePane.OnPathChange(func(_ string) {
       fb.app.Sync()
   })
   ```
   注意：参数改为 `_ string` 因为 path 不再使用（与 localPane 的 OnPathChange 回调保持一致）。

2. **更新 NewRecentDirs 调用**（file_browser.go 第 107 行）：
   ```go
   // 修改前：
   fb.recentDirs = NewRecentDirs()

   // 修改后：
   fb.recentDirs = NewRecentDirs(fb.log, fb.server.Host, fb.server.User)
   ```

3. **传输记录保持不变**：
   - `initiateTransfer` 第 398 行 `fb.recentDirs.Record(fb.remotePane.GetCurrentPath())` — 保留
   - `initiateDirTransfer` 第 504 行 `fb.recentDirs.Record(fb.remotePane.GetCurrentPath())` — 保留
   - RecentDirs onSelect 回调中的 Record 也保留（用户从弹窗选择目录时也记录，符合 MRU 语义）

设计原理：
- 导航记录噪音太大：用户每进入一个子目录就会污染列表，而用户只关心实际传输过的目录
- 仅保留传输记录：上传/下载成功后的目录才是真正有"最近使用"价值的
- onSelect 回调保留 Record：用户从弹窗选目录也是一种"使用"信号，保持 MRU 语义正确
  </action>
  <verify>
    <automated>cd /home/huanglin/code/lazyssh && go build ./...</automated>
  </verify>
  <done>
    - 远程面板导航不再触发 Record()
    - NewRecentDirs 传入 logger 和服务器标识
    - 文件传输和目录传输成功后仍然记录
    - 弹窗选择目录后仍然记录
    - `go build ./...` 编译通过
  </done>
</task>

</tasks>

<verification>
1. `go build ./...` 编译通过
2. `go vet ./...` 无警告
3. 确认 file_browser.go 中 OnPathChange 回调不再调用 Record
4. 确认 initiateTransfer 和 initiateDirTransfer 中 Record 调用保留
5. 确认 NewRecentDirs 接受正确参数
</verification>

<success_criteria>
- 远程目录导航不记录到最近列表
- 仅传输成功后记录远程目录
- 应用重启后最近目录从 `~/.lazyssh/recent-dirs/{user@host}.json` 恢复
- 不同服务器的最近目录独立存储
- `go build ./...` 和 `go vet ./...` 通过
</success_criteria>

<output>
After completion, create `.planning/quick/260414-oow-recent-dirs-transfer-only-recording-pers/260414-oow-SUMMARY.md`
</output>
