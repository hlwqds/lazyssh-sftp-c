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
)

// TestRecordSinglePath verifies that Record adds a path and GetPaths returns it.
func TestRecordSinglePath(t *testing.T) {
	rd := NewRecentDirs()
	rd.Record("/home/user/docs")
	paths := rd.GetPaths()
	if len(paths) != 1 || paths[0] != "/home/user/docs" {
		t.Errorf("expected [\"/home/user/docs\"], got %v", paths)
	}
}

// TestRecordMultiplePaths verifies MRU ordering after recording 3 different paths.
func TestRecordMultiplePaths(t *testing.T) {
	rd := NewRecentDirs()
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
	rd := NewRecentDirs()
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
	rd := NewRecentDirs()
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
	rd := NewRecentDirs()
	rd.Record(".")
	rd.Record("./docs")
	paths := rd.GetPaths()
	if len(paths) != 0 {
		t.Errorf("expected 0 paths, got %v", paths)
	}
}

// TestRecordTrimsTrailingSlash verifies that trailing slashes are removed.
func TestRecordTrimsTrailingSlash(t *testing.T) {
	rd := NewRecentDirs()
	rd.Record("/home/user/")
	paths := rd.GetPaths()
	if len(paths) != 1 || paths[0] != "/home/user" {
		t.Errorf("expected [\"/home/user\"], got %v", paths)
	}
}

// TestGetPathsEmpty verifies that GetPaths on empty RecentDirs returns empty slice.
func TestGetPathsEmpty(t *testing.T) {
	rd := NewRecentDirs()
	paths := rd.GetPaths()
	if len(paths) != 0 {
		t.Errorf("expected 0 paths, got %v", paths)
	}
}

// TestRecordDuplicateDoesNotCreateDuplicates verifies that recording the same path
// consecutively does not create duplicate entries.
func TestRecordDuplicateDoesNotCreateDuplicates(t *testing.T) {
	rd := NewRecentDirs()
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
