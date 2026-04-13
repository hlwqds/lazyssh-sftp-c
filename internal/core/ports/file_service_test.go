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
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/Adembc/lazyssh/internal/core/domain"
)

// mockFileService is a minimal mock implementing FileService for compilation verification.
type mockFileService struct{}

func (m *mockFileService) ListDir(path string, showHidden bool, sortField domain.FileSortField, sortAsc bool) ([]domain.FileInfo, error) {
	return nil, nil
}

// mockSFTPService is a minimal mock implementing SFTPService for compilation verification.
type mockSFTPService struct {
	mockFileService
}

func (m *mockSFTPService) Connect(server domain.Server) error {
	return nil
}

func (m *mockSFTPService) Close() error {
	return nil
}

func (m *mockSFTPService) IsConnected() bool {
	return false
}

func (m *mockSFTPService) CreateRemoteFile(path string) (io.WriteCloser, error) {
	return nil, nil
}

func (m *mockSFTPService) OpenRemoteFile(path string) (io.ReadCloser, error) {
	return nil, nil
}

func (m *mockSFTPService) MkdirAll(path string) error {
	return nil
}

func (m *mockSFTPService) WalkDir(path string) ([]string, error) {
	return nil, nil
}

func (m *mockSFTPService) Stat(path string) (os.FileInfo, error) {
	return nil, fmt.Errorf("not found")
}

func (m *mockSFTPService) Remove(path string) error {
	return nil
}

// TestSFTPServiceStat verifies SFTPService interface has Stat method.
func TestSFTPServiceStat(t *testing.T) {
	sftpType := reflect.TypeOf((*SFTPService)(nil)).Elem()
	method, ok := sftpType.MethodByName("Stat")
	if !ok {
		t.Fatal("SFTPService interface missing Stat method")
	}
	// Verify Stat signature: Stat(path string) (os.FileInfo, error)
	if method.Type.NumIn() != 2 { // receiver + path
		t.Errorf("Stat should have 1 parameter, got %d", method.Type.NumIn()-1)
	}
	if method.Type.NumOut() != 2 {
		t.Errorf("Stat should have 2 return values, got %d", method.Type.NumOut())
	}
}

// TestSFTPServiceRemove verifies SFTPService interface has Remove method.
func TestSFTPServiceRemove(t *testing.T) {
	sftpType := reflect.TypeOf((*SFTPService)(nil)).Elem()
	method, ok := sftpType.MethodByName("Remove")
	if !ok {
		t.Fatal("SFTPService interface missing Remove method")
	}
	// Verify Remove signature: Remove(path string) error
	if method.Type.NumIn() != 2 { // receiver + path
		t.Errorf("Remove should have 1 parameter, got %d", method.Type.NumIn()-1)
	}
	if method.Type.NumOut() != 1 {
		t.Errorf("Remove should have 1 return value, got %d", method.Type.NumOut())
	}
}

// TestFileServiceInterface verifies FileService interface exists and has correct methods.
func TestFileServiceInterface(t *testing.T) {
	fsType := reflect.TypeOf((*FileService)(nil)).Elem()

	method, ok := fsType.MethodByName("ListDir")
	if !ok {
		t.Fatal("FileService interface missing ListDir method")
	}

	// Verify ListDir signature: ListDir(path string, showHidden bool, sortField domain.FileSortField, sortAsc bool) ([]domain.FileInfo, error)
	methodType := method.Type
	if methodType.NumIn() != 4 {
		t.Errorf("ListDir should have 4 parameters, got %d", methodType.NumIn())
	}
	if methodType.NumOut() != 2 {
		t.Errorf("ListDir should have 2 return values, got %d", methodType.NumOut())
	}
}

// TestSFTPServiceInterface verifies SFTPService interface embeds FileService and adds Connect/Close/IsConnected.
func TestSFTPServiceInterface(t *testing.T) {
	sftpType := reflect.TypeOf((*SFTPService)(nil)).Elem()

	// SFTPService should have FileService methods
	_, ok := sftpType.MethodByName("ListDir")
	if !ok {
		t.Fatal("SFTPService interface missing ListDir method (from FileService)")
	}

	// SFTPService should have its own methods
	for _, method := range []string{"Connect", "Close", "IsConnected", "CreateRemoteFile", "OpenRemoteFile", "MkdirAll", "WalkDir", "Stat", "Remove"} {
		_, ok := sftpType.MethodByName(method)
		if !ok {
			t.Errorf("SFTPService interface missing %s method", method)
		}
	}
}

// TestFileServiceMockImplements verifies mock struct satisfies FileService interface.
func TestFileServiceMockImplements(t *testing.T) {
	var _ FileService = (*mockFileService)(nil)
}

// TestSFTPServiceMockImplements verifies mock struct satisfies SFTPService interface.
func TestSFTPServiceMockImplements(t *testing.T) {
	var _ SFTPService = (*mockSFTPService)(nil)
}
