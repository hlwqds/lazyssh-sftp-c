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
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TestNewInputDialog verifies that NewInputDialog creates a dialog with visible=false and empty InputField.
func TestNewInputDialog(t *testing.T) {
	app := tview.NewApplication()
	id := NewInputDialog(app)
	if id == nil {
		t.Fatal("expected non-nil InputDialog")
	}
	if id.IsVisible() {
		t.Error("expected InputDialog to be invisible after creation")
	}
	if id.GetText() != "" {
		t.Errorf("expected empty InputField text, got %q", id.GetText())
	}
	if id.app != app {
		t.Error("expected app to be stored")
	}
}

// TestInputDialogShowHide verifies Show sets visible=true and Hide sets visible=false.
func TestInputDialogShowHide(t *testing.T) {
	app := tview.NewApplication()
	id := NewInputDialog(app)

	id.Show("Rename", "Name: ", "old_name.txt")
	if !id.IsVisible() {
		t.Error("expected visible after Show()")
	}

	id.Hide()
	if id.IsVisible() {
		t.Error("expected invisible after Hide()")
	}
}

// TestInputDialogHandleKeyNotVisible verifies HandleKey returns event when not visible (pass-through).
func TestInputDialogHandleKeyNotVisible(t *testing.T) {
	app := tview.NewApplication()
	id := NewInputDialog(app)
	ev := tcell.NewEventKey(tcell.KeyRune, 'a', 0)
	result := id.HandleKey(ev)
	if result != ev {
		t.Error("expected event to pass through when not visible")
	}
}

// TestInputDialogHandleKeyRoutesToInputField verifies HandleKey routes all keys to inputField.InputHandler() when visible.
func TestInputDialogHandleKeyRoutesToInputField(t *testing.T) {
	app := tview.NewApplication()
	id := NewInputDialog(app)
	id.Show("Rename", "Name: ", "")

	// Type characters into the input field
	aEv := tcell.NewEventKey(tcell.KeyRune, 'a', 0)
	id.HandleKey(aEv)
	bEv := tcell.NewEventKey(tcell.KeyRune, 'b', 0)
	id.HandleKey(bEv)
	cEv := tcell.NewEventKey(tcell.KeyRune, 'c', 0)
	id.HandleKey(cEv)

	text := id.GetText()
	if text != "abc" {
		t.Errorf("expected InputField text \"abc\", got %q", text)
	}
}

// TestInputDialogEnterTriggersOnSubmit verifies InputField Enter triggers onSubmit callback with text and hides.
func TestInputDialogEnterTriggersOnSubmit(t *testing.T) {
	app := tview.NewApplication()
	id := NewInputDialog(app)
	id.Show("Rename", "Name: ", "config")

	var submittedText string
	id.SetOnSubmit(func(text string) {
		submittedText = text
	})

	// Press Enter
	enterEv := tcell.NewEventKey(tcell.KeyEnter, 0, 0)
	id.HandleKey(enterEv)

	if submittedText != "config" {
		t.Errorf("expected submitted text \"config\", got %q", submittedText)
	}
	if id.IsVisible() {
		t.Error("expected dialog to be hidden after Enter")
	}
}

// TestInputDialogEnterEmptyDoesNotSubmit verifies Enter with empty text does not trigger onSubmit.
func TestInputDialogEnterEmptyDoesNotSubmit(t *testing.T) {
	app := tview.NewApplication()
	id := NewInputDialog(app)
	id.Show("New Directory", "Name: ", "")

	submitted := false
	id.SetOnSubmit(func(text string) {
		submitted = true
	})

	enterEv := tcell.NewEventKey(tcell.KeyEnter, 0, 0)
	id.HandleKey(enterEv)

	if submitted {
		t.Error("expected onSubmit NOT to be called for empty text")
	}
	if !id.IsVisible() {
		t.Error("expected dialog to remain visible after Enter with empty text")
	}
}

// TestInputDialogEscTriggersOnCancel verifies InputField Esc triggers onCancel callback and hides.
func TestInputDialogEscTriggersOnCancel(t *testing.T) {
	app := tview.NewApplication()
	id := NewInputDialog(app)
	id.Show("Rename", "Name: ", "config.yaml")

	cancelCalled := false
	id.SetOnCancel(func() {
		cancelCalled = true
	})

	escEv := tcell.NewEventKey(tcell.KeyEscape, 0, 0)
	id.HandleKey(escEv)

	if !cancelCalled {
		t.Error("expected onCancel to be called when Esc pressed")
	}
	if id.IsVisible() {
		t.Error("expected dialog to be hidden after Esc")
	}
}

// TestInputDialogSetTitle verifies SetTitle updates the dialog title.
func TestInputDialogSetTitle(t *testing.T) {
	app := tview.NewApplication()
	id := NewInputDialog(app)
	id.SetTitle("New Directory")
	if id.title != "New Directory" {
		t.Errorf("expected title \"New Directory\", got %q", id.title)
	}
}

// TestInputDialogSetText verifies SetText sets the InputField text.
func TestInputDialogSetText(t *testing.T) {
	app := tview.NewApplication()
	id := NewInputDialog(app)
	id.SetText("my_new_name.txt")
	if id.GetText() != "my_new_name.txt" {
		t.Errorf("expected InputField text \"my_new_name.txt\", got %q", id.GetText())
	}
}

// TestInputDialogSetLabel verifies SetLabel sets the InputField label.
func TestInputDialogSetLabel(t *testing.T) {
	app := tview.NewApplication()
	id := NewInputDialog(app)
	id.SetLabel("Directory name: ")
	if id.label != "Directory name: " {
		t.Errorf("expected label \"Directory name: \", got %q", id.label)
	}
}

// TestInputDialogGetText verifies GetText returns the InputField text.
func TestInputDialogGetText(t *testing.T) {
	app := tview.NewApplication()
	id := NewInputDialog(app)
	id.SetText("test_value")
	text := id.GetText()
	if text != "test_value" {
		t.Errorf("expected \"test_value\", got %q", text)
	}
}

// TestInputDialogNoCallbackPanic verifies that pressing Enter/Esc without callbacks does not panic.
func TestInputDialogNoCallbackPanic(t *testing.T) {
	app := tview.NewApplication()
	id := NewInputDialog(app)
	id.Show("Test", "Name: ", "test")

	// Enter with text should not panic without onSubmit
	enterEv := tcell.NewEventKey(tcell.KeyEnter, 0, 0)
	id.HandleKey(enterEv)

	id.Show("Test", "Name: ", "test")
	// Esc should not panic without onCancel
	escEv := tcell.NewEventKey(tcell.KeyEscape, 0, 0)
	id.HandleKey(escEv)
}

// TestInputDialogDraw verifies Draw does not panic when visible.
func TestInputDialogDraw(t *testing.T) {
	app := tview.NewApplication()
	id := NewInputDialog(app)
	id.Show("Rename", "Name: ", "config.yaml")

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	screen.SetSize(80, 24)

	// Draw should not panic
	id.Draw(screen)
}

// TestInputDialogDrawNotVisible verifies Draw is no-op when not visible.
func TestInputDialogDrawNotVisible(t *testing.T) {
	app := tview.NewApplication()
	id := NewInputDialog(app)

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	screen.SetSize(80, 24)

	// Should not panic
	id.Draw(screen)
}

// TestInputDialogShowSetsFields verifies Show() sets title, label, and text.
func TestInputDialogShowSetsFields(t *testing.T) {
	app := tview.NewApplication()
	id := NewInputDialog(app)

	id.Show("Rename", "New name: ", "original.txt")

	if id.title != "Rename" {
		t.Errorf("expected title \"Rename\", got %q", id.title)
	}
	if id.label != "New name: " {
		t.Errorf("expected label \"New name: \", got %q", id.label)
	}
	if id.GetText() != "original.txt" {
		t.Errorf("expected text \"original.txt\", got %q", id.GetText())
	}
}
