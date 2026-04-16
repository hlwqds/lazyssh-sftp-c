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

	"github.com/Adembc/lazyssh/internal/core/domain"
	"github.com/Adembc/lazyssh/internal/core/ports"
	"github.com/gdamore/tcell/v2"
)

// handleGlobalKeys handles global keyboard events for the DualRemoteFileBrowser.
// Event propagation chain:
// 1. Overlay visibility check -> InputDialog/ConfirmDialog intercept when visible
// 2. DualRemoteFileBrowser.SetInputCapture -> handles Tab, Esc, d, R, m, s, S
// 3. FocusedPane.SetInputCapture -> handles h, Backspace, Space, . (from RemotePane)
// 4. Table.InputHandler -> handles j/k/arrow/Enter/PgUp/PgDn (built-in)
func (drb *DualRemoteFileBrowser) handleGlobalKeys(event *tcell.EventKey) *tcell.EventKey {
	// TransferModal has highest overlay priority (full-screen overlay)
	if drb.transferModal != nil && drb.transferModal.IsVisible() {
		return drb.transferModal.HandleKey(event)
	}
	// Overlay key interception: check BEFORE any other key handling (same as FileBrowser)
	// InputDialog has highest priority (text input must consume all keys)
	if drb.inputDialog != nil && drb.inputDialog.IsVisible() {
		return drb.inputDialog.HandleKey(event)
	}
	// ConfirmDialog next (consumes y/n/Esc)
	if drb.confirmDialog != nil && drb.confirmDialog.IsVisible() {
		return drb.confirmDialog.HandleKey(event)
	}

	switch event.Key() { //nolint:exhaustive // keyboard handler: intentionally handles only specific keys
	case tcell.KeyTab:
		drb.switchFocus()
		return nil
	case tcell.KeyF5:
		if !drb.transferring {
			drb.handleF5Transfer()
		}
		return nil
	case tcell.KeyESC:
		if drb.clipboard.Active && !drb.transferring {
			// Clear clipboard on Esc (D-06 lifecycle)
			drb.clipboard = Clipboard{}
			drb.refreshPane(drb.activePane)
			drb.setStatusBarDefault()
			return nil
		}
		drb.close()
		return nil
	}
	switch event.Rune() {
	case 'c':
		if !drb.transferring {
			drb.handleCopy()
		}
		return nil
	case 'x':
		if !drb.transferring {
			drb.handleMove()
		}
		return nil
	case 'p':
		if !drb.transferring {
			drb.handleCrossRemotePaste()
		}
		return nil
	case 'd':
		drb.handleDelete()
		return nil
	case 'R':
		drb.handleRename()
		return nil
	case 'm':
		drb.handleMkdir()
		return nil
	case 's':
		drb.cycleSortField()
		return nil
	case 'S':
		drb.reverseSort()
		return nil
	}
	return event // pass to focused pane's InputCapture
}

// switchFocus toggles focus between the source and target panes (D-03).
func (drb *DualRemoteFileBrowser) switchFocus() {
	if drb.activePane == 0 {
		// Switch from source to target
		drb.sourcePane.SetFocused(false)
		drb.targetPane.SetFocused(true)
		drb.activePane = 1
		drb.app.SetFocus(drb.targetPane)
	} else {
		// Switch from target to source
		drb.targetPane.SetFocused(false)
		drb.sourcePane.SetFocused(true)
		drb.activePane = 0
		drb.app.SetFocus(drb.sourcePane)
	}
	drb.updateStatusBarConnection() // update active panel indicator
}

// close closes the dual remote browser and returns to the main view.
// It cleans up the AfterDrawFunc and closes both SFTP connections in a goroutine (D-11).
func (drb *DualRemoteFileBrowser) close() {
	drb.app.SetAfterDrawFunc(nil) // remove status bar redraw callback
	go func() {
		_ = drb.sourceSFTP.Close()
		_ = drb.targetSFTP.Close()
	}()
	if drb.onClose != nil {
		drb.onClose()
	}
}

// handleDelete handles the 'd' key: delete selected file(s) or directory on the active remote pane.
// For multi-select (Space): shows batch delete confirmation with count and total size.
// For single selection: shows file details (name, size, type, modified time).
// Uses goroutine + QueueUpdateDraw to avoid blocking UI.
func (drb *DualRemoteFileBrowser) handleDelete() {
	paneIdx := drb.activePane
	sftp := drb.currentSFTPService()
	pane := drb.currentPane()

	// Remote pane connection check
	if !pane.IsConnected() {
		drb.showStatusError("Not connected to " + drb.activePanelLabel())
		return
	}

	// Check for multi-select
	selectedFiles := drb.getSelectedFiles()
	if len(selectedFiles) > 0 {
		drb.handleBatchDelete(paneIdx, sftp, selectedFiles)
		return
	}

	// Single selection: get current FileInfo
	row, _ := drb.getActiveSelection()
	cell := drb.getActiveCell(row, 0)
	if cell == nil {
		return
	}
	fi, ok := cell.GetReference().(domain.FileInfo)
	if !ok {
		return
	}

	currentPath := drb.getCurrentPanePath()
	fullPath := drb.buildPath(currentPath, fi.Name)

	// Build confirmation message (same format as FileBrowser)
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
		detail = "Directory not empty, all contents will be deleted"
	}

	drb.confirmDialog.SetOnConfirm(func() {
		go func() {
			var err error
			if fi.IsDir {
				err = sftp.RemoveAll(fullPath)
			} else {
				err = sftp.Remove(fullPath)
			}
			drb.app.QueueUpdateDraw(func() {
				if err != nil {
					drb.showStatusError(fmt.Sprintf("Delete failed: %s", trimError(err.Error(), 50)))
					return
				}
				drb.refreshAndReposition(paneIdx, row)
			})
		}()
	})

	drb.confirmDialog.SetOnCancel(nil)
	drb.confirmDialog.Show("Delete?", message, detail)
}

// handleBatchDelete handles deletion of multiple space-selected files on the active remote pane.
// Shows count and total size summary, then deletes sequentially in a goroutine.
func (drb *DualRemoteFileBrowser) handleBatchDelete(paneIdx int, sftp ports.FileService, files []domain.FileInfo) {
	currentPath := drb.getCurrentPanePath()

	// Calculate total size (skip directories)
	var totalSize int64
	for _, fi := range files {
		if !fi.IsDir {
			totalSize += fi.Size
		}
	}

	message := fmt.Sprintf("Delete %d items? Total size: %s", len(files), formatSize(totalSize))

	drb.confirmDialog.SetOnConfirm(func() {
		go func() {
			var firstErr error
			for _, fi := range files {
				fullPath := drb.buildPath(currentPath, fi.Name)
				var err error
				if fi.IsDir {
					err = sftp.RemoveAll(fullPath)
				} else {
					err = sftp.Remove(fullPath)
				}
				if err != nil && firstErr == nil {
					firstErr = err
					drb.log.Errorw("batch delete failed", "file", fi.Name, "error", err)
				}
			}
			drb.app.QueueUpdateDraw(func() {
				drb.refreshPane(paneIdx)
				if firstErr != nil {
					drb.showStatusError(fmt.Sprintf("Delete failed: %s", trimError(firstErr.Error(), 50)))
				}
			})
		}()
	})

	drb.confirmDialog.SetOnCancel(nil)
	drb.confirmDialog.Show("Delete Multiple?", message, "")
}

// handleRename handles the 'R' key: rename selected file/directory on the active remote pane.
// Shows InputDialog pre-filled with current name. Checks for name conflicts
// and prompts with ConfirmDialog if target already exists.
func (drb *DualRemoteFileBrowser) handleRename() {
	paneIdx := drb.activePane
	sftp := drb.currentSFTPService()
	pane := drb.currentPane()

	// Remote pane connection check
	if !pane.IsConnected() {
		drb.showStatusError("Not connected to " + drb.activePanelLabel())
		return
	}

	row, _ := drb.getActiveSelection()
	cell := drb.getActiveCell(row, 0)
	if cell == nil {
		return
	}
	fi, ok := cell.GetReference().(domain.FileInfo)
	if !ok {
		return
	}

	currentPath := drb.getCurrentPanePath()
	oldFullPath := drb.buildPath(currentPath, fi.Name)

	drb.inputDialog.SetOnSubmit(func(newName string) {
		// Empty check
		if newName == "" {
			return
		}
		// No change check
		if newName == fi.Name {
			return
		}

		newFullPath := drb.buildPath(currentPath, newName)

		// Check for name conflict
		if _, err := sftp.Stat(newFullPath); err == nil {
			// Target exists -- show confirm dialog for overwrite
			drb.confirmDialog.SetOnConfirm(func() {
				go func() {
					err := sftp.Rename(oldFullPath, newFullPath)
					drb.app.QueueUpdateDraw(func() {
						if err != nil {
							drb.showStatusError(fmt.Sprintf("Rename failed: %s", trimError(err.Error(), 50)))
							return
						}
						drb.refreshPane(paneIdx)
						drb.focusOnItem(paneIdx, newName)
					})
				}()
			})
			drb.confirmDialog.SetOnCancel(nil)
			drb.confirmDialog.Show("Name Conflict", fmt.Sprintf("'%s' already exists. Overwrite?", newName), "")
			return
		}

		// No conflict -- rename directly
		go func() {
			err := sftp.Rename(oldFullPath, newFullPath)
			drb.app.QueueUpdateDraw(func() {
				if err != nil {
					drb.showStatusError(fmt.Sprintf("Rename failed: %s", trimError(err.Error(), 50)))
					return
				}
				drb.refreshPane(paneIdx)
				drb.focusOnItem(paneIdx, newName)
			})
		}()
	})

	drb.inputDialog.SetOnCancel(nil)
	drb.inputDialog.Show("Rename", "New name: ", fi.Name)
}

// handleMkdir handles the 'm' key: create new directory on the active remote pane.
// Shows InputDialog with empty input. After creation, positions cursor on new directory.
func (drb *DualRemoteFileBrowser) handleMkdir() {
	paneIdx := drb.activePane
	sftp := drb.currentSFTPService()
	pane := drb.currentPane()

	// Remote pane connection check
	if !pane.IsConnected() {
		drb.showStatusError("Not connected to " + drb.activePanelLabel())
		return
	}

	currentPath := drb.getCurrentPanePath()

	drb.inputDialog.SetOnSubmit(func(dirName string) {
		if dirName == "" {
			return
		}

		fullPath := drb.buildPath(currentPath, dirName)

		go func() {
			err := sftp.Mkdir(fullPath)
			drb.app.QueueUpdateDraw(func() {
				if err != nil {
					drb.showStatusError(fmt.Sprintf("Mkdir failed: %s", trimError(err.Error(), 50)))
					return
				}
				drb.refreshPane(paneIdx)
				drb.focusOnItem(paneIdx, dirName)
			})
		}()
	})

	drb.inputDialog.SetOnCancel(nil)
	drb.inputDialog.Show("New Directory", "Directory name: ", "")
}

// handleCopy handles the 'c' key: mark selected file for copy operation.
// Sets Clipboard with OpCopy, refreshes pane to show green [C] prefix.
func (drb *DualRemoteFileBrowser) handleCopy() {
	row, _ := drb.getActiveSelection()
	cell := drb.getActiveCell(row, 0)
	if cell == nil {
		return
	}
	fi, ok := cell.GetReference().(domain.FileInfo)
	if !ok {
		return
	}
	pane := drb.currentPane()
	if !pane.IsConnected() {
		drb.showStatusError("Not connected to " + drb.activePanelLabel())
		return
	}

	drb.clipboard = Clipboard{
		Active:     true,
		SourcePane: drb.activePane, // 0=source, 1=target
		FileInfo:   fi,
		SourceDir:  drb.getCurrentPanePath(),
		Operation:  OpCopy,
	}

	drb.refreshPane(drb.activePane)
	drb.focusOnItem(drb.activePane, fi.Name)
	drb.updateStatusBarTemp(fmt.Sprintf("[#00FF7F]Clipboard: %s[-]", fi.Name))
}

// handleMove handles the 'x' key: mark selected file for move operation.
// Sets Clipboard with OpMove, refreshes pane to show red [M] prefix.
func (drb *DualRemoteFileBrowser) handleMove() {
	row, _ := drb.getActiveSelection()
	cell := drb.getActiveCell(row, 0)
	if cell == nil {
		return
	}
	fi, ok := cell.GetReference().(domain.FileInfo)
	if !ok {
		return
	}
	pane := drb.currentPane()
	if !pane.IsConnected() {
		drb.showStatusError("Not connected to " + drb.activePanelLabel())
		return
	}

	drb.clipboard = Clipboard{
		Active:     true,
		SourcePane: drb.activePane,
		FileInfo:   fi,
		SourceDir:  drb.getCurrentPanePath(),
		Operation:  OpMove,
	}

	drb.refreshPane(drb.activePane)
	drb.focusOnItem(drb.activePane, fi.Name)
	drb.updateStatusBarTemp("[#FF6B6B]Move: " + fi.Name + "[-]")
}

// buildCrossConflictHandler creates a conflict handler for cross-remote transfers.
// Uses dstSFTP.Stat to check for existing files on the target server.
// Shows conflict dialog via TransferModal and blocks the transfer goroutine until user responds.
func (drb *DualRemoteFileBrowser) buildCrossConflictHandler(ctx context.Context, dstSFTP ports.SFTPService, dstDir string) domain.ConflictHandler {
	return func(fileName string) (domain.ConflictAction, string) {
		actionCh := make(chan domain.ConflictAction, 1)

		// Gather existing file info on target server
		var existingInfo string
		dstPath := joinPath(dstDir, fileName)
		if fi, err := dstSFTP.Stat(dstPath); err == nil {
			existingInfo = fmt.Sprintf("%s, %s", formatSize(fi.Size()), fi.ModTime().Format("2006-01-02 15:04"))
		}

		drb.app.QueueUpdateDraw(func() {
			drb.transferModal.ShowConflict(fileName, existingInfo, actionCh)
		})

		var action domain.ConflictAction
		select {
		case action = <-actionCh:
		case <-ctx.Done():
			return domain.ConflictSkip, ""
		}

		switch action {
		case domain.ConflictSkip:
			drb.app.QueueUpdateDraw(func() {
				drb.updateStatusBarTemp(fmt.Sprintf("[#FFA500]Skipped: %s[-]", fileName))
			})
			return domain.ConflictSkip, ""
		case domain.ConflictRename:
			// Find next available name on target server
			candidate := nextAvailableName(joinPath(dstDir, fileName), func(path string) (os.FileInfo, error) {
				return dstSFTP.Stat(path)
			})
			return domain.ConflictRename, candidate
		case domain.ConflictOverwrite:
			return domain.ConflictOverwrite, ""
		}
		return domain.ConflictOverwrite, ""
	}
}

// handleCrossRemotePaste handles the 'p' key: paste (copy or move) file from clipboard to opposite pane.
// Validates both panels connected, disallows same-pane paste, shows TransferModal with two-stage progress.
func (drb *DualRemoteFileBrowser) handleCrossRemotePaste() {
	if !drb.clipboard.Active {
		return
	}

	// Determine source and target panes
	srcPaneIdx := drb.clipboard.SourcePane // 0=source, 1=target
	dstPaneIdx := 1 - srcPaneIdx           // opposite pane

	srcPane := drb.paneForIdx(srcPaneIdx)
	dstPane := drb.paneForIdx(dstPaneIdx)
	srcSFTP := drb.sftpForIdx(srcPaneIdx)
	dstSFTP := drb.sftpForIdx(dstPaneIdx)

	// Both panes must be connected
	if !srcPane.IsConnected() || !dstPane.IsConnected() {
		drb.showStatusError("Both panels must be connected")
		return
	}

	// Disallow paste in same pane
	if drb.activePane == srcPaneIdx {
		drb.showStatusError("Switch to target panel first (Tab)")
		return
	}

	targetName := drb.clipboard.FileInfo.Name
	srcPath := joinPath(drb.clipboard.SourceDir, targetName)
	dstPath := joinPath(dstPane.GetCurrentPath(), targetName)
	isMove := drb.clipboard.Operation == OpMove
	sourceAlias := drb.aliasForIdx(srcPaneIdx)
	targetAlias := drb.aliasForIdx(dstPaneIdx)

	// Set up transfer state
	drb.transferring = true
	ctx, cancel := context.WithCancel(context.Background())
	drb.transferCancel = cancel

	// Configure TransferModal dismiss callback
	drb.transferModal.SetDismissCallback(func() {
		if drb.transferModal.IsCanceled() {
			if drb.transferCancel != nil {
				drb.transferCancel()
			}
			return
		}
		drb.transferring = false
		drb.app.SetRoot(drb, true)
		drb.app.SetFocus(drb.currentPane())
	})

	drb.transferModal.ShowCrossRemote(sourceAlias, targetAlias, targetName)
	drb.app.QueueUpdateDraw(func() {})

	go func() {
		defer cancel()

		// Build two-stage progress callback
		var dlDone bool
		combinedProgress := func(p domain.TransferProgress) {
			if p.Done && !dlDone {
				dlDone = true
				drb.app.QueueUpdateDraw(func() {
					drb.transferModal.ResetProgress()
					drb.transferModal.fileLabel = fmt.Sprintf("Uploading to %s: %s", targetAlias, targetName)
				})
				return
			}
			label := fmt.Sprintf("Downloading from %s: %s", sourceAlias, targetName)
			drb.app.QueueUpdateDraw(func() {
				drb.transferModal.fileLabel = label
				drb.transferModal.Update(p)
			})
		}

		// Build conflict handler for target server
		onConflict := drb.buildCrossConflictHandler(ctx, dstSFTP, dstPane.GetCurrentPath())

		// Execute relay transfer
		var relayErr error
		if drb.clipboard.FileInfo.IsDir {
			_, relayErr = drb.relaySvc.RelayDir(ctx, srcPath, dstPath, combinedProgress, onConflict)
		} else {
			relayErr = drb.relaySvc.RelayFile(ctx, srcPath, dstPath, combinedProgress, onConflict)
		}

		if relayErr != nil {
			drb.app.QueueUpdateDraw(func() {
				drb.showStatusError(fmt.Sprintf("Transfer failed: %s", trimError(relayErr.Error(), 50)))
				drb.transferring = false
				drb.transferModal.Hide()
			})
			return // D-07: do NOT clear clipboard on failure
		}

		// Move operation: delete source file after successful relay (D-08)
		if isMove {
			var delErr error
			if drb.clipboard.FileInfo.IsDir {
				delErr = srcSFTP.RemoveAll(srcPath)
			} else {
				delErr = srcSFTP.Remove(srcPath)
			}
			if delErr != nil {
				// D-08: Rollback -- try to remove target copy
				drb.log.Errorw("move delete failed, rolling back", "src", srcPath, "err", delErr)
				if rmErr := dstSFTP.Remove(dstPath); rmErr != nil {
					drb.log.Errorw("rollback failed", "dst", dstPath, "err", rmErr)
					drb.app.QueueUpdateDraw(func() {
						drb.showStatusError(fmt.Sprintf("Move failed. Manual cleanup needed: %s", dstPath))
					})
				} else {
					drb.app.QueueUpdateDraw(func() {
						drb.showStatusError(fmt.Sprintf("Delete failed: %s", trimError(delErr.Error(), 50)))
					})
				}
				drb.app.QueueUpdateDraw(func() {
					drb.transferring = false
					drb.transferModal.Hide()
				})
				return // D-07: do NOT clear clipboard on failure
			}
		}

		// Success: clear clipboard, refresh panes
		drb.app.QueueUpdateDraw(func() {
			drb.clipboard = Clipboard{}
			drb.transferring = false
			drb.transferModal.Hide()
			drb.app.SetRoot(drb, true)
			drb.app.SetFocus(drb.currentPane())
			// Refresh both panes to update file listings
			drb.sourcePane.Refresh()
			drb.targetPane.Refresh()
			// Focus on transferred item in target pane
			drb.focusOnItem(dstPaneIdx, targetName)
			drb.setStatusBarDefault()
		})
	}()
}

// handleF5Transfer handles the F5 key: directly transfer selected file/directory to opposite panel.
// For files: transfers immediately. For directories: shows ConfirmDialog before transfer.
func (drb *DualRemoteFileBrowser) handleF5Transfer() {
	pane := drb.currentPane()
	if !pane.IsConnected() {
		drb.showStatusError("Not connected to " + drb.activePanelLabel())
		return
	}

	row, _ := drb.getActiveSelection()
	cell := drb.getActiveCell(row, 0)
	if cell == nil {
		return
	}
	fi, ok := cell.GetReference().(domain.FileInfo)
	if !ok {
		return
	}

	// Determine opposite pane
	dstPaneIdx := 1 - drb.activePane
	dstPane := drb.paneForIdx(dstPaneIdx)

	if !dstPane.IsConnected() {
		drb.showStatusError("Target panel not connected: " + drb.aliasForIdx(dstPaneIdx))
		return
	}

	// D-05: Directory transfer requires confirmation
	if fi.IsDir {
		message := fmt.Sprintf("Transfer directory '%s' to %s?", fi.Name, drb.aliasForIdx(dstPaneIdx))
		drb.confirmDialog.SetOnConfirm(func() {
			drb.executeF5Transfer(fi, dstPaneIdx)
		})
		drb.confirmDialog.SetOnCancel(nil)
		drb.confirmDialog.Show("Transfer Directory?", message, "Recursive transfer may take a while")
		return
	}

	// File: transfer immediately (no confirmation per D-05)
	drb.executeF5Transfer(fi, dstPaneIdx)
}

// executeF5Transfer performs the actual F5 relay transfer in a goroutine.
func (drb *DualRemoteFileBrowser) executeF5Transfer(fi domain.FileInfo, dstPaneIdx int) {
	srcPaneIdx := drb.activePane
	dstSFTP := drb.sftpForIdx(dstPaneIdx)

	srcPath := joinPath(drb.getCurrentPanePath(), fi.Name)
	dstPath := joinPath(drb.paneForIdx(dstPaneIdx).GetCurrentPath(), fi.Name)
	sourceAlias := drb.aliasForIdx(srcPaneIdx)
	targetAlias := drb.aliasForIdx(dstPaneIdx)

	drb.transferring = true
	ctx, cancel := context.WithCancel(context.Background())
	drb.transferCancel = cancel

	drb.transferModal.SetDismissCallback(func() {
		if drb.transferModal.IsCanceled() {
			if drb.transferCancel != nil {
				drb.transferCancel()
			}
			return
		}
		drb.transferring = false
		drb.app.SetRoot(drb, true)
		drb.app.SetFocus(drb.currentPane())
	})

	drb.transferModal.ShowCrossRemote(sourceAlias, targetAlias, fi.Name)
	drb.app.QueueUpdateDraw(func() {})

	go func() {
		defer cancel()

		var dlDone bool
		combinedProgress := func(p domain.TransferProgress) {
			if p.Done && !dlDone {
				dlDone = true
				drb.app.QueueUpdateDraw(func() {
					drb.transferModal.ResetProgress()
					drb.transferModal.fileLabel = fmt.Sprintf("Uploading to %s: %s", targetAlias, fi.Name)
				})
				return
			}
			label := fmt.Sprintf("Downloading from %s: %s", sourceAlias, fi.Name)
			drb.app.QueueUpdateDraw(func() {
				drb.transferModal.fileLabel = label
				drb.transferModal.Update(p)
			})
		}

		onConflict := drb.buildCrossConflictHandler(ctx, dstSFTP, drb.paneForIdx(dstPaneIdx).GetCurrentPath())

		var err error
		if fi.IsDir {
			_, err = drb.relaySvc.RelayDir(ctx, srcPath, dstPath, combinedProgress, onConflict)
		} else {
			err = drb.relaySvc.RelayFile(ctx, srcPath, dstPath, combinedProgress, onConflict)
		}

		drb.app.QueueUpdateDraw(func() {
			drb.transferring = false
			drb.transferModal.Hide()
			if err != nil {
				drb.showStatusError(fmt.Sprintf("Transfer failed: %s", trimError(err.Error(), 50)))
				return
			}
			drb.sourcePane.Refresh()
			drb.targetPane.Refresh()
			drb.focusOnItem(dstPaneIdx, fi.Name)
			drb.app.SetRoot(drb, true)
			drb.app.SetFocus(drb.currentPane())
		})
	}()
}
