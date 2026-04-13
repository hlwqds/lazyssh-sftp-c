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
	"context"

	"github.com/Adembc/lazyssh/internal/core/domain"
)

// TransferService provides file transfer operations between local filesystem and remote SFTP.
// Implementations coordinate local os operations with remote SFTPService operations.
// All methods accept ctx context.Context as first parameter for cancel propagation.
// All methods accept onConflict callback for conflict resolution (nil = always overwrite).
type TransferService interface {
	// UploadFile uploads a single file from local to remote.
	// ctx controls cancellation — returns context.Canceled if ctx is done.
	// onProgress is called periodically during transfer.
	// onConflict is called when remote file exists; returns action and new path for Rename.
	UploadFile(ctx context.Context, localPath, remotePath string, onProgress func(domain.TransferProgress), onConflict domain.ConflictHandler) error

	// DownloadFile downloads a single file from remote to local.
	// ctx controls cancellation — returns context.Canceled if ctx is done.
	// onProgress is called periodically during transfer.
	// onConflict is called when local file exists; returns action and new path for Rename.
	DownloadFile(ctx context.Context, remotePath, localPath string, onProgress func(domain.TransferProgress), onConflict domain.ConflictHandler) error

	// UploadDir recursively uploads a directory from local to remote.
	// ctx controls cancellation — stops remaining files if ctx is done.
	// onConflict is called for each conflicting file.
	// Returns list of failed file paths (empty = all success).
	UploadDir(ctx context.Context, localPath, remotePath string, onProgress func(domain.TransferProgress), onConflict domain.ConflictHandler) ([]string, error)

	// DownloadDir recursively downloads a directory from remote to local.
	// ctx controls cancellation — stops remaining files if ctx is done.
	// onConflict is called for each conflicting file.
	// Returns list of failed file paths (empty = all success).
	DownloadDir(ctx context.Context, remotePath, localPath string, onProgress func(domain.TransferProgress), onConflict domain.ConflictHandler) ([]string, error)
}
