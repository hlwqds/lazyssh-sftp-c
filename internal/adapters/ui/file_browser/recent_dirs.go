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
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"go.uber.org/zap"
)

// maxRecentDirs is the maximum number of directory paths retained in the MRU list.
const maxRecentDirs = 10

// RecentDirs maintains a persistent MRU (Most Recently Used) list of remote directory paths.
// It follows the TransferModal overlay pattern: embeds *tview.Box, uses visible flag.
// Paths are persisted to disk at ~/.lazyssh/recent-dirs/{user@host}.json.
// Phase 4 provides the data layer; Phase 5 adds Draw() rendering and HandleKey() navigation.
type RecentDirs struct {
	*tview.Box
	paths         []string
	visible       bool
	selectedIndex int
	onSelect      func(path string)
	currentPath   string
	log           *zap.SugaredLogger
	serverKey     string // "user@host" for per-server isolation
	filePath      string // absolute path to the JSON persistence file
}

// NewRecentDirs creates a new RecentDirs overlay component.
// It loads previously persisted paths from ~/.lazyssh/recent-dirs/{user@host}.json.
// The serverKey parameter should be in "user@host" format for per-server isolation.
func NewRecentDirs(log *zap.SugaredLogger, serverHost, serverUser string) *RecentDirs {
	serverKey := serverUser + "@" + serverHost
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	filePath := filepath.Join(homeDir, ".lazyssh", "recent-dirs", serverKey+".json")

	rd := &RecentDirs{
		Box:       tview.NewBox(),
		paths:     make([]string, 0, maxRecentDirs),
		visible:   false,
		log:       log,
		serverKey: serverKey,
		filePath:  filePath,
	}
	rd.SetBorder(true).
		SetBorderColor(tcell.Color238).
		SetTitleColor(tcell.Color250).
		SetBackgroundColor(tcell.Color232)
	rd.loadFromDisk()
	return rd
}

// Record adds a path to the MRU list. If the path already exists, it is moved to the front.
// Relative paths (starting with ".") are silently skipped.
// Paths are normalized by removing trailing slashes.
// The list is capped at maxRecentDirs entries; oldest entries are truncated.
// After updating the in-memory list, the change is persisted to disk.
func (rd *RecentDirs) Record(path string) {
	normalized := strings.TrimRight(path, "/")
	if strings.HasPrefix(normalized, ".") {
		return
	}
	// Move-to-front deduplication: remove existing entry if present
	for i, p := range rd.paths {
		if p == normalized {
			rd.paths = append(rd.paths[:i], rd.paths[i+1:]...)
			break
		}
	}
	// Prepend to front
	rd.paths = append([]string{normalized}, rd.paths...)
	// Truncate to max
	if len(rd.paths) > maxRecentDirs {
		rd.paths = rd.paths[:maxRecentDirs]
	}
	rd.saveToDisk()
}

// loadFromDisk loads previously persisted paths from the JSON file.
// If the file does not exist, the paths slice remains empty (silent, no error).
func (rd *RecentDirs) loadFromDisk() {
	if _, err := os.Stat(rd.filePath); os.IsNotExist(err) {
		return
	}

	data, err := os.ReadFile(rd.filePath)
	if err != nil {
		rd.log.Errorw("failed to read recent dirs file", "path", rd.filePath, "error", err)
		return
	}

	if len(data) == 0 {
		return
	}

	var paths []string
	if err := json.Unmarshal(data, &paths); err != nil {
		rd.log.Errorw("failed to parse recent dirs JSON", "path", rd.filePath, "error", err)
		return
	}

	rd.paths = paths
}

// saveToDisk persists the current path list to disk as JSON.
// Errors are logged but do not propagate — Record should not fail due to I/O issues.
func (rd *RecentDirs) saveToDisk() {
	dir := filepath.Dir(rd.filePath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		rd.log.Errorw("failed to create recent dirs directory", "path", dir, "error", err)
		return
	}

	data, err := json.MarshalIndent(rd.paths, "", "  ")
	if err != nil {
		rd.log.Errorw("failed to marshal recent dirs", "path", rd.filePath, "error", err)
		return
	}

	if err := os.WriteFile(rd.filePath, data, 0o600); err != nil {
		rd.log.Errorw("failed to write recent dirs file", "path", rd.filePath, "error", err)
	}
}

// GetPaths returns a copy of the MRU path list (most recent first).
func (rd *RecentDirs) GetPaths() []string {
	result := make([]string, len(rd.paths))
	copy(result, rd.paths)
	return result
}

// Draw renders the recent dirs overlay to the screen.
// Layout per CONTEXT.md D-01/D-03/UI-SPEC:
//   - Width: 60% of terminal (max 80 columns)
//   - Height: len(paths)+2 (min 5, max 15)
//   - Centered on screen
//
// Rendering order per Pitfall 3: SetRect -> Box.DrawForSubclass -> fill selected row bg -> tview.Print text.
func (rd *RecentDirs) Draw(screen tcell.Screen) {
	if !rd.visible {
		return
	}

	termWidth, termHeight := screen.Size()

	// Calculate popup dimensions (D-01, D-03, Pitfall 6)
	width := termWidth * 60 / 100
	if width > 80 {
		width = 80
	}
	height := len(rd.paths) + 2
	if height < 5 {
		height = 5 // minimum for empty state text
	}
	if height > 15 {
		height = 15
	}
	x := (termWidth - width) / 2
	y := (termHeight - height) / 2

	// Position and draw border/background (Pitfall 4: SetRect before DrawForSubclass)
	rd.SetTitle(" Recent Directories ") // D-13
	rd.SetRect(x, y, width, height)
	rd.Box.DrawForSubclass(screen, rd)

	ix, iy, iw, ih := rd.GetInnerRect()

	if len(rd.paths) == 0 {
		// Empty state: centered "暂无最近目录" (D-06, POPUP-05)
		tview.Print(screen, "暂无最近目录", ix, iy+ih/2, iw, tview.AlignCenter, tcell.Color240)
		return
	}

	// List rendering: background fill BEFORE text (Pitfall 3)
	for i, path := range rd.paths {
		row := iy + i
		if row >= iy+ih {
			break
		}

		// Determine colors (D-04, D-05, AUX-01)
		fgColor := tcell.Color250 // default white text
		isSelected := i == rd.selectedIndex
		isCurrent := path == rd.currentPath

		if isCurrent {
			fgColor = tcell.ColorYellow // AUX-01: current path in yellow
		}

		// Fill selected row background first (Pitfall 3: bg before text)
		if isSelected {
			bgStyle := tcell.StyleDefault.Background(tcell.Color236) // D-04
			for col := ix; col < ix+iw; col++ {
				screen.SetContent(col, row, ' ', nil, bgStyle)
			}
		}

		// Render path text on top of background
		tview.Print(screen, path, ix+1, row, iw-2, tview.AlignLeft, fgColor)
	}
}

// Show makes the overlay visible and resets selection to the first item.
func (rd *RecentDirs) Show() {
	rd.visible = true
	rd.selectedIndex = 0
}

// Hide hides the overlay.
func (rd *RecentDirs) Hide() {
	rd.visible = false
}

// IsVisible returns whether the overlay is currently displayed.
func (rd *RecentDirs) IsVisible() bool {
	return rd.visible
}

// SetOnSelect sets the callback invoked when the user presses Enter on a list item.
func (rd *RecentDirs) SetOnSelect(fn func(path string)) {
	rd.onSelect = fn
}

// SetCurrentPath sets the current remote directory path for highlighting (AUX-01).
// Trailing slashes are stripped for consistent comparison.
func (rd *RecentDirs) SetCurrentPath(path string) {
	rd.currentPath = strings.TrimRight(path, "/")
}

// GetCurrentPath returns the stored current path (for testing).
func (rd *RecentDirs) GetCurrentPath() string {
	return rd.currentPath
}

// GetSelectedIndex returns the current selection index (for testing).
func (rd *RecentDirs) GetSelectedIndex() int {
	return rd.selectedIndex
}

// HandleKey processes keyboard input for the recent dirs popup.
// When visible, all keys are consumed (return nil) per D-08.
// j/k/Down/Up navigate selection, Enter selects, Esc hides.
func (rd *RecentDirs) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if !rd.visible {
		return event
	}

	switch event.Key() { //nolint:exhaustive // keyboard handler: intentionally handles only specific keys
	case tcell.KeyEscape:
		rd.Hide()
		return nil
	case tcell.KeyEnter:
		if len(rd.paths) > 0 && rd.onSelect != nil {
			rd.onSelect(rd.paths[rd.selectedIndex])
		}
		return nil
	case tcell.KeyDown:
		rd.selectedIndex++
		if rd.selectedIndex >= len(rd.paths) {
			rd.selectedIndex = len(rd.paths) - 1
		}
		return nil
	case tcell.KeyUp:
		rd.selectedIndex--
		if rd.selectedIndex < 0 {
			rd.selectedIndex = 0
		}
		return nil
	}

	switch event.Rune() {
	case 'j':
		rd.selectedIndex++
		if rd.selectedIndex >= len(rd.paths) {
			rd.selectedIndex = len(rd.paths) - 1
		}
		return nil
	case 'k':
		rd.selectedIndex--
		if rd.selectedIndex < 0 {
			rd.selectedIndex = 0
		}
		return nil
	}

	// Full key interception: consume all other keys when visible (D-08)
	return nil
}
