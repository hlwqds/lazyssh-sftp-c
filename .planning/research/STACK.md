# Technology Stack Research

**Analysis Date:** 2026-04-15
**Domain:** TUI File Operations for Go SSH Manager (v1.2)

---

## v1.2 File Management Operations -- Stack Additions

**结论: 零新外部依赖。** 所有文件管理操作（删除/重命名/新建/复制/移动）所需的全部原语已存在于 `github.com/pkg/sftp v1.13.10`（已在 go.mod 中作为 indirect 依赖）和 Go 标准库 `os` 包中。无需引入任何新依赖。

### SFTP Protocol Primitives

`pkg/sftp` v1.13.10 客户端提供了 v1.2 所需的全部方法，经源码验证（`client.go` 行号）：

| SFTP Method | Protocol | Signature | v1.2 Use Case | Confidence |
|-------------|----------|-----------|---------------|------------|
| `Remove(path)` | SSH_FXP_REMOVE | `(path string) error` | 删除单个文件或空目录 | HIGH (已实现) |
| `RemoveDirectory(path)` | SSH_FXP_RMDIR | `(path string) error` | 删除空目录（语义更明确） | HIGH |
| `RemoveAll(path)` | 自实现（遍历+Remove） | `(path string) error` | **递归删除非空目录** -- 核心依赖 | HIGH |
| `Rename(old, new)` | SSH_FXP_RENAME | `(oldname, newname string) error` | **重命名文件/目录** -- 核心依赖 | HIGH |
| `PosixRename(old, new)` | posix-rename@openssh.com | `(oldname, newname string) error` | **移动覆盖目标**（POSIX 语义：目标存在时替换） | HIGH |
| `Mkdir(path)` | SSH_FXP_MKDIR | `(path string) error` | 新建单个目录 | HIGH |
| `MkdirAll(path)` | 自实现（遍历+Mkdir） | `(path string) error` | 新建目录（已实现） | HIGH (已实现) |
| `Stat(path)` | SSH_FXP_LSTAT/STAT | `(path string) (os.FileInfo, error)` | 检查目标是否存在（冲突检测） | HIGH (已实现) |
| `Open(path)` | SSH_FXP_OPEN | `(path string) (*File, error)` | 读取源文件（复制操作） | HIGH (已实现) |
| `Create(path)` | SSH_FXP_OPEN (write) | `(path string) (*File, error)` | 写入目标文件（复制操作） | HIGH (已实现) |
| `Walk(path)` | 自实现（ReadDir 递归） | `(path string, walkFn WalkFunc) error` | 遍历目录（递归复制） | HIGH |

### Local Filesystem Equivalents (Go `os` Package)

| Operation | `os` Function | Signature | Notes |
|-----------|---------------|-----------|-------|
| 删除文件/空目录 | `os.Remove` | `(path string) error` | 已在 TransferService 中使用 |
| 递归删除目录 | `os.RemoveAll` | `(path string) error` | 标准库提供，无需自行实现 |
| 重命名/移动 | `os.Rename` | `(oldpath, newpath string) error` | 跨目录移动也支持（同一文件系统） |
| 新建目录 | `os.Mkdir` | `(path string, perm fs.FileMode) error` | 创建单个目录 |
| 新建目录（递归） | `os.MkdirAll` | `(path string, perm fs.FileMode) error` | 已在 TransferService 中使用 |
| 读取文件 | `os.Open` | `(name string) (*File, error)` | 已在 TransferService 中使用 |
| 创建文件 | `os.Create` | `(name string) (*File, error)` | 已在 TransferService 中使用 |
| 检查文件信息 | `os.Stat` | `(name string) (FileInfo, error)` | 已在 TransferService 中使用 |

---

### New Port Interface Methods

v1.2 需要在现有接口上添加方法，遵循 Clean Architecture 的 Port/Adapter 模式。

#### FileService Interface (本地+远程通用)

当前 `FileService` 仅有 `ListDir`。v1.2 需要添加文件管理方法，使本地面板和远程面板共享同一接口：

```go
// FileService provides file operations for local and remote filesystems.
type FileService interface {
    // --- Existing ---
    ListDir(path string, showHidden bool, sortField domain.FileSortField, sortAsc bool) ([]domain.FileInfo, error)

    // --- v1.2 Additions ---
    // Remove deletes a single file or empty directory.
    Remove(path string) error

    // RemoveAll recursively deletes a directory and all its contents.
    RemoveAll(path string) error

    // Rename renames or moves a file/directory.
    // For cross-filesystem moves, implementations must handle copy+delete.
    Rename(oldPath, newPath string) error

    // Mkdir creates a single directory. Returns error if parent doesn't exist.
    Mkdir(path string) error

    // MkdirAll creates directories recursively, skipping existing ones.
    MkdirAll(path string) error

    // Stat returns file info for the given path.
    Stat(path string) (os.FileInfo, error)
}
```

**设计决策: 将 Remove/Stat 从 SFTPService 下沉到 FileService。**

理由: 删除和查看文件信息是本地和远程都需要的通用操作。当前 `Remove` 和 `Stat` 仅在 `SFTPService` 中，本地面板无法使用。将它们提升到 `FileService` 接口使两个面板共享统一操作模型，避免在 UI 层做 `if local then os.Remove else sftp.Remove` 的类型判断。

#### SFTPService Interface (远程特有)

`SFTPService` 继承 `FileService`，额外方法保持不变：

```go
// SFTPService provides SFTP connection and remote file operations.
type SFTPService interface {
    FileService  // inherits Remove, RemoveAll, Rename, Mkdir, MkdirAll, Stat

    // --- Connection lifecycle (unchanged) ---
    Connect(server domain.Server) error
    Close() error
    IsConnected() bool
    HomeDir() string

    // --- Remote I/O (unchanged) ---
    CreateRemoteFile(path string) (io.WriteCloser, error)
    OpenRemoteFile(path string) (io.ReadCloser, error)
    WalkDir(path string) ([]string, error)
}
```

**注意:** `Remove` 和 `Stat` 从 `SFTPService` 的独立声明变为继承自 `FileService`，行为不变，只是接口层级调整。现有代码编译无需修改（`SFTPService` 仍然有这两个方法）。

#### CopyService Interface (新增)

复制操作没有直接的 SFTP 原语，需要读取源+写入目标。这需要一个新接口：

```go
// CopyService provides copy operations within a single filesystem
// (local-to-local or remote-to-remote).
// Cross-pane copy (local-to-remote, remote-to-local) uses existing TransferService.
type CopyService interface {
    // CopyFile copies a single file within the same filesystem.
    // src and dst are absolute paths on the same filesystem.
    CopyFile(ctx context.Context, srcPath, dstPath string, onProgress func(domain.TransferProgress), onConflict domain.ConflictHandler) error

    // CopyDir recursively copies a directory within the same filesystem.
    // Returns list of failed file paths (empty = all success).
    CopyDir(ctx context.Context, srcPath, dstPath string, onProgress func(domain.TransferProgress), onConflict domain.ConflictHandler) ([]string, error)
}
```

**设计决策: 独立 CopyService 而非扩展 FileService。**

理由: 复制是重量级 I/O 操作，需要 `context.Context` 取消、进度回调、冲突处理 -- 与 FileService 的简单 CRUD 语义不同。独立接口保持职责清晰，也便于 UI 层分别注入依赖。

---

### Implementation Strategy

#### SFTPClient Adapter Changes

在 `internal/adapters/data/sftp_client/sftp_client.go` 中添加新方法：

| Method | Implementation | Notes |
|--------|---------------|-------|
| `RemoveAll(path)` | `client.RemoveAll(path)` | 一行代理，pkg/sftp 已实现递归删除 |
| `Rename(old, new)` | `client.Rename(old, new)` | 标准重命名 |
| `Mkdir(path)` | `client.Mkdir(path)` | 单层目录创建 |
| `Stat(path)` | `client.Stat(path)` | 已实现，下沉到 FileService 接口 |

现有方法无需修改：`Remove`、`MkdirAll` 已实现。所有新方法遵循现有的 mutex 锁模式（`c.mu.Lock()` -> 获取 client -> `c.mu.Unlock()`）。

#### LocalFS Adapter Changes

在 `internal/adapters/data/local_fs/local_fs.go` 中添加新方法：

| Method | Implementation | Notes |
|--------|---------------|-------|
| `Remove(path)` | `os.Remove(path)` | 一行代理 |
| `RemoveAll(path)` | `os.RemoveAll(path)` | 一行代理 |
| `Rename(old, new)` | `os.Rename(old, new)` | 一行代理 |
| `Mkdir(path)` | `os.Mkdir(path, 0o750)` | 使用项目一致的权限值 |
| `MkdirAll(path)` | `os.MkdirAll(path, 0o750)` | 使用项目一致的权限值 |
| `Stat(path)` | `os.Stat(path)` | 一行代理 |

#### CopyService Adapters

需要两个实现：

1. **`LocalCopyService`** (`internal/adapters/data/local_fs/local_copy.go`)
   - `os.Open` + `os.Create` + 32KB buffer `copyWithProgress`
   - 可复用 `transfer_service.go` 的 `copyWithProgress` 模式（提取为公共函数或独立实现）

2. **`RemoteCopyService`** (`internal/adapters/data/sftp_client/remote_copy.go`)
   - `sftpClient.OpenRemoteFile` + `sftpClient.CreateRemoteFile` + 32KB buffer
   - 依赖 SFTPService 获取 reader/writer

**32KB Buffer Copy Pattern（复用现有模式）:**

```go
// 已在 transfer_service.go:436-485 中验证的模式
func copyWithProgress(ctx context.Context, src io.Reader, dst io.Writer,
    srcPath, displayPath string, total int64, onProgress func(domain.TransferProgress)) error {
    buf := make([]byte, 32*1024)
    var transferred int64
    for {
        select {
        case <-ctx.Done():
            return context.Canceled
        default:
        }
        n, readErr := src.Read(buf)
        if n > 0 {
            _, writeErr := dst.Write(buf[:n])
            if writeErr != nil {
                return fmt.Errorf("write: %w", writeErr)
            }
            transferred += int64(n)
            // progress callback...
        }
        if readErr != nil { break }
    }
    return nil
}
```

建议将 `copyWithProgress` 从 `transfer` 包提取为 `internal/core/services/copy.go` 或 `internal/adapters/data/common/copy.go`，供 TransferService 和两个 CopyService 共用。

---

### UI Components (tview/tcell)

文件管理操作的 UI 组件全部基于已有 tview/tcell 原语：

| UI Component | Technology | Pattern Reference |
|-------------|------------|-------------------|
| 删除确认对话框 | `tview.Modal` + `app.SetRoot()` | `handlers.go:272-276` -- 已有确认弹窗模式 |
| 重命名内联编辑 | `tview.InputField` + 覆盖层 | TransferModal 的 overlay draw chain 模式 |
| 新建目录输入框 | `tview.InputField` + `tview.Modal` | 同确认弹窗模式，替换文本为输入框 |
| 操作进度显示 | 复用 TransferModal | 已有 progress/cancelConfirm/summary 状态机 |
| 标记状态指示 | `tview.Table` 单元格颜色/前缀 | 现有 FileBrowser 表格渲染 |

### Keyboard Binding Integration

新快捷键在 `file_browser_handlers.go` 的 `handleGlobalKeys` 和 pane `SetInputCapture` 中添加：

| Key | Context | Action | Location |
|-----|---------|--------|----------|
| `d` | 有选中项 | 删除（弹出确认对话框） | Pane InputCapture |
| `R` | 有选中项 | 重命名（弹出内联编辑） | Pane InputCapture |
| `m` | 面板活跃 | 新建目录（弹出输入框） | handleGlobalKeys 或 Pane InputCapture |
| `c` | 有选中项 | 标记复制（状态切换） | Pane InputCapture |
| `x` | 有选中项 | 标记移动（状态切换） | Pane InputCapture |
| `p` | 有标记项 + 切换到目标面板 | 执行粘贴 | handleGlobalKeys |

**标记状态管理:** 需要在 `FileBrowser` 中新增 `markedItem` 字段，存储 `(sourcePane int, sourcePath string, operation copyOrMove)`。面板切换（Tab）后按 `p` 执行操作。

---

### What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `SFTP Rename` 移动到已存在目标 | SSH_FXP_RENAME 对已存在目标的行为取决于服务器实现（RFC 未定义） | `PosixRename` -- POSIX 语义明确要求原子替换目标 |
| 自实现递归删除 | 易遗漏符号链接、权限错误、竞态条件 | `pkg/sftp.RemoveAll` -- 库已处理边界情况 |
| 自实现递归复制 | 需处理进度回调、取消传播、冲突处理、部分清理 | 基于 `copyWithProgress` + `WalkDir`/`filepath.WalkDir` 组合 |
| `os.Rename` 跨文件系统移动 | `os.Rename` 跨文件系统返回 `EXDEV` 错误 | 检测 `EXDEV` 后降级为 copy+delete（v1.2 暂不处理跨文件系统，单面板内移动通常同文件系统） |
| 新增第三方依赖 | 项目约束"零外部依赖" | 全部使用已有 pkg/sftp + os 标准库 |

---

### Dependency Summary

| Category | Before v1.2 | After v1.2 | Change |
|----------|-------------|------------|--------|
| External Go deps | pkg/sftp v1.13.10 (indirect) | pkg/sftp v1.13.10 (indirect -> direct) | 仅将 indirect 改为 direct |
| New Go packages | -- | `context`, `io`, `os`, `path/filepath` | 全部标准库，已在项目中使用 |
| New tview widgets | -- | `tview.InputField` | 已在 ServerForm 中使用 |
| New interfaces | FileService, SFTPService, TransferService | + CopyService | 1 个新接口 |

**总新增依赖: 0 个外部依赖。** 仅需将 `pkg/sftp` 从 `go.mod` 的 `// indirect` 改为直接依赖（因为 `SFTPClient` 将使用 `RemoveAll`、`Rename` 等新方法）。

---

## Existing Stack (Preserved from v1.0/v1.1)

| Component | Technology | Version |
|-----------|-----------|---------|
| Language | Go | 1.24.6 |
| TUI Framework | tview/tcell | tview latest, tcell/v2 2.9.0 |
| CLI | Cobra | 1.9.1 |
| Logging | Zap | 1.27.0 |
| SSH Config | ssh_config (forked) | 1.4.0 |
| SFTP Client | pkg/sftp | v1.13.10 |

## Sources

- **HIGH confidence (项目源码):**
  - `internal/core/ports/file_service.go` -- FileService + SFTPService 接口定义
  - `internal/core/ports/transfer.go` -- TransferService 接口定义
  - `internal/adapters/data/sftp_client/sftp_client.go` -- SFTPClient 实现
  - `internal/adapters/data/local_fs/local_fs.go` -- LocalFS 实现
  - `internal/adapters/data/transfer/transfer_service.go` -- TransferService 实现 + copyWithProgress 模式
  - `go.mod` -- pkg/sftp v1.13.10 依赖声明
  - `internal/core/domain/file_info.go` -- FileInfo 域模型

- **HIGH confidence (pkg/sftp 源码验证):**
  - `~/go/pkg/mod/github.com/pkg/sftp@v1.13.10/client.go`
    - `Remove` (line 804), `RemoveDirectory` (line 866), `RemoveAll` (line 1038)
    - `Rename` (line 892), `PosixRename` (line 912)
    - `Mkdir` (line 971), `MkdirAll` (line 992)

---
*Stack research: 2026-04-15 (v1.2 File Management Operations)*
*Original: 2026-04-13 (v1.0 File Transfer)*
*Updated: 2026-04-14 (v1.1 Recent Remote Directories)*
