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

// TransferProgress represents the current state of an in-progress file transfer.
// It is passed from the transfer service layer to UI components via callbacks.
type TransferProgress struct {
	FileName  string // current file being transferred
	FilePath  string // full path of the current file
	BytesDone int64  // bytes transferred so far for current file
	BytesTotal int64 // total bytes of the current file
	FileIndex int    // current file index in multi-file transfers (1-based)
	FileTotal int    // total number of files in the transfer (0 if single file)
	Done      bool   // true when the entire transfer is complete
	Failed    bool   // true when the transfer has failed
}
