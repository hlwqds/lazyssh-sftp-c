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
	"time"
)

// FileInfo represents a file or directory entry in a file listing.
// It is the single source of truth for file metadata across all adapters
// (local filesystem and SFTP remote filesystem).
type FileInfo struct {
	Name      string
	Size      int64
	Mode      fs.FileMode
	ModTime   time.Time
	IsDir     bool
	IsSymlink bool
}

// FileSortField defines which field to sort file listings by.
type FileSortField string

const (
	// SortByName sorts file entries by name (case-insensitive).
	SortByName FileSortField = "name"
	// SortBySize sorts file entries by file size.
	SortBySize FileSortField = "size"
	// SortByDate sorts file entries by modification time.
	SortByDate FileSortField = "date"
)
