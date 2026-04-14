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
	"github.com/gdamore/tcell/v2"
)

// handleGlobalKeys handles global keyboard events for the FileBrowser.
// Event propagation chain:
// 1. Overlay visibility check -> RecentDirs intercepts all keys when visible (D-08)
// 2. FileBrowser.SetInputCapture -> handles Tab, Esc, s, S, r
// 3. FocusedPane.SetInputCapture -> handles h, Backspace, Space, .
// 4. Table.InputHandler -> handles j/k/arrow/Enter/PgUp/PgDn (built-in)
//
// Esc handling (D-03 double-Esc pattern):
//   - If transfer modal is visible, delegate to TransferModal.HandleKey
//     which manages progress->cancelConfirm->summary mode transitions
//   - Otherwise, close the file browser
func (fb *FileBrowser) handleGlobalKeys(event *tcell.EventKey) *tcell.EventKey {
	// Overlay key interception: check BEFORE any other key handling (D-08, Pitfall 2)
	if fb.recentDirs != nil && fb.recentDirs.IsVisible() {
		return fb.recentDirs.HandleKey(event)
	}

	switch event.Key() { //nolint:exhaustive // keyboard handler: intentionally handles only specific keys
	case tcell.KeyTab:
		fb.switchFocus()
		return nil
	case tcell.KeyESC:
		if fb.transferModal != nil && fb.transferModal.IsVisible() {
			fb.transferModal.HandleKey(event) // delegates to modal's HandleKey
			return nil
		}
		fb.close()
		return nil
	case tcell.KeyF5:
		fb.initiateDirTransfer()
		return nil
	}
	switch event.Rune() {
	case 'r':
		if fb.activePane == 1 && fb.remotePane.IsConnected() {
			fb.recentDirs.SetCurrentPath(fb.remotePane.GetCurrentPath())
			fb.recentDirs.Show()
			return nil
		}
	case 's':
		fb.cycleSortField()
		return nil
	case 'S':
		fb.reverseSort()
		return nil
	}
	return event // pass to focused pane's InputCapture
}

// switchFocus toggles focus between the local and remote panes.
func (fb *FileBrowser) switchFocus() {
	if fb.activePane == 0 {
		// Switch from local to remote
		fb.localPane.SetFocused(false)
		fb.remotePane.SetFocused(true)
		fb.activePane = 1
		fb.app.SetFocus(fb.remotePane)
	} else {
		// Switch from remote to local
		fb.remotePane.SetFocused(false)
		fb.localPane.SetFocused(true)
		fb.activePane = 0
		fb.app.SetFocus(fb.localPane)
	}
}

// close closes the file browser and returns to the main view.
// It cleans up the SFTP connection in a goroutine (per Pitfall 3).
func (fb *FileBrowser) close() {
	fb.app.SetAfterDrawFunc(nil) // remove status bar redraw callback
	go func() {
		_ = fb.sftpService.Close()
	}()
	if fb.onClose != nil {
		fb.onClose()
	}
}

// cycleSortField cycles the sort field for the currently focused pane.
// Sort fields cycle: Name -> Size -> Date -> Name (preserving direction).
func (fb *FileBrowser) cycleSortField() {
	if fb.activePane == 0 {
		mode := fb.localPane.GetSortMode().ToggleField()
		fb.localPane.SetSortMode(mode)
	} else {
		mode := fb.remotePane.GetSortMode().ToggleField()
		fb.remotePane.SetSortMode(mode)
	}
}

// reverseSort reverses the sort direction for the currently focused pane.
func (fb *FileBrowser) reverseSort() {
	if fb.activePane == 0 {
		mode := fb.localPane.GetSortMode().Reverse()
		fb.localPane.SetSortMode(mode)
	} else {
		mode := fb.remotePane.GetSortMode().Reverse()
		fb.remotePane.SetSortMode(mode)
	}
}
