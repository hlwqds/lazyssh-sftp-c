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

// RelayTransferService transfers files between two remote servers via local relay.
// Files are downloaded from the source SFTP to a local temp file/directory,
// then uploaded from the temp to the target SFTP.
type RelayTransferService interface {
	// RelayFile transfers a single file from source to target via local temp.
	// Download from srcSFTP -> temp file -> upload to dstSFTP.
	// Temp file is cleaned up on all code paths (success, error, cancel).
	RelayFile(ctx context.Context, srcPath, dstPath string,
		onProgress func(domain.TransferProgress),
		onConflict domain.ConflictHandler) error

	// RelayDir transfers a directory recursively from source to target via local temp.
	// Download from srcSFTP -> temp dir -> upload to dstSFTP.
	// Returns list of failed file paths (empty = all success).
	// Temp directory is cleaned up on all code paths.
	RelayDir(ctx context.Context, srcPath, dstPath string,
		onProgress func(domain.TransferProgress),
		onConflict domain.ConflictHandler) ([]string, error)
}
