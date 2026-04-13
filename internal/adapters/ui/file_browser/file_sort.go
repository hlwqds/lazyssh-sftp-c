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
	"sort"
	"strings"

	"github.com/Adembc/lazyssh/internal/core/domain"
)

// FileSortMode defines the sort field and direction for file listings.
// It follows the same toggle/reverse pattern as ui.SortMode.
type FileSortMode int

const (
	FileSortByNameAsc FileSortMode = iota
	FileSortByNameDesc
	FileSortBySizeAsc
	FileSortBySizeDesc
	FileSortByDateAsc
	FileSortByDateDesc
)

// String returns a human-readable representation of the sort mode.
// Format: "Name [asc]", "Size [desc]", etc.
func (m FileSortMode) String() string {
	field := m.Field()
	dir := "asc"
	if !m.Ascending() {
		dir = "desc"
	}
	// Capitalize field name
	f := strings.ToUpper(string(field[0:1])) + string(field[1:])
	return f + " [" + dir + "]"
}

// Field returns the domain.FileSortField corresponding to this sort mode.
func (m FileSortMode) Field() domain.FileSortField {
	switch m {
	case FileSortByNameAsc, FileSortByNameDesc:
		return domain.SortByName
	case FileSortBySizeAsc, FileSortBySizeDesc:
		return domain.SortBySize
	case FileSortByDateAsc, FileSortByDateDesc:
		return domain.SortByDate
	default:
		return domain.SortByName
	}
}

// Ascending returns true if the sort direction is ascending.
func (m FileSortMode) Ascending() bool {
	switch m {
	case FileSortByNameAsc, FileSortBySizeAsc, FileSortByDateAsc:
		return true
	default:
		return false
	}
}

// ToggleField cycles through sort fields (Name -> Size -> Date -> Name)
// while preserving the current sort direction.
func (m FileSortMode) ToggleField() FileSortMode {
	asc := m.Ascending()
	switch m {
	case FileSortByNameAsc, FileSortByNameDesc:
		if asc {
			return FileSortBySizeAsc
		}
		return FileSortBySizeDesc
	case FileSortBySizeAsc, FileSortBySizeDesc:
		if asc {
			return FileSortByDateAsc
		}
		return FileSortByDateDesc
	case FileSortByDateAsc, FileSortByDateDesc:
		if asc {
			return FileSortByNameAsc
		}
		return FileSortByNameDesc
	default:
		return FileSortByNameAsc
	}
}

// Reverse flips the sort direction within the current field.
func (m FileSortMode) Reverse() FileSortMode {
	switch m {
	case FileSortByNameAsc:
		return FileSortByNameDesc
	case FileSortByNameDesc:
		return FileSortByNameAsc
	case FileSortBySizeAsc:
		return FileSortBySizeDesc
	case FileSortBySizeDesc:
		return FileSortBySizeAsc
	case FileSortByDateAsc:
		return FileSortByDateDesc
	case FileSortByDateDesc:
		return FileSortByDateAsc
	default:
		return FileSortByNameAsc
	}
}

// sortFileEntries sorts file entries with directories always listed before files.
// Within each group (dirs, files), entries are sorted by the specified field and direction.
func sortFileEntries(entries []domain.FileInfo, mode FileSortMode) {
	field := mode.Field()
	asc := mode.Ascending()

	// Partition into directories and files
	var dirs, files []domain.FileInfo
	for _, e := range entries {
		if e.IsDir {
			dirs = append(dirs, e)
		} else {
			files = append(files, e)
		}
	}

	// Sort each partition
	sortEntries(dirs, field, asc)
	sortEntries(files, field, asc)

	// Concatenate: directories first, then files
	n := 0
	copy(entries[n:], dirs)
	n += len(dirs)
	copy(entries[n:], files)
}

// sortEntries sorts a slice of FileInfo by the given field and direction.
func sortEntries(entries []domain.FileInfo, field domain.FileSortField, asc bool) {
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		var less bool
		switch field {
		case domain.SortByName:
			less = strings.ToLower(a.Name) < strings.ToLower(b.Name)
		case domain.SortBySize:
			less = a.Size < b.Size
		case domain.SortByDate:
			less = a.ModTime.Before(b.ModTime)
		default:
			less = strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}
		if asc {
			return less
		}
		return !less
	})
}
