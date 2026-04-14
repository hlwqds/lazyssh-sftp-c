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
	"path/filepath"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"github.com/Adembc/lazyssh/internal/core/ports"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"go.uber.org/zap"
)

// LocalPane is a tview.Table-based component for browsing local files.
// It displays file listings in 4 columns: Name, Size, Modified, Permissions.
type LocalPane struct {
	*tview.Table
	log          *zap.SugaredLogger
	fileService  ports.FileService
	currentPath  string
	sortMode     FileSortMode
	showHidden   bool
	selected     map[string]bool // multi-select state: file name -> selected
	onPathChange func(path string)
	onFileAction func(fi domain.FileInfo)
}

// NewLocalPane creates a new LocalPane for browsing the local filesystem.
func NewLocalPane(log *zap.SugaredLogger, fs ports.FileService, initialPath string) *LocalPane {
	lp := &LocalPane{
		Table:       tview.NewTable(),
		log:         log,
		fileService: fs,
		currentPath: initialPath,
		sortMode:    FileSortByNameAsc,
		showHidden:  false,
		selected:    make(map[string]bool),
	}
	lp.build()
	lp.SetBackgroundColor(tcell.ColorDefault) // blend with kitty's native background
	return lp
}

// build configures the Table with selection, borders, header, and key handling.
func (lp *LocalPane) build() {
	lp.SetSelectable(true, false) // row selection only (per UI-SPEC Pitfall 1)
	lp.SetBorder(true).
		SetBorderColor(tcell.Color238).
		SetTitleColor(tcell.Color250).
		SetTitleAlign(tview.AlignLeft)

	// Intercept pane-specific keys; pass through to Table for j/k/arrows
	lp.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'h':
			lp.NavigateToParent()
			return nil
		case ' ':
			lp.ToggleSelection()
			return nil
		case '.':
			lp.ToggleHidden()
			return nil
		}
		//nolint:exhaustive // We only handle specific keys and pass through others
		switch event.Key() {
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			lp.NavigateToParent()
			return nil
		}
		return event // pass to Table built-in for j/k/arrows/PgUp/PgDn/Enter
	})

	// Enter key: navigate into directory
	lp.SetSelectedFunc(func(row, _ int) {
		cell := lp.GetCell(row, 0)
		if cell == nil {
			return
		}
		ref := cell.GetReference()
		if ref == nil {
			return
		}
		fi, ok := ref.(domain.FileInfo)
		if !ok || !fi.IsDir {
			if lp.onFileAction != nil {
				lp.onFileAction(fi)
			}
			return
		}
		lp.NavigateInto(fi.Name)
	})

	lp.UpdateTitle()
}

// Refresh reloads the file listing for the current path.
func (lp *LocalPane) Refresh() {
	entries, err := lp.fileService.ListDir(lp.currentPath, lp.showHidden, lp.sortMode.Field(), lp.sortMode.Ascending())
	if err != nil {
		lp.log.Errorw("failed to list directory", "path", lp.currentPath, "error", err)
		lp.clearAndShowEmpty("Error: " + err.Error())
		return
	}
	if len(entries) == 0 {
		lp.clearAndShowEmpty("Empty directory")
		return
	}
	lp.populateTable(entries)
}

// populateTable fills the table with file entries.
func (lp *LocalPane) populateTable(entries []domain.FileInfo) {
	lp.Clear()

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
		lp.SetCell(0, col, cell)
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
		if lp.selected[fi.Name] {
			nameText = "* " + nameText
			nameColor = tcell.GetColor("#FFD700") // gold for selected (UI-SPEC)
		}
		nameCell := tview.NewTableCell(nameText).
			SetTextColor(nameColor).
			SetAlign(tview.AlignLeft).
			SetExpansion(1).
			SetReference(fi)
		lp.SetCell(row, 0, nameCell)

		// Size cell
		sizeText := "-"
		if !fi.IsDir {
			sizeText = formatSize(fi.Size)
		}
		sizeCell := tview.NewTableCell(sizeText).
			SetTextColor(tcell.Color245). // SecondaryText
			SetAlign(tview.AlignRight).
			SetMaxWidth(8)
		lp.SetCell(row, 1, sizeCell)

		// Modified cell
		modText := fi.ModTime.Format("2006-01-02 15:04")
		modCell := tview.NewTableCell(modText).
			SetTextColor(tcell.Color245).
			SetAlign(tview.AlignLeft).
			SetMaxWidth(16)
		lp.SetCell(row, 2, modCell)

		// Permissions cell
		permText := fi.Mode.String()
		permCell := tview.NewTableCell(permText).
			SetTextColor(tcell.Color245).
			SetAlign(tview.AlignLeft).
			SetMaxWidth(10)
		lp.SetCell(row, 3, permCell)
	}

	// Select first data row
	if len(entries) > 0 {
		lp.Select(1, 0)
	}

	lp.UpdateTitle()
}

// clearAndShowEmpty clears the table and shows a centered message.
func (lp *LocalPane) clearAndShowEmpty(msg string) {
	lp.Clear()
	cell := tview.NewTableCell(msg).
		SetTextColor(tcell.Color245). // SecondaryText
		SetAlign(tview.AlignCenter)
	lp.SetCell(0, 0, cell)
}

// NavigateToParent goes up one directory level.
func (lp *LocalPane) NavigateToParent() {
	parent := filepath.Dir(lp.currentPath)
	if parent == lp.currentPath {
		return // already at root
	}
	lp.currentPath = parent
	lp.selected = make(map[string]bool)
	lp.Refresh()
}

// NavigateInto enters a subdirectory.
func (lp *LocalPane) NavigateInto(dirName string) {
	lp.currentPath = filepath.Join(lp.currentPath, dirName)
	lp.selected = make(map[string]bool)
	lp.Refresh()
	if lp.onPathChange != nil {
		lp.onPathChange(lp.currentPath)
	}
}

// ToggleHidden flips the hidden files visibility flag.
func (lp *LocalPane) ToggleHidden() {
	lp.showHidden = !lp.showHidden
	lp.Refresh()
}

// ToggleSelection toggles the multi-select state for the current row.
func (lp *LocalPane) ToggleSelection() {
	row, _ := lp.GetSelection()
	if row < 1 {
		return // header row or no selection
	}
	cell := lp.GetCell(row, 0)
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
	lp.selected[fi.Name] = !lp.selected[fi.Name]
	lp.Refresh()
}

// SelectedFiles returns all FileInfo entries that are currently selected.
func (lp *LocalPane) SelectedFiles() []domain.FileInfo {
	entries, err := lp.fileService.ListDir(lp.currentPath, lp.showHidden, lp.sortMode.Field(), lp.sortMode.Ascending())
	if err != nil {
		return nil
	}
	var result []domain.FileInfo
	for _, fi := range entries {
		if lp.selected[fi.Name] {
			result = append(result, fi)
		}
	}
	return result
}

// UpdateTitle sets the pane title with the current path and sort mode.
func (lp *LocalPane) UpdateTitle() {
	title := fmt.Sprintf(" %s -- Sort: %s ", lp.currentPath, lp.sortMode.String())
	lp.SetTitle(title)
}

// SetFocused updates the border color based on focus state.
func (lp *LocalPane) SetFocused(focused bool) {
	if focused {
		lp.SetBorderColor(tcell.Color248) // brighter when focused
	} else {
		lp.SetBorderColor(tcell.Color238) // dimmer when unfocused
	}
}

// OnPathChange sets a callback invoked when the user navigates to a new directory.
func (lp *LocalPane) OnPathChange(fn func(path string)) *LocalPane {
	lp.onPathChange = fn
	return lp
}

// OnFileAction sets a callback invoked when the user presses Enter on a non-directory file.
func (lp *LocalPane) OnFileAction(fn func(fi domain.FileInfo)) *LocalPane {
	lp.onFileAction = fn
	return lp
}

// GetSortMode returns the current sort mode.
func (lp *LocalPane) GetSortMode() FileSortMode {
	return lp.sortMode
}

// SetSortMode updates the sort mode and refreshes the listing.
func (lp *LocalPane) SetSortMode(mode FileSortMode) {
	lp.sortMode = mode
	lp.Refresh()
}

// GetCurrentPath returns the current directory path.
func (lp *LocalPane) GetCurrentPath() string {
	return lp.currentPath
}

// formatSize converts a byte count to a human-readable string.
func formatSize(bytes int64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%.1fG", float64(bytes)/float64(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1fM", float64(bytes)/float64(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.1fK", float64(bytes)/float64(1<<10))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
