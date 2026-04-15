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
	case tcell.KeyESC:
		drb.close()
		return nil
	}
	switch event.Rune() {
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
