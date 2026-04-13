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

import "github.com/Adembc/lazyssh/internal/core/domain"

// TransferService provides file transfer operations between local filesystem and remote SFTP.
// Implementations coordinate local os operations with remote SFTPService operations.
type TransferService interface {
	// UploadFile uploads a single file from local to remote.
	// onProgress is called periodically during transfer.
	UploadFile(localPath, remotePath string, onProgress func(domain.TransferProgress)) error

	// DownloadFile downloads a single file from remote to local.
	// onProgress is called periodically during transfer.
	DownloadFile(remotePath, localPath string, onProgress func(domain.TransferProgress)) error

	// UploadDir recursively uploads a directory from local to remote.
	// Returns list of failed file paths (empty = all success).
	UploadDir(localPath, remotePath string, onProgress func(domain.TransferProgress)) ([]string, error)

	// DownloadDir recursively downloads a directory from remote to local.
	// Returns list of failed file paths (empty = all success).
	DownloadDir(remotePath, localPath string, onProgress func(domain.TransferProgress)) ([]string, error)
}
