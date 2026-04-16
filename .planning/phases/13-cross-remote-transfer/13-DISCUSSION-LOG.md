# Phase 13: Cross-Remote Transfer - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-16
**Phase:** 13-cross-remote-transfer
**Areas discussed:** Transfer backend, TransferModal reuse, F5 direct transfer, Batch & cancel

---

## Transfer backend

| Option | Description | Selected |
|--------|-------------|----------|
| 新建 CrossRemote 方法 | TransferService 新增 CrossRemoteCopyFile/CrossRemoteDir，接收两个 SFTPService 参数 | ✓ |
| 组合 Download+Upload | DualRemoteFileBrowser 直接编排 DownloadFile + UploadFile | |
| You decide | Claude 自行决定 | |

**User's choice:** 新建 CrossRemote 方法
**Notes:** 复用现有 download→temp→upload 基础设施但支持跨连接

## TransferModal reuse

| Option | Description | Selected |
|--------|-------------|----------|
| 扩展 TransferModal | 新增 modeCrossRemote，复用 progress/cancelConfirm/conflictDialog | ✓ |
| 新建独立 overlay | 独立 CrossRemoteProgress overlay | |
| You decide | Claude 自行决定 | |

**User's choice:** 扩展 TransferModal
**Notes:** 复用现有状态机基础设施

## Progress bar behavior

| Option | Description | Selected |
|--------|-------------|----------|
| 重置进度条 | 两阶段切换时重置为 0%，标题切换来源/目标 | ✓ |
| 连续进度条 | 下载 50% + 上传 50%，连续递增 | |
| 纯文本状态 | 只显示文本，无进度条 | |

**User's choice:** 重置进度条

## F5 direct transfer

| Option | Description | Selected |
|--------|-------------|----------|
| F5 only | F5 触发传输，Enter 保持进入目录 | ✓ |
| F5 + Enter | 两者都可触发传输 | |
| No F5 | 只通过 c/x + p | |

**User's choice:** F5 only
**Notes:** Enter 保持进入目录行为

## F5 confirm dialog

| Option | Description | Selected |
|--------|-------------|----------|
| 文件直接，目录确认 | 文件直接传输，目录弹出 ConfirmDialog | ✓ |
| 全部直接传输 | 不确认 | |
| 全部确认 | 文件和目录都确认 | |

**User's choice:** 文件直接，目录确认
**Notes:** 目录递归传输可能很大，需要确认

## Batch operations

| Option | Description | Selected |
|--------|-------------|----------|
| 单文件 only | 与 Phase 7/8 一致，Space 仅用于批量删除 | ✓ |
| 支持批量粘贴 | Space 多选 + p 批量跨远端 | |
| You decide | Claude 自行决定 | |

**User's choice:** 单文件 only
**Notes:** 保持简单，与现有剪贴板模式一致

## Cancel cleanup

| Option | Description | Selected |
|--------|-------------|----------|
| 清理 temp + 目标 | 清理本地 temp + 目标端部分文件 | ✓ |
| 只清理 temp | 保留目标端已上传文件 | |
| You decide | Claude 自行决定 | |

**User's choice:** 清理 temp + 目标

## Move failure rollback

| Option | Description | Selected |
|--------|-------------|----------|
| 清理目标副本 | 与 Phase 8 D-04 一致，尝试清理目标 | ✓ |
| 保留两端 | 源+目标各一份，用户手动处理 | |

**User's choice:** 清理目标副本
**Notes:** 与 Phase 8 移动失败策略一致

## Claude's Discretion

- CrossRemoteCopyFile/CrossRemoteDir 具体方法签名
- TransferModal modeCrossRemote UI 布局细节
- temp 目录位置
- [C]/[M] 前缀颜色
- F5 目录确认提示文本
- 状态栏提示文本

## Deferred Ideas

None
