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

package transfer

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"github.com/Adembc/lazyssh/internal/core/ports"
	"go.uber.org/zap"
)

// transferService coordinates local filesystem and remote SFTP operations
// for file and directory transfers.
type transferService struct {
	log  *zap.SugaredLogger
	sftp ports.SFTPService
}

// Compile-time interface satisfaction check.
var _ ports.TransferService = (*transferService)(nil)

// New creates a new TransferService adapter.
func New(log *zap.SugaredLogger, sftp ports.SFTPService) *transferService {
	return &transferService{log: log, sftp: sftp}
}

// UploadFile uploads a single file from local to remote.
// onProgress is called periodically during transfer.
func (ts *transferService) UploadFile(localPath, remotePath string, onProgress func(domain.TransferProgress)) error {
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local %s: %w", localPath, err)
	}
	defer localFile.Close()

	stat, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("stat local %s: %w", localPath, err)
	}
	total := stat.Size()

	remoteFile, err := ts.sftp.CreateRemoteFile(remotePath)
	if err != nil {
		return fmt.Errorf("create remote %s: %w", remotePath, err)
	}
	defer remoteFile.Close()

	return ts.copyWithProgress(localFile, remoteFile, localPath, localPath, total, onProgress)
}

// DownloadFile downloads a single file from remote to local.
// onProgress is called periodically during transfer.
func (ts *transferService) DownloadFile(remotePath, localPath string, onProgress func(domain.TransferProgress)) error {
	remoteFile, err := ts.sftp.OpenRemoteFile(remotePath)
	if err != nil {
		return fmt.Errorf("open remote %s: %w", remotePath, err)
	}
	defer remoteFile.Close()

	// Create parent directory if needed
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return fmt.Errorf("mkdir local %s: %w", filepath.Dir(localPath), err)
	}

	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("create local %s: %w", localPath, err)
	}
	defer localFile.Close()

	// Remote file size is not available through io.ReadCloser interface;
	// pass 0 to indicate unknown size.
	return ts.copyWithProgress(remoteFile, localFile, remotePath, localPath, 0, onProgress)
}

// UploadDir recursively uploads a directory from local to remote.
// Returns list of failed file paths (empty = all success).
func (ts *transferService) UploadDir(localPath, remotePath string, onProgress func(domain.TransferProgress)) ([]string, error) {
	// First pass: count total files
	var fileCount int
	err := filepath.WalkDir(localPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			fileCount++
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk local %s: %w", localPath, err)
	}

	if fileCount == 0 {
		return nil, nil
	}

	// Create remote root directory
	if err := ts.sftp.MkdirAll(remotePath); err != nil {
		return nil, fmt.Errorf("mkdir remote %s: %w", remotePath, err)
	}

	// Second pass: transfer files
	var failed []string
	fileIndex := 0
	err = filepath.WalkDir(localPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			failed = append(failed, path)
			return nil
		}
		if d.IsDir() {
			// Create corresponding remote directory
			rel, _ := filepath.Rel(localPath, path)
			remoteDir := joinRemotePath(remotePath, filepath.ToSlash(rel))
			if err := ts.sftp.MkdirAll(remoteDir); err != nil {
				ts.log.Warnw("failed to create remote directory", "dir", remoteDir, "error", err)
			}
			return nil
		}

		fileIndex++
		rel, _ := filepath.Rel(localPath, path)
		remoteFile := joinRemotePath(remotePath, filepath.ToSlash(rel))

		// Ensure parent directory exists
		parentDir := remotePath + "/" + filepath.ToSlash(filepath.Dir(rel))
		if parentDir != remotePath+"/." {
			if err := ts.sftp.MkdirAll(parentDir); err != nil {
				ts.log.Warnw("failed to create remote parent dir", "dir", parentDir, "error", err)
			}
		}

		if err := ts.uploadSingleFile(path, remoteFile, onProgress, fileIndex, fileCount); err != nil {
			ts.log.Warnw("upload failed", "file", path, "error", err)
			failed = append(failed, path)
			if onProgress != nil {
				onProgress(domain.TransferProgress{
					FileName:   filepath.Base(path),
					FilePath:   path,
					FileIndex:  fileIndex,
					FileTotal:  fileCount,
					Failed:     true,
					FailError:  err.Error(),
				})
			}
		}
		return nil
	})

	if err != nil {
		return failed, fmt.Errorf("upload dir %s: %w", localPath, err)
	}

	if len(failed) == 0 {
		return nil, nil
	}
	return failed, nil
}

// DownloadDir recursively downloads a directory from remote to local.
// Returns list of failed file paths (empty = all success).
func (ts *transferService) DownloadDir(remotePath, localPath string, onProgress func(domain.TransferProgress)) ([]string, error) {
	// Get list of all remote files
	remoteFiles, err := ts.sftp.WalkDir(remotePath)
	if err != nil {
		return nil, fmt.Errorf("walk remote %s: %w", remotePath, err)
	}

	if len(remoteFiles) == 0 {
		return nil, nil
	}

	// Create local root directory
	if err := os.MkdirAll(localPath, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir local %s: %w", localPath, err)
	}

	fileCount := len(remoteFiles)
	var failed []string

	for i, remoteFile := range remoteFiles {
		// Calculate local path from remote relative path
		rel := strings.TrimPrefix(remoteFile, remotePath)
		rel = strings.TrimPrefix(rel, "/")
		localFile := filepath.Join(localPath, filepath.FromSlash(rel))

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(localFile), 0o755); err != nil {
			ts.log.Warnw("failed to create local parent dir", "dir", filepath.Dir(localFile), "error", err)
			failed = append(failed, remoteFile)
			if onProgress != nil {
				onProgress(domain.TransferProgress{
					FileName:   filepath.Base(remoteFile),
					FilePath:   remoteFile,
					FileIndex:  i + 1,
					FileTotal:  fileCount,
					Failed:     true,
					FailError:  err.Error(),
				})
			}
			continue
		}

		if err := ts.downloadSingleFile(remoteFile, localFile, onProgress, i+1, fileCount); err != nil {
			ts.log.Warnw("download failed", "file", remoteFile, "error", err)
			failed = append(failed, remoteFile)
			if onProgress != nil {
				onProgress(domain.TransferProgress{
					FileName:   filepath.Base(remoteFile),
					FilePath:   remoteFile,
					FileIndex:  i + 1,
					FileTotal:  fileCount,
					Failed:     true,
					FailError:  err.Error(),
				})
			}
		}
	}

	if len(failed) == 0 {
		return nil, nil
	}
	return failed, nil
}

// uploadSingleFile uploads a single file with progress tracking for directory transfers.
func (ts *transferService) uploadSingleFile(localPath, remotePath string, onProgress func(domain.TransferProgress), fileIndex, fileTotal int) error {
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local %s: %w", localPath, err)
	}
	defer localFile.Close()

	stat, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("stat local %s: %w", localPath, err)
	}

	remoteFile, err := ts.sftp.CreateRemoteFile(remotePath)
	if err != nil {
		return fmt.Errorf("create remote %s: %w", remotePath, err)
	}
	defer remoteFile.Close()

	progress := func(p domain.TransferProgress) {
		p.FileIndex = fileIndex
		p.FileTotal = fileTotal
		if onProgress != nil {
			onProgress(p)
		}
	}

	return ts.copyWithProgress(localFile, remoteFile, localPath, localPath, stat.Size(), progress)
}

// downloadSingleFile downloads a single file with progress tracking for directory transfers.
func (ts *transferService) downloadSingleFile(remotePath, localPath string, onProgress func(domain.TransferProgress), fileIndex, fileTotal int) error {
	remoteFile, err := ts.sftp.OpenRemoteFile(remotePath)
	if err != nil {
		return fmt.Errorf("open remote %s: %w", remotePath, err)
	}
	defer remoteFile.Close()

	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("create local %s: %w", localPath, err)
	}
	defer localFile.Close()

	progress := func(p domain.TransferProgress) {
		p.FileIndex = fileIndex
		p.FileTotal = fileTotal
		if onProgress != nil {
			onProgress(p)
		}
	}

	return ts.copyWithProgress(remoteFile, localFile, remotePath, localPath, 0, progress)
}

// copyWithProgress copies data from src to dst using a 32KB buffer,
// calling onProgress after each chunk.
func (ts *transferService) copyWithProgress(src io.Reader, dst io.Writer, srcPath, displayPath string, total int64, onProgress func(domain.TransferProgress)) error {
	buf := make([]byte, 32*1024)
	var transferred int64
	fileName := filepath.Base(displayPath)

	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			_, writeErr := dst.Write(buf[:n])
			if writeErr != nil {
				return fmt.Errorf("write to %s: %w", displayPath, writeErr)
			}
			transferred += int64(n)
			if onProgress != nil {
				onProgress(domain.TransferProgress{
					FileName:   fileName,
					FilePath:   displayPath,
					BytesDone:  transferred,
					BytesTotal: total,
				})
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return fmt.Errorf("read from %s: %w", srcPath, readErr)
		}
	}

	// Send final Done event
	if onProgress != nil {
		onProgress(domain.TransferProgress{
			FileName:   fileName,
			FilePath:   displayPath,
			BytesDone:  transferred,
			BytesTotal: total,
			Done:       true,
		})
	}

	return nil
}

// joinRemotePath joins remote path segments using "/" separator.
func joinRemotePath(base, rel string) string {
	if strings.HasSuffix(base, "/") {
		return base + rel
	}
	return base + "/" + rel
}
