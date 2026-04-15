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
	"github.com/rivo/tview"
)

// ConfirmDialog is an overlay component for yes/no confirmation prompts.
// It follows the TransferModal/RecentDirs overlay pattern: embeds *tview.Box,
// uses a visible flag, and implements Draw() and HandleKey() for manual lifecycle
// management outside tview's focus system.
//
// Use cases:
//   - Delete confirmation (single file details or multi-select summary)
//   - Recursive delete warning with detail line
//
// Layout (centered popup):
//
//	Row 0: title (e.g. "Delete file?") in cancelWarningColor
//	Row 1: message (file details or summary) in tcell.Color255
//	Row 2: detail (optional, recursive warning) in conflictWarningColor
//	Row 3: empty separator
//	Row 4: "[y] Yes  [n] No" in tcell.Color255
//	Footer: "Press Esc to cancel" in tcell.Color245
type ConfirmDialog struct {
	*tview.Box
	app       *tview.Application
	visible   bool
	title     string // e.g. "Delete file?"
	message   string // e.g. "config.yaml (1.2K, File, 2024-03-15)"
	detail    string // e.g. "Directory not empty, all contents will be deleted" (D-05)
	onConfirm func()
	onCancel  func()
}

// NewConfirmDialog creates a new ConfirmDialog overlay component.
// The dialog starts invisible and must be shown via Show().
func NewConfirmDialog(app *tview.Application) *ConfirmDialog {
	cd := &ConfirmDialog{
		Box:     tview.NewBox(),
		app:     app,
		visible: false,
	}
	cd.SetBorder(true).
		SetBorderColor(tcell.Color238).
		SetTitleColor(tcell.Color250).
		SetBackgroundColor(tcell.Color232)
	return cd
}

// Show makes the dialog visible and sets its content.
// title is the prompt header (e.g. "Delete file?").
// message is the main information line (e.g. file details or batch summary).
// detail is an optional warning line (e.g. recursive delete notice), may be empty.
func (cd *ConfirmDialog) Show(title, message, detail string) {
	cd.visible = true
	cd.title = title
	cd.message = message
	cd.detail = detail
}

// Hide hides the dialog.
func (cd *ConfirmDialog) Hide() {
	cd.visible = false
}

// IsVisible returns whether the dialog is currently displayed.
func (cd *ConfirmDialog) IsVisible() bool {
	return cd.visible
}

// SetMessage updates the dialog message (main information line).
func (cd *ConfirmDialog) SetMessage(msg string) {
	cd.message = msg
}

// SetDetail updates the detail line (warning/info below the message).
func (cd *ConfirmDialog) SetDetail(detail string) {
	cd.detail = detail
}

// SetWarning updates the warning line. Alias for SetDetail, used for recursive
// delete warnings to make intent clear at call sites.
func (cd *ConfirmDialog) SetWarning(warning string) {
	cd.detail = warning
}

// SetOnConfirm sets the callback invoked when the user presses 'y'.
func (cd *ConfirmDialog) SetOnConfirm(fn func()) {
	cd.onConfirm = fn
}

// SetOnCancel sets the callback invoked when the user presses 'n' or Esc.
func (cd *ConfirmDialog) SetOnCancel(fn func()) {
	cd.onCancel = fn
}

// Draw renders the confirmation dialog to the screen.
// No-op when not visible.
//
// Rendering order per Pitfall 3: SetRect -> DrawForSubclass -> fill bg -> tview.Print.
// Layout:
//   - Width: 60% of terminal (max 70 columns)
//   - Height: 7 (title + message + optional detail + separator + options + footer)
//   - Centered on screen
func (cd *ConfirmDialog) Draw(screen tcell.Screen) {
	if !cd.visible {
		return
	}

	termWidth, termHeight := screen.Size()

	// Calculate popup dimensions (Pitfall 6: responsive sizing)
	width := termWidth * 60 / 100
	if width > 70 {
		width = 70
	}
	height := 7
	if cd.detail != "" {
		height = 8
	}
	x := (termWidth - width) / 2
	y := (termHeight - height) / 2

	// Position and draw border/background (Pitfall 4: SetRect before DrawForSubclass)
	cd.SetTitle(" " + cd.title + " ")
	cd.SetRect(x, y, width, height)
	cd.Box.DrawForSubclass(screen, cd)

	ix, iy, iw, _ := cd.GetInnerRect()

	// Row 0: title (gold warning color)
	row := iy
	tview.Print(screen, cd.title, ix, row, iw, tview.AlignCenter, cancelWarningColor)

	// Row 1: message
	row++
	tview.Print(screen, cd.message, ix, row, iw, tview.AlignCenter, tcell.Color255)

	// Row 2: detail (optional, orange warning color)
	row++
	if cd.detail != "" {
		tview.Print(screen, cd.detail, ix, row, iw, tview.AlignCenter, conflictWarningColor)
		row++ // extra row for separator
	}

	// Separator row
	row++

	// Options row: "[y] Yes  [n] No"
	tview.Print(screen, "[y] Yes  [n] No", ix, row, iw, tview.AlignCenter, tcell.Color255)

	// Footer: "Press Esc to cancel"
	tview.Print(screen, "Press Esc to cancel", ix, iy+height-2, iw, tview.AlignCenter, tcell.Color245)
}

// HandleKey processes keyboard input for the confirmation dialog.
// When visible, all keys are consumed (return nil) per D-08 full interception.
// 'y' triggers onConfirm, 'n' and Esc trigger onCancel, all hide the dialog.
func (cd *ConfirmDialog) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if !cd.visible {
		return event
	}

	switch event.Rune() {
	case 'y':
		cd.Hide()
		if cd.onConfirm != nil {
			cd.onConfirm()
		}
		return nil
	case 'n':
		cd.Hide()
		if cd.onCancel != nil {
			cd.onCancel()
		}
		return nil
	}

	if event.Key() == tcell.KeyEscape {
		cd.Hide()
		if cd.onCancel != nil {
			cd.onCancel()
		}
		return nil
	}

	// Consume all other keys when visible (Pitfall 4: full key interception)
	return nil
}
