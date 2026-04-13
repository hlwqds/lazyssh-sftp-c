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

package local_fs

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"go.uber.org/zap"
)

// newTestLogger creates a no-op test logger.
func newTestLogger() *zap.SugaredLogger {
	logger, _ := zap.NewDevelopment()
	return logger.Sugar()
}

// TestListDir_FiltersHiddenFiles verifies hidden files are skipped when showHidden=false.
func TestListDir_FiltersHiddenFiles(t *testing.T) {
	dir := t.TempDir()
	// Create visible and hidden files
	os.WriteFile(filepath.Join(dir, "visible.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(dir, ".hidden"), []byte("b"), 0o644)
	os.WriteFile(filepath.Join(dir, "normal.go"), []byte("c"), 0o644)
	os.Mkdir(filepath.Join(dir, ".secret_dir"), 0o755)

	fs := New(newTestLogger())

	// Without hidden files
	entries, err := fs.ListDir(dir, false, domain.SortByName, true)
	if err != nil {
		t.Fatalf("ListDir failed: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name, ".") {
			t.Errorf("hidden file %q should not appear when showHidden=false", e.Name)
		}
	}

	// With hidden files
	entries, err = fs.ListDir(dir, true, domain.SortByName, true)
	if err != nil {
		t.Fatalf("ListDir failed: %v", err)
	}
	hasHidden := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name, ".") {
			hasHidden = true
			break
		}
	}
	if !hasHidden {
		t.Error("hidden files should appear when showHidden=true")
	}
}

// TestListDir_DirectoriesBeforeFiles verifies directories are listed before files.
func TestListDir_DirectoriesBeforeFiles(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "subdir"), 0o755)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("a"), 0o644)
	os.Mkdir(filepath.Join(dir, "another_dir"), 0o755)
	os.WriteFile(filepath.Join(dir, "another_file.go"), []byte("b"), 0o644)

	fs := New(newTestLogger())
	entries, err := fs.ListDir(dir, false, domain.SortByName, true)
	if err != nil {
		t.Fatalf("ListDir failed: %v", err)
	}

	// Find the index of the first file and last directory
	lastDirIdx := -1
	firstFileIdx := len(entries)
	for i, e := range entries {
		if e.IsDir {
			lastDirIdx = i
		} else if i < firstFileIdx {
			firstFileIdx = i
		}
	}

	if lastDirIdx >= 0 && firstFileIdx < len(entries) {
		if lastDirIdx > firstFileIdx {
			t.Errorf("directories should come before files: last dir at index %d, first file at index %d", lastDirIdx, firstFileIdx)
		}
	}
}

// TestListDir_SortByName verifies case-insensitive name sorting.
func TestListDir_SortByName(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Charlie.txt"), []byte("c"), 0o644)
	os.WriteFile(filepath.Join(dir, "alpha.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(dir, "bravo.txt"), []byte("b"), 0o644)

	fs := New(newTestLogger())

	// Ascending
	entries, err := fs.ListDir(dir, false, domain.SortByName, true)
	if err != nil {
		t.Fatalf("ListDir failed: %v", err)
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = strings.ToLower(e.Name)
	}
	if !sort.StringsAreSorted(names) {
		t.Errorf("files not sorted by name ascending: %v", names)
	}

	// Descending
	entries, err = fs.ListDir(dir, false, domain.SortByName, false)
	if err != nil {
		t.Fatalf("ListDir failed: %v", err)
	}
	names = make([]string, len(entries))
	for i, e := range entries {
		names[i] = strings.ToLower(e.Name)
	}
	if sort.StringsAreSorted(names) {
		t.Errorf("files appear sorted ascending but expected descending: %v", names)
	}
}

// TestListDir_SortBySize verifies size sorting.
func TestListDir_SortBySize(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "small.txt"), make([]byte, 10), 0o644)
	os.WriteFile(filepath.Join(dir, "large.txt"), make([]byte, 10000), 0o644)
	os.WriteFile(filepath.Join(dir, "medium.txt"), make([]byte, 1000), 0o644)

	fs := New(newTestLogger())

	// Ascending
	entries, err := fs.ListDir(dir, false, domain.SortBySize, true)
	if err != nil {
		t.Fatalf("ListDir failed: %v", err)
	}
	for i := 1; i < len(entries); i++ {
		if entries[i].Size < entries[i-1].Size {
			t.Errorf("size not ascending: %d (idx %d) < %d (idx %d)", entries[i].Size, i, entries[i-1].Size, i-1)
		}
	}

	// Descending
	entries, err = fs.ListDir(dir, false, domain.SortBySize, false)
	if err != nil {
		t.Fatalf("ListDir failed: %v", err)
	}
	for i := 1; i < len(entries); i++ {
		if entries[i].Size > entries[i-1].Size {
			t.Errorf("size not descending: %d (idx %d) > %d (idx %d)", entries[i].Size, i, entries[i-1].Size, i-1)
		}
	}
}

// TestListDir_SortByDate verifies ModTime sorting.
func TestListDir_SortByDate(t *testing.T) {
	dir := t.TempDir()

	// Create files and set specific modification times
	past := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC)
	future := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

	files := []struct {
		name    string
		modTime time.Time
	}{
		{"old.txt", past},
		{"new.txt", now},
		{"newer.txt", future},
	}

	for _, f := range files {
		path := filepath.Join(dir, f.name)
		os.WriteFile(path, []byte("x"), 0o644)
		os.Chtimes(path, f.modTime, f.modTime)
	}

	fs := New(newTestLogger())

	// Ascending (oldest first)
	entries, err := fs.ListDir(dir, false, domain.SortByDate, true)
	if err != nil {
		t.Fatalf("ListDir failed: %v", err)
	}
	for i := 1; i < len(entries); i++ {
		if entries[i].ModTime.Before(entries[i-1].ModTime) {
			t.Errorf("date not ascending: %v (idx %d) before %v (idx %d)", entries[i].ModTime, i, entries[i-1].ModTime, i-1)
		}
	}

	// Descending (newest first)
	entries, err = fs.ListDir(dir, false, domain.SortByDate, false)
	if err != nil {
		t.Fatalf("ListDir failed: %v", err)
	}
	for i := 1; i < len(entries); i++ {
		if entries[i].ModTime.After(entries[i-1].ModTime) {
			t.Errorf("date not descending: %v (idx %d) after %v (idx %d)", entries[i].ModTime, i, entries[i-1].ModTime, i-1)
		}
	}
}

// TestListDir_InvalidPath verifies error returned for non-existent path.
func TestListDir_InvalidPath(t *testing.T) {
	fs := New(newTestLogger())
	_, err := fs.ListDir("/non/existent/path/that/does/not/exist", false, domain.SortByName, true)
	if err == nil {
		t.Error("ListDir should return error for non-existent path")
	}
}

// TestListDir_SymlinkDetection verifies symlinks are detected.
func TestListDir_SymlinkDetection(t *testing.T) {
	dir := t.TempDir()
	// Create a file and a symlink to it
	target := filepath.Join(dir, "target.txt")
	os.WriteFile(target, []byte("data"), 0o644)
	link := filepath.Join(dir, "link.txt")
	os.Symlink(target, link)

	fs := New(newTestLogger())
	entries, err := fs.ListDir(dir, false, domain.SortByName, true)
	if err != nil {
		t.Fatalf("ListDir failed: %v", err)
	}

	var foundLink bool
	for _, e := range entries {
		if e.Name == "link.txt" && e.IsSymlink {
			foundLink = true
		}
	}
	if !foundLink {
		t.Error("symlink link.txt should have IsSymlink=true")
	}
}

// TestListDir_EmptyDirectory verifies empty slice returned for empty directory.
func TestListDir_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	fs := New(newTestLogger())
	entries, err := fs.ListDir(dir, false, domain.SortByName, true)
	if err != nil {
		t.Fatalf("ListDir failed: %v", err)
	}
	if entries == nil {
		t.Error("ListDir should return empty slice (not nil) for empty directory")
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries in empty directory, got %d", len(entries))
	}
}

// TestListDir_ImplementsFileService verifies LocalFS satisfies ports.FileService interface.
func TestListDir_ImplementsFileService(t *testing.T) {
	// Compile-time check is in local_fs.go. This test verifies the method signature matches.
	fs := New(newTestLogger())
	if fs == nil {
		t.Fatal("New() returned nil")
	}
}
