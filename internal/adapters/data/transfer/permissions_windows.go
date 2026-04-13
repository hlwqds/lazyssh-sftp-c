//go:build windows

package transfer

import (
	"os"

	"go.uber.org/zap"
)

// setFilePermissions is a no-op on Windows.
// Windows NTFS does not support Unix-style permission bits.
func setFilePermissions(path string, mode os.FileMode, log *zap.SugaredLogger) {
	log.Debugw("skipping chmod on Windows (NTFS)", "path", path)
}
