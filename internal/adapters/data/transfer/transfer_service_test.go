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
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"go.uber.org/zap"
)

// newTestLogger creates a no-op test logger.
func newTestLogger() *zap.SugaredLogger {
	logger, _ := zap.NewDevelopment()
	return logger.Sugar()
}

// mockSFTPService implements ports.SFTPService for testing.
type mockSFTPService struct {
	connected bool
	// For CreateRemoteFile/OpenRemoteFile: control what they return
	createErr error
	openErr   error
	openData  []byte
}

func (m *mockSFTPService) ListDir(path string, showHidden bool, sortField domain.FileSortField, sortAsc bool) ([]domain.FileInfo, error) {
	return nil, nil
}

func (m *mockSFTPService) Connect(server domain.Server) error {
	m.connected = true
	return nil
}

func (m *mockSFTPService) Close() error {
	m.connected = false
	return nil
}

func (m *mockSFTPService) IsConnected() bool {
	return m.connected
}

func (m *mockSFTPService) CreateRemoteFile(path string) (io.WriteCloser, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	return &mockWriteCloser{buf: &strings.Builder{}}, nil
}

func (m *mockSFTPService) OpenRemoteFile(path string) (io.ReadCloser, error) {
	if m.openErr != nil {
		return nil, m.openErr
	}
	return io.NopCloser(strings.NewReader(string(m.openData))), nil
}

func (m *mockSFTPService) MkdirAll(path string) error {
	return nil
}

func (m *mockSFTPService) WalkDir(path string) ([]string, error) {
	return nil, nil
}

// mockWriteCloser is a simple io.WriteCloser that writes to a strings.Builder.
type mockWriteCloser struct {
	buf    *strings.Builder
	closed bool
}

func (m *mockWriteCloser) Write(p []byte) (n int, err error) {
	return m.buf.Write(p)
}

func (m *mockWriteCloser) Close() error {
	m.closed = true
	return nil
}

// TestNew verifies the constructor returns a non-nil service.
func TestNew(t *testing.T) {
	svc := New(newTestLogger(), &mockSFTPService{})
	if svc == nil {
		t.Fatal("New() returned nil")
	}
}

// TestUploadFile_LocalFileNotFound verifies error when source file does not exist.
func TestUploadFile_LocalFileNotFound(t *testing.T) {
	svc := New(newTestLogger(), &mockSFTPService{})
	err := svc.UploadFile("/nonexistent/path/file.txt", "/remote/file.txt", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent local file, got nil")
	}
	if !strings.Contains(err.Error(), "open local") {
		t.Errorf("expected 'open local' error, got: %v", err)
	}
}

// TestUploadFile_RemoteCreateError verifies error when remote file creation fails.
func TestUploadFile_RemoteCreateError(t *testing.T) {
	// Create a temp local file
	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "testfile.txt")
	if err := os.WriteFile(localFile, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	mock := &mockSFTPService{
		connected: true,
		createErr: errors.New("permission denied"),
	}
	svc := New(newTestLogger(), mock)
	err := svc.UploadFile(localFile, "/remote/file.txt", nil)
	if err == nil {
		t.Fatal("expected error for remote create failure, got nil")
	}
	if !strings.Contains(err.Error(), "create remote") {
		t.Errorf("expected 'create remote' error, got: %v", err)
	}
}

// TestDownloadFile_RemoteOpenError verifies error when remote file open fails.
func TestDownloadFile_RemoteOpenError(t *testing.T) {
	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "download.txt")

	mock := &mockSFTPService{
		connected: true,
		openErr:   errors.New("file not found"),
	}
	svc := New(newTestLogger(), mock)
	err := svc.DownloadFile("/remote/missing.txt", localFile, nil)
	if err == nil {
		t.Fatal("expected error for remote open failure, got nil")
	}
	if !strings.Contains(err.Error(), "open remote") {
		t.Errorf("expected 'open remote' error, got: %v", err)
	}
}

// TestUploadFile_ProgressCallback verifies progress callbacks fire during upload.
func TestUploadFile_ProgressCallback(t *testing.T) {
	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "upload.txt")
	content := "test content for progress"
	if err := os.WriteFile(localFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	mock := &mockSFTPService{
		connected: true,
	}
	svc := New(newTestLogger(), mock)

	var progressCount int
	var lastDone bool
	err := svc.UploadFile(localFile, "/remote/upload.txt", func(p domain.TransferProgress) {
		progressCount++
		lastDone = p.Done
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if progressCount == 0 {
		t.Error("expected at least one progress callback")
	}
	if !lastDone {
		t.Error("expected final progress callback with Done=true")
	}
}

// TestUploadDir_EmptyDirectory verifies uploading an empty directory returns no errors.
func TestUploadDir_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	emptyDir := filepath.Join(tmpDir, "empty")
	if err := os.MkdirAll(emptyDir, 0o755); err != nil {
		t.Fatal(err)
	}

	mock := &mockSFTPService{connected: true}
	svc := New(newTestLogger(), mock)

	failed, err := svc.UploadDir(emptyDir, "/remote/empty", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failed) != 0 {
		t.Errorf("expected no failed files, got: %v", failed)
	}
}

// TestDownloadDir_EmptyRemote verifies downloading from an empty remote returns no errors.
func TestDownloadDir_EmptyRemote(t *testing.T) {
	tmpDir := t.TempDir()

	mock := &mockSFTPService{connected: true}
	svc := New(newTestLogger(), mock)

	failed, err := svc.DownloadDir("/remote/empty", tmpDir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failed) != 0 {
		t.Errorf("expected no failed files, got: %v", failed)
	}
}
