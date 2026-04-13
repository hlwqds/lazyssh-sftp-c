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

package domain

import (
	"io/fs"
	"testing"
	"time"
)

// TestFileInfoFields verifies FileInfo struct has all required fields.
func TestFileInfoFields(t *testing.T) {
	fi := FileInfo{
		Name:      "test.txt",
		Size:      1024,
		Mode:      0o644,
		ModTime:   time.Now(),
		IsDir:     false,
		IsSymlink: false,
	}

	if fi.Name != "test.txt" {
		t.Errorf("Name = %q, want %q", fi.Name, "test.txt")
	}
	if fi.Size != 1024 {
		t.Errorf("Size = %d, want %d", fi.Size, 1024)
	}
	if fi.Mode != 0o644 {
		t.Errorf("Mode = %v, want %v", fi.Mode, 0o644)
	}
	if fi.IsDir != false {
		t.Errorf("IsDir = %v, want %v", fi.IsDir, false)
	}
	if fi.IsSymlink != false {
		t.Errorf("IsSymlink = %v, want %v", fi.IsSymlink, false)
	}
}

// TestFileInfoDirEntry verifies FileInfo correctly represents a directory.
func TestFileInfoDirEntry(t *testing.T) {
	fi := FileInfo{
		Name:      "docs",
		Size:      4096,
		Mode:      fs.ModeDir | 0o755,
		ModTime:   time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
		IsDir:     true,
		IsSymlink: false,
	}

	if !fi.IsDir {
		t.Error("IsDir should be true for directory entry")
	}
	if fi.Name != "docs" {
		t.Errorf("Name = %q, want %q", fi.Name, "docs")
	}
}

// TestFileInfoSymlinkEntry verifies FileInfo correctly represents a symlink.
func TestFileInfoSymlinkEntry(t *testing.T) {
	fi := FileInfo{
		Name:      "link_to_dir",
		Size:      0,
		Mode:      fs.ModeSymlink | 0o777,
		ModTime:   time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		IsDir:     false,
		IsSymlink: true,
	}

	if !fi.IsSymlink {
		t.Error("IsSymlink should be true for symlink entry")
	}
}

// TestFileSortFieldConstants verifies sort field constants exist and have correct values.
func TestFileSortFieldConstants(t *testing.T) {
	tests := []struct {
		field    FileSortField
		expected string
	}{
		{SortByName, "name"},
		{SortBySize, "size"},
		{SortByDate, "date"},
	}

	for _, tt := range tests {
		if string(tt.field) != tt.expected {
			t.Errorf("FileSortField(%q) = %q, want %q", tt.field, string(tt.field), tt.expected)
		}
	}
}

// TestFileSortFieldType verifies FileSortField is a string type.
func TestFileSortFieldType(t *testing.T) {
	var sf FileSortField = "custom"
	if sf != "custom" {
		t.Errorf("FileSortField should accept custom string values, got %q", sf)
	}
}
