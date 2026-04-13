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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"github.com/Adembc/lazyssh/internal/core/ports"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"go.uber.org/zap"
)

// FileBrowser is the root component for the dual-pane file browser.
// It is a self-contained tview.Primitive that can be set as root via app.SetRoot().
// Layout: FlexRow with content (FlexColumn: LocalPane + RemotePane) and StatusBar.
type FileBrowser struct {
	*tview.Flex
	app           *tview.Application
	log           *zap.SugaredLogger
	fileService   ports.FileService
	sftpService   ports.SFTPService
	transferSvc   ports.TransferService
	server        domain.Server
	localPane     *LocalPane
	remotePane    *RemotePane
	statusBar     *tview.TextView
	transferModal *TransferModal
	activePane    int // 0 = local, 1 = remote
	transferring  bool
	onClose       func()
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
	// Determine initial local path (D-10: home directory)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "/"
		fb.log.Warnw("failed to get home directory, using /", "error", err)
	}

	// Create panes
	fb.localPane = NewLocalPane(fb.log, fb.fileService, homeDir)
	fb.remotePane = NewRemotePane(fb.log, fb.sftpService, fb.server)

	// Create transfer modal
	fb.transferModal = NewTransferModal(fb.app)
	fb.transferModal.SetDismissCallback(func() {
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

	// Create status bar
	fb.statusBar = tview.NewTextView()
	fb.statusBar.SetDynamicColors(true)
	fb.statusBar.SetBackgroundColor(tcell.Color235)
	fb.statusBar.SetTextAlign(tview.AlignCenter)
	fb.setStatusBarDefault()

	// Build dual-pane content layout (50:50 per D-04)
	content := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(fb.localPane, 0, 1, true).   // 50% width, initially focused
		AddItem(fb.remotePane, 0, 1, false)  // 50% width

	// Build root layout: content + status bar
	fb.SetDirection(tview.FlexRow).
		AddItem(content, 0, 1, true).      // content area, takes all space
		AddItem(fb.statusBar, 1, 0, false) // 1 row status bar

	// Set initial focus state
	fb.activePane = 0
	fb.localPane.SetFocused(true)

	// Global input capture for Tab, Esc, s, S
	fb.SetInputCapture(fb.handleGlobalKeys)

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

// setStatusBarDefault sets the default status bar text with keyboard hints.
func (fb *FileBrowser) setStatusBarDefault() {
	fb.statusBar.SetText("[white]Tab[-] Switch pane  [white]h[-] Up  [white].[-] Hidden  [white]s[-] Sort  [white]Esc[-] Back")
}

// updateStatusBarConnection prepends connection status to the status bar text.
func (fb *FileBrowser) updateStatusBarConnection(msg string) {
	fb.statusBar.SetText(msg + "  [white]Tab[-] Switch pane  [white]h[-] Up  [white].[-] Hidden  [white]s[-] Sort  [white]Esc[-] Back")
}

// updateStatusBarSelection prepends selection count to the status bar text.
func (fb *FileBrowser) updateStatusBarSelection(count int) {
	if count > 0 {
		fb.statusBar.SetText(fmt.Sprintf("[#FFD700]%d files selected[-]  [white]Tab[-] Switch pane  [white]h[-] Up  [white].[-] Hidden  [white]s[-] Sort  [white]Esc[-] Back", count))
	} else {
		fb.setStatusBarDefault()
	}
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
	direction := "Uploading"
	if fb.activePane == 1 {
		direction = "Downloading"
	}

	// Show modal
	fb.transferModal.SetDismissCallback(func() {
		fb.transferring = false
		fb.app.SetRoot(fb, true)
		fb.app.SetFocus(fb.currentPane())
	})
	fb.transferModal.Show(direction, files[0].Name)

	// Start transfer in goroutine
	go func() {
		var firstErr error
		for i, fi := range files {
			var err error
			if fb.activePane == 0 {
				// Upload
				localPath := filepath.Join(fb.localPane.GetCurrentPath(), fi.Name)
				remotePath := joinPath(fb.remotePane.GetCurrentPath(), fi.Name)
				err = fb.transferSvc.UploadFile(localPath, remotePath, func(p domain.TransferProgress) {
					p.FileIndex = i + 1
					p.FileTotal = len(files)
					fb.app.QueueUpdateDraw(func() {
						fb.transferModal.Update(p)
					})
				})
			} else {
				// Download
				remotePath := joinPath(fb.remotePane.GetCurrentPath(), fi.Name)
				localPath := filepath.Join(fb.localPane.GetCurrentPath(), fi.Name)
				err = fb.transferSvc.DownloadFile(remotePath, localPath, func(p domain.TransferProgress) {
					p.FileIndex = i + 1
					p.FileTotal = len(files)
					fb.app.QueueUpdateDraw(func() {
						fb.transferModal.Update(p)
					})
				})
			}
			if err != nil && firstErr == nil {
				firstErr = err
				fb.log.Errorw("file transfer failed", "file", fi.Name, "error", err)
			}
		}

		fb.app.QueueUpdateDraw(func() {
			if firstErr != nil {
				failedCount := 1
				if len(files) > 1 {
					failedCount = 1
				}
				fb.transferModal.ShowSummary(len(files)-failedCount, failedCount, []string{firstErr.Error()})
			} else {
				fb.transferModal.Hide()
				// Auto-refresh target pane (D-12)
				if fb.activePane == 0 {
					fb.remotePane.Refresh()
				} else {
					fb.localPane.Refresh()
				}
			}
		})
	}()
}

// initiateDirTransfer starts a recursive directory transfer for the current pane's directory.
// F5 on local pane uploads the current directory to the remote pane's current path.
// F5 on remote pane downloads the current directory to the local pane's current path.
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
	direction := "Uploading"
	if fb.activePane == 1 {
		direction = "Downloading"
	}

	fb.transferModal.SetDismissCallback(func() {
		fb.transferring = false
		fb.app.SetRoot(fb, true)
		fb.app.SetFocus(fb.currentPane())
	})
	fb.transferModal.Show(direction, dirName)

	go func() {
		var failed []string
		var err error

		if fb.activePane == 0 {
			// Upload directory
			remoteBase := joinPath(fb.remotePane.GetCurrentPath(), dirName)
			failed, err = fb.transferSvc.UploadDir(dirPath, remoteBase, func(p domain.TransferProgress) {
				fb.app.QueueUpdateDraw(func() {
					fb.transferModal.Update(p)
				})
			})
		} else {
			// Download directory
			localBase := filepath.Join(fb.localPane.GetCurrentPath(), dirName)
			failed, err = fb.transferSvc.DownloadDir(dirPath, localBase, func(p domain.TransferProgress) {
				fb.app.QueueUpdateDraw(func() {
					fb.transferModal.Update(p)
				})
			})
		}

		fb.app.QueueUpdateDraw(func() {
			if err != nil {
				fb.log.Errorw("directory transfer failed", "error", err)
				fb.transferModal.ShowSummary(0, 1, []string{err.Error()})
			} else if len(failed) > 0 {
				fb.transferModal.ShowSummary(0, len(failed), failed)
			} else {
				fb.transferModal.Hide()
				// Auto-refresh target pane (D-12)
				if fb.activePane == 0 {
					fb.remotePane.Refresh()
				} else {
					fb.localPane.Refresh()
				}
			}
		})
	}()
}

// updateStatusBarTemp sets a temporary status bar message with keyboard hints.
func (fb *FileBrowser) updateStatusBarTemp(msg string) {
	fb.statusBar.SetText(msg + "  [white]Tab[-] Switch pane  [white]h[-] Up  [white].[-] Hidden  [white]s[-] Sort  [white]Esc[-] Back")
}
