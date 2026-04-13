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
	app         *tview.Application
	log         *zap.SugaredLogger
	fileService ports.FileService
	sftpService ports.SFTPService
	server      domain.Server
	localPane   *LocalPane
	remotePane  *RemotePane
	statusBar   *tview.TextView
	activePane  int // 0 = local, 1 = remote
	onClose     func()
}

// NewFileBrowser creates a new FileBrowser with dual-pane layout.
func NewFileBrowser(
	app *tview.Application,
	log *zap.SugaredLogger,
	fs ports.FileService,
	sftp ports.SFTPService,
	server domain.Server,
	onClose func(),
) *FileBrowser {
	fb := &FileBrowser{
		Flex:        tview.NewFlex(),
		app:         app,
		log:         log,
		fileService: fs,
		sftpService: sftp,
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
