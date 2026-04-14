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
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"go.uber.org/zap"
)

// newTestRecentDirs creates a RecentDirs for testing with a temp directory
// to avoid polluting the real ~/.lazyssh/recent-dirs/ path.
func newTestRecentDirs(t *testing.T) *RecentDirs {
	t.Helper()
	tmpDir := t.TempDir()
	log := zap.NewNop().Sugar()
	serverKey := "test@example.com"
	filePath := filepath.Join(tmpDir, serverKey+".json")

	rd := &RecentDirs{
		paths:     make([]string, 0, maxRecentDirs),
		visible:   false,
		log:       log,
		serverKey: serverKey,
		filePath:  filePath,
	}
	rd.Box = tview.NewBox()
	rd.SetBorder(true).
		SetBorderColor(tcell.Color238).
		SetTitleColor(tcell.Color250).
		SetBackgroundColor(tcell.Color232)
	return rd
}

// TestRecordSinglePath verifies that Record adds a path and GetPaths returns it.
func TestRecordSinglePath(t *testing.T) {
	rd := newTestRecentDirs(t)
	rd.Record("/home/user/docs")
	paths := rd.GetPaths()
	if len(paths) != 1 || paths[0] != "/home/user/docs" {
		t.Errorf("expected [\"/home/user/docs\"], got %v", paths)
	}
}

// TestRecordMultiplePaths verifies MRU ordering after recording 3 different paths.
func TestRecordMultiplePaths(t *testing.T) {
	rd := newTestRecentDirs(t)
	rd.Record("/home/user/docs")
	rd.Record("/var/log")
	rd.Record("/tmp/build")
	paths := rd.GetPaths()
	expected := []string{"/tmp/build", "/var/log", "/home/user/docs"}
	if len(paths) != 3 {
		t.Fatalf("expected 3 paths, got %d", len(paths))
	}
	for i, p := range expected {
		if paths[i] != p {
			t.Errorf("index %d: expected %q, got %q", i, p, paths[i])
		}
	}
}

// TestRecordMoveToFront verifies that re-recording an existing path moves it to front.
func TestRecordMoveToFront(t *testing.T) {
	rd := newTestRecentDirs(t)
	rd.Record("/a")
	rd.Record("/b")
	rd.Record("/a") // move /a to front
	paths := rd.GetPaths()
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(paths))
	}
	if paths[0] != "/a" {
		t.Errorf("expected first path to be \"/a\", got %q", paths[0])
	}
	if paths[1] != "/b" {
		t.Errorf("expected second path to be \"/b\", got %q", paths[1])
	}
}

// TestRecordTruncation verifies that the list is capped at maxRecentDirs (10).
func TestRecordTruncation(t *testing.T) {
	rd := newTestRecentDirs(t)
	for i := 0; i < 11; i++ {
		rd.Record("/dir/" + string(rune('a'+i)))
	}
	paths := rd.GetPaths()
	if len(paths) != maxRecentDirs {
		t.Errorf("expected %d paths, got %d", maxRecentDirs, len(paths))
	}
	// The oldest entry ("/dir/a") should be truncated; most recent should be first
	if paths[0] != "/dir/k" {
		t.Errorf("expected first path to be \"/dir/k\", got %q", paths[0])
	}
	if paths[len(paths)-1] != "/dir/b" {
		t.Errorf("expected last path to be \"/dir/b\", got %q", paths[len(paths)-1])
	}
}

// TestRecordSkipsRelativePaths verifies that relative paths starting with "." are skipped.
func TestRecordSkipsRelativePaths(t *testing.T) {
	rd := newTestRecentDirs(t)
	rd.Record(".")
	rd.Record("./docs")
	paths := rd.GetPaths()
	if len(paths) != 0 {
		t.Errorf("expected 0 paths, got %v", paths)
	}
}

// TestRecordTrimsTrailingSlash verifies that trailing slashes are removed.
func TestRecordTrimsTrailingSlash(t *testing.T) {
	rd := newTestRecentDirs(t)
	rd.Record("/home/user/")
	paths := rd.GetPaths()
	if len(paths) != 1 || paths[0] != "/home/user" {
		t.Errorf("expected [\"/home/user\"], got %v", paths)
	}
}

// TestGetPathsEmpty verifies that GetPaths on empty RecentDirs returns empty slice.
func TestGetPathsEmpty(t *testing.T) {
	rd := newTestRecentDirs(t)
	paths := rd.GetPaths()
	if len(paths) != 0 {
		t.Errorf("expected 0 paths, got %v", paths)
	}
}

// TestRecordDuplicateDoesNotCreateDuplicates verifies that recording the same path
// consecutively does not create duplicate entries.
func TestRecordDuplicateDoesNotCreateDuplicates(t *testing.T) {
	rd := newTestRecentDirs(t)
	rd.Record("/home/user")
	rd.Record("/home/user")
	paths := rd.GetPaths()
	if len(paths) != 1 {
		t.Errorf("expected 1 path, got %d: %v", len(paths), paths)
	}
	if paths[0] != "/home/user" {
		t.Errorf("expected \"/home/user\", got %q", paths[0])
	}
}

// TestHandleKeyNotVisible passes through events when popup is hidden.
func TestHandleKeyNotVisible(t *testing.T) {
	rd := newTestRecentDirs(t)
	ev := tcell.NewEventKey(tcell.KeyRune, 'j', 0)
	result := rd.HandleKey(ev)
	if result != ev {
		t.Error("expected event to pass through when not visible")
	}
}

// TestHandleKeyEscHidesPopup verifies Esc hides the popup and consumes the event.
func TestHandleKeyEscHidesPopup(t *testing.T) {
	rd := newTestRecentDirs(t)
	rd.Record("/a")
	rd.Record("/b")
	rd.Show()
	ev := tcell.NewEventKey(tcell.KeyEscape, 0, 0)
	result := rd.HandleKey(ev)
	if result != nil {
		t.Error("expected Esc to consume event (return nil)")
	}
	if rd.IsVisible() {
		t.Error("expected popup to be hidden after Esc")
	}
}

// TestHandleKeyEnterSelectsPath verifies Enter calls onSelect with selected path.
func TestHandleKeyEnterSelectsPath(t *testing.T) {
	rd := newTestRecentDirs(t)
	rd.Record("/a")
	rd.Record("/b")
	rd.Record("/c")
	rd.Show()

	var selectedPath string
	rd.SetOnSelect(func(path string) {
		selectedPath = path
	})

	// Move selection down by one
	downEv := tcell.NewEventKey(tcell.KeyDown, 0, 0)
	rd.HandleKey(downEv)

	// Press Enter
	enterEv := tcell.NewEventKey(tcell.KeyEnter, 0, 0)
	result := rd.HandleKey(enterEv)
	if result != nil {
		t.Error("expected Enter to consume event (return nil)")
	}
	if selectedPath != "/b" {
		t.Errorf("expected selected path \"/b\", got %q", selectedPath)
	}
}

// TestHandleKeyEnterEmptyList verifies Enter is a no-op when list is empty.
func TestHandleKeyEnterEmptyList(t *testing.T) {
	rd := newTestRecentDirs(t)
	rd.Show()

	called := false
	rd.SetOnSelect(func(path string) {
		called = true
	})

	enterEv := tcell.NewEventKey(tcell.KeyEnter, 0, 0)
	rd.HandleKey(enterEv)
	if called {
		t.Error("expected onSelect NOT to be called for empty list")
	}
}

// TestHandleKeyJKNavigation verifies j/k rune keys move selection.
func TestHandleKeyJKNavigation(t *testing.T) {
	rd := newTestRecentDirs(t)
	rd.Record("/a")
	rd.Record("/b")
	rd.Record("/c")
	rd.Show()

	// j moves down
	jEv := tcell.NewEventKey(tcell.KeyRune, 'j', 0)
	rd.HandleKey(jEv)
	if rd.GetSelectedIndex() != 1 {
		t.Errorf("expected selectedIndex=1 after j, got %d", rd.GetSelectedIndex())
	}

	// j moves down again
	rd.HandleKey(jEv)
	if rd.GetSelectedIndex() != 2 {
		t.Errorf("expected selectedIndex=2 after j, got %d", rd.GetSelectedIndex())
	}

	// j clamps at last
	rd.HandleKey(jEv)
	if rd.GetSelectedIndex() != 2 {
		t.Errorf("expected selectedIndex=2 (clamped), got %d", rd.GetSelectedIndex())
	}

	// k moves up
	kEv := tcell.NewEventKey(tcell.KeyRune, 'k', 0)
	rd.HandleKey(kEv)
	if rd.GetSelectedIndex() != 1 {
		t.Errorf("expected selectedIndex=1 after k, got %d", rd.GetSelectedIndex())
	}

	// k clamps at 0
	kEv2 := tcell.NewEventKey(tcell.KeyRune, 'k', 0)
	rd.HandleKey(kEv2)
	kEv3 := tcell.NewEventKey(tcell.KeyRune, 'k', 0)
	rd.HandleKey(kEv3)
	if rd.GetSelectedIndex() != 0 {
		t.Errorf("expected selectedIndex=0 (clamped), got %d", rd.GetSelectedIndex())
	}
}

// TestHandleKeyArrowNavigation verifies arrow keys move selection same as j/k.
func TestHandleKeyArrowNavigation(t *testing.T) {
	rd := newTestRecentDirs(t)
	rd.Record("/a")
	rd.Record("/b")
	rd.Show()

	downEv := tcell.NewEventKey(tcell.KeyDown, 0, 0)
	rd.HandleKey(downEv)
	if rd.GetSelectedIndex() != 1 {
		t.Errorf("expected selectedIndex=1 after Down, got %d", rd.GetSelectedIndex())
	}

	upEv := tcell.NewEventKey(tcell.KeyUp, 0, 0)
	rd.HandleKey(upEv)
	if rd.GetSelectedIndex() != 0 {
		t.Errorf("expected selectedIndex=0 after Up, got %d", rd.GetSelectedIndex())
	}
}

// TestHandleKeyConsumesAllWhenVisible verifies all keys return nil when popup visible (D-08).
func TestHandleKeyConsumesAllWhenVisible(t *testing.T) {
	rd := newTestRecentDirs(t)
	rd.Record("/a")
	rd.Show()

	// Random key should be consumed
	randomEv := tcell.NewEventKey(tcell.KeyRune, 'x', 0)
	result := rd.HandleKey(randomEv)
	if result != nil {
		t.Error("expected all keys to be consumed (return nil) when popup visible")
	}
}

// TestShowResetsSelectedIndex verifies Show() resets selectedIndex to 0.
func TestShowResetsSelectedIndex(t *testing.T) {
	rd := newTestRecentDirs(t)
	rd.Record("/a")
	rd.Record("/b")
	rd.Record("/c")
	rd.Show()

	// Move selection to index 2
	downEv := tcell.NewEventKey(tcell.KeyDown, 0, 0)
	rd.HandleKey(downEv)
	rd.HandleKey(downEv)
	if rd.GetSelectedIndex() != 2 {
		t.Fatalf("setup: expected selectedIndex=2, got %d", rd.GetSelectedIndex())
	}

	// Hide and show again
	rd.Hide()
	rd.Show()
	if rd.GetSelectedIndex() != 0 {
		t.Errorf("expected selectedIndex=0 after Show(), got %d", rd.GetSelectedIndex())
	}
}

// TestSetCurrentPath verifies SetCurrentPath trims trailing slashes.
func TestSetCurrentPath(t *testing.T) {
	rd := newTestRecentDirs(t)
	rd.SetCurrentPath("/home/user/docs/")
	if rd.GetCurrentPath() != "/home/user/docs" {
		t.Errorf("expected \"/home/user/docs\", got %q", rd.GetCurrentPath())
	}
}

// TestSetOnSelect verifies callback is stored and callable.
func TestSetOnSelect(t *testing.T) {
	rd := newTestRecentDirs(t)
	var received string
	rd.SetOnSelect(func(path string) {
		received = path
	})
	// Can't directly call onSelect, but entering a path with Enter will invoke it
	rd.Record("/test")
	rd.Show()
	enterEv := tcell.NewEventKey(tcell.KeyEnter, 0, 0)
	rd.HandleKey(enterEv)
	if received != "/test" {
		t.Errorf("expected \"/test\", got %q", received)
	}
}

// TestPersistenceSaveAndLoad verifies that Record persists paths to disk
// and a new RecentDirs instance can load them back.
func TestPersistenceSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	log := zap.NewNop().Sugar()
	serverKey := "persist@example.com"
	filePath := filepath.Join(tmpDir, serverKey+".json")

	// Create first instance and record paths
	rd1 := &RecentDirs{
		paths:     make([]string, 0, maxRecentDirs),
		visible:   false,
		log:       log,
		serverKey: serverKey,
		filePath:  filePath,
	}
	rd1.Box = tview.NewBox()
	rd1.SetBorder(true).
		SetBorderColor(tcell.Color238).
		SetTitleColor(tcell.Color250).
		SetBackgroundColor(tcell.Color232)

	rd1.Record("/home/user/docs")
	rd1.Record("/var/log/app")

	// Verify file was created
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("expected persistence file to be created")
	}

	// Create second instance loading from same file
	rd2 := &RecentDirs{
		paths:     make([]string, 0, maxRecentDirs),
		visible:   false,
		log:       log,
		serverKey: serverKey,
		filePath:  filePath,
	}
	rd2.Box = tview.NewBox()
	rd2.SetBorder(true).
		SetBorderColor(tcell.Color238).
		SetTitleColor(tcell.Color250).
		SetBackgroundColor(tcell.Color232)
	rd2.loadFromDisk()

	paths := rd2.GetPaths()
	if len(paths) != 2 {
		t.Fatalf("expected 2 loaded paths, got %d", len(paths))
	}
	if paths[0] != "/var/log/app" {
		t.Errorf("expected first path \"/var/log/app\", got %q", paths[0])
	}
	if paths[1] != "/home/user/docs" {
		t.Errorf("expected second path \"/home/user/docs\", got %q", paths[1])
	}
}

// TestPersistenceMissingFile verifies that loading from a non-existent file
// returns an empty paths list without error.
func TestPersistenceMissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	log := zap.NewNop().Sugar()
	filePath := filepath.Join(tmpDir, "nonexistent@example.com.json")

	rd := &RecentDirs{
		paths:     make([]string, 0, maxRecentDirs),
		visible:   false,
		log:       log,
		serverKey: "nonexistent@example.com",
		filePath:  filePath,
	}
	rd.Box = tview.NewBox()
	rd.SetBorder(true).
		SetBorderColor(tcell.Color238).
		SetTitleColor(tcell.Color250).
		SetBackgroundColor(tcell.Color232)
	rd.loadFromDisk()

	paths := rd.GetPaths()
	if len(paths) != 0 {
		t.Errorf("expected 0 paths for missing file, got %v", paths)
	}
}
