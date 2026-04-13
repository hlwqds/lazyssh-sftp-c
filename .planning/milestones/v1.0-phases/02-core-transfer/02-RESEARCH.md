# Phase 2: Core Transfer — Research

**Researched:** 2026-04-13
**Status:** Complete

## 技术原理分析

### 1. pkg/sftp 文件传输 API

`github.com/pkg/sftp` 提供了完整的 SFTP 文件 I/O 能力，已作为 indirect dependency 存在于 go.sum 中（v1.13.10）。

**核心 API：**
- `client.Create(path) (*sftp.File, error)` — 创建远程文件用于写入，返回 `io.WriteCloser`
- `client.Open(path) (*sftp.File, error)` — 打开远程文件用于读取，返回 `io.ReadCloser`
- `client.Mkdir(path) error` — 创建远程目录（单层）
- `client.ReadDir(path) ([]os.FileInfo, error)` — 列出目录内容
- `client.Remove(path) error` — 删除远程文件（用于取消清理）
- `client.Stat(path) (os.FileInfo, error)` — 获取远程文件信息
- `client.Chtimes(path, atime, mtime time.Time) error` — 修改文件时间戳（可选）

**文件传输实现模式：**
```go
// Upload: local → remote
src, _ := os.Open(localPath)
defer src.Close()
dst, _ := sftpClient.Create(remotePath)
defer dst.Close()
io.CopyBuffer(dst, src, make([]byte, 32*1024)) // 32KB buffer
```

**目录递归传输模式：**
- 使用 `filepath.WalkDir()` (local) 或递归 `client.ReadDir()` (remote) 遍历目录树
- 先创建目标目录结构，再逐文件传输
- 跳过失败文件，记录失败列表

### 2. 进度回调与速度计算

**io.Copy 不支持进度回调。** 需要自定义 copying 循环：

```go
buf := make([]byte, 32*1024)
for {
    n, err := src.Read(buf)
    if n > 0 {
        dst.Write(buf[:n])
        transferred += int64(n)
        onProgress(TransferProgress{BytesDone: transferred, BytesTotal: total})
    }
    if err != nil { break }
}
```

**速度计算：滑动窗口平均**
- 维护一个固定大小的采样窗口（5 个采样点）
- 每次进度更新记录 `(timestamp, bytesDone)` 对
- 速度 = (最新 bytesDone - 最旧 bytesDone) / (最新时间 - 最旧时间)
- 避免瞬时波动，提供稳定显示

**ETA 计算：**
- `ETA = (BytesTotal - BytesDone) / Speed`
- 当 Speed == 0 或传输刚开始时显示 "calculating..."

### 3. tview 自定义组件模式

**tview.Primitive 接口：** 任何实现 `Draw(screen tcell.Screen)` 和 `GetRect() (int, int, int, int)` 的类型都可以作为 tview 组件。

**TransferModal 的两种实现方式：**

| 方式 | 优点 | 缺点 |
|------|------|------|
| 组合现有 tview 组件（Grid + TextView） | 简单，利用现有布局 | 进度条需要用 TextView 文本渲染 |
| 自定义 Primitive 实现 Draw() | 完全控制渲染 | 代码量大，需处理 focus、mouse 等 |

**推荐：组合方式。** 使用 `tview.Grid` 作为容器，内部放 `tview.TextView` 用于文件名/速度/ETA。进度条用单独的 `tview.TextView`，通过 `SetText()` 更新 `█░` 字符串。这与 Phase 1 的组件模式一致（StatusBar 也是 `tview.TextView`）。

**Modal 覆盖层显示方式：**
```go
// 使用 tview.Pages 实现覆盖层
pages := tview.NewPages()
pages.AddAndSwitchToPage("transfer", modal, true)
pages.AddPage("browser", fileBrowser, true, false) // behind
app.SetRoot(pages, true)
```

或者直接在 FileBrowser 上方叠加 Modal，通过 `app.SetRoot()` 切换。

### 4. 现有代码集成点分析

**SFTPClient 内部状态：**
- `client *sftp.Client` — SFTP 客户端，受 `mu sync.Mutex` 保护
- 现有 `ListDir()` 使用 `client.ReadDir(path)` — 可以直接使用 `client.Create()`/`client.Open()`
- 需要添加的方法通过同样的 mutex 模式保护 client 访问

**LocalPane SetSelectedFunc (line 88-102):**
```go
lp.SetSelectedFunc(func(row, _ int) {
    // ... get FileInfo from cell reference
    if !ok || !fi.IsDir {
        return  // ← Phase 2: 这里需要改为调用 onFileAction(fi) 回调
    }
    lp.NavigateInto(fi.Name) // 目录导航保持不变
})
```

**RemotePane SetSelectedFunc (line 95-112):** 同样结构，需要添加 `!fi.IsDir` 分支。

**FileBrowser.handleGlobalKeys (line 26-45):**
- Tab, Esc, s, S 已处理
- F5 需要添加到 switch 中
- F5 在 `event.Key() switch` 中添加 `case tcell.KeyF5`

**依赖注入链：**
```
cmd/main.go → ui.NewTUI() → FileBrowser 构造 → pane 构造
```
TransferService 需要从 main.go 一路传递到 FileBrowser：
```
main.go: transferService := transfer.New(log, sftpService)
tui.go: NewTUI(..., transferService)
file_browser.go: NewFileBrowser(..., transferService)
```

### 5. 跨平台注意事项（Phase 2 预留）

- **路径分隔符：** 本地使用 `filepath.Join()` (自动处理 `/` vs `\`)，远程始终使用 `/`（SFTP 是 Unix 协议）
- **文件权限：** SFTP 传输会保留 Unix 权限位，Windows 上部分权限不适用（忽略错误即可）
- **符号链接：** `filepath.WalkDir()` 默认不跟随符号链接（Phase 2 不处理符号链接传输）

## 方案对比

### TransferService 放置位置

| 方案 | 位置 | 优点 | 缺点 |
|------|------|------|------|
| A: Port interface in ports/ | `ports/file_service.go` | 符合现有架构 | 文件变大 |
| B: 新建 ports/transfer.go | `ports/transfer.go` | 关注点分离 | 新文件 |
| C: 在 services/ 层 | `services/transfer_service.go` | 符合 service 层模式 | services 目前只有 serverService |

**推荐：方案 B** — 新建 `ports/transfer.go`，保持 port 接口的内聚性。TransferService 是一个新的能力维度，与 FileService 的 "列表" 关注点不同。

### 目录传输的遍历策略

| 方案 | 实现 | 优点 | 缺点 |
|------|------|------|------|
| A: 先计算总文件数，再传输 | Walk → count → Walk again → transfer | 进度显示 "file 3/10" | 遍历两次 |
| B: 边遍历边传输 | Walk → for each file: count++, transfer | 只遍历一次 | 不知道总数，进度显示 "file 3/?" |

**推荐：方案 A（先计算总数）。** CONTEXT.md specific ideas 明确提到 "目录传输先遍历计算总文件数"。虽然遍历两次，但远程目录遍历很快（只是 ReadDir），用户体验更好。

## 推荐理由

1. **TransferProgress 放在 domain/** — 它描述传输状态，属于 domain 概念，不是 I/O 抽象
2. **TransferService 作为独立 port** — 与 FileService 分离，职责清晰
3. **SFTPService 扩展而非新建接口** — 保持现有接口稳定，通过方法扩展暴露文件 I/O
4. **组合式 Modal** — 遵循 Phase 1 的组件模式（StatusBar = TextView），不引入新的渲染范式
5. **io.Copy 替换为手动 copy 循环** — 唯一能实现进度回调的方式
6. **滑动窗口速度计算** — 比瞬时速度更稳定，比全局平均更响应变化

---

*Phase: 02-core-transfer*
*Research completed: 2026-04-13*
