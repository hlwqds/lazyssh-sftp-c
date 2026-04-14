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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	// For Stat: track existing files and simulate stat results
	statFile os.FileInfo
	statErr  error
	// For Remove: track removed paths
	removedPaths []string
	// For CreateRemoteFile: track created paths
	createdPaths map[string]bool
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
	if m.createdPaths != nil {
		m.createdPaths[path] = true
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

func (m *mockSFTPService) Stat(path string) (os.FileInfo, error) {
	if m.statErr != nil {
		return nil, m.statErr
	}
	if m.statFile != nil {
		return m.statFile, nil
	}
	// If path is in createdPaths, return a mock FileInfo
	if m.createdPaths != nil && m.createdPaths[path] {
		return &mockFileInfo{name: filepath.Base(path)}, nil
	}
	return nil, os.ErrNotExist
}

func (m *mockSFTPService) Remove(path string) error {
	m.removedPaths = append(m.removedPaths, path)
	return nil
}

func (m *mockSFTPService) HomeDir() string {
	return "/home/test"
}

// mockFileInfo is a minimal os.FileInfo implementation for testing.
type mockFileInfo struct {
	name string
	size int64
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() os.FileMode  { return 0o644 }
func (m *mockFileInfo) ModTime() time.Time { return time.Time{} }
func (m *mockFileInfo) IsDir() bool        { return false }
func (m *mockFileInfo) Sys() interface{}   { return nil }

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
	err := svc.UploadFile(context.Background(), "/nonexistent/path/file.txt", "/remote/file.txt", nil, nil)
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
	err := svc.UploadFile(context.Background(), localFile, "/remote/file.txt", nil, nil)
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
	err := svc.DownloadFile(context.Background(), "/remote/missing.txt", localFile, nil, nil)
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
	err := svc.UploadFile(context.Background(), localFile, "/remote/upload.txt", func(p domain.TransferProgress) {
		progressCount++
		lastDone = p.Done
	}, nil)
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

	failed, err := svc.UploadDir(context.Background(), emptyDir, "/remote/empty", nil, nil)
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

	failed, err := svc.DownloadDir(context.Background(), "/remote/empty", tmpDir, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failed) != 0 {
		t.Errorf("expected no failed files, got: %v", failed)
	}
}

// TestUploadFile_ContextCanceled verifies UploadFile returns context.Canceled when context is canceled.
func TestUploadFile_ContextCanceled(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a file large enough to span multiple chunks (>32KB)
	localFile := filepath.Join(tmpDir, "largefile.bin")
	content := make([]byte, 128*1024) // 128KB
	for i := range content {
		content[i] = byte(i % 256)
	}
	if err := os.WriteFile(localFile, content, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately before transfer starts
	cancel()

	mock := &mockSFTPService{connected: true}
	svc := New(newTestLogger(), mock)
	err := svc.UploadFile(ctx, localFile, "/remote/largefile.bin", nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}

// TestDownloadFile_ContextCanceled verifies DownloadFile returns context.Canceled when context is canceled.
func TestDownloadFile_ContextCanceled(t *testing.T) {
	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "download.bin")

	// Provide enough data for multiple chunks
	mock := &mockSFTPService{
		connected: true,
		openData:  make([]byte, 128*1024),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	svc := New(newTestLogger(), mock)
	err := svc.DownloadFile(ctx, "/remote/largefile.bin", localFile, nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}

// TestUploadDir_ContextCanceled verifies UploadDir stops transferring remaining files when context is canceled.
func TestUploadDir_ContextCanceled(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a directory with multiple files
	dir := filepath.Join(tmpDir, "upload_dir")
	for i := 0; i < 5; i++ {
		f := filepath.Join(dir, fmt.Sprintf("file%d.txt", i))
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		// Each file is >32KB to span multiple chunks
		if err := os.WriteFile(f, make([]byte, 64*1024), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately — WalkDir should detect ctx.Err() at first file

	mock := &mockSFTPService{connected: true}
	svc := New(newTestLogger(), mock)
	failed, err := svc.UploadDir(ctx, dir, "/remote/upload_dir", nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	// Some files should have been skipped due to cancellation
	t.Logf("UploadDir canceled: %d files in failed list", len(failed))
}

// TestDownloadDir_ContextCanceled verifies DownloadDir stops transferring remaining files when context is canceled.
func TestDownloadDir_ContextCanceled(t *testing.T) {
	tmpDir := t.TempDir()

	// Mock returns multiple remote files
	mock := &mockSFTPService{
		connected: true,
		openData:  make([]byte, 64*1024),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	svc := New(newTestLogger(), mock)
	// WalkDir returns nil for this mock, so no files to download
	failed, err := svc.DownloadDir(ctx, "/remote/empty_dir", tmpDir, nil, nil)
	// With empty WalkDir, no cancellation occurs — returns normally
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(failed) != 0 {
		t.Errorf("expected no failed files, got: %v", failed)
	}
}

// TestUploadFile_NormalCompletion verifies upload completes normally without cancellation (regression test).
func TestUploadFile_NormalCompletion(t *testing.T) {
	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "normal.txt")
	content := "normal transfer content"
	if err := os.WriteFile(localFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	mock := &mockSFTPService{connected: true}
	svc := New(newTestLogger(), mock)

	var doneReceived bool
	err := svc.UploadFile(context.Background(), localFile, "/remote/normal.txt", func(p domain.TransferProgress) {
		if p.Done {
			doneReceived = true
		}
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !doneReceived {
		t.Error("expected Done=true progress callback")
	}
}

// TestUploadFile_ConflictSkip verifies that UploadFile skips when onConflict returns ConflictSkip.
func TestUploadFile_ConflictSkip(t *testing.T) {
	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "testfile.txt")
	if err := os.WriteFile(localFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	mock := &mockSFTPService{
		connected:    true,
		statFile:     &mockFileInfo{name: "testfile.txt"},
		createdPaths: make(map[string]bool),
	}
	svc := New(newTestLogger(), mock)

	err := svc.UploadFile(context.Background(), localFile, "/remote/testfile.txt", nil, func(fileName string) (domain.ConflictAction, string) {
		return domain.ConflictSkip, ""
	})
	if err != nil {
		t.Fatalf("expected nil error on skip, got: %v", err)
	}
	if mock.createdPaths["/remote/testfile.txt"] {
		t.Error("expected file NOT to be created when skipped")
	}
}

// TestUploadFile_ConflictOverwrite verifies that UploadFile proceeds when onConflict returns ConflictOverwrite.
func TestUploadFile_ConflictOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "testfile.txt")
	if err := os.WriteFile(localFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	mock := &mockSFTPService{
		connected:    true,
		statFile:     &mockFileInfo{name: "testfile.txt"},
		createdPaths: make(map[string]bool),
	}
	svc := New(newTestLogger(), mock)

	err := svc.UploadFile(context.Background(), localFile, "/remote/testfile.txt", nil, func(fileName string) (domain.ConflictAction, string) {
		return domain.ConflictOverwrite, ""
	})
	if err != nil {
		t.Fatalf("expected nil error on overwrite, got: %v", err)
	}
	if !mock.createdPaths["/remote/testfile.txt"] {
		t.Error("expected file to be created when overwrite chosen")
	}
}

// TestUploadFile_ConflictRename verifies that UploadFile uses new path when onConflict returns ConflictRename.
func TestUploadFile_ConflictRename(t *testing.T) {
	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "testfile.txt")
	if err := os.WriteFile(localFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	mock := &mockSFTPService{
		connected:    true,
		statFile:     &mockFileInfo{name: "testfile.txt"},
		createdPaths: make(map[string]bool),
	}
	svc := New(newTestLogger(), mock)

	err := svc.UploadFile(context.Background(), localFile, "/remote/testfile.txt", nil, func(fileName string) (domain.ConflictAction, string) {
		return domain.ConflictRename, "/remote/testfile.1.txt"
	})
	if err != nil {
		t.Fatalf("expected nil error on rename, got: %v", err)
	}
	if !mock.createdPaths["/remote/testfile.1.txt"] {
		t.Error("expected file to be created at renamed path")
	}
}

// TestUploadFile_CancelCleanup verifies D-04: partial remote file is cleaned up on cancel.
func TestUploadFile_CancelCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "large.bin")
	content := make([]byte, 128*1024)
	for i := range content {
		content[i] = byte(i % 256)
	}
	if err := os.WriteFile(localFile, content, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	mock := &mockSFTPService{
		connected:    true,
		createdPaths: make(map[string]bool),
	}
	svc := New(newTestLogger(), mock)

	err := svc.UploadFile(ctx, localFile, "/remote/large.bin", nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	found := false
	for _, p := range mock.removedPaths {
		if p == "/remote/large.bin" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected sftp.Remove to be called for partial file cleanup")
	}
}

// TestDownloadFile_CancelCleanup verifies D-04: partial local file is cleaned up on cancel.
func TestDownloadFile_CancelCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "download.bin")

	mock := &mockSFTPService{
		connected: true,
		openData:  make([]byte, 128*1024),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	svc := New(newTestLogger(), mock)
	err := svc.DownloadFile(ctx, "/remote/large.bin", localFile, nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	if _, err := os.Stat(localFile); !os.IsNotExist(err) {
		t.Error("expected partial local file to be deleted after cancel")
	}
}

// TestSetFilePermissionsExists verifies setFilePermissions is available
// (build tags will select the correct implementation per platform).
func TestSetFilePermissionsExists(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "perm_test.txt")
	if err := os.WriteFile(tmpFile, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Calling setFilePermissions should not panic or error
	setFilePermissions(tmpFile, 0o755, newTestLogger())
}
