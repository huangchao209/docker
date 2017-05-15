// +build linux freebsd solaris

package scopedpath

import (
	"os"
	"path/filepath"
)

// cleanResourcePath cleans a resource path and prepares to combine with mnt path
func cleanResourcePath(path string) string {
	return filepath.Join(string(os.PathSeparator), path)
}
