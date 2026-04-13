//go:build !windows

package transfer

import (
	"os"

	"go.uber.org/zap"
)

// setFilePermissions sets file permissions on Unix-like systems.
// Logs a warning if chmod fails (e.g., permission denied).
func setFilePermissions(path string, mode os.FileMode, log *zap.SugaredLogger) {
	if err := os.Chmod(path, mode); err != nil {
		log.Warnw("failed to set file permissions", "path", path, "mode", mode, "error", err)
	}
}
