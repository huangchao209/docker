package scopedpath

import (
	"path/filepath"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/symlink"
	"github.com/docker/docker/pkg/system"
)

// EvalScopedPath evaluates `path` in the scope of `root`, with proper path
// sanitisation. Symlinks are all scoped to `root`, as though `root` was `/`.
// For example, if path = "/b/c" and root = "/a", EvalScopedPath will return
// "/a/b/c"
func EvalScopedPath(path, root string) (string, error) {
	cleanPath := cleanResourcePath(path)
	return symlink.FollowSymlinkInScope(filepath.Join(root, cleanPath), root)
}

// EvalScopedPathAbs evaluates `path` in scope of `root`. Returns the evaluated
// absolute path of `path` (resolvedPath), the absolute path of `path` relative
// to `root` (absPath), and an error.
func EvalScopedPathAbs(path, root string) (resolvedPath, absPath string, err error) {
	// Check if a drive letter supplied, it must be the system drive. No-op except on Windows
	path, err = system.CheckSystemDriveAndRemoveDriveLetter(path)
	if err != nil {
		return "", "", err
	}

	// Consider the given path as an absolute path in the respect to root
	absPath = archive.PreserveTrailingDotOrSeparator(filepath.Join(string(filepath.Separator), path), path)

	// Split the absPath into its Directory and Base components. We will
	// resolve the dir in the scope of `root` then append the base.
	dirPath, basePath := filepath.Split(absPath)

	resolvedDirPath, err := EvalScopedPath(dirPath, root)
	if err != nil {
		return "", "", err
	}

	// resolvedDirPath will have been cleaned (no trailing path separators) so
	// we can manually join it with the base path element.
	resolvedPath = resolvedDirPath + string(filepath.Separator) + basePath

	return resolvedPath, absPath, nil
}
