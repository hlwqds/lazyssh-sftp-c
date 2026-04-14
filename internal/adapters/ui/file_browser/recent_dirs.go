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
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// maxRecentDirs is the maximum number of directory paths retained in the MRU list.
const maxRecentDirs = 10

// RecentDirs maintains an in-memory MRU (Most Recently Used) list of remote directory paths.
// It follows the TransferModal overlay pattern: embeds *tview.Box, uses visible flag.
// Phase 4 provides the data layer; Phase 5 adds Draw() rendering.
type RecentDirs struct {
	*tview.Box
	paths   []string
	visible bool
}

// NewRecentDirs creates a new RecentDirs overlay component with an empty path list.
func NewRecentDirs() *RecentDirs {
	rd := &RecentDirs{
		Box:     tview.NewBox(),
		paths:   make([]string, 0, maxRecentDirs),
		visible: false,
	}
	rd.SetBorder(true).
		SetBorderColor(tcell.Color238).
		SetTitleColor(tcell.Color250).
		SetBackgroundColor(tcell.Color232)
	return rd
}

// Record adds a path to the MRU list. If the path already exists, it is moved to the front.
// Relative paths (starting with ".") are silently skipped.
// Paths are normalized by removing trailing slashes.
// The list is capped at maxRecentDirs entries; oldest entries are truncated.
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
}

// GetPaths returns a copy of the MRU path list (most recent first).
func (rd *RecentDirs) GetPaths() []string {
	result := make([]string, len(rd.paths))
	copy(result, rd.paths)
	return result
}

// Draw renders the recent dirs overlay to the screen.
// Phase 5 will implement the actual list rendering.
func (rd *RecentDirs) Draw(screen tcell.Screen) {
	if !rd.visible {
		return
	}
	rd.Box.DrawForSubclass(screen, rd)
	// Phase 5: render directory list
}

// Show makes the overlay visible.
func (rd *RecentDirs) Show() {
	rd.visible = true
}

// Hide hides the overlay.
func (rd *RecentDirs) Hide() {
	rd.visible = false
}

// IsVisible returns whether the overlay is currently displayed.
func (rd *RecentDirs) IsVisible() bool {
	return rd.visible
}
