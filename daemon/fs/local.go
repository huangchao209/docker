package fs

import (
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/scopedpath"
)

type localfs struct {
	root string
}

func (lfs *localfs) Remote() bool {
	return false
}

func (lfs *localfs) HostPathName() string {
	return lfs.root
}

func (lfs *localfs) ExtractArchive(input io.Reader, path string, options *archive.TarOptions) error {
	// This will evaluate the last path element if it is a symlink.
	resolvedPath, err := scopedpath.EvalScopedPath(path, lfs.root)
	if err != nil {
		return err
	}
	return chrootarchive.Untar(input, resolvedPath, options)
}

func (lfs *localfs) ArchivePath(path string, options *archive.TarOptions) (io.ReadCloser, error) {
	resolvedPath, err := scopedpath.EvalScopedPath(path, lfs.root)
	if err != nil {
		return nil, err
	}
	return archive.TarWithOptions(resolvedPath, options)
}

func (lfs *localfs) Readlink(name string) (string, error) {
	stat, err := lfs.Lstat(name)
	if err != nil {
		return "", err
	}

	if stat.Mode()&os.ModeSymlink != 0 {
		return lfs.readlink(stat.Name())
	}
	return "", ErrNotSymlink
}

// Unexported function that resolves an absolute path to a symlink.
// absPath must be an symlink.
func (lfs *localfs) readlink(absPath string) (string, error) {
	// Fully evaluate the symlink in the scope of the container rootfs.
	hostPath, err := scopedpath.EvalScopedPath(absPath, lfs.root)
	if err != nil {
		return "", err
	}

	linkTarget, err := filepath.Rel(lfs.root, hostPath)
	if err != nil {
		return "", err
	}

	// Make it an absolute path.
	linkTarget = filepath.Join(string(filepath.Separator), linkTarget)
	return linkTarget, nil
}

func (lfs *localfs) Stat(name string) (os.FileInfo, error) {
	return lfs.statGeneric(name, os.Stat)
}

func (lfs *localfs) Lstat(name string) (os.FileInfo, error) {
	return lfs.statGeneric(name, os.Lstat)
}

func (lfs *localfs) statGeneric(name string, statFunc func(string) (os.FileInfo, error)) (os.FileInfo, error) {
	resolvedPath, _, err := scopedpath.EvalScopedPathAbs(name, lfs.root)
	if err != nil {
		return nil, err
	}
	return statFunc(resolvedPath)
}

func (lfs *localfs) ResolvePath(name string) (string, string, error) {
	return scopedpath.EvalScopedPathAbs(name, lfs.root)
}
