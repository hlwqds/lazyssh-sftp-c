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
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"github.com/Adembc/lazyssh/internal/core/ports"
	"go.uber.org/zap"
)

// LocalFS implements ports.FileService for the local filesystem using os.ReadDir.
type LocalFS struct {
	log *zap.SugaredLogger
}

// New creates a new LocalFS adapter.
func New(log *zap.SugaredLogger) *LocalFS {
	return &LocalFS{log: log}
}

// ListDir returns a sorted, optionally filtered list of files in the given path.
// Directories are always listed before files within each sort group.
func (l *LocalFS) ListDir(path string, showHidden bool, sortField domain.FileSortField, sortAsc bool) ([]domain.FileInfo, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", path, err)
	}

	result := make([]domain.FileInfo, 0, len(entries))
	for _, e := range entries {
		if !showHidden && strings.HasPrefix(e.Name(), ".") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			l.log.Debugw("skipping file with unreadable info", "name", e.Name(), "error", err)
			continue
		}
		result = append(result, domain.FileInfo{
			Name:      e.Name(),
			Size:      info.Size(),
			Mode:      info.Mode(),
			ModTime:   info.ModTime(),
			IsDir:     e.IsDir(),
			IsSymlink: e.Type()&fs.ModeSymlink != 0,
		})
	}

	sortFileEntries(result, sortField, sortAsc)
	return result, nil
}

// sortFileEntries sorts file entries with directories first, then by the specified field.
func sortFileEntries(entries []domain.FileInfo, sortField domain.FileSortField, sortAsc bool) {
	// Partition into directories and files
	var dirs, files []domain.FileInfo
	for _, e := range entries {
		if e.IsDir {
			dirs = append(dirs, e)
		} else {
			files = append(files, e)
		}
	}

	// Sort each partition by the specified field
	sortSlice(dirs, sortField, sortAsc)
	sortSlice(files, sortField, sortAsc)

	// Concatenate dirs + files back into entries
	n := 0
	for _, d := range dirs {
		entries[n] = d
		n++
	}
	for _, f := range files {
		entries[n] = f
		n++
	}
}

// sortSlice sorts a slice of FileInfo by the specified field and direction.
func sortSlice(entries []domain.FileInfo, sortField domain.FileSortField, sortAsc bool) {
	sort.SliceStable(entries, func(i, j int) bool {
		var less bool
		switch sortField {
		case domain.SortByName:
			less = strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
		case domain.SortBySize:
			less = entries[i].Size < entries[j].Size
		case domain.SortByDate:
			less = entries[i].ModTime.Before(entries[j].ModTime)
		}
		if sortAsc {
			return less
		}
		return !less
	})
}

// Remove deletes a single file or empty directory.
func (l *LocalFS) Remove(path string) error {
	return os.Remove(path)
}

// RemoveAll recursively deletes a directory and all its contents.
func (l *LocalFS) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// Rename renames or moves a file/directory within the same filesystem.
func (l *LocalFS) Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

// Mkdir creates a single directory. Returns error if parent doesn't exist or directory already exists.
func (l *LocalFS) Mkdir(path string) error {
	return os.Mkdir(path, 0o750)
}

// Stat returns file info for the given path.
func (l *LocalFS) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

// Copy copies a single file from src to dst, preserving permissions and modification time.
func (l *LocalFS) Copy(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy data: %w", err)
	}

	// Preserve permissions (D-07)
	if err := os.Chmod(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}
	// Preserve modification time (D-07)
	if err := os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime()); err != nil {
		return fmt.Errorf("chtimes: %w", err)
	}

	return nil
}

// CopyDir recursively copies a directory from src to dst.
// Preserves directory structure, file permissions, and modification times.
func (l *LocalFS) CopyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source dir: %w", err)
	}

	if err := os.Mkdir(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("create destination dir: %w", err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("read source dir: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := l.CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := l.Copy(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// Compile-time interface satisfaction check.
var _ ports.FileService = (*LocalFS)(nil)
