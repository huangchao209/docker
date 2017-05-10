package container

import (
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
)

// StatPath is the unexported version of StatPath. Locks and mounts should
// be acquired before calling this method and the given path should be fully
// resolved to a path on the host corresponding to the given absolute path
// inside the container.
func (container *Container) StatPath(resolvedPath, absPath string) (stat *types.ContainerPathStat, err error) {
	lstat, err := os.Lstat(resolvedPath)
	if err != nil {
		return nil, err
	}

	var linkTarget string
	if lstat.Mode()&os.ModeSymlink != 0 {
		// Fully evaluate the symlink in the scope of the container rootfs.
		hostPath, err := container.GetResourcePath(absPath)
		if err != nil {
			return nil, err
		}

		linkTarget, err = filepath.Rel(container.BaseFS, hostPath)
		if err != nil {
			return nil, err
		}

		// Make it an absolute path.
		linkTarget = filepath.Join(string(filepath.Separator), linkTarget)
	}

	return &types.ContainerPathStat{
		Name:       filepath.Base(absPath),
		Size:       lstat.Size(),
		Mode:       lstat.Mode(),
		Mtime:      lstat.ModTime(),
		LinkTarget: linkTarget,
	}, nil
}
