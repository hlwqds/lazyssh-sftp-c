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
	"time"

	"github.com/Adembc/lazyssh/internal/adapters/data/sftp_client"
	"github.com/Adembc/lazyssh/internal/adapters/data/transfer"
	"github.com/Adembc/lazyssh/internal/core/domain"
	"github.com/Adembc/lazyssh/internal/core/ports"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"go.uber.org/zap"
)

// DualRemoteFileBrowser is a standalone component for browsing two remote servers
// simultaneously. It provides the foundation for Phase 13 cross-remote transfers.
//
// Key design decisions (from CONTEXT.md):
//   - D-01: Independent component, does NOT reuse FileBrowser (avoids 15+ activePane assumptions)
//   - D-02: Two independent sftp_client.New() instances (not tui.sftpService, per Pitfall 6)
//   - D-03: Tab switches focus between left (source) and right (target) panes
//   - D-04: 50:50 FlexColumn layout matching FileBrowser pattern
//   - D-05: Own ConfirmDialog and InputDialog instances (Pitfall 3)
//   - D-07: Parallel goroutine connection for both SFTP instances
//   - D-08: Graceful partial failure (one pane shows error, other remains usable)
//   - D-09: Status bar shows connection states + active panel indicator + key hints
//   - D-10: No clipboard (c/x/p) support -- deferred to Phase 13
//   - D-11: Esc closes browser, cleans up AfterDrawFunc, closes both SFTP connections
type DualRemoteFileBrowser struct {
	*tview.Flex
	app           *tview.Application
	log           *zap.SugaredLogger
	sourcePane    *RemotePane       // left: source server
	targetPane    *RemotePane       // right: target server
	sourceSFTP    ports.SFTPService // independent SFTP instance for source
	targetSFTP    ports.SFTPService // independent SFTP instance for target
	statusBar     *tview.TextView
	headerBar     *tview.TextView // "Source: alias (host) | Target: alias (host)"
	confirmDialog  *ConfirmDialog                     // own instance (Pitfall 3)
	inputDialog    *InputDialog                        // own instance (Pitfall 3)
	transferModal  *TransferModal                      // cross-remote transfer progress overlay
	relaySvc       ports.RelayTransferService           // relay transfer between two SFTP connections
	clipboard      Clipboard                            // cross-remote clipboard state (SourcePane: 0=source, 1=target)
	transferring   bool                                 // true during active transfer (guards F5/c/p)
	transferCancel context.CancelFunc                   // cancel function for active transfer
	activePane     int                                  // 0 = source, 1 = target
	sourceServer  domain.Server
	targetServer  domain.Server
	onClose       func()
}

// NewDualRemoteFileBrowser creates a new DualRemoteFileBrowser with two independent
// SFTP connections. Both connections are established in parallel goroutines.
func NewDualRemoteFileBrowser(
	app *tview.Application,
	log *zap.SugaredLogger,
	source, target domain.Server,
	onClose func(),
) *DualRemoteFileBrowser {
	drb := &DualRemoteFileBrowser{
		Flex:          tview.NewFlex(),
		app:           app,
		log:           log,
		sourceServer:  source,
		targetServer:  target,
		onClose:       onClose,
	}

	// Create two independent SFTP client instances (D-02: NOT tui.sftpService)
	drb.sourceSFTP = sftp_client.New(log)
	drb.targetSFTP = sftp_client.New(log)

	// Create two RemotePane instances with their own SFTP connections
	drb.sourcePane = NewRemotePane(log, drb.sourceSFTP, source)
	drb.targetPane = NewRemotePane(log, drb.targetSFTP, target)

	// Create overlay dialogs (D-05: own instances)
	drb.confirmDialog = NewConfirmDialog(app)
	drb.inputDialog = NewInputDialog(app)
	drb.transferModal = NewTransferModal(app)
	drb.relaySvc = transfer.NewRelay(log, drb.sourceSFTP, drb.targetSFTP)

	// Clipboard provider for [C]/[M] prefix rendering in both panes
	drb.sourcePane.SetClipboardProvider(func() (bool, string, string, ClipboardOp) {
		return drb.clipboard.Active, drb.clipboard.FileInfo.Name, drb.clipboard.SourceDir, drb.clipboard.Operation
	})
	drb.targetPane.SetClipboardProvider(func() (bool, string, string, ClipboardOp) {
		return drb.clipboard.Active, drb.clipboard.FileInfo.Name, drb.clipboard.SourceDir, drb.clipboard.Operation
	})

	drb.build()
	return drb
}

// build initializes the layout, panes, header, status bar, and starts parallel SFTP connections.
func (drb *DualRemoteFileBrowser) build() {
	// Use ColorDefault to blend with kitty's native background (same as FileBrowser)
	drb.SetBackgroundColor(tcell.ColorDefault)

	// Create header bar (D-01: "Source: alias (host) | Target: alias (host)")
	drb.headerBar = tview.NewTextView()
	drb.headerBar.SetDynamicColors(true)
	drb.headerBar.SetBackgroundColor(tcell.Color235)
	drb.headerBar.SetWrap(false)
	drb.headerBar.SetTextAlign(tview.AlignCenter)
	drb.updateHeaderBar()

	// Create content FlexColumn: 50:50 split
	content := tview.NewFlex().SetDirection(tview.FlexColumn)
	content.SetBackgroundColor(tcell.ColorDefault)
	content.
		AddItem(drb.sourcePane, 0, 1, true).  // 50% width, initially focused
		AddItem(drb.targetPane, 0, 1, false) // 50% width

	// Create status bar
	drb.statusBar = tview.NewTextView()
	drb.statusBar.SetDynamicColors(true)
	drb.statusBar.SetBackgroundColor(tcell.Color235)
	drb.statusBar.SetWrap(false)
	drb.statusBar.SetTextAlign(tview.AlignCenter)
	drb.setStatusBarDefault()

	// Build root layout: header + content + status bar
	drb.SetDirection(tview.FlexRow).
		AddItem(drb.headerBar, 1, 0, false). // header: fixed 1 row
		AddItem(content, 0, 1, true).        // content takes remaining height
		AddItem(drb.statusBar, 1, 0, false)  // status bar: fixed 1 row

	// Set initial focus state (D-03)
	drb.activePane = 0
	drb.sourcePane.SetFocused(true)

	// Global input capture for Tab, Esc, d, R, m, s, S
	drb.SetInputCapture(drb.handleGlobalKeys)

	// AfterDrawFunc for status bar redraw (same pattern as FileBrowser)
	drb.app.SetAfterDrawFunc(func(screen tcell.Screen) {
		// Skip when overlays are visible
		if drb.confirmDialog != nil && drb.confirmDialog.IsVisible() {
			screen.Sync()
			return
		}
		if drb.inputDialog != nil && drb.inputDialog.IsVisible() {
			screen.Sync()
			return
		}
		if drb.transferModal != nil && drb.transferModal.IsVisible() {
			screen.Sync()
			return
		}
		_, _, width, height := drb.GetRect()
		if height >= 1 && drb.statusBar != nil {
			sy := height - 1
			bgColor := tcell.Color235
			bgStyle := tcell.StyleDefault.Background(bgColor)

			for col := 0; col < width; col++ {
				screen.SetContent(col, sy, ' ', nil, bgStyle)
			}
			tview.Print(screen, drb.statusBar.GetText(true), 0, sy, width, tview.AlignCenter, tcell.Color250)
			for col := 0; col < width; col++ {
				mainChar, _, style, _ := screen.GetContent(col, sy)
				screen.SetContent(col, sy, mainChar, nil, style.Background(bgColor))
			}
		}
		screen.Sync()
	})

	// Start parallel SFTP connections (D-07, D-08)
	go func() {
		err := drb.sourceSFTP.Connect(drb.sourceServer)
		drb.app.QueueUpdateDraw(func() {
			if err != nil {
				drb.sourcePane.ShowError(err.Error())
			} else {
				drb.sourcePane.ShowConnected()
			}
			drb.updateStatusBarConnection()
		})
	}()
	go func() {
		err := drb.targetSFTP.Connect(drb.targetServer)
		drb.app.QueueUpdateDraw(func() {
			if err != nil {
				drb.targetPane.ShowError(err.Error())
			} else {
				drb.targetPane.ShowConnected()
			}
			drb.updateStatusBarConnection()
		})
	}()
}

// Draw overrides Flex.Draw to draw overlays after the main content.
// Overlays (ConfirmDialog, InputDialog) are drawn on top after the main content.
func (drb *DualRemoteFileBrowser) Draw(screen tcell.Screen) {
	drb.Flex.Draw(screen)
	if drb.confirmDialog != nil && drb.confirmDialog.IsVisible() {
		drb.confirmDialog.Draw(screen)
	}
	if drb.inputDialog != nil && drb.inputDialog.IsVisible() {
		drb.inputDialog.Draw(screen)
	}
	if drb.transferModal != nil && drb.transferModal.IsVisible() {
		drb.transferModal.Draw(screen)
	}
}

// updateHeaderBar updates the header text with server alias, host, and port info.
func (drb *DualRemoteFileBrowser) updateHeaderBar() {
	sourceHost := drb.formatHost(drb.sourceServer)
	targetHost := drb.formatHost(drb.targetServer)
	drb.headerBar.SetText(fmt.Sprintf(
		"[yellow]Source:[-] [#5FAFFF]%s[-] (%s) [#666666]|[-] [yellow]Target:[-] [#5FAFFF]%s[-] (%s)",
		drb.sourceServer.Alias, sourceHost,
		drb.targetServer.Alias, targetHost,
	))
}

// formatHost returns "host:port" if port is not 22, otherwise just "host".
func (drb *DualRemoteFileBrowser) formatHost(s domain.Server) string {
	host := s.Host
	if s.Port != 0 && s.Port != 22 {
		host = fmt.Sprintf("%s:%d", host, s.Port)
	}
	return host
}

// setStatusBarDefault sets the default status bar text with keyboard hints (D-10, D-09).
func (drb *DualRemoteFileBrowser) setStatusBarDefault() {
	drb.statusBar.SetText("[white]c[-] Copy  [white]x[-] Move  [white]p[-] Paste  [white]F5[-] Transfer  [white]Tab[-] Switch  [white]d[-] Delete  [white]Esc[-] Back")
}

// updateStatusBarConnection prepends connection status for both servers to the status bar text (D-09).
// Format: [alias] Connected/Error  [alias] Connected/Error  [active panel]  [key hints]
func (drb *DualRemoteFileBrowser) updateStatusBarConnection() {
	sourceStatus := "[#A0FFA0]Connected[-]"
	if !drb.sourcePane.IsConnected() {
		sourceStatus = "[#FF6B6B]Error[-]"
	}
	targetStatus := "[#A0FFA0]Connected[-]"
	if !drb.targetPane.IsConnected() {
		targetStatus = "[#FF6B6B]Error[-]"
	}

	activeLabel := "[white]\u25cf Source[-]"
	if drb.activePane == 1 {
		activeLabel = "[white]\u25cf Target[-]"
	}

	text := fmt.Sprintf("[#5FAFFF]%s[-] %s  [#5FAFFF]%s[-] %s  %s  %s",
		drb.sourceServer.Alias, sourceStatus,
		drb.targetServer.Alias, targetStatus,
		activeLabel,
		"[white]c[-] Copy  [white]x[-] Move  [white]p[-] Paste  [white]F5[-] Transfer  [white]Tab[-] Switch  [white]d[-] Delete  [white]Esc[-] Back",
	)
	drb.statusBar.SetText(text)
}

// updateStatusBarTemp prepends a colored message before the default keyboard hints.
func (drb *DualRemoteFileBrowser) updateStatusBarTemp(msg string) {
	drb.statusBar.SetText(msg + "  [white]c[-] Copy  [white]x[-] Move  [white]p[-] Paste  [white]F5[-] Transfer  [white]Tab[-] Switch  [white]d[-] Delete  [white]Esc[-] Back")
}

// currentSFTPService returns the SFTP service for the currently active pane.
func (drb *DualRemoteFileBrowser) currentSFTPService() ports.SFTPService {
	if drb.activePane == 0 {
		return drb.sourceSFTP
	}
	return drb.targetSFTP
}

// currentPane returns the RemotePane for the currently active pane.
func (drb *DualRemoteFileBrowser) currentPane() *RemotePane {
	if drb.activePane == 0 {
		return drb.sourcePane
	}
	return drb.targetPane
}

// activePanelLabel returns "Source" or "Target" based on the active pane.
func (drb *DualRemoteFileBrowser) activePanelLabel() string {
	if drb.activePane == 0 {
		return "Source"
	}
	return "Target"
}

// showStatusError displays a red error message in the status bar that auto-clears after 3 seconds.
// Uses the package-level statusErrorTimer pattern from file_browser.go.
func (drb *DualRemoteFileBrowser) showStatusError(msg string) {
	if statusErrorTimer != nil {
		statusErrorTimer.Stop()
	}
	drb.statusBar.SetText(fmt.Sprintf("[#FF6B6B]%s[-]", msg))
	statusErrorTimer = time.AfterFunc(3*time.Second, func() {
		drb.app.QueueUpdateDraw(func() {
			drb.setStatusBarDefault()
		})
	})
}

// Helper methods adapted from file_browser.go for dual-remote (both panes are RemotePane).

// getActiveSelection returns the row and column of the current selection in the active pane.
func (drb *DualRemoteFileBrowser) getActiveSelection() (int, int) {
	return drb.currentPane().GetSelection()
}

// getActiveCell returns the TableCell at the given row and column in the active pane.
//
//nolint:unparam // col is always 0 but kept for table cell API consistency
func (drb *DualRemoteFileBrowser) getActiveCell(row, col int) *tview.TableCell {
	return drb.currentPane().GetCell(row, col)
}

// getCurrentPanePath returns the current directory path of the active pane.
func (drb *DualRemoteFileBrowser) getCurrentPanePath() string {
	return drb.currentPane().GetCurrentPath()
}

// getSelectedFiles returns all space-selected files in the active pane.
func (drb *DualRemoteFileBrowser) getSelectedFiles() []domain.FileInfo {
	return drb.currentPane().SelectedFiles()
}

// buildPath constructs a full path using joinPath (Unix-style for remote paths).
func (drb *DualRemoteFileBrowser) buildPath(base, name string) string {
	return joinPath(base, name)
}

// refreshPane refreshes the file listing in the given pane (0=source, 1=target).
func (drb *DualRemoteFileBrowser) refreshPane(paneIdx int) {
	if paneIdx == 0 {
		drb.sourcePane.Refresh()
	} else {
		drb.targetPane.Refresh()
	}
}

// refreshAndReposition refreshes the listing and positions the cursor at the given row.
// Clamps the row to the valid range [1, totalRows-1].
func (drb *DualRemoteFileBrowser) refreshAndReposition(paneIdx int, deletedRow int) {
	drb.refreshPane(paneIdx)
	var pane *RemotePane
	if paneIdx == 0 {
		pane = drb.sourcePane
	} else {
		pane = drb.targetPane
	}
	targetRow := deletedRow
	if targetRow < 1 {
		targetRow = 1
	}
	rowCount := pane.GetRowCount()
	if targetRow >= rowCount {
		targetRow = rowCount - 1
	}
	if targetRow < 1 {
		targetRow = 1
	}
	pane.Select(targetRow, 0)
}

// focusOnItem finds a file by name in the given pane and selects it.
func (drb *DualRemoteFileBrowser) focusOnItem(paneIdx int, name string) {
	var pane *RemotePane
	if paneIdx == 0 {
		pane = drb.sourcePane
	} else {
		pane = drb.targetPane
	}
	// Search through table rows to find the file by name
	for row := 1; row < pane.GetRowCount(); row++ {
		cell := pane.GetCell(row, 0)
		if cell == nil {
			continue
		}
		ref := cell.GetReference()
		if ref == nil {
			continue
		}
		fi, ok := ref.(domain.FileInfo)
		if ok && fi.Name == name {
			pane.Select(row, 0)
			return
		}
	}
}

// cycleSortField cycles the sort field for the currently focused pane.
func (drb *DualRemoteFileBrowser) cycleSortField() {
	pane := drb.currentPane()
	mode := pane.GetSortMode().ToggleField()
	pane.SetSortMode(mode)
}

// reverseSort reverses the sort direction for the currently focused pane.
func (drb *DualRemoteFileBrowser) reverseSort() {
	pane := drb.currentPane()
	mode := pane.GetSortMode().Reverse()
	pane.SetSortMode(mode)
}

// paneForIdx returns the RemotePane for the given index (0=source, 1=target).
func (drb *DualRemoteFileBrowser) paneForIdx(idx int) *RemotePane {
	if idx == 0 {
		return drb.sourcePane
	}
	return drb.targetPane
}

// sftpForIdx returns the SFTPService for the given index (0=source, 1=target).
func (drb *DualRemoteFileBrowser) sftpForIdx(idx int) ports.SFTPService {
	if idx == 0 {
		return drb.sourceSFTP
	}
	return drb.targetSFTP
}

// aliasForIdx returns the server alias for the given index (0=source, 1=target).
func (drb *DualRemoteFileBrowser) aliasForIdx(idx int) string {
	if idx == 0 {
		return drb.sourceServer.Alias
	}
	return drb.targetServer.Alias
}

