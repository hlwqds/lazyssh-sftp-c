// Copyright 2025.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package file_browser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"github.com/Adembc/lazyssh/internal/core/ports"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"go.uber.org/zap"
)

// Direction labels for transfer display.
const (
	directionUpload   = "Uploading"
	directionDownload = "Downloading"
)

// msgDirNotEmpty is the warning shown when deleting a non-empty directory.
const msgDirNotEmpty = "Directory not empty, all contents will be deleted"

// ClipboardOp represents the clipboard operation type.
type ClipboardOp int

const (
	// OpCopy marks the clipboard for copy operation.
	OpCopy ClipboardOp = iota
	// OpMove marks the clipboard for move operation (Phase 8).
	OpMove
)

// Clipboard holds the state for copy/move clipboard operations.
// Stored on FileBrowser (not per-pane) for cross-directory navigation persistence (CLP-02, D-04).
type Clipboard struct {
	Active     bool
	SourcePane int // 0 = local, 1 = remote
	FileInfo   domain.FileInfo
	SourceDir  string
	Operation  ClipboardOp
}

// FileBrowser is the root component for the dual-pane file browser.
// It is a self-contained tview.Primitive that can be set as root via app.SetRoot().
// Layout: FlexRow with content (FlexColumn: LocalPane + RemotePane) and StatusBar.
type FileBrowser struct {
	*tview.Flex
	app            *tview.Application
	log            *zap.SugaredLogger
	fileService    ports.FileService
	sftpService    ports.SFTPService
	transferSvc    ports.TransferService
	server         domain.Server
	localPane      *LocalPane
	remotePane     *RemotePane
	statusBar      *tview.TextView
	transferModal  *TransferModal
	recentDirs     *RecentDirs // in-memory MRU list of recent remote directories
	confirmDialog  *ConfirmDialog
	inputDialog    *InputDialog
	clipboard      Clipboard // Phase 7: copy/paste state
	activePane     int       // 0 = local, 1 = remote
	transferring   bool
	transferCancel context.CancelFunc // cancel function for active transfer context
	onClose        func()
}

// NewFileBrowser creates a new FileBrowser with dual-pane layout.
func NewFileBrowser(
	app *tview.Application,
	log *zap.SugaredLogger,
	fs ports.FileService,
	sftp ports.SFTPService,
	ts ports.TransferService,
	server domain.Server,
	onClose func(),
) *FileBrowser {
	fb := &FileBrowser{
		Flex:        tview.NewFlex(),
		app:         app,
		log:         log,
		fileService: fs,
		sftpService: sftp,
		transferSvc: ts,
		server:      server,
		onClose:     onClose,
	}
	fb.build()
	return fb
}

// build initializes the layout, panes, status bar, and SFTP connection.
func (fb *FileBrowser) build() {
	// Use ColorDefault so the background blends with kitty's native background.
	// When kitty has background_opacity < 1, specific colors like Color234 create
	// a visible mismatch against the composited desktop background, causing
	// stale content to appear as "ghost" artifacts. ColorDefault lets kitty
	// use its own configured background (#1e1e2e with Catppuccin Mocha).
	fb.SetBackgroundColor(tcell.ColorDefault)

	// Determine initial local path (D-10: home directory)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "/"
		fb.log.Warnw("failed to get home directory, using /", "error", err)
	}

	// Create panes
	fb.localPane = NewLocalPane(fb.log, fb.fileService, homeDir)
	fb.remotePane = NewRemotePane(fb.log, fb.sftpService, fb.server)

	// Wire clipboard provider for [C]/[M] prefix rendering (Phase 7/8)
	fb.localPane.SetClipboardProvider(func() (bool, string, string, ClipboardOp) {
		return fb.clipboard.Active, fb.clipboard.FileInfo.Name, fb.clipboard.SourceDir, fb.clipboard.Operation
	})
	fb.remotePane.SetClipboardProvider(func() (bool, string, string, ClipboardOp) {
		return fb.clipboard.Active, fb.clipboard.FileInfo.Name, fb.clipboard.SourceDir, fb.clipboard.Operation
	})

	// Create transfer modal
	fb.transferModal = NewTransferModal(fb.app, fb.log)

	// Create recent directories tracker (Phase 4: data layer, Phase 5: popup UI)
	fb.recentDirs = NewRecentDirs(fb.log, fb.server.Host, fb.server.User)

	// Create overlay dialogs for file operations (Phase 6)
	fb.confirmDialog = NewConfirmDialog(fb.app)
	fb.inputDialog = NewInputDialog(fb.app)

	// Wire onSelect callback: Hide -> NavigateTo -> Record -> SetFocus (D-10)
	fb.recentDirs.SetOnSelect(func(path string) {
		fb.recentDirs.Hide()
		fb.remotePane.NavigateTo(path)
		fb.recentDirs.Record(path)
		fb.app.SetFocus(fb.remotePane)
	})

	fb.transferModal.SetDismissCallback(func() {
		if fb.transferModal.IsCanceled() {
			if fb.transferCancel != nil {
				fb.transferCancel()
			}
			// Do not close modal — wait for goroutine to complete and show canceled summary
			return
		}
		// Normal dismiss (summary mode: any key closes)
		fb.transferring = false
		fb.app.SetRoot(fb, true)
		fb.app.SetFocus(fb.currentPane())
	})

	// Wire pane file action callbacks for file transfer
	fb.localPane.OnFileAction(func(fi domain.FileInfo) {
		fb.initiateTransfer()
	})
	fb.remotePane.OnFileAction(func(fi domain.FileInfo) {
		fb.initiateTransfer()
	})

	// NOTE: OnPathChange callbacks removed — app.Sync() was causing the terminal
	// to receive stale content followed by new content in quick succession,
	// leading to visual overlap artifacts in GPU-accelerated terminals.

	// Create status bar
	fb.statusBar = tview.NewTextView()
	fb.statusBar.SetDynamicColors(true)
	fb.statusBar.SetBackgroundColor(tcell.Color235)
	fb.statusBar.SetWrap(false)
	fb.statusBar.SetTextAlign(tview.AlignCenter)
	fb.setStatusBarDefault()

	// Build dual-pane content layout (50:50 per D-04)
	content := tview.NewFlex().SetDirection(tview.FlexColumn)
	content.SetBackgroundColor(tcell.ColorDefault) // blend with kitty's native background
	content.
		AddItem(fb.localPane, 0, 1, true).  // 50% width, initially focused
		AddItem(fb.remotePane, 0, 1, false) // 50% width

	// Build root layout: content + status bar.
	// Status bar is a proper Flex child with fixed 1-row height so that
	// the content Flex (and its Table children) cannot overflow into it.
	fb.SetDirection(tview.FlexRow).
		AddItem(content, 0, 1, true).      // content takes remaining height
		AddItem(fb.statusBar, 1, 0, false) // status bar: fixed 1 row

	// Set initial focus state
	fb.activePane = 0
	fb.localPane.SetFocused(true)

	// Global input capture for Tab, Esc, s, S
	fb.SetInputCapture(fb.handleGlobalKeys)

	// Use AfterDrawFunc to redraw the status bar and force a full terminal sync.
	// This runs after root.Draw() AND all deferred draws (Flex defers focused items).
	//
	// The screen.Sync() call is a workaround for tcell v2.9.0 dirty tracking:
	// CellBuffer.Fill() (called by screen.Clear()) updates cell content but
	// does not invalidate them. In some cases, tcell's draw loop skips cells
	// whose content changed, causing stale content to persist on screen.
	// Sync() forces Invalidate() on all cells, ensuring every cell update
	// reaches the terminal. Placed in AfterDrawFunc (not BeforeDrawFunc) so
	// the sync sends the final content in one flush, avoiding blank flashes.
	fb.app.SetAfterDrawFunc(func(screen tcell.Screen) {
		// Skip status bar when transfer modal is visible (it has its own UI)
		if fb.transferModal != nil && fb.transferModal.IsVisible() {
			screen.Sync()
			return
		}
		_, _, width, height := fb.GetRect()
		if height >= 1 && fb.statusBar != nil {
			sy := height - 1
			bgColor := tcell.Color235
			bgStyle := tcell.StyleDefault.Background(bgColor)

			for col := 0; col < width; col++ {
				screen.SetContent(col, sy, ' ', nil, bgStyle)
			}
			tview.Print(screen, fb.statusBar.GetText(true), 0, sy, width, tview.AlignCenter, tcell.Color250)
			for col := 0; col < width; col++ {
				mainChar, _, style, _ := screen.GetContent(col, sy)
				screen.SetContent(col, sy, mainChar, nil, style.Background(bgColor))
			}
		}
		screen.Sync()
	})

	// Start SFTP connection in background (per RESEARCH Pattern 3, Pitfall 2)
	go func() {
		err := fb.sftpService.Connect(fb.server)
		fb.app.QueueUpdateDraw(func() {
			if err != nil {
				fb.remotePane.ShowError(err.Error())
				shortErr := trimError(err.Error(), 40)
				fb.updateStatusBarConnection(fmt.Sprintf("[#FF6B6B]Connection failed: %s[-]", shortErr))
			} else {
				fb.remotePane.ShowConnected()
				fb.updateStatusBarConnection(fmt.Sprintf("[#A0FFA0]Connected: %s@%s[-]", fb.server.User, fb.server.Host))
			}
		})
	}()

	// Load initial local directory listing
	fb.localPane.Refresh()
}

// Draw overrides Flex.Draw to draw overlays after the main content.
// Overlays (TransferModal, RecentDirs) are drawn on top after the main content.
func (fb *FileBrowser) Draw(screen tcell.Screen) {
	fb.Flex.Draw(screen)
	// Draw overlays on top of main content (Pattern 1, Pitfall 1 fix)
	if fb.transferModal != nil && fb.transferModal.IsVisible() {
		fb.transferModal.Draw(screen)
	}
	if fb.recentDirs != nil && fb.recentDirs.IsVisible() {
		fb.recentDirs.Draw(screen)
	}
	if fb.confirmDialog != nil && fb.confirmDialog.IsVisible() {
		fb.confirmDialog.Draw(screen)
	}
	if fb.inputDialog != nil && fb.inputDialog.IsVisible() {
		fb.inputDialog.Draw(screen)
	}
}

// setStatusBarDefault sets the default status bar text with keyboard hints.
func (fb *FileBrowser) setStatusBarDefault() {
	fb.statusBar.SetText("[white]Tab[-] Switch  [white]c[-] Copy  [white]x[-] Move  [white]p[-] Paste  [white]d[-] Delete  [white]R[-] Rename  [white]m[-] Mkdir  [white]s[-] Sort  [white]F5[-] Transfer  [white]Esc[-] Back")
}

// updateStatusBarConnection prepends connection status to the status bar text.
func (fb *FileBrowser) updateStatusBarConnection(msg string) {
	fb.statusBar.SetText(msg + "  [white]Tab[-] Switch  [white]c[-] Copy  [white]x[-] Move  [white]p[-] Paste  [white]d[-] Delete  [white]R[-] Rename  [white]m[-] Mkdir  [white]s[-] Sort  [white]F5[-] Transfer  [white]Esc[-] Back")
}

// GetLocalPane returns the local file pane.
func (fb *FileBrowser) GetLocalPane() *LocalPane {
	return fb.localPane
}

// GetRemotePane returns the remote file pane.
func (fb *FileBrowser) GetRemotePane() *RemotePane {
	return fb.remotePane
}

// GetServer returns the server this browser is connected to.
func (fb *FileBrowser) GetServer() domain.Server {
	return fb.server
}

// currentPane returns the currently focused pane as a tview.Primitive.
func (fb *FileBrowser) currentPane() tview.Primitive {
	if fb.activePane == 0 {
		return fb.localPane
	}
	return fb.remotePane
}

// initiateTransfer starts a file transfer for the currently selected file(s).
// If multiple files are space-selected, all are transferred sequentially.
// Direction is determined by activePane: 0=upload (local->remote), 1=download (remote->local).
// Creates a cancellable context that is triggered when the user confirms cancellation.
func (fb *FileBrowser) initiateTransfer() {
	if fb.transferring {
		return // already transferring
	}

	// Check remote connection for both directions
	if !fb.remotePane.IsConnected() {
		fb.updateStatusBarTemp("[#FF6B6B]Not connected to remote[-]")
		return
	}

	// Collect files to transfer
	var files []domain.FileInfo
	if fb.activePane == 0 {
		files = fb.localPane.SelectedFiles()
		if len(files) == 0 {
			row, _ := fb.localPane.GetSelection()
			cell := fb.localPane.GetCell(row, 0)
			if cell != nil {
				if fi, ok := cell.GetReference().(domain.FileInfo); ok && !fi.IsDir {
					files = []domain.FileInfo{fi}
				}
			}
		}
	} else {
		files = fb.remotePane.SelectedFiles()
		if len(files) == 0 {
			row, _ := fb.remotePane.GetSelection()
			cell := fb.remotePane.GetCell(row, 0)
			if cell != nil {
				if fi, ok := cell.GetReference().(domain.FileInfo); ok && !fi.IsDir {
					files = []domain.FileInfo{fi}
				}
			}
		}
	}

	if len(files) == 0 {
		return
	}

	fb.transferring = true
	direction := directionUpload
	if fb.activePane == 1 {
		direction = directionDownload
	}

	// Create cancellable context for this transfer
	ctx, cancel := context.WithCancel(context.Background())
	fb.transferCancel = cancel

	// Show modal
	fb.transferModal.SetDismissCallback(func() {
		if fb.transferModal.IsCanceled() {
			if fb.transferCancel != nil {
				fb.transferCancel()
			}
			// Do not close modal — wait for goroutine to complete and show canceled summary
			return
		}
		// Normal dismiss (summary mode: any key closes)
		fb.transferring = false
		fb.app.SetRoot(fb, true)
		fb.app.SetFocus(fb.currentPane())
	})
	fb.transferModal.Show(direction, files[0].Name)

	// Start transfer in goroutine
	go func() {
		var firstErr error
		onConflict := fb.buildConflictHandler(ctx)
		for i, fi := range files {
			var err error
			if fb.activePane == 0 {
				// Upload
				localPath := filepath.Join(fb.localPane.GetCurrentPath(), fi.Name)
				remotePath := joinPath(fb.remotePane.GetCurrentPath(), fi.Name)
				err = fb.transferSvc.UploadFile(ctx, localPath, remotePath, func(p domain.TransferProgress) {
					p.FileIndex = i + 1
					p.FileTotal = len(files)
					fb.app.QueueUpdateDraw(func() {
						fb.transferModal.Update(p)
					})
				}, onConflict)
			} else {
				// Download
				remotePath := joinPath(fb.remotePane.GetCurrentPath(), fi.Name)
				localPath := filepath.Join(fb.localPane.GetCurrentPath(), fi.Name)
				err = fb.transferSvc.DownloadFile(ctx, remotePath, localPath, func(p domain.TransferProgress) {
					p.FileIndex = i + 1
					p.FileTotal = len(files)
					fb.app.QueueUpdateDraw(func() {
						fb.transferModal.Update(p)
					})
				}, onConflict)
			}
			if err != nil && firstErr == nil {
				firstErr = err
				fb.log.Errorw("file transfer failed", "file", fi.Name, "error", err)
			}
			// If context was canceled, stop processing remaining files
			if ctx.Err() != nil {
				break
			}
		}

		fb.app.QueueUpdateDraw(func() {
			if ctx.Err() == context.Canceled {
				// Show canceled summary — user pressed y/Enter/Esc in cancel confirm
				fb.transferModal.ShowCanceledSummary()
			} else if firstErr != nil {
				failedCount := 1
				if len(files) > 1 {
					failedCount = 1
				}
				fb.transferModal.ShowSummary(len(files)-failedCount, failedCount, []string{firstErr.Error()})
			} else {
				fb.transferModal.Hide()
				// Record remote directory path after successful file transfer
				fb.recentDirs.Record(fb.remotePane.GetCurrentPath())
				// Auto-refresh target pane (D-12)
				if fb.activePane == 0 {
					fb.remotePane.Refresh()
				} else {
					fb.localPane.Refresh()
				}
			}
			fb.transferCancel = nil
		})
	}()
}

// initiateDirTransfer starts a recursive directory transfer for the current pane's directory.
// F5 on local pane uploads the current directory to the remote pane's current path.
// F5 on remote pane downloads the current directory to the local pane's current path.
// Creates a cancellable context that is triggered when the user confirms cancellation.
func (fb *FileBrowser) initiateDirTransfer() {
	if fb.transferring {
		return
	}

	// Check remote connection
	if !fb.remotePane.IsConnected() {
		fb.updateStatusBarTemp("[#FF6B6B]Not connected to remote[-]")
		return
	}

	var dirPath string
	var dirName string

	if fb.activePane == 0 {
		dirPath = fb.localPane.GetCurrentPath()
		dirName = filepath.Base(dirPath)
	} else {
		dirPath = fb.remotePane.GetCurrentPath()
		// For remote: extract dir name from path
		parts := strings.Split(dirPath, "/")
		dirName = parts[len(parts)-1]
	}

	if dirPath == "" || dirPath == "/" || dirPath == "~" || dirName == "" || dirName == "/" {
		fb.updateStatusBarTemp("[#FF6B6B]Cannot transfer root directory[-]")
		return
	}

	fb.transferring = true
	direction := directionUpload
	if fb.activePane == 1 {
		direction = directionDownload
	}

	// Create cancellable context for this transfer
	ctx, cancel := context.WithCancel(context.Background())
	fb.transferCancel = cancel

	fb.transferModal.SetDismissCallback(func() {
		if fb.transferModal.IsCanceled() {
			if fb.transferCancel != nil {
				fb.transferCancel()
			}
			// Do not close modal — wait for goroutine to complete and show canceled summary
			return
		}
		// Normal dismiss (summary mode: any key closes)
		fb.transferring = false
		fb.app.SetRoot(fb, true)
		fb.app.SetFocus(fb.currentPane())
	})
	fb.transferModal.Show(direction, dirName)

	go func() {
		var failed []string
		var err error

		onConflict := fb.buildConflictHandler(ctx)
		if fb.activePane == 0 {
			// Upload directory
			remoteBase := joinPath(fb.remotePane.GetCurrentPath(), dirName)
			failed, err = fb.transferSvc.UploadDir(ctx, dirPath, remoteBase, func(p domain.TransferProgress) {
				fb.app.QueueUpdateDraw(func() {
					fb.transferModal.Update(p)
				})
			}, onConflict)
		} else {
			// Download directory
			localBase := filepath.Join(fb.localPane.GetCurrentPath(), dirName)
			failed, err = fb.transferSvc.DownloadDir(ctx, dirPath, localBase, func(p domain.TransferProgress) {
				fb.app.QueueUpdateDraw(func() {
					fb.transferModal.Update(p)
				})
			}, onConflict)
		}

		fb.app.QueueUpdateDraw(func() {
			if ctx.Err() == context.Canceled {
				// Show canceled summary
				fb.transferModal.ShowCanceledSummary()
			} else if err != nil {
				fb.log.Errorw("directory transfer failed", "error", err)
				fb.transferModal.ShowSummary(0, 1, []string{err.Error()})
			} else if len(failed) > 0 {
				fb.transferModal.ShowSummary(0, len(failed), failed)
			} else {
				fb.transferModal.Hide()
				// Record remote directory path after successful directory transfer
				fb.recentDirs.Record(fb.remotePane.GetCurrentPath())
				// Auto-refresh target pane (D-12)
				if fb.activePane == 0 {
					fb.remotePane.Refresh()
				} else {
					fb.localPane.Refresh()
				}
			}
			fb.transferCancel = nil
		})
	}()
}

// updateStatusBarTemp sets a temporary status bar message with keyboard hints.
func (fb *FileBrowser) updateStatusBarTemp(msg string) {
	fb.statusBar.SetText(msg + "  [white]Tab[-] Switch  [white]c[-] Copy  [white]x[-] Move  [white]p[-] Paste  [white]d[-] Delete  [white]R[-] Rename  [white]m[-] Mkdir  [white]s[-] Sort  [white]F5[-] Transfer  [white]Esc[-] Back")
}

// buildConflictHandler creates the onConflict callback for file transfers.
// It uses a buffered channel (capacity 1) for goroutine synchronization:
// the transfer goroutine blocks on <-actionCh while the UI thread handles user input.
// The ctx parameter allows context cancellation to unblock the goroutine if the
// user cancels the transfer while a conflict dialog is showing.
func (fb *FileBrowser) buildConflictHandler(ctx context.Context) domain.ConflictHandler {
	return func(fileName string) (domain.ConflictAction, string) {
		actionCh := make(chan domain.ConflictAction, 1)

		// Gather existing file info for the dialog
		var existingInfo string
		if fb.activePane == 0 {
			// Upload: check remote file info
			if fi, err := fb.sftpService.Stat(joinPath(fb.remotePane.GetCurrentPath(), fileName)); err == nil {
				existingInfo = fmt.Sprintf("%s, %s", formatSize(fi.Size()), fi.ModTime().Format("2006-01-02 15:04"))
			}
		} else {
			// Download: check local file info
			if fi, err := os.Stat(filepath.Join(fb.localPane.GetCurrentPath(), fileName)); err == nil {
				existingInfo = fmt.Sprintf("%s, %s", formatSize(fi.Size()), fi.ModTime().Format("2006-01-02 15:04"))
			}
		}

		// Show conflict dialog on UI thread
		fb.app.QueueUpdateDraw(func() {
			fb.transferModal.ShowConflict(fileName, existingInfo, actionCh)
		})

		// Block until user makes a choice or context is canceled
		var action domain.ConflictAction
		select {
		case action = <-actionCh:
		case <-ctx.Done():
			return domain.ConflictSkip, ""
		}

		switch action {
		case domain.ConflictSkip:
			fb.app.QueueUpdateDraw(func() {
				fb.updateStatusBarTemp(fmt.Sprintf("[#FFA500]Skipped: %s[-]", fileName))
			})
			return action, ""
		case domain.ConflictRename:
			var newPath string
			if fb.activePane == 0 {
				newPath = nextAvailableName(joinPath(fb.remotePane.GetCurrentPath(), fileName), fb.sftpService.Stat)
			} else {
				newPath = nextAvailableName(filepath.Join(fb.localPane.GetCurrentPath(), fileName), os.Stat)
			}
			baseName := filepath.Base(newPath)
			fb.app.QueueUpdateDraw(func() {
				fb.updateStatusBarTemp(fmt.Sprintf("[#FFA500]Renamed to: %s[-]", baseName))
			})
			return action, newPath
		case domain.ConflictOverwrite:
			fb.app.QueueUpdateDraw(func() {
				fb.updateStatusBarTemp(fmt.Sprintf("[#FFA500]Overwrote: %s[-]", fileName))
			})
			return action, ""
		default:
			return action, ""
		}
	}
}

// nextAvailableName finds a non-conflicting file name by appending incremental suffixes.
// Format: {stem}.{counter}{extension} (e.g., file.1.txt, file.2.txt).
// Tries counters 1 through 100. Returns original path if all candidates exist.
func nextAvailableName(path string, statFunc func(string) (os.FileInfo, error)) string {
	ext := filepath.Ext(path)
	name := filepath.Base(path)
	dir := filepath.Dir(path)
	stem := name[:len(name)-len(ext)]

	for i := 1; i <= 100; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s.%d%s", stem, i, ext))
		if _, err := statFunc(candidate); err != nil {
			return candidate
		}
	}
	return path
}

// statusErrorTimer is a package-level timer for status bar error messages.
// Only one error timer is active at a time; a new error cancels the previous timer.
var statusErrorTimer *time.Timer

// showStatusError displays a red error message in the status bar that auto-clears after 3 seconds.
// Used for file operation failures (delete, rename, mkdir) per Pitfall 7.
func (fb *FileBrowser) showStatusError(msg string) {
	if statusErrorTimer != nil {
		statusErrorTimer.Stop()
	}
	fb.statusBar.SetText(fmt.Sprintf("[#FF6B6B]%s[-]", msg))
	statusErrorTimer = time.AfterFunc(3*time.Second, func() {
		fb.app.QueueUpdateDraw(func() {
			fb.setStatusBarDefault()
		})
	})
}

// handleDelete handles the 'd' key: delete selected file(s) or directory.
// For multi-select (Space): shows batch delete confirmation with count and total size.
// For single selection: shows file details (name, size, type, modified time).
// For directories: shows recursive warning in detail line.
// Uses goroutine + QueueUpdateDraw to avoid blocking UI (Pitfall 2).
func (fb *FileBrowser) handleDelete() {
	paneIdx, fs := fb.activePane, fb.getFileService()

	// Remote pane connection check
	if paneIdx == 1 && !fb.remotePane.IsConnected() {
		fb.showStatusError("Not connected to remote")
		return
	}

	// Check for multi-select
	selectedFiles := fb.getSelectedFiles(paneIdx)
	if len(selectedFiles) > 0 {
		fb.handleBatchDelete(paneIdx, fs, selectedFiles)
		return
	}

	// Single selection: get current FileInfo
	row, _ := fb.getActiveSelection()
	cell := fb.getActiveCell(row, 0)
	if cell == nil {
		return
	}
	fi, ok := cell.GetReference().(domain.FileInfo)
	if !ok {
		return
	}

	currentPath := fb.getCurrentPanePath()
	fullPath := fb.buildPath(paneIdx, currentPath, fi.Name)

	// Build confirmation message (D-03: file name, size, type, modified time)
	fileType := "Directory"
	if !fi.IsDir {
		fileType = "File"
	}
	sizeStr := "-"
	if !fi.IsDir {
		sizeStr = formatSize(fi.Size)
	}
	message := fmt.Sprintf("%s  (%s, %s, %s)", fi.Name, sizeStr, fileType, fi.ModTime.Format("2006-01-02 15:04"))

	detail := ""
	if fi.IsDir {
		detail = msgDirNotEmpty
	}

	fb.confirmDialog.SetOnConfirm(func() {
		go func() {
			var err error
			if fi.IsDir {
				err = fs.RemoveAll(fullPath)
			} else {
				err = fs.Remove(fullPath)
			}
			fb.app.QueueUpdateDraw(func() {
				if err != nil {
					fb.showStatusError(fmt.Sprintf("Delete failed: %s", trimError(err.Error(), 50)))
					return
				}
				fb.refreshAndReposition(paneIdx, row)
			})
		}()
	})

	fb.confirmDialog.SetOnCancel(nil)
	fb.confirmDialog.Show("Delete?", message, detail)
}

// handleBatchDelete handles deletion of multiple space-selected files.
// Shows count and total size summary, then deletes sequentially in a goroutine.
func (fb *FileBrowser) handleBatchDelete(paneIdx int, fs ports.FileService, files []domain.FileInfo) {
	currentPath := fb.getCurrentPanePath()

	// Calculate total size (skip directories, per Research open question 3)
	var totalSize int64
	for _, fi := range files {
		if !fi.IsDir {
			totalSize += fi.Size
		}
	}

	message := fmt.Sprintf("Delete %d items? Total size: %s", len(files), formatSize(totalSize))

	fb.confirmDialog.SetOnConfirm(func() {
		go func() {
			var firstErr error
			for _, fi := range files {
				fullPath := fb.buildPath(paneIdx, currentPath, fi.Name)
				var err error
				if fi.IsDir {
					err = fs.RemoveAll(fullPath)
				} else {
					err = fs.Remove(fullPath)
				}
				if err != nil && firstErr == nil {
					firstErr = err
					fb.log.Errorw("batch delete failed", "file", fi.Name, "error", err)
				}
			}
			fb.app.QueueUpdateDraw(func() {
				fb.refreshPane(paneIdx)
				if firstErr != nil {
					fb.showStatusError(fmt.Sprintf("Delete failed: %s", trimError(firstErr.Error(), 50)))
				}
			})
		}()
	})

	fb.confirmDialog.SetOnCancel(nil)
	fb.confirmDialog.Show("Delete Multiple?", message, "")
}

// handleRename handles the 'R' key: rename selected file/directory.
// Shows InputDialog pre-filled with current name. Checks for name conflicts
// and prompts with ConfirmDialog if target already exists (REN-02).
func (fb *FileBrowser) handleRename() {
	paneIdx, fs := fb.activePane, fb.getFileService()

	// Remote pane connection check
	if paneIdx == 1 && !fb.remotePane.IsConnected() {
		fb.showStatusError("Not connected to remote")
		return
	}

	row, _ := fb.getActiveSelection()
	cell := fb.getActiveCell(row, 0)
	if cell == nil {
		return
	}
	fi, ok := cell.GetReference().(domain.FileInfo)
	if !ok {
		return
	}

	currentPath := fb.getCurrentPanePath()
	oldFullPath := fb.buildPath(paneIdx, currentPath, fi.Name)

	fb.inputDialog.SetOnSubmit(func(newName string) {
		// Empty check (defensive, InputDialog already guards)
		if newName == "" {
			return
		}
		// No change check
		if newName == fi.Name {
			return
		}

		newFullPath := fb.buildPath(paneIdx, currentPath, newName)

		// Check for name conflict (REN-02)
		if _, err := fs.Stat(newFullPath); err == nil {
			// Target exists -- show confirm dialog for overwrite
			fb.confirmDialog.SetOnConfirm(func() {
				go func() {
					err := fs.Rename(oldFullPath, newFullPath)
					fb.app.QueueUpdateDraw(func() {
						if err != nil {
							fb.showStatusError(fmt.Sprintf("Rename failed: %s", trimError(err.Error(), 50)))
							return
						}
						fb.refreshPane(paneIdx)
						fb.focusOnItem(paneIdx, newName)
					})
				}()
			})
			fb.confirmDialog.SetOnCancel(nil)
			fb.confirmDialog.Show("Name Conflict", fmt.Sprintf("'%s' already exists. Overwrite?", newName), "")
			return
		}

		// No conflict -- rename directly
		go func() {
			err := fs.Rename(oldFullPath, newFullPath)
			fb.app.QueueUpdateDraw(func() {
				if err != nil {
					fb.showStatusError(fmt.Sprintf("Rename failed: %s", trimError(err.Error(), 50)))
					return
				}
				fb.refreshPane(paneIdx)
				fb.focusOnItem(paneIdx, newName)
			})
		}()
	})

	fb.inputDialog.SetOnCancel(nil)
	fb.inputDialog.Show("Rename", "New name: ", fi.Name)
}

// handleMkdir handles the 'm' key: create new directory in current path.
// Shows InputDialog with empty input. After creation, positions cursor on new directory (MKD-02).
func (fb *FileBrowser) handleMkdir() {
	paneIdx, fs := fb.activePane, fb.getFileService()

	// Remote pane connection check
	if paneIdx == 1 && !fb.remotePane.IsConnected() {
		fb.showStatusError("Not connected to remote")
		return
	}

	currentPath := fb.getCurrentPanePath()

	fb.inputDialog.SetOnSubmit(func(dirName string) {
		if dirName == "" {
			return
		}

		fullPath := fb.buildPath(paneIdx, currentPath, dirName)

		go func() {
			err := fs.Mkdir(fullPath)
			fb.app.QueueUpdateDraw(func() {
				if err != nil {
					fb.showStatusError(fmt.Sprintf("Mkdir failed: %s", trimError(err.Error(), 50)))
					return
				}
				fb.refreshPane(paneIdx)
				fb.focusOnItem(paneIdx, dirName)
			})
		}()
	})

	fb.inputDialog.SetOnCancel(nil)
	fb.inputDialog.Show("New Directory", "Directory name: ", "")
}

// handleCopy handles the 'c' key: mark current file as copy source (CPY-01, D-03).
// Sets clipboard state on FileBrowser. Replaces any existing clipboard entry (D-05).
func (fb *FileBrowser) handleCopy() {
	// Get current selection
	row, _ := fb.getActiveSelection()
	cell := fb.getActiveCell(row, 0)
	if cell == nil {
		return
	}
	fi, ok := cell.GetReference().(domain.FileInfo)
	if !ok {
		return
	}

	// Remote pane connection check
	if fb.activePane == 1 && !fb.remotePane.IsConnected() {
		fb.showStatusError("Not connected to remote")
		return
	}

	// Set clipboard state (D-03: single file, D-04: store source info)
	fb.clipboard = Clipboard{
		Active:     true,
		SourcePane: fb.activePane,
		FileInfo:   fi,
		SourceDir:  fb.getCurrentPanePath(),
		Operation:  OpCopy,
	}

	// Refresh to show [C] prefix, then restore cursor position
	fb.refreshPane(fb.activePane)
	fb.focusOnItem(fb.activePane, fi.Name)
	fb.updateStatusBarTemp(fmt.Sprintf("[#00FF7F]Clipboard: %s[-]", fi.Name))
}

// handleMove handles the 'x' key: mark current file as move source (MOV-01, D-06).
// Mirrors handleCopy exactly but uses OpMove and red color for status feedback.
func (fb *FileBrowser) handleMove() {
	// Get current selection
	row, _ := fb.getActiveSelection()
	cell := fb.getActiveCell(row, 0)
	if cell == nil {
		return
	}
	fi, ok := cell.GetReference().(domain.FileInfo)
	if !ok {
		return
	}

	// Remote pane connection check
	if fb.activePane == 1 && !fb.remotePane.IsConnected() {
		fb.showStatusError("Not connected to remote")
		return
	}

	// Set clipboard state with OpMove (D-06)
	fb.clipboard = Clipboard{
		Active:     true,
		SourcePane: fb.activePane,
		FileInfo:   fi,
		SourceDir:  fb.getCurrentPanePath(),
		Operation:  OpMove, // D-06
	}

	// Refresh to show [M] prefix, then restore cursor position
	fb.refreshPane(fb.activePane)
	fb.focusOnItem(fb.activePane, fi.Name)
	fb.updateStatusBarTemp(fmt.Sprintf("[#FF6B6B]Move: %s[-]", fi.Name)) // D-06, UI-SPEC red
}

// handlePaste handles the 'p' key: paste copied/moved file to current directory (CPY-02, MOV-02).
// All logic runs in a goroutine for buildConflictHandler channel sync (D-09).
// Dispatches by clipboard.Operation: OpCopy -> existing paste handlers, OpMove -> move handlers.
// Conflict dialog shown for ALL paste operations when target exists (D-01).
func (fb *FileBrowser) handlePaste() {
	// Guard: empty clipboard (D-07: silent)
	if !fb.clipboard.Active {
		return
	}
	// Guard: cross-pane paste not supported (v1.3+)
	if fb.clipboard.SourcePane != fb.activePane {
		fb.showStatusError("Cross-pane paste not supported (v1.3+)")
		return
	}
	// Remote pane connection check
	if fb.activePane == 1 && !fb.remotePane.IsConnected() {
		fb.showStatusError("Not connected to remote")
		return
	}

	currentPath := fb.getCurrentPanePath()
	sourcePath := fb.buildPath(fb.clipboard.SourcePane, fb.clipboard.SourceDir, fb.clipboard.FileInfo.Name)
	targetName := fb.clipboard.FileInfo.Name
	targetPath := fb.buildPath(fb.activePane, currentPath, targetName)

	// All logic in goroutine for buildConflictHandler channel sync (D-09)
	go func() {
		// Conflict check for ALL paste operations (D-01)
		var statFunc func(string) (os.FileInfo, error)
		if fb.activePane == 0 {
			statFunc = fb.fileService.Stat
		} else {
			statFunc = fb.sftpService.Stat
		}
		if _, err := statFunc(targetPath); err == nil {
			// Target exists -- show conflict dialog
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			onConflict := fb.buildConflictHandler(ctx)
			action, newPath := onConflict(targetName)
			switch action {
			case domain.ConflictSkip:
				return // user chose to skip
			case domain.ConflictRename:
				targetName = filepath.Base(newPath)
				targetPath = fb.buildPath(fb.activePane, currentPath, targetName)
			case domain.ConflictOverwrite:
				// continue with original targetPath
			}
		}

		// Operation dispatch (D-07)
		if fb.clipboard.Operation == OpMove {
			if currentPath == fb.clipboard.SourceDir {
				// Same-directory move optimization: use Rename (atomic) per RESEARCH Pitfall 2
				fb.handleSameDirMove(sourcePath, targetPath, targetName)
			} else if fb.activePane == 0 {
				fb.handleLocalMove(sourcePath, targetPath, targetName)
			} else {
				fb.handleRemoteMove(sourcePath, targetPath, targetName)
			}
		} else {
			// OpCopy -- existing behavior
			if fb.activePane == 0 {
				fb.handleLocalPaste(sourcePath, targetPath, targetName)
			} else {
				fb.handleRemotePaste(sourcePath, targetPath, targetName)
			}
		}
	}()
}

// handleLocalPaste performs a local file copy (CPY-02).
// Called from handlePaste's goroutine, so no additional goroutine needed.
func (fb *FileBrowser) handleLocalPaste(sourcePath, targetPath, targetName string) {
	var err error
	if fb.clipboard.FileInfo.IsDir {
		err = fb.fileService.CopyDir(sourcePath, targetPath)
	} else {
		err = fb.fileService.Copy(sourcePath, targetPath)
	}
	fb.app.QueueUpdateDraw(func() {
		if err != nil {
			fb.showStatusError(fmt.Sprintf("Copy failed: %s", trimError(err.Error(), 50)))
			return // D-07: do NOT clear clipboard on failure
		}
		fb.clipboard = Clipboard{} // D-07: clear on success
		fb.refreshPane(fb.activePane)
		fb.focusOnItem(fb.activePane, targetName)
		fb.updateStatusBarTemp(fmt.Sprintf("[#00FF7F]Copied: %s[-]", targetName))
	})
}

// handleSameDirMove moves a file within the same directory using Rename (atomic operation).
// This is more efficient than Copy+Delete and avoids partial failure states (RESEARCH Pitfall 2).
func (fb *FileBrowser) handleSameDirMove(sourcePath, targetPath, targetName string) {
	var err error
	if fb.activePane == 0 {
		err = fb.fileService.Rename(sourcePath, targetPath)
	} else {
		err = fb.sftpService.Rename(sourcePath, targetPath)
	}
	fb.app.QueueUpdateDraw(func() {
		if err != nil {
			fb.showStatusError(fmt.Sprintf("Move failed: %s", trimError(err.Error(), 50)))
			return // D-07: do NOT clear clipboard on failure
		}
		fb.clipboard = Clipboard{} // D-07: clear on success
		fb.refreshPane(fb.activePane)
		fb.focusOnItem(fb.activePane, targetName)
		fb.updateStatusBarTemp(fmt.Sprintf("[#FF6B6B]Moved: %s[-]", targetName))
	})
}

// handleLocalMove moves a file across directories via Copy + Delete source (D-03).
// On delete failure: attempts cleanup of target copy (D-04).
func (fb *FileBrowser) handleLocalMove(sourcePath, targetPath, targetName string) {
	var err error
	if fb.clipboard.FileInfo.IsDir {
		err = fb.fileService.CopyDir(sourcePath, targetPath)
	} else {
		err = fb.fileService.Copy(sourcePath, targetPath)
	}
	if err != nil {
		fb.app.QueueUpdateDraw(func() {
			fb.showStatusError(fmt.Sprintf("Move failed: %s", trimError(err.Error(), 50)))
			// D-04: source preserved on copy failure, clipboard NOT cleared
		})
		return
	}
	// Copy succeeded -- delete source
	if fb.clipboard.FileInfo.IsDir {
		err = fb.fileService.RemoveAll(sourcePath)
	} else {
		err = fb.fileService.Remove(sourcePath)
	}
	fb.app.QueueUpdateDraw(func() {
		if err != nil {
			// D-04: copy succeeded but delete failed -- attempt cleanup target
			fb.log.Errorw("move: failed to delete source, attempting cleanup",
				"source", sourcePath, "target", targetPath, "error", err)
			var cleanupErr error
			if fb.clipboard.FileInfo.IsDir {
				cleanupErr = fb.fileService.RemoveAll(targetPath)
			} else {
				cleanupErr = fb.fileService.Remove(targetPath)
			}
			if cleanupErr != nil {
				fb.log.Warnw("move: cleanup also failed", "target", targetPath, "error", cleanupErr)
				fb.showStatusError("Move failed: source kept, target copy may need manual cleanup")
			} else {
				fb.showStatusError("Move failed: source kept, target cleaned up")
			}
			return // D-07: do NOT clear clipboard on failure
		}
		fb.clipboard = Clipboard{} // D-07: clear on success
		fb.refreshPane(fb.activePane)
		fb.focusOnItem(fb.activePane, targetName)
		fb.updateStatusBarTemp(fmt.Sprintf("[#FF6B6B]Moved: %s[-]", targetName))
	})
}

// handleRemoteMove performs a remote file move via CopyRemoteFile/CopyRemoteDir + SFTP Remove (D-03, D-04).
// Phase 1: Copy (modeMove progress). Phase 2: Delete source ("Deleting source..." text).
// On Phase 2 failure: attempt cleanup of target copy (D-04).
func (fb *FileBrowser) handleRemoteMove(sourcePath, targetPath, targetName string) {
	fb.transferring = true
	ctx, cancel := context.WithCancel(context.Background())
	fb.transferCancel = cancel

	fb.transferModal.SetDismissCallback(func() {
		if fb.transferModal.IsCanceled() {
			if fb.transferCancel != nil {
				fb.transferCancel()
			}
			return
		}
		fb.transferring = false
		fb.app.SetRoot(fb, true)
		fb.app.SetFocus(fb.currentPane())
	})
	fb.transferModal.ShowMove(fb.clipboard.FileInfo.Name)

	go func() {
		// Phase 1: Copy remote source to target
		onConflict := fb.buildConflictHandler(ctx)
		var copyErr error

		if fb.clipboard.FileInfo.IsDir {
			moveProgress := func(p domain.TransferProgress) {
				label := p.FileName
				if p.Done {
					label = "Uploading: " + label
				} else {
					label = "Downloading: " + label
				}
				fb.app.QueueUpdateDraw(func() {
					fb.transferModal.fileLabel = label
					fb.transferModal.Update(p)
				})
			}
			_, copyErr = fb.transferSvc.CopyRemoteDir(ctx, sourcePath, targetPath, moveProgress, onConflict)
		} else {
			moveProgress := func(p domain.TransferProgress) {
				label := p.FileName
				if p.Done {
					label = "Uploading: " + label
				} else {
					label = "Downloading: " + label
				}
				fb.app.QueueUpdateDraw(func() {
					fb.transferModal.fileLabel = label
					fb.transferModal.Update(p)
				})
			}
			copyErr = fb.transferSvc.CopyRemoteFile(ctx, sourcePath, targetPath, moveProgress, onConflict)
		}

		if copyErr != nil {
			// Copy failed -- source preserved (D-04: MOV-03 satisfied)
			fb.app.QueueUpdateDraw(func() {
				fb.showStatusError(fmt.Sprintf("Move failed: %s", trimError(copyErr.Error(), 50)))
				fb.transferring = false
				fb.transferModal.Hide()
				// D-07: do NOT clear clipboard on failure
			})
			return
		}

		// Phase 2: Delete source (D-08: show "Deleting source...")
		fb.app.QueueUpdateDraw(func() {
			fb.transferModal.fileLabel = "Deleting source..."
			fb.transferModal.infoLine = ""
			fb.transferModal.etaLine = ""
		})

		var removeErr error
		if fb.clipboard.FileInfo.IsDir {
			removeErr = fb.sftpService.RemoveAll(sourcePath)
		} else {
			removeErr = fb.sftpService.Remove(sourcePath)
		}

		fb.app.QueueUpdateDraw(func() {
			fb.transferCancel = nil
			if removeErr != nil {
				// D-04: Copy succeeded but delete failed -- try cleanup target
				fb.log.Errorw("move: failed to delete source, attempting cleanup",
					"source", sourcePath, "error", removeErr)
				var cleanupErr error
				if fb.clipboard.FileInfo.IsDir {
					cleanupErr = fb.sftpService.RemoveAll(targetPath)
				} else {
					cleanupErr = fb.sftpService.Remove(targetPath)
				}
				if cleanupErr != nil {
					fb.log.Warnw("move: cleanup also failed", "target", targetPath, "error", cleanupErr)
					fb.showStatusError("Move failed: source kept, target copy may need manual cleanup")
				} else {
					fb.showStatusError("Move failed: source kept, target cleaned up")
				}
				fb.transferring = false
				fb.transferModal.Hide()
				return // D-07: do NOT clear clipboard on failure
			}
			// Success
			fb.transferModal.Hide()
			fb.transferring = false
			fb.clipboard = Clipboard{} // D-07: clear on success
			fb.refreshPane(fb.activePane)
			fb.focusOnItem(fb.activePane, targetName)
			fb.updateStatusBarTemp(fmt.Sprintf("[#FF6B6B]Moved: %s[-]", targetName))
		})
	}()
}

// handleRemotePaste performs a remote file copy via TransferModal (CPY-02, D-08).
func (fb *FileBrowser) handleRemotePaste(sourcePath, targetPath, targetName string) {
	fb.transferring = true
	ctx, cancel := context.WithCancel(context.Background())
	fb.transferCancel = cancel

	fb.transferModal.SetDismissCallback(func() {
		if fb.transferModal.IsCanceled() {
			if fb.transferCancel != nil {
				fb.transferCancel()
			}
			return
		}
		fb.transferring = false
		fb.app.SetRoot(fb, true)
		fb.app.SetFocus(fb.currentPane())
	})
	fb.transferModal.ShowCopy(fb.clipboard.FileInfo.Name)

	go func() {
		var err error
		onConflict := fb.buildConflictHandler(ctx)

		if fb.clipboard.FileInfo.IsDir {
			err = fb.remotePasteDir(ctx, sourcePath, targetPath, onConflict)
		} else {
			err = fb.remotePasteFile(ctx, sourcePath, targetPath, targetName, onConflict)
		}

		fb.app.QueueUpdateDraw(func() {
			fb.transferCancel = nil
			if ctx.Err() == context.Canceled {
				fb.transferModal.ShowCanceledSummary()
				return
			}
			if err != nil {
				fb.showStatusError(fmt.Sprintf("Copy failed: %s", trimError(err.Error(), 50)))
				fb.transferring = false
				fb.transferModal.Hide()
				return // D-05: do NOT clear clipboard on failure
			}
			fb.transferModal.Hide()
			fb.transferring = false
			fb.clipboard = Clipboard{} // D-05: clear on success
			fb.refreshPane(fb.activePane)
			fb.focusOnItem(fb.activePane, targetName)
			fb.updateStatusBarTemp(fmt.Sprintf("[#00FF7F]Copied: %s[-]", targetName))
		})
	}()
}

// remotePasteDir copies a remote directory by downloading to temp and re-uploading (D-01).
func (fb *FileBrowser) remotePasteDir(ctx context.Context, sourcePath, targetPath string, onConflict domain.ConflictHandler) error {
	dlProgress := func(p domain.TransferProgress) {
		if p.FileName != "" {
			p.FileName = "Downloading: " + p.FileName
		}
		fb.app.QueueUpdateDraw(func() {
			fb.transferModal.fileLabel = p.FileName
			fb.transferModal.Update(p)
		})
	}
	tmpDir, tmpErr := os.MkdirTemp("", "lazyssh-copydir-*")
	if tmpErr != nil {
		fb.app.QueueUpdateDraw(func() {
			fb.showStatusError(fmt.Sprintf("Copy failed: %s", trimError(tmpErr.Error(), 50)))
			fb.transferring = false
			fb.transferModal.Hide()
		})
		return tmpErr
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()
	srcBase := filepath.Base(sourcePath)
	tmpBase := filepath.Join(tmpDir, srcBase)
	dlFailed, dlErr := fb.transferSvc.DownloadDir(ctx, sourcePath, tmpBase, dlProgress, nil)
	if dlErr != nil && ctx.Err() == nil {
		fb.app.QueueUpdateDraw(func() {
			fb.showStatusError(fmt.Sprintf("Copy failed: %s", trimError(dlErr.Error(), 50)))
			fb.transferring = false
			fb.transferModal.Hide()
		})
		return dlErr
	}

	// Reset progress bar and speed samples for upload phase
	fb.app.QueueUpdateDraw(func() {
		fb.transferModal.ResetProgress()
	})

	ulProgress := func(p domain.TransferProgress) {
		if p.FileName != "" {
			p.FileName = "Uploading: " + p.FileName
		}
		fb.app.QueueUpdateDraw(func() {
			fb.transferModal.fileLabel = p.FileName
			fb.transferModal.Update(p)
		})
	}
	ulFailed, err := fb.transferSvc.UploadDir(ctx, tmpBase, targetPath, ulProgress, onConflict)

	if err == nil && len(dlFailed) == 0 && len(ulFailed) == 0 {
		return nil
	}
	if err == nil {
		allFailed := make([]string, len(dlFailed)+len(ulFailed))
		copy(allFailed, dlFailed)
		copy(allFailed[len(dlFailed):], ulFailed)
		return fmt.Errorf("%d files failed", len(allFailed))
	}
	return err
}

// remotePasteFile copies a single remote file using CopyRemoteFile (Plan 01).
func (fb *FileBrowser) remotePasteFile(ctx context.Context, sourcePath, targetPath, targetName string, onConflict domain.ConflictHandler) error {
	var dlDone bool
	combinedProgress := func(p domain.TransferProgress) {
		if p.Done && !dlDone {
			dlDone = true
			fb.app.QueueUpdateDraw(func() {
				fb.transferModal.ResetProgress()
				fb.transferModal.fileLabel = fmt.Sprintf("Uploading: %s", targetName)
			})
			return
		}
		label := fmt.Sprintf("Downloading: %s", fb.clipboard.FileInfo.Name)
		fb.app.QueueUpdateDraw(func() {
			fb.transferModal.fileLabel = label
			fb.transferModal.Update(p)
		})
	}
	return fb.transferSvc.CopyRemoteFile(ctx, sourcePath, targetPath, combinedProgress, onConflict)
}

// getFileService returns the appropriate FileService for the active pane.// getFileService returns the appropriate FileService for the active pane.
func (fb *FileBrowser) getFileService() ports.FileService {
	if fb.activePane == 0 {
		return fb.fileService
	}
	return fb.sftpService
}

// getActiveSelection returns the row and column of the current selection in the active pane.
func (fb *FileBrowser) getActiveSelection() (int, int) {
	if fb.activePane == 0 {
		return fb.localPane.GetSelection()
	}
	return fb.remotePane.GetSelection()
}

// getActiveCell returns the TableCell at the given row and column in the active pane.
//
//nolint:unparam // col is always 0 but kept for table cell API consistency
func (fb *FileBrowser) getActiveCell(row, col int) *tview.TableCell {
	if fb.activePane == 0 {
		return fb.localPane.GetCell(row, col)
	}
	return fb.remotePane.GetCell(row, col)
}

// getCurrentPanePath returns the current directory path of the active pane.
func (fb *FileBrowser) getCurrentPanePath() string {
	if fb.activePane == 0 {
		return fb.localPane.GetCurrentPath()
	}
	return fb.remotePane.GetCurrentPath()
}

// getSelectedFiles returns all space-selected files in the given pane.
func (fb *FileBrowser) getSelectedFiles(paneIdx int) []domain.FileInfo {
	if paneIdx == 0 {
		return fb.localPane.SelectedFiles()
	}
	return fb.remotePane.SelectedFiles()
}

// buildPath constructs a full path for the given pane index.
func (fb *FileBrowser) buildPath(paneIdx int, base, name string) string {
	if paneIdx == 0 {
		return filepath.Join(base, name)
	}
	return joinPath(base, name)
}

// refreshPane refreshes the file listing in the given pane.
func (fb *FileBrowser) refreshPane(paneIdx int) {
	if paneIdx == 0 {
		fb.localPane.Refresh()
	} else {
		fb.remotePane.Refresh()
	}
}

// refreshAndReposition refreshes the listing and positions the cursor at the given row.
// Clamps the row to the valid range [1, totalRows-1] (DEL-04).
func (fb *FileBrowser) refreshAndReposition(paneIdx int, deletedRow int) {
	fb.refreshPane(paneIdx)
	// After refresh, clamp the selection to a valid row
	targetRow := deletedRow
	if targetRow < 1 {
		targetRow = 1
	}
	if paneIdx == 0 {
		rowCount := fb.localPane.GetRowCount()
		if targetRow >= rowCount {
			targetRow = rowCount - 1
		}
		if targetRow < 1 {
			targetRow = 1
		}
		fb.localPane.Select(targetRow, 0)
	} else {
		rowCount := fb.remotePane.GetRowCount()
		if targetRow >= rowCount {
			targetRow = rowCount - 1
		}
		if targetRow < 1 {
			targetRow = 1
		}
		fb.remotePane.Select(targetRow, 0)
	}
}

// focusOnItem finds a file by name in the given pane and selects it.
// Used after rename and mkdir to position cursor on the new/renamed item (MKD-02).
func (fb *FileBrowser) focusOnItem(paneIdx int, name string) {
	var table *tview.Table
	if paneIdx == 0 {
		table = fb.localPane.Table
	} else {
		table = fb.remotePane.Table
	}

	rows := table.GetRowCount()
	for row := 1; row < rows; row++ {
		cell := table.GetCell(row, 0)
		if cell == nil {
			continue
		}
		ref := cell.GetReference()
		if ref == nil {
			continue
		}
		fi, ok := ref.(domain.FileInfo)
		if ok && fi.Name == name {
			table.Select(row, 0)
			return
		}
	}
}
