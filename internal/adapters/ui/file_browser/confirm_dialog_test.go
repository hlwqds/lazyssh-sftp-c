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

// TestNewConfirmDialog verifies that NewConfirmDialog creates a dialog with visible=false.
func TestNewConfirmDialog(t *testing.T) {
	app := tview.NewApplication()
	cd := NewConfirmDialog(app)
	if cd == nil {
		t.Fatal("expected non-nil ConfirmDialog")
	}
	if cd.IsVisible() {
		t.Error("expected ConfirmDialog to be invisible after creation")
	}
	if cd.app != app {
		t.Error("expected app to be stored")
	}
}

// TestConfirmDialogShowHide verifies Show sets visible=true and Hide sets visible=false.
func TestConfirmDialogShowHide(t *testing.T) {
	app := tview.NewApplication()
	cd := NewConfirmDialog(app)

	cd.Show("Delete file?", "config.yaml (1.2K)", "")
	if !cd.IsVisible() {
		t.Error("expected visible after Show()")
	}

	cd.Hide()
	if cd.IsVisible() {
		t.Error("expected invisible after Hide()")
	}
}

// TestConfirmDialogHandleKeyNotVisible verifies HandleKey returns event when not visible (pass-through).
func TestConfirmDialogHandleKeyNotVisible(t *testing.T) {
	app := tview.NewApplication()
	cd := NewConfirmDialog(app)
	ev := tcell.NewEventKey(tcell.KeyRune, 'y', 0)
	result := cd.HandleKey(ev)
	if result != ev {
		t.Error("expected event to pass through when not visible")
	}
}

// TestConfirmDialogHandleKeyConsumesAllWhenVisible verifies HandleKey returns nil (consumes all keys) when visible.
func TestConfirmDialogHandleKeyConsumesAllWhenVisible(t *testing.T) {
	app := tview.NewApplication()
	cd := NewConfirmDialog(app)
	cd.Show("Test?", "test", "")

	// Random key should be consumed
	randomEv := tcell.NewEventKey(tcell.KeyRune, 'x', 0)
	result := cd.HandleKey(randomEv)
	if result != nil {
		t.Error("expected all keys to be consumed (return nil) when visible")
	}
}

// TestConfirmDialogHandleKeyY verifies 'y' triggers onConfirm callback and hides.
func TestConfirmDialogHandleKeyY(t *testing.T) {
	app := tview.NewApplication()
	cd := NewConfirmDialog(app)
	cd.Show("Delete file?", "config.yaml", "")

	confirmCalled := false
	cd.SetOnConfirm(func() {
		confirmCalled = true
	})

	yEv := tcell.NewEventKey(tcell.KeyRune, 'y', 0)
	cd.HandleKey(yEv)
	if !confirmCalled {
		t.Error("expected onConfirm to be called when 'y' pressed")
	}
	if cd.IsVisible() {
		t.Error("expected dialog to be hidden after 'y'")
	}
}

// TestConfirmDialogHandleKeyN verifies 'n' triggers onCancel callback and hides.
func TestConfirmDialogHandleKeyN(t *testing.T) {
	app := tview.NewApplication()
	cd := NewConfirmDialog(app)
	cd.Show("Delete file?", "config.yaml", "")

	cancelCalled := false
	cd.SetOnCancel(func() {
		cancelCalled = true
	})

	nEv := tcell.NewEventKey(tcell.KeyRune, 'n', 0)
	cd.HandleKey(nEv)
	if !cancelCalled {
		t.Error("expected onCancel to be called when 'n' pressed")
	}
	if cd.IsVisible() {
		t.Error("expected dialog to be hidden after 'n'")
	}
}

// TestConfirmDialogHandleKeyEsc verifies Esc triggers onCancel callback and hides.
func TestConfirmDialogHandleKeyEsc(t *testing.T) {
	app := tview.NewApplication()
	cd := NewConfirmDialog(app)
	cd.Show("Delete file?", "config.yaml", "")

	cancelCalled := false
	cd.SetOnCancel(func() {
		cancelCalled = true
	})

	escEv := tcell.NewEventKey(tcell.KeyEscape, 0, 0)
	cd.HandleKey(escEv)
	if !cancelCalled {
		t.Error("expected onCancel to be called when Esc pressed")
	}
	if cd.IsVisible() {
		t.Error("expected dialog to be hidden after Esc")
	}
}

// TestConfirmDialogSetMessage verifies SetMessage updates the dialog message.
func TestConfirmDialogSetMessage(t *testing.T) {
	app := tview.NewApplication()
	cd := NewConfirmDialog(app)
	cd.SetMessage("config.yaml (1.2K, File, 2024-03-15)")
	if cd.message != "config.yaml (1.2K, File, 2024-03-15)" {
		t.Errorf("expected message to be updated, got %q", cd.message)
	}
}

// TestConfirmDialogSetDetail verifies SetDetail updates the detail line.
func TestConfirmDialogSetDetail(t *testing.T) {
	app := tview.NewApplication()
	cd := NewConfirmDialog(app)
	cd.SetDetail("Directory not empty, all contents will be deleted")
	if cd.detail != "Directory not empty, all contents will be deleted" {
		t.Errorf("expected detail to be updated, got %q", cd.detail)
	}
}

// TestConfirmDialogSetWarning verifies SetWarning updates the warning line (for recursive delete warning).
func TestConfirmDialogSetWarning(t *testing.T) {
	app := tview.NewApplication()
	cd := NewConfirmDialog(app)
	cd.SetWarning("This is a warning message")
	if cd.detail != "This is a warning message" {
		t.Errorf("expected warning to be set as detail, got %q", cd.detail)
	}
}

// TestConfirmDialogNoCallbackPanic verifies that pressing y/n/Esc without callbacks does not panic.
func TestConfirmDialogNoCallbackPanic(t *testing.T) {
	app := tview.NewApplication()
	cd := NewConfirmDialog(app)
	cd.Show("Test?", "test", "")

	// These should not panic even without callbacks
	yEv := tcell.NewEventKey(tcell.KeyRune, 'y', 0)
	cd.HandleKey(yEv)

	cd.Show("Test?", "test", "")
	nEv := tcell.NewEventKey(tcell.KeyRune, 'n', 0)
	cd.HandleKey(nEv)

	cd.Show("Test?", "test", "")
	escEv := tcell.NewEventKey(tcell.KeyEscape, 0, 0)
	cd.HandleKey(escEv)
}

// TestConfirmDialogDraw verifies Draw does not panic when visible.
func TestConfirmDialogDraw(t *testing.T) {
	app := tview.NewApplication()
	cd := NewConfirmDialog(app)
	cd.Show("Delete file?", "config.yaml (1.2K)", "Directory not empty")

	// Create a mock screen for Draw
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	// Set a reasonable terminal size
	screen.SetSize(80, 24)

	// Draw should not panic
	cd.Draw(screen)
}

// TestConfirmDialogDrawNotVisible verifies Draw is no-op when not visible.
func TestConfirmDialogDrawNotVisible(t *testing.T) {
	app := tview.NewApplication()
	cd := NewConfirmDialog(app)

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	screen.SetSize(80, 24)

	// Should not panic and should not render anything
	cd.Draw(screen)
}
