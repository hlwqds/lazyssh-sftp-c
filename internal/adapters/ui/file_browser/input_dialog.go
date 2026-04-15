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

// InputDialog is an overlay component for text input prompts.
// It follows the TransferModal/RecentDirs overlay pattern: embeds *tview.Box,
// uses a visible flag, and implements Draw() and HandleKey() for manual lifecycle
// management outside tview's focus system.
//
// Internally embeds a tview.InputField for text editing. Key routing is handled
// via HandleKey -> inputField.InputHandler() without using tview's focus system
// (per Claude's Discretion + ARCHITECTURE.md Pattern 2).
//
// Use cases:
//   - Rename file/directory (R key): pre-filled with current name
//   - New directory (m key): empty input
//
// Layout (centered popup):
//
//	Row 0: title (e.g. "Rename") in tcell.Color255
//	Row 1: label + InputField (e.g. "Name: " + text input)
//	Row 2: empty separator
//	Row 3: "[Enter] Confirm  [Esc] Cancel" in tcell.Color245
type InputDialog struct {
	*tview.Box
	app        *tview.Application
	visible    bool
	title      string // e.g. "Rename" or "New Directory"
	label      string // e.g. "Name: "
	inputField *tview.InputField
	onSubmit   func(text string)
	onCancel   func()
}

// NewInputDialog creates a new InputDialog overlay component.
// The dialog starts invisible and must be shown via Show().
// Enter/Esc handling is set up via tview.InputField's doneFunc callback,
// avoiding double-trigger issues (Pitfall 3).
func NewInputDialog(app *tview.Application) *InputDialog {
	id := &InputDialog{
		Box:     tview.NewBox(),
		app:     app,
		visible: false,
	}
	id.inputField = tview.NewInputField()
	id.inputField.SetFieldBackgroundColor(tcell.Color236)
	id.inputField.SetFieldTextColor(tcell.Color255)
	id.inputField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			text := id.inputField.GetText()
			if text != "" {
				if id.onSubmit != nil {
					id.onSubmit(text)
				}
				id.Hide()
			}
			// Empty text: don't hide, keep dialog open for input
		} else if key == tcell.KeyEscape {
			if id.onCancel != nil {
				id.onCancel()
			}
			id.Hide()
		}
	})
	id.SetBorder(true).
		SetBorderColor(tcell.Color238).
		SetTitleColor(tcell.Color250).
		SetBackgroundColor(tcell.Color232)
	return id
}

// Show makes the dialog visible and sets its content.
// title is the prompt header (e.g. "Rename" or "New Directory").
// label is the input field label (e.g. "Name: ").
// text is the pre-filled text for the input field.
func (id *InputDialog) Show(title, label, text string) {
	id.visible = true
	id.title = title
	id.label = label
	id.inputField.SetText(text)
}

// Hide hides the dialog.
func (id *InputDialog) Hide() {
	id.visible = false
}

// IsVisible returns whether the dialog is currently displayed.
func (id *InputDialog) IsVisible() bool {
	return id.visible
}

// SetTitle updates the dialog title.
func (id *InputDialog) SetTitle(title string) {
	id.title = title
}

// SetText sets the InputField text content.
func (id *InputDialog) SetText(text string) {
	id.inputField.SetText(text)
}

// SetLabel sets the InputField label.
func (id *InputDialog) SetLabel(label string) {
	id.label = label
}

// GetText returns the current InputField text.
func (id *InputDialog) GetText() string {
	return id.inputField.GetText()
}

// SetOnSubmit sets the callback invoked when the user presses Enter with non-empty text.
func (id *InputDialog) SetOnSubmit(fn func(text string)) {
	id.onSubmit = fn
}

// SetOnCancel sets the callback invoked when the user presses Esc.
func (id *InputDialog) SetOnCancel(fn func()) {
	id.onCancel = fn
}

// Draw renders the input dialog to the screen.
// No-op when not visible.
//
// Rendering order per Pitfall 3: SetRect -> DrawForSubclass -> fill bg -> render InputField.
// Layout:
//   - Width: 60% of terminal (max 60 columns)
//   - Height: 7 (fixed)
//   - Centered on screen
func (id *InputDialog) Draw(screen tcell.Screen) {
	if !id.visible {
		return
	}

	termWidth, termHeight := screen.Size()

	// Calculate popup dimensions
	width := termWidth * 60 / 100
	if width > 60 {
		width = 60
	}
	height := 7
	x := (termWidth - width) / 2
	y := (termHeight - height) / 2

	// Position and draw border/background (Pitfall 4: SetRect before DrawForSubclass)
	id.SetTitle(" " + id.title + " ")
	id.SetRect(x, y, width, height)
	id.Box.DrawForSubclass(screen, id)

	ix, iy, iw, _ := id.GetInnerRect()

	// Row 0: title
	row := iy
	tview.Print(screen, id.title, ix, row, iw, tview.AlignCenter, tcell.Color255)

	// Row 1: label + InputField
	row++
	// Render label text
	tview.Print(screen, id.label, ix+1, row, iw-2, tview.AlignLeft, tcell.Color248)
	// Position and draw InputField to the right of the label
	labelWidth := len(id.label)
	if labelWidth > iw-4 {
		labelWidth = iw - 4
	}
	inputFieldX := ix + 1 + labelWidth
	inputFieldWidth := iw - 2 - labelWidth
	if inputFieldWidth < 1 {
		inputFieldWidth = 1
	}
	id.inputField.SetRect(inputFieldX, row, inputFieldWidth, 1)
	id.inputField.Draw(screen)

	// Row 2: empty separator
	row++

	// Row 3: footer hint
	tview.Print(screen, "[Enter] Confirm  [Esc] Cancel", ix, row+1, iw, tview.AlignCenter, tcell.Color245)
}

// HandleKey processes keyboard input for the input dialog.
// When visible, all keys are routed to inputField.InputHandler() and consumed (return nil).
//
// Key design: Enter/Esc are handled exclusively by InputField's doneFunc (set in NewInputDialog).
// HandleKey does NOT check for Enter/Esc to avoid Pitfall 3 (double-trigger).
func (id *InputDialog) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if !id.visible {
		return event
	}

	// Route all keys to the InputField (per Claude's Discretion)
	handler := id.inputField.InputHandler()
	handler(event, func(tview.Primitive) {})

	// Consume all keys when visible (full key interception, per D-08)
	return nil
}
