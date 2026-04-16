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
	"context"
	"errors"
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
// ctx controls cancellation — returns context.Canceled if ctx is done.
// onProgress is called periodically during transfer.
// onConflict is called when remote file exists; nil means always overwrite.
func (ts *transferService) UploadFile(ctx context.Context, localPath, remotePath string, onProgress func(domain.TransferProgress), onConflict domain.ConflictHandler) error {
	// Conflict detection (D-06)
	if onConflict != nil {
		if _, err := ts.sftp.Stat(remotePath); err == nil {
			action, newPath := onConflict(filepath.Base(remotePath))
			switch action {
			case domain.ConflictSkip:
				return nil
			case domain.ConflictRename:
				remotePath = newPath
			case domain.ConflictOverwrite:
				// continue with original path
			}
		}
	}

	localFile, err := os.Open(localPath) //nolint:gosec // G304: path from user file browser selection
	if err != nil {
		return fmt.Errorf("open local %s: %w", localPath, err)
	}
	defer func() { _ = localFile.Close() }()

	stat, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("stat local %s: %w", localPath, err)
	}
	total := stat.Size()

	remoteFile, err := ts.sftp.CreateRemoteFile(remotePath)
	if err != nil {
		return fmt.Errorf("create remote %s: %w", remotePath, err)
	}

	err = ts.copyWithProgress(ctx, localFile, remoteFile, localPath, localPath, total, onProgress)
	_ = remoteFile.Close()

	// D-04: cancel cleanup — delete partial remote file
	if errors.Is(err, context.Canceled) {
		ts.log.Infow("transfer canceled, cleaning up partial file", "path", remotePath)
		if removeErr := ts.sftp.Remove(remotePath); removeErr != nil {
			ts.log.Warnw("failed to cleanup partial remote file", "path", remotePath, "error", removeErr)
		}
		return context.Canceled
	}
	return err
}

// DownloadFile downloads a single file from remote to local.
// ctx controls cancellation — returns context.Canceled if ctx is done.
// onProgress is called periodically during transfer.
// onConflict is called when local file exists; nil means always overwrite.
func (ts *transferService) DownloadFile(ctx context.Context, remotePath, localPath string, onProgress func(domain.TransferProgress), onConflict domain.ConflictHandler) error {
	// Conflict detection (D-06)
	if onConflict != nil {
		if _, err := os.Stat(localPath); err == nil {
			action, newPath := onConflict(filepath.Base(localPath))
			switch action {
			case domain.ConflictSkip:
				return nil
			case domain.ConflictRename:
				localPath = newPath
			case domain.ConflictOverwrite:
				// continue with original path
			}
		}
	}

	remoteFile, err := ts.sftp.OpenRemoteFile(remotePath)
	if err != nil {
		return fmt.Errorf("open remote %s: %w", remotePath, err)
	}
	defer func() { _ = remoteFile.Close() }()

	// Get remote file size for progress tracking
	var total int64
	if remoteStat, statErr := ts.sftp.Stat(remotePath); statErr == nil {
		total = remoteStat.Size()
	}

	// Create parent directory if needed
	if err := os.MkdirAll(filepath.Dir(localPath), 0o750); err != nil {
		return fmt.Errorf("mkdir local %s: %w", filepath.Dir(localPath), err)
	}

	localFile, err := os.Create(localPath) //nolint:gosec // G304: path from user file browser selection
	if err != nil {
		return fmt.Errorf("create local %s: %w", localPath, err)
	}

	err = ts.copyWithProgress(ctx, remoteFile, localFile, remotePath, localPath, total, onProgress)
	_ = localFile.Close()

	// D-04: cancel cleanup — delete partial local file
	if errors.Is(err, context.Canceled) {
		ts.log.Infow("transfer canceled, cleaning up partial file", "path", localPath)
		if removeErr := os.Remove(localPath); removeErr != nil {
			ts.log.Warnw("failed to cleanup partial local file", "path", localPath, "error", removeErr)
		}
		return context.Canceled
	}
	if err != nil {
		return err
	}

	// Set standard file permissions on downloaded file (cross-platform)
	setFilePermissions(localPath, 0o644, ts.log)
	return nil
}

// UploadDir recursively uploads a directory from local to remote.
// ctx controls cancellation — stops remaining files if ctx is done.
// onConflict is called for each conflicting file; nil means always overwrite.
// Returns list of failed file paths (empty = all success).
func (ts *transferService) UploadDir(ctx context.Context, localPath, remotePath string, onProgress func(domain.TransferProgress), onConflict domain.ConflictHandler) ([]string, error) {
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

		// Check for cancellation before processing each file
		if ctx.Err() != nil {
			return context.Canceled
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

		if err := ts.uploadSingleFile(ctx, path, remoteFile, onProgress, fileIndex, fileCount, onConflict); err != nil {
			if errors.Is(err, context.Canceled) {
				return context.Canceled
			}
			ts.log.Warnw("upload failed", "file", path, "error", err)
			failed = append(failed, path)
			if onProgress != nil {
				onProgress(domain.TransferProgress{
					FileName:  filepath.Base(path),
					FilePath:  path,
					FileIndex: fileIndex,
					FileTotal: fileCount,
					Failed:    true,
					FailError: err.Error(),
				})
			}
		}
		return nil
	})

	if errors.Is(err, context.Canceled) {
		return failed, context.Canceled
	}
	if err != nil {
		return failed, fmt.Errorf("upload dir %s: %w", localPath, err)
	}

	if len(failed) == 0 {
		return nil, nil
	}
	return failed, nil
}

// DownloadDir recursively downloads a directory from remote to local.
// ctx controls cancellation — stops remaining files if ctx is done.
// onConflict is called for each conflicting file; nil means always overwrite.
// Returns list of failed file paths (empty = all success).
func (ts *transferService) DownloadDir(ctx context.Context, remotePath, localPath string, onProgress func(domain.TransferProgress), onConflict domain.ConflictHandler) ([]string, error) {
	// Get list of all remote files
	remoteFiles, err := ts.sftp.WalkDir(remotePath)
	if err != nil {
		return nil, fmt.Errorf("walk remote %s: %w", remotePath, err)
	}

	if len(remoteFiles) == 0 {
		return nil, nil
	}

	// Create local root directory
	if err := os.MkdirAll(localPath, 0o750); err != nil {
		return nil, fmt.Errorf("mkdir local %s: %w", localPath, err)
	}

	fileCount := len(remoteFiles)
	var failed []string

	for i, remoteFile := range remoteFiles {
		// Check for cancellation before each file
		if ctx.Err() != nil {
			return failed, context.Canceled
		}

		// Calculate local path from remote relative path
		rel := strings.TrimPrefix(remoteFile, remotePath)
		rel = strings.TrimPrefix(rel, "/")
		localFile := filepath.Join(localPath, filepath.FromSlash(rel))

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(localFile), 0o750); err != nil {
			ts.log.Warnw("failed to create local parent dir", "dir", filepath.Dir(localFile), "error", err)
			failed = append(failed, remoteFile)
			if onProgress != nil {
				onProgress(domain.TransferProgress{
					FileName:  filepath.Base(remoteFile),
					FilePath:  remoteFile,
					FileIndex: i + 1,
					FileTotal: fileCount,
					Failed:    true,
					FailError: err.Error(),
				})
			}
			continue
		}

		if err := ts.downloadSingleFile(ctx, remoteFile, localFile, onProgress, i+1, fileCount, onConflict); err != nil {
			if errors.Is(err, context.Canceled) {
				return failed, context.Canceled
			}
			ts.log.Warnw("download failed", "file", remoteFile, "error", err)
			failed = append(failed, remoteFile)
			if onProgress != nil {
				onProgress(domain.TransferProgress{
					FileName:  filepath.Base(remoteFile),
					FilePath:  remoteFile,
					FileIndex: i + 1,
					FileTotal: fileCount,
					Failed:    true,
					FailError: err.Error(),
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
func (ts *transferService) uploadSingleFile(ctx context.Context, localPath, remotePath string, onProgress func(domain.TransferProgress), fileIndex, fileTotal int, onConflict domain.ConflictHandler) error {
	// Conflict detection for individual file in directory transfer
	if onConflict != nil {
		if _, err := ts.sftp.Stat(remotePath); err == nil {
			action, newPath := onConflict(filepath.Base(remotePath))
			switch action {
			case domain.ConflictSkip:
				return nil
			case domain.ConflictRename:
				remotePath = newPath
			case domain.ConflictOverwrite:
				// continue with original path
			}
		}
	}

	localFile, err := os.Open(localPath) //nolint:gosec // G304: path from user file browser selection
	if err != nil {
		return fmt.Errorf("open local %s: %w", localPath, err)
	}
	defer func() { _ = localFile.Close() }()

	stat, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("stat local %s: %w", localPath, err)
	}

	remoteFile, err := ts.sftp.CreateRemoteFile(remotePath)
	if err != nil {
		return fmt.Errorf("create remote %s: %w", remotePath, err)
	}

	err = ts.copyWithProgress(ctx, localFile, remoteFile, localPath, localPath, stat.Size(), func(p domain.TransferProgress) {
		p.FileIndex = fileIndex
		p.FileTotal = fileTotal
		if onProgress != nil {
			onProgress(p)
		}
	})
	_ = remoteFile.Close()

	// D-04: cancel cleanup
	if errors.Is(err, context.Canceled) {
		if removeErr := ts.sftp.Remove(remotePath); removeErr != nil {
			ts.log.Warnw("failed to cleanup partial remote file", "path", remotePath, "error", removeErr)
		}
		return context.Canceled
	}
	return err
}

// downloadSingleFile downloads a single file with progress tracking for directory transfers.
func (ts *transferService) downloadSingleFile(ctx context.Context, remotePath, localPath string, onProgress func(domain.TransferProgress), fileIndex, fileTotal int, onConflict domain.ConflictHandler) error {
	// Conflict detection for individual file in directory transfer
	if onConflict != nil {
		if _, err := os.Stat(localPath); err == nil {
			action, newPath := onConflict(filepath.Base(localPath))
			switch action {
			case domain.ConflictSkip:
				return nil
			case domain.ConflictRename:
				localPath = newPath
			case domain.ConflictOverwrite:
				// continue with original path
			}
		}
	}

	remoteFile, err := ts.sftp.OpenRemoteFile(remotePath)
	if err != nil {
		return fmt.Errorf("open remote %s: %w", remotePath, err)
	}
	defer func() { _ = remoteFile.Close() }()

	localFile, err := os.Create(localPath) //nolint:gosec // G304: path from user file browser selection
	if err != nil {
		return fmt.Errorf("create local %s: %w", localPath, err)
	}

	err = ts.copyWithProgress(ctx, remoteFile, localFile, remotePath, localPath, 0, func(p domain.TransferProgress) {
		p.FileIndex = fileIndex
		p.FileTotal = fileTotal
		if onProgress != nil {
			onProgress(p)
		}
	})
	_ = localFile.Close()

	// D-04: cancel cleanup
	if errors.Is(err, context.Canceled) {
		if removeErr := os.Remove(localPath); removeErr != nil {
			ts.log.Warnw("failed to cleanup partial local file", "path", localPath, "error", removeErr)
		}
		return context.Canceled
	}
	if err != nil {
		return err
	}

	// Set standard file permissions on downloaded file (cross-platform)
	setFilePermissions(localPath, 0o644, ts.log)
	return nil
}

// CopyRemoteFile copies a file within the remote filesystem by downloading to a
// temporary local file and re-uploading to the destination path (D-01).
// Temp file is cleaned up via defer regardless of success or failure.
func (ts *transferService) CopyRemoteFile(
	ctx context.Context,
	remoteSrc, remoteDst string,
	onProgress func(domain.TransferProgress),
	onConflict domain.ConflictHandler,
) error {
	// Create temp file for intermediate storage
	tmpFile, err := os.CreateTemp("", "lazyssh-copy-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()                       // DownloadFile will create its own handle
	defer func() { _ = os.Remove(tmpPath) }() // always clean up temp (Pitfall 3)

	// Phase 1: Download remote source to temp
	dlProgress := func(p domain.TransferProgress) {
		if onProgress != nil {
			onProgress(p)
		}
	}
	if err := ts.DownloadFile(ctx, remoteSrc, tmpPath, dlProgress, nil); err != nil {
		return fmt.Errorf("download for copy: %w", err)
	}

	// Phase 2: Upload temp to remote destination
	ulProgress := func(p domain.TransferProgress) {
		if onProgress != nil {
			onProgress(p)
		}
	}
	if err := ts.UploadFile(ctx, tmpPath, remoteDst, ulProgress, onConflict); err != nil {
		return fmt.Errorf("upload for copy: %w", err)
	}

	return nil
}

// CopyRemoteDir copies a directory within the remote filesystem by downloading
// to a temporary local directory and re-uploading to the destination path (D-01).
func (ts *transferService) CopyRemoteDir(
	ctx context.Context,
	remoteSrc, remoteDst string,
	onProgress func(domain.TransferProgress),
	onConflict domain.ConflictHandler,
) ([]string, error) {
	// Create temp directory for intermediate storage
	tmpDir, err := os.MkdirTemp("", "lazyssh-copydir-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }() // always clean up (Pitfall 3)

	// Extract directory name for temp sub-path
	srcBase := filepath.Base(remoteSrc)
	tmpBase := filepath.Join(tmpDir, srcBase)

	// Phase 1: Download remote directory to temp
	dlProgress := func(p domain.TransferProgress) {
		if onProgress != nil {
			onProgress(p)
		}
	}
	dlFailed, err := ts.DownloadDir(ctx, remoteSrc, tmpBase, dlProgress, nil)
	if err != nil {
		return dlFailed, fmt.Errorf("download dir for copy: %w", err)
	}

	// Phase 2: Upload temp directory to remote destination
	ulProgress := func(p domain.TransferProgress) {
		if onProgress != nil {
			onProgress(p)
		}
	}
	ulFailed, err := ts.UploadDir(ctx, tmpBase, remoteDst, ulProgress, onConflict)
	if err != nil {
		// Combine failed files from both phases
		allFailed := make([]string, len(dlFailed)+len(ulFailed))
		copy(allFailed, dlFailed)
		copy(allFailed[len(dlFailed):], ulFailed)
		return allFailed, fmt.Errorf("upload dir for copy: %w", err)
	}

	// Combine any failed files from both phases
	allFailed := make([]string, len(dlFailed)+len(ulFailed))
	copy(allFailed, dlFailed)
	copy(allFailed[len(dlFailed):], ulFailed)
	return allFailed, nil
}

// DownloadTo downloads a file from remote to local without conflict checking.
// Pure data transfer — no dialogs, no interaction.
func (ts *transferService) DownloadTo(ctx context.Context, remotePath, localPath string, onProgress func(domain.TransferProgress)) error {
	ts.log.Debugw("[Transfer] DownloadTo start", "remote", remotePath, "local", localPath)

	remoteFile, err := ts.sftp.OpenRemoteFile(remotePath)
	if err != nil {
		return fmt.Errorf("open remote %s: %w", remotePath, err)
	}
	defer func() { _ = remoteFile.Close() }()

	// Get remote file size for progress
	var total int64
	if remoteStat, statErr := ts.sftp.Stat(remotePath); statErr == nil {
		total = remoteStat.Size()
	}
	ts.log.Debugw("[Transfer] DownloadTo stat", "remote", remotePath, "size", total)

	if err := os.MkdirAll(filepath.Dir(localPath), 0o750); err != nil {
		return fmt.Errorf("mkdir local %s: %w", filepath.Dir(localPath), err)
	}

	localFile, err := os.Create(localPath) //nolint:gosec // G304: path from temp file
	if err != nil {
		return fmt.Errorf("create local %s: %w", localPath, err)
	}

	err = ts.copyWithProgress(ctx, remoteFile, localFile, remotePath, localPath, total, onProgress)
	_ = localFile.Close()

	if errors.Is(err, context.Canceled) {
		ts.log.Infow("[Transfer] DownloadTo canceled, cleaning up", "path", localPath)
		if removeErr := os.Remove(localPath); removeErr != nil {
			ts.log.Warnw("[Transfer] failed to cleanup partial file", "path", localPath, "error", removeErr)
		}
		return context.Canceled
	}
	if err != nil {
		ts.log.Errorw("[Transfer] DownloadTo failed", "remote", remotePath, "error", err)
		return err
	}

	setFilePermissions(localPath, 0o644, ts.log)
	ts.log.Debugw("[Transfer] DownloadTo done", "remote", remotePath, "local", localPath)
	return nil
}

// UploadFrom uploads a file from local to remote without conflict checking.
// Pure data transfer — no dialogs, no interaction.
func (ts *transferService) UploadFrom(ctx context.Context, localPath, remotePath string, onProgress func(domain.TransferProgress)) error {
	ts.log.Debugw("[Transfer] UploadFrom start", "local", localPath, "remote", remotePath)

	localFile, err := os.Open(localPath) //nolint:gosec // G304: path from temp file
	if err != nil {
		return fmt.Errorf("open local %s: %w", localPath, err)
	}
	defer func() { _ = localFile.Close() }()

	stat, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("stat local %s: %w", localPath, err)
	}
	total := stat.Size()
	ts.log.Debugw("[Transfer] UploadFrom stat", "local", localPath, "size", total)

	remoteFile, err := ts.sftp.CreateRemoteFile(remotePath)
	if err != nil {
		return fmt.Errorf("create remote %s: %w", remotePath, err)
	}

	err = ts.copyWithProgress(ctx, localFile, remoteFile, localPath, localPath, total, onProgress)
	_ = remoteFile.Close()

	if errors.Is(err, context.Canceled) {
		ts.log.Infow("[Transfer] UploadFrom canceled, cleaning up", "path", remotePath)
		if removeErr := ts.sftp.Remove(remotePath); removeErr != nil {
			ts.log.Warnw("[Transfer] failed to cleanup partial remote", "path", remotePath, "error", removeErr)
		}
		return context.Canceled
	}
	if err != nil {
		ts.log.Errorw("[Transfer] UploadFrom failed", "local", localPath, "error", err)
		return err
	}

	ts.log.Debugw("[Transfer] UploadFrom done", "local", localPath, "remote", remotePath)
	return nil
}

// DownloadDirTo downloads a directory from remote to local without conflict checking.
func (ts *transferService) DownloadDirTo(ctx context.Context, remotePath, localPath string, onProgress func(domain.TransferProgress)) ([]string, error) {
	return ts.DownloadDir(ctx, remotePath, localPath, onProgress, nil)
}

// UploadDirFrom uploads a directory from local to remote without conflict checking.
func (ts *transferService) UploadDirFrom(ctx context.Context, localPath, remotePath string, onProgress func(domain.TransferProgress)) ([]string, error) {
	return ts.UploadDir(ctx, localPath, remotePath, onProgress, nil)
}

// copyWithProgress copies data from src to dst using a 32KB buffer,
// calling onProgress after each chunk. Checks ctx.Done() before each
// Read to support cancellation with at most 32KB delay.
func (ts *transferService) copyWithProgress(ctx context.Context, src io.Reader, dst io.Writer, srcPath, displayPath string, total int64, onProgress func(domain.TransferProgress)) error {
	buf := make([]byte, 32*1024)
	var transferred int64
	fileName := filepath.Base(displayPath)

	for {
		// Check cancellation before each chunk read
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
		}

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
