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
	"fmt"
	"os"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"github.com/Adembc/lazyssh/internal/core/ports"
	"go.uber.org/zap"
)

// relayTransferService transfers files between two independent SFTP connections
// via a local temporary file/directory relay. It composes two transfer.New()
// instances — one per SFTP connection — to reuse the existing 32KB buffered
// copy logic with progress and cancellation support.
type relayTransferService struct {
	log     *zap.SugaredLogger
	srcSFTP ports.SFTPService
	dstSFTP ports.SFTPService
}

// NewRelay creates a new RelayTransferService adapter.
// srcSFTP is the source remote connection, dstSFTP is the target remote connection.
func NewRelay(log *zap.SugaredLogger, srcSFTP, dstSFTP ports.SFTPService) *relayTransferService {
	return &relayTransferService{log: log, srcSFTP: srcSFTP, dstSFTP: dstSFTP}
}

// RelayFile transfers a single file from source to target via local temp.
// Download from srcSFTP -> temp file -> upload to dstSFTP.
// Temp file is cleaned up on all code paths (success, error, cancel).
func (rs *relayTransferService) RelayFile(
	ctx context.Context,
	srcPath, dstPath string,
	onProgress func(domain.TransferProgress),
	onConflict domain.ConflictHandler,
) error {
	// Create temp file for intermediate storage
	tmpFile, err := os.CreateTemp("", "lazyssh-relay-*")
	if err != nil {
		return fmt.Errorf("create relay temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()                       // DownloadFile will create its own handle
	defer func() { _ = os.Remove(tmpPath) }() // always clean up temp on all paths

	// Phase 1: Download from source SFTP to temp
	dlSvc := New(rs.log, rs.srcSFTP)
	dlProgress := func(p domain.TransferProgress) {
		if onProgress != nil {
			onProgress(p)
		}
	}
	if err := dlSvc.DownloadFile(ctx, srcPath, tmpPath, dlProgress, nil); err != nil {
		return fmt.Errorf("relay download from source: %w", err)
	}

	// Phase 2: Upload from temp to target SFTP
	ulSvc := New(rs.log, rs.dstSFTP)
	ulProgress := func(p domain.TransferProgress) {
		if onProgress != nil {
			onProgress(p)
		}
	}
	if err := ulSvc.UploadFile(ctx, tmpPath, dstPath, ulProgress, onConflict); err != nil {
		return fmt.Errorf("relay upload to target: %w", err)
	}

	return nil
}

// RelayDir transfers a directory recursively from source to target via local temp.
// Download from srcSFTP -> temp dir -> upload to dstSFTP.
// Returns list of failed file paths (empty = all success).
// Temp directory is cleaned up on all code paths.
func (rs *relayTransferService) RelayDir(
	ctx context.Context,
	srcPath, dstPath string,
	onProgress func(domain.TransferProgress),
	onConflict domain.ConflictHandler,
) ([]string, error) {
	// Create temp directory for intermediate storage
	tmpDir, err := os.MkdirTemp("", "lazyssh-relaydir-*")
	if err != nil {
		return nil, fmt.Errorf("create relay temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }() // always clean up on all paths

	// Extract directory name for temp sub-path
	srcBase := extractBaseName(srcPath)
	tmpBase := tmpDir + "/" + srcBase

	// Phase 1: Download directory from source SFTP to temp
	dlSvc := New(rs.log, rs.srcSFTP)
	dlProgress := func(p domain.TransferProgress) {
		if onProgress != nil {
			onProgress(p)
		}
	}
	dlFailed, err := dlSvc.DownloadDir(ctx, srcPath, tmpBase, dlProgress, nil)
	if err != nil {
		return dlFailed, fmt.Errorf("relay download dir from source: %w", err)
	}

	// Phase 2: Upload directory from temp to target SFTP
	ulSvc := New(rs.log, rs.dstSFTP)
	ulProgress := func(p domain.TransferProgress) {
		if onProgress != nil {
			onProgress(p)
		}
	}
	ulFailed, err := ulSvc.UploadDir(ctx, tmpBase, dstPath, ulProgress, onConflict)
	if err != nil {
		// Combine failed files from both phases
		allFailed := make([]string, len(dlFailed)+len(ulFailed))
		copy(allFailed, dlFailed)
		copy(allFailed[len(dlFailed):], ulFailed)
		return allFailed, fmt.Errorf("relay upload dir to target: %w", err)
	}

	// Combine any failed files from both phases
	allFailed := make([]string, len(dlFailed)+len(ulFailed))
	copy(allFailed, dlFailed)
	copy(allFailed[len(dlFailed):], ulFailed)
	return allFailed, nil
}

// extractBaseName returns the last path component from a remote path.
// Handles both Unix-style paths (/a/b/c -> c) and Windows-style paths (C:\a\b\c -> c).
func extractBaseName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}
