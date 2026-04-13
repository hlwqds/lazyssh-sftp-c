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
	"strings"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"github.com/Adembc/lazyssh/internal/core/ports"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"go.uber.org/zap"
)

// RemotePane is a tview.Table-based component for browsing remote files via SFTP.
// It displays connection states (Connecting, Connected, Error) and file listings.
type RemotePane struct {
	*tview.Table
	log         *zap.SugaredLogger
	sftpService ports.SFTPService
	server      domain.Server
	currentPath string
	sortMode    FileSortMode
	showHidden  bool
	selected    map[string]bool // multi-select state: file name -> selected
	connected   bool
	onPathChange func(path string)
	onFileAction func(fi domain.FileInfo)
}

// NewRemotePane creates a new RemotePane for browsing remote files.
func NewRemotePane(log *zap.SugaredLogger, sftp ports.SFTPService, server domain.Server) *RemotePane {
	rp := &RemotePane{
		Table:       tview.NewTable(),
		log:         log,
		sftpService: sftp,
		server:      server,
		currentPath: "~", // SSH default home
		sortMode:    FileSortByNameAsc,
		showHidden:  false,
		selected:    make(map[string]bool),
		connected:   false,
	}
	rp.build()
	return rp
}

// build configures the Table with selection, borders, header, and key handling.
func (rp *RemotePane) build() {
	rp.SetSelectable(true, false) // row selection only
	rp.SetFixed(1, 0)            // fixed header row
	rp.SetBorder(true).
		SetBorderColor(tcell.Color238).
		SetTitleColor(tcell.Color250).
		SetTitleAlign(tview.AlignLeft)

	// Intercept pane-specific keys; pass through to Table for j/k/arrows
	rp.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if !rp.connected {
			return event // ignore keys when not connected
		}
		switch event.Rune() {
		case 'h':
			rp.NavigateToParent()
			return nil
		case ' ':
			rp.ToggleSelection()
			return nil
		case '.':
			rp.ToggleHidden()
			return nil
		}
		//nolint:exhaustive // We only handle specific keys and pass through others
		switch event.Key() {
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			rp.NavigateToParent()
			return nil
		}
		return event // pass to Table built-in for j/k/arrows/PgUp/PgDn/Enter
	})

	// Enter key: navigate into directory (only when connected)
	rp.SetSelectedFunc(func(row, _ int) {
		if !rp.connected {
			return
		}
		cell := rp.GetCell(row, 0)
		if cell == nil {
			return
		}
		ref := cell.GetReference()
		if ref == nil {
			return
		}
		fi, ok := ref.(domain.FileInfo)
		if !ok || !fi.IsDir {
			if rp.onFileAction != nil {
				rp.onFileAction(fi)
			}
			return
		}
		rp.NavigateInto(fi.Name)
	})

	// Initial state: show "Connecting..." placeholder
	rp.ShowConnecting()
}

// ShowConnecting displays the connecting state with centered placeholder text.
func (rp *RemotePane) ShowConnecting() {
	rp.Clear()
	rp.connected = false
	rp.SetTitle(fmt.Sprintf(" %s@%s -- Connecting... ", rp.server.User, rp.server.Host))
	rp.SetSelectable(false, false)

	cell := tview.NewTableCell("Connecting...").
		SetTextColor(tcell.Color245). // SecondaryText / Connecting Text
		SetAlign(tview.AlignCenter)
	rp.SetCell(0, 0, cell)
}

// ShowError displays an SFTP connection error message.
func (rp *RemotePane) ShowError(errMsg string) {
	rp.Clear()
	rp.connected = false
	shortMsg := trimError(errMsg, 60)
	rp.SetTitle(fmt.Sprintf(" %s@%s -- [#FF6B6B]Error[-] ", rp.server.User, rp.server.Host))
	rp.SetSelectable(false, false)

	headerCell := tview.NewTableCell("[#FF6B6B]SFTP connection failed:[-]").
		SetAlign(tview.AlignCenter)
	rp.SetCell(0, 0, headerCell)

	errCell := tview.NewTableCell(fmt.Sprintf("[#FF6B6B]%s[-]", shortMsg)).
		SetAlign(tview.AlignCenter)
	rp.SetCell(1, 0, errCell)
}

// ShowConnected enables selection and loads the initial directory listing.
func (rp *RemotePane) ShowConnected() {
	rp.connected = true
	rp.SetSelectable(true, false)
	rp.Refresh()
}

// Refresh reloads the file listing for the current remote path.
func (rp *RemotePane) Refresh() {
	if !rp.connected {
		return
	}
	entries, err := rp.sftpService.ListDir(rp.currentPath, rp.showHidden, rp.sortMode.Field(), rp.sortMode.Ascending())
	if err != nil {
		rp.log.Errorw("failed to list remote directory", "path", rp.currentPath, "error", err)
		rp.clearAndShowEmpty("Error: " + err.Error())
		return
	}
	if len(entries) == 0 {
		rp.clearAndShowEmpty("Empty directory")
		return
	}
	rp.populateTable(entries)
}

// populateTable fills the table with remote file entries.
func (rp *RemotePane) populateTable(entries []domain.FileInfo) {
	rp.Clear()

	// Header row
	headerStyle := tcell.StyleDefault.Bold(true).Foreground(tcell.Color255).Background(tcell.Color235)
	headers := []struct {
		text      string
		align     int
		maxWidth  int
		expansion int
	}{
		{"Name", tview.AlignLeft, 0, 1},
		{"Size", tview.AlignRight, 8, 0},
		{"Modified", tview.AlignLeft, 16, 0},
		{"Permissions", tview.AlignLeft, 10, 0},
	}

	for col, h := range headers {
		cell := tview.NewTableCell(h.text).
			SetStyle(headerStyle).
			SetAlign(h.align).
			SetMaxWidth(h.maxWidth).
			SetExpansion(h.expansion).
			SetSelectable(false)
		rp.SetCell(0, col, cell)
	}

	// Data rows
	for i, fi := range entries {
		row := i + 1

		// Name cell
		nameText := fi.Name
		if fi.IsDir {
			nameText += "/"
		}
		nameColor := tcell.Color252 // regular file color
		if fi.IsDir {
			nameColor = tcell.Color33 // blue for directories (D-03)
		}
		if rp.selected[fi.Name] {
			nameText = "* " + nameText
			nameColor = tcell.GetColor("#FFD700") // gold for selected (UI-SPEC)
		}
		nameCell := tview.NewTableCell(nameText).
			SetTextColor(nameColor).
			SetAlign(tview.AlignLeft).
			SetExpansion(1).
			SetReference(fi)
		rp.SetCell(row, 0, nameCell)

		// Size cell
		sizeText := "-"
		if !fi.IsDir {
			sizeText = formatSize(fi.Size)
		}
		sizeCell := tview.NewTableCell(sizeText).
			SetTextColor(tcell.Color245).
			SetAlign(tview.AlignRight).
			SetMaxWidth(8)
		rp.SetCell(row, 1, sizeCell)

		// Modified cell
		modText := fi.ModTime.Format("2006-01-02 15:04")
		modCell := tview.NewTableCell(modText).
			SetTextColor(tcell.Color245).
			SetAlign(tview.AlignLeft).
			SetMaxWidth(16)
		rp.SetCell(row, 2, modCell)

		// Permissions cell
		permText := fi.Mode.String()
		permCell := tview.NewTableCell(permText).
			SetTextColor(tcell.Color245).
			SetAlign(tview.AlignLeft).
			SetMaxWidth(10)
		rp.SetCell(row, 3, permCell)
	}

	// Select first data row
	if len(entries) > 0 {
		rp.Select(1, 0)
	}

	rp.UpdateTitle()
}

// clearAndShowEmpty clears the table and shows a centered message.
func (rp *RemotePane) clearAndShowEmpty(msg string) {
	rp.Clear()
	cell := tview.NewTableCell(msg).
		SetTextColor(tcell.Color245).
		SetAlign(tview.AlignCenter)
	rp.SetCell(0, 0, cell)
}

// NavigateToParent goes up one directory level.
func (rp *RemotePane) NavigateToParent() {
	if !rp.connected {
		return
	}
	// Simple path splitting for remote paths (Unix-style)
	parent := parentPath(rp.currentPath)
	if parent == rp.currentPath {
		return // already at root
	}
	rp.currentPath = parent
	rp.selected = make(map[string]bool)
	rp.Refresh()
}

// NavigateInto enters a subdirectory.
func (rp *RemotePane) NavigateInto(dirName string) {
	if !rp.connected {
		return
	}
	rp.currentPath = joinPath(rp.currentPath, dirName)
	rp.selected = make(map[string]bool)
	rp.Refresh()
	if rp.onPathChange != nil {
		rp.onPathChange(rp.currentPath)
	}
}

// ToggleHidden flips the hidden files visibility flag.
func (rp *RemotePane) ToggleHidden() {
	rp.showHidden = !rp.showHidden
	rp.Refresh()
}

// ToggleSelection toggles the multi-select state for the current row.
func (rp *RemotePane) ToggleSelection() {
	if !rp.connected {
		return
	}
	row, _ := rp.GetSelection()
	if row < 1 {
		return
	}
	cell := rp.GetCell(row, 0)
	if cell == nil {
		return
	}
	ref := cell.GetReference()
	if ref == nil {
		return
	}
	fi, ok := ref.(domain.FileInfo)
	if !ok {
		return
	}
	rp.selected[fi.Name] = !rp.selected[fi.Name]
	rp.Refresh()
}

// SelectedFiles returns all FileInfo entries that are currently selected.
func (rp *RemotePane) SelectedFiles() []domain.FileInfo {
	if !rp.connected {
		return nil
	}
	entries, err := rp.sftpService.ListDir(rp.currentPath, rp.showHidden, rp.sortMode.Field(), rp.sortMode.Ascending())
	if err != nil {
		return nil
	}
	var result []domain.FileInfo
	for _, fi := range entries {
		if rp.selected[fi.Name] {
			result = append(result, fi)
		}
	}
	return result
}

// UpdateTitle sets the pane title with server info, path, and sort mode.
func (rp *RemotePane) UpdateTitle() {
	if !rp.connected {
		return
	}
	title := fmt.Sprintf(" %s@%s:%s -- Sort: %s ", rp.server.User, rp.server.Host, rp.currentPath, rp.sortMode.String())
	rp.SetTitle(title)
}

// SetFocused updates the border color based on focus state.
func (rp *RemotePane) SetFocused(focused bool) {
	if focused {
		rp.SetBorderColor(tcell.Color248) // brighter when focused
	} else {
		rp.SetBorderColor(tcell.Color238) // dimmer when unfocused
	}
}

// OnPathChange sets a callback invoked when the user navigates to a new directory.
func (rp *RemotePane) OnPathChange(fn func(path string)) *RemotePane {
	rp.onPathChange = fn
	return rp
}

// OnFileAction sets a callback invoked when the user presses Enter on a non-directory file.
func (rp *RemotePane) OnFileAction(fn func(fi domain.FileInfo)) *RemotePane {
	rp.onFileAction = fn
	return rp
}

// GetSortMode returns the current sort mode.
func (rp *RemotePane) GetSortMode() FileSortMode {
	return rp.sortMode
}

// SetSortMode updates the sort mode and refreshes the listing.
func (rp *RemotePane) SetSortMode(mode FileSortMode) {
	rp.sortMode = mode
	if rp.connected {
		rp.Refresh()
	}
}

// GetCurrentPath returns the current remote directory path.
func (rp *RemotePane) GetCurrentPath() string {
	return rp.currentPath
}

// IsConnected returns whether the SFTP connection is active.
func (rp *RemotePane) IsConnected() bool {
	return rp.connected
}

// parentPath returns the parent directory of a Unix-style path.
func parentPath(p string) string {
	if p == "" || p == "/" || p == "~" {
		return p
	}
	// Remove trailing slash
	p = strings.TrimRight(p, "/")
	idx := strings.LastIndex(p, "/")
	if idx <= 0 {
		return "/"
	}
	return p[:idx]
}

// joinPath joins a base path with a directory name using Unix-style separators.
func joinPath(base, name string) string {
	if strings.HasSuffix(base, "/") {
		return base + name
	}
	return base + "/" + name
}
