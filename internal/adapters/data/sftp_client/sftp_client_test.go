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

package sftp_client

import (
	"strings"
	"testing"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"github.com/Adembc/lazyssh/internal/core/ports"
	"go.uber.org/zap"
)

// newTestLogger creates a no-op test logger.
func newTestLogger() *zap.SugaredLogger {
	logger, _ := zap.NewDevelopment()
	return logger.Sugar()
}

// TestNew creates a client and verifies it is not nil.
func TestNew(t *testing.T) {
	c := New(newTestLogger())
	if c == nil {
		t.Fatal("New() returned nil")
	}
}

// TestIsConnected_BeforeConnect verifies IsConnected returns false before Connect.
func TestIsConnected_BeforeConnect(t *testing.T) {
	c := New(newTestLogger())
	if c.IsConnected() {
		t.Error("IsConnected should return false before Connect")
	}
}

// TestClose_Unconnected verifies Close on unconnected client does not panic.
func TestClose_Unconnected(t *testing.T) {
	c := New(newTestLogger())
	// Should not panic
	err := c.Close()
	if err != nil {
		t.Errorf("Close on unconnected client should not return error, got: %v", err)
	}
}

// TestHomeDir_BeforeConnect verifies HomeDir returns empty before Connect.
func TestHomeDir_BeforeConnect(t *testing.T) {
	c := New(newTestLogger())
	if c.HomeDir() != "" {
		t.Errorf("HomeDir should return empty before Connect, got: %q", c.HomeDir())
	}
}

// TestBuildSSHArgs_SimpleServer verifies buildSSHArgs produces correct args for a simple user@host server.
func TestBuildSSHArgs_SimpleServer(t *testing.T) {
	server := domain.Server{
		Alias: "myserver",
		Host:  "192.168.1.100",
		User:  "admin",
	}

	args := buildSSHArgs(server)

	// Should start with "ssh"
	if args[0] != "ssh" {
		t.Errorf("args[0] = %q, want %q", args[0], "ssh")
	}

	// Should end with user@host
	last := args[len(args)-1]
	if last != "admin@192.168.1.100" {
		t.Errorf("last arg = %q, want %q", last, "admin@192.168.1.100")
	}
}

// TestBuildSSHArgs_HostOnly verifies buildSSHArgs uses host when user is empty.
func TestBuildSSHArgs_HostOnly(t *testing.T) {
	server := domain.Server{
		Alias: "myserver",
		Host:  "192.168.1.100",
	}

	args := buildSSHArgs(server)
	last := args[len(args)-1]
	if last != "192.168.1.100" {
		t.Errorf("last arg = %q, want %q", last, "192.168.1.100")
	}
}

// TestBuildSSHArgs_AliasFallback verifies buildSSHArgs uses alias when both user and host are empty.
func TestBuildSSHArgs_AliasFallback(t *testing.T) {
	server := domain.Server{
		Alias: "myserver",
	}

	args := buildSSHArgs(server)
	last := args[len(args)-1]
	if last != "myserver" {
		t.Errorf("last arg = %q, want %q", last, "myserver")
	}
}

// TestBuildSSHArgs_ProxyJump verifies buildSSHArgs includes -J flag when ProxyJump is set.
func TestBuildSSHArgs_ProxyJump(t *testing.T) {
	server := domain.Server{
		Alias:     "target",
		Host:      "10.0.0.1",
		User:      "admin",
		ProxyJump: "bastion",
	}

	args := buildSSHArgs(server)

	foundProxyJump := false
	for i, arg := range args {
		if arg == "-J" && i+1 < len(args) && args[i+1] == "bastion" {
			foundProxyJump = true
			break
		}
	}
	if !foundProxyJump {
		t.Errorf("expected -J bastion in args, got: %v", args)
	}
}

// TestBuildSSHArgs_Port verifies buildSSHArgs includes -p flag for non-default ports.
func TestBuildSSHArgs_Port(t *testing.T) {
	tests := []struct {
		name     string
		port     int
		wantPort bool
	}{
		{"default port 22", 22, false},
		{"zero port", 0, false},
		{"custom port 2222", 2222, true},
		{"port 8022", 8022, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := domain.Server{
				Alias: "srv",
				Host:  "1.2.3.4",
				Port:  tt.port,
			}

			args := buildSSHArgs(server)

			foundPort := false
			for i, arg := range args {
				if arg == "-p" && i+1 < len(args) {
					foundPort = true
					break
				}
			}

			if tt.wantPort && !foundPort {
				t.Errorf("expected -p flag for port %d, got: %v", tt.port, args)
			}
			if !tt.wantPort && foundPort {
				t.Errorf("did not expect -p flag for port %d, got: %v", tt.port, args)
			}
		})
	}
}

// TestBuildSSHArgs_IdentityFile verifies buildSSHArgs includes -i flags for identity files.
func TestBuildSSHArgs_IdentityFile(t *testing.T) {
	server := domain.Server{
		Alias:         "srv",
		Host:          "1.2.3.4",
		IdentityFiles: []string{"~/.ssh/id_ed25519", "~/.ssh/id_rsa"},
	}

	args := buildSSHArgs(server)

	idCount := 0
	for i, arg := range args {
		if arg == "-i" && i+1 < len(args) {
			idCount++
		}
	}
	if idCount != 2 {
		t.Errorf("expected 2 -i flags, got %d in args: %v", idCount, args)
	}
}

// TestBuildSSHArgs_IdentityFileWithSpaces verifies identity files with spaces are quoted.
func TestBuildSSHArgs_IdentityFileWithSpaces(t *testing.T) {
	server := domain.Server{
		Alias:         "srv",
		Host:          "1.2.3.4",
		IdentityFiles: []string{"my key file"},
	}

	args := buildSSHArgs(server)

	for i, arg := range args {
		if arg == "-i" && i+1 < len(args) {
			if !strings.Contains(args[i+1], "\"") {
				t.Errorf("identity file with spaces should be quoted, got: %q", args[i+1])
			}
		}
	}
}

// TestBuildSSHArgs_RemoteCommand verifies buildSSHArgs appends RemoteCommand after host.
func TestBuildSSHArgs_RemoteCommand(t *testing.T) {
	server := domain.Server{
		Alias:         "srv",
		Host:          "1.2.3.4",
		RemoteCommand: "uptime",
	}

	args := buildSSHArgs(server)
	last := args[len(args)-1]
	if last != "uptime" {
		t.Errorf("last arg = %q, want %q (RemoteCommand)", last, "uptime")
	}
}

// TestBuildSSHArgs_RemoteCommandNone verifies buildSSHArgs handles RemoteCommand=none.
func TestBuildSSHArgs_RemoteCommandNone(t *testing.T) {
	server := domain.Server{
		Alias:         "srv",
		Host:          "1.2.3.4",
		RemoteCommand: "none",
	}

	args := buildSSHArgs(server)

	// RemoteCommand=none should produce "-o RemoteCommand=none"
	found := false
	for i, arg := range args {
		if arg == "-o" && i+1 < len(args) && args[i+1] == "RemoteCommand=none" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected '-o RemoteCommand=none' for RemoteCommand=none, got: %v", args)
	}
}

// TestBuildSSHArgs_Compression verifies buildSSHArgs adds -C flag when Compression=yes.
func TestBuildSSHArgs_Compression(t *testing.T) {
	server := domain.Server{
		Alias:       "srv",
		Host:        "1.2.3.4",
		Compression: "yes",
	}

	args := buildSSHArgs(server)

	found := false
	for _, arg := range args {
		if arg == "-C" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected -C flag when Compression=yes, got: %v", args)
	}
}

// TestRemoveAll_NotConnected verifies RemoveAll returns error when not connected.
func TestRemoveAll_NotConnected(t *testing.T) {
	c := New(newTestLogger())
	err := c.RemoveAll("/some/path")
	if err == nil {
		t.Error("RemoveAll should return error when not connected")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("RemoveAll error should mention 'not connected', got: %v", err)
	}
}

// TestRename_NotConnected verifies Rename returns error when not connected.
func TestRename_NotConnected(t *testing.T) {
	c := New(newTestLogger())
	err := c.Rename("/old/path", "/new/path")
	if err == nil {
		t.Error("Rename should return error when not connected")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("Rename error should mention 'not connected', got: %v", err)
	}
}

// TestMkdir_NotConnected verifies Mkdir returns error when not connected.
func TestMkdir_NotConnected(t *testing.T) {
	c := New(newTestLogger())
	err := c.Mkdir("/some/dir")
	if err == nil {
		t.Error("Mkdir should return error when not connected")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("Mkdir error should mention 'not connected', got: %v", err)
	}
}

// TestSFTPClient_ImplementsSFTPService verifies SFTPClient satisfies the full SFTPService interface.
func TestSFTPClient_ImplementsSFTPService(t *testing.T) {
	var _ ports.SFTPService = (*SFTPClient)(nil)
}
