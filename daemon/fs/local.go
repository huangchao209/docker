package fs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/pathutils"
	"github.com/docker/docker/pkg/system"
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
	// Check if a drive letter supplied, it must be the system drive. No-op except on Windows
	path, err := system.CheckSystemDriveAndRemoveDriveLetter(path)
	if err != nil {
		return err
	}

	// The destination path needs to be resolved to a host path, with all
	// symbolic links followed in the scope of the container's rootfs. Note
	// that we do not use `container.ResolvePath(path)` here because we need
	// to also evaluate the last path element if it is a symlink. This is so
	// that you can extract an archive to a symlink that points to a directory.

	// Consider the given path as an absolute path in the container.
	absPath := archive.PreserveTrailingDotOrSeparator(filepath.Join(string(filepath.Separator), path), path)

	// This will evaluate the last path element if it is a symlink.
	resolvedPath, err := pathutils.EvalScopedPath(absPath, lfs.root)
	if err != nil {
		return err
	}

	stat, err := os.Lstat(resolvedPath)
	if err != nil {
		return err
	}

	if !stat.IsDir() {
		return ErrExtractPointNotDirectory
	}

	// Need to check if the path is in a volume. If it is, it cannot be in a
	// read-only volume. If it is not in a volume, the container cannot be
	// configured with a read-only rootfs.

	// Use the resolved path relative to the container rootfs as the new
	// absPath. This way we fully follow any symlinks in a volume that may
	// lead back outside the volume.
	//
	// The Windows implementation of filepath.Rel in golang 1.4 does not
	// support volume style file path semantics. On Windows when using the
	// filter driver, we are guaranteed that the path will always be
	// a volume file path.
	var baseRel string
	if strings.HasPrefix(resolvedPath, `\\?\Volume{`) {
		if strings.HasPrefix(resolvedPath, lfs.root) {
			baseRel = resolvedPath[len(lfs.root):]
			if baseRel[:1] == `\` {
				baseRel = baseRel[1:]
			}
		}
	} else {
		baseRel, err = filepath.Rel(lfs.root, resolvedPath)
	}
	if err != nil {
		return err
	}
	// Make it an absolute path.
	absPath = filepath.Join(string(filepath.Separator), baseRel)
	fmt.Println("XXX", resolvedPath)
	if err := chrootarchive.Untar(input, resolvedPath, options); err != nil {
		return err
	}
	return nil
}

func (lfs *localfs) ArchivePath(path string, options *archive.TarOptions) (io.ReadCloser, error) {
	resolvedPath, absPath, err := pathutils.EvalScopedPathAbs(path, lfs.root)
	if err != nil {
		return nil, err
	}

	lstat, err := os.Lstat(resolvedPath)
	if err != nil {
		return nil, err
	}

	// We need to rebase the archive entries if the last element of the
	// resolved path was a symlink that was evaluated and is now different
	// than the requested path. For example, if the given path was "/foo/bar/",
	// but it resolved to "/var/lib/docker/containers/{id}/foo/baz/", we want
	// to ensure that the archive entries start with "bar" and not "baz". This
	// also catches the case when the root directory of the container is
	// requested: we want the archive entries to start with "/" and not the
	// container ID.
	opts := archive.TarOptions{}
	if options != nil {
		opts = *options
	}
	if lstat.IsDir() && opts.IncludeSourceDir {
		// If we want to include the source dir, all the files will be prefixed with a ".",
		// so we want to rebase the "." to the requested base path.
		opts.IncludeFiles = []string{"."}
		opts.RebaseNames = map[string]string{
			".": filepath.Base(absPath),
		}
	} else if lstat.IsDir() {
		// If we don't include the source dir, then no rebasing is needed.
		opts.IncludeFiles = []string{"."}
	} else {
		// In a regular file case, TarWithOptions will already split the path and add
		// the filename to the Include list. We just need to rebase to the name in the container.
		opts.RebaseNames = map[string]string{
			filepath.Base(resolvedPath): filepath.Base(absPath),
		}
	}
	return archive.TarWithOptions(resolvedPath, &opts)
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
	hostPath, err := pathutils.EvalScopedPath(absPath, lfs.root)
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
	resolvedPath, _, err := pathutils.EvalScopedPathAbs(name, lfs.root)
	if err != nil {
		return nil, err
	}
	return statFunc(resolvedPath)
}

func (lfs *localfs) AbsPath(name string) string {
	return archive.PreserveTrailingDotOrSeparator(filepath.Join(string(filepath.Separator), name), name)
}
