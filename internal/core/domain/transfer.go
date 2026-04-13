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

// TransferProgress represents the state of an ongoing file transfer.
// It is a pure data struct used as a callback parameter to report transfer
// progress to the UI layer or any observer.
type TransferProgress struct {
	FileName   string  // current file name (not full path)
	FilePath   string  // full path for display
	BytesDone  int64   // bytes transferred so far for current file
	BytesTotal int64   // total bytes for current file (0 if unknown)
	Speed      float64 // bytes per second
	FileIndex  int     // current file in multi-file transfer (1-based, 0 if single file)
	FileTotal  int     // total files in multi-file transfer (0 if single file or unknown)
	IsDir      bool    // true when entering a directory (informational event)
	Done       bool    // true when current file transfer is complete
	Failed     bool    // true when current file failed
	FailError  string  // error message if Failed is true
}

// ConflictAction represents the user's decision when a file conflict is detected.
type ConflictAction int

const (
	// ConflictOverwrite overwrites the existing file.
	ConflictOverwrite ConflictAction = iota
	// ConflictSkip skips the conflicting file.
	ConflictSkip
	// ConflictRename renames the file with an incremental suffix (e.g., file.1.txt).
	ConflictRename
)

// ConflictHandler is a callback invoked when a destination file already exists.
// It receives the file name and returns the chosen action and new path (for Rename).
// The new path is only used when action is ConflictRename.
type ConflictHandler func(fileName string) (ConflictAction, string)
