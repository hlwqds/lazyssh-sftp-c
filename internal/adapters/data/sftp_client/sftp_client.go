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
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"github.com/Adembc/lazyssh/internal/core/ports"
	"github.com/pkg/sftp"
	"go.uber.org/zap"
)

// SFTPClient implements ports.SFTPService using system SSH binary via pkg/sftp NewClientPipe.
// This approach reuses the user's SSH configuration (keys, agents, known_hosts)
// without introducing new security risks.
type SFTPClient struct {
	log     *zap.SugaredLogger
	client  *sftp.Client
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	mu      sync.Mutex
	homeDir string // remote home directory
}

// New creates a new SFTPClient adapter.
func New(log *zap.SugaredLogger) *SFTPClient {
	return &SFTPClient{log: log}
}

// Connect establishes an SFTP connection to the given server using system SSH binary.
// It builds SSH arguments from the Server entity, appends "-s sftp" to request the
// SFTP subsystem, and creates an SFTP client via NewClientPipe.
func (c *SFTPClient) Connect(server domain.Server) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	args := buildSSHArgs(server)
	// Append SFTP subsystem request
	args = append(args, "-s", "sftp")

	c.log.Infow("starting SFTP connection", "host", server.Host, "alias", server.Alias)

	cmd := exec.Command("ssh", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create stdout pipe: %w", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("create stdin pipe: %w", err)
	}
	// SSH errors go to terminal stderr for debugging
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start ssh process: %w", err)
	}

	// Create SFTP client over the SSH pipe
	client, err := sftp.NewClientPipe(stdout, stdin)
	if err != nil {
		// Clean up the SSH process if SFTP handshake fails
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fmt.Errorf("sftp handshake: %w", err)
	}

	c.client = client
	c.cmd = cmd
	c.stdin = stdin

	// Get remote home directory
	homeDir, err := client.RealPath("~")
	if err != nil {
		c.log.Warnw("failed to get remote home dir via RealPath, falling back to Getwd", "error", err)
		homeDir, err = client.Getwd()
		if err != nil {
			c.log.Warnw("failed to get remote working directory, using /", "error", err)
			homeDir = "/"
		}
	}
	c.homeDir = homeDir

	c.log.Infow("SFTP connection established", "host", server.Host, "homeDir", c.homeDir)
	return nil
}

// Close terminates the SFTP connection and cleans up the SSH process.
// It is safe to call Close on an unconnected client (no-op).
func (c *SFTPClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var errs []string

	if c.client != nil {
		if err := c.client.Close(); err != nil {
			errs = append(errs, fmt.Sprintf("close sftp client: %v", err))
		}
		c.client = nil
	}

	if c.stdin != nil {
		_ = c.stdin.Close()
		c.stdin = nil
	}

	if c.cmd != nil && c.cmd.Process != nil {
		if err := c.cmd.Process.Kill(); err != nil {
			c.log.Debugw("failed to kill ssh process", "error", err)
		}
		if err := c.cmd.Wait(); err != nil {
			c.log.Debugw("ssh process wait error", "error", err)
		}
		c.cmd = nil
	}

	c.homeDir = ""

	if len(errs) > 0 {
		return fmt.Errorf("errors during close: %s", strings.Join(errs, "; "))
	}
	return nil
}

// IsConnected returns whether the SFTP connection is active.
func (c *SFTPClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.client != nil
}

// HomeDir returns the remote home directory obtained during Connect.
func (c *SFTPClient) HomeDir() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.homeDir
}

// ListDir returns a sorted, optionally filtered list of files in the given remote path.
// Directories are always listed before files within each sort group.
func (c *SFTPClient) ListDir(path string, showHidden bool, sortField domain.FileSortField, sortAsc bool) ([]domain.FileInfo, error) {
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()

	if client == nil {
		return nil, fmt.Errorf("not connected: call Connect first")
	}

	entries, err := client.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("sftp readdir %s: %w", path, err)
	}

	result := make([]domain.FileInfo, 0, len(entries))
	for _, e := range entries {
		if !showHidden && strings.HasPrefix(e.Name(), ".") {
			continue
		}
		result = append(result, domain.FileInfo{
			Name:      e.Name(),
			Size:      e.Size(),
			Mode:      e.Mode(),
			ModTime:   e.ModTime(),
			IsDir:     e.IsDir(),
			IsSymlink: e.Mode()&fs.ModeSymlink != 0,
		})
	}

	sortSFTPEntries(result, sortField, sortAsc)
	return result, nil
}

// sortSFTPEntries sorts file entries with directories first, then by the specified field.
func sortSFTPEntries(entries []domain.FileInfo, sortField domain.FileSortField, sortAsc bool) {
	var dirs, files []domain.FileInfo
	for _, e := range entries {
		if e.IsDir {
			dirs = append(dirs, e)
		} else {
			files = append(files, e)
		}
	}

	sftpSortSlice(dirs, sortField, sortAsc)
	sftpSortSlice(files, sortField, sortAsc)

	n := 0
	for _, d := range dirs {
		entries[n] = d
		n++
	}
	for _, f := range files {
		entries[n] = f
		n++
	}
}

// sftpSortSlice sorts a slice of FileInfo by the specified field and direction.
func sftpSortSlice(entries []domain.FileInfo, sortField domain.FileSortField, sortAsc bool) {
	sort.SliceStable(entries, func(i, j int) bool {
		var less bool
		switch sortField {
		case domain.SortBySize:
			less = entries[i].Size < entries[j].Size
		case domain.SortByDate:
			less = entries[i].ModTime.Before(entries[j].ModTime)
		default: // SortByName
			less = strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
		}
		if sortAsc {
			return less
		}
		return !less
	})
}

// Compile-time interface satisfaction check.
var _ ports.SFTPService = (*SFTPClient)(nil)
