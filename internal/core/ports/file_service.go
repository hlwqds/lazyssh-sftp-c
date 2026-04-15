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

package ports

import (
	"io"
	"os"

	"github.com/Adembc/lazyssh/internal/core/domain"
)

// FileService provides file listing and management operations for local and remote filesystems.
// Implementations must sort directories before files within each sort group.
type FileService interface {
	// ListDir returns a sorted, optionally filtered list of files in the given path.
	// sortField: domain.SortByName, domain.SortBySize, or domain.SortByDate
	// sortAsc: true for ascending, false for descending
	ListDir(path string, showHidden bool, sortField domain.FileSortField, sortAsc bool) ([]domain.FileInfo, error)

	// Remove deletes a single file or empty directory.
	Remove(path string) error
	// RemoveAll recursively deletes a directory and all its contents.
	RemoveAll(path string) error
	// Rename renames or moves a file/directory within the same filesystem.
	Rename(oldPath, newPath string) error
	// Mkdir creates a single directory. Returns error if parent doesn't exist or directory already exists.
	Mkdir(path string) error
	// Stat returns file info for the given path.
	Stat(path string) (os.FileInfo, error)
}

// SFTPService provides SFTP connection and remote file operations.
// It extends FileService with connection lifecycle management.
type SFTPService interface {
	FileService

	// Connect establishes an SFTP connection to the given server using system SSH binary.
	Connect(server domain.Server) error
	// Close terminates the SFTP connection and cleans up the SSH process.
	Close() error
	// IsConnected returns whether the SFTP connection is active.
	IsConnected() bool

	// CreateRemoteFile creates a new remote file for writing.
	CreateRemoteFile(path string) (io.WriteCloser, error)
	// OpenRemoteFile opens an existing remote file for reading.
	OpenRemoteFile(path string) (io.ReadCloser, error)
	// MkdirAll creates remote directories recursively, skipping existing ones.
	MkdirAll(path string) error
	// WalkDir returns all file paths (not directories) under the given remote path, recursively.
	WalkDir(path string) ([]string, error)

	// Stat returns file info for the given remote path.
	// Returns error if the file does not exist.
	Stat(path string) (os.FileInfo, error)
	// Remove deletes the remote file or empty directory.
	Remove(path string) error
	// HomeDir returns the remote home directory resolved during Connect().
	// The path is guaranteed to be absolute (via Getwd/RealPath).
	HomeDir() string
}
