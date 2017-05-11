package fs

import (
	"errors"
	"io"

	"os"

	"github.com/docker/docker/pkg/archive"
)

// ErrExtractPointNotDirectory is used to convey that the operation to extract
// a tar archive to a directory in a container has failed because the specified
// path does not refer to a directory.
var ErrExtractPointNotDirectory = errors.New("extraction point is not a directory")

// ErrNotSymlink is used by Readlink to indicate that the given path was not
// a symlink
var ErrNotSymlink = errors.New("Not a symlink: %s")

type FilesystemOperator interface {
	// Remote returns true if the file system is remote. false otherwise.
	Remote() bool

	// Returns the path on the host if the file system is local. Meaningless if file system is remote.
	HostPathName() string

	// ExtractArchive takes in an archive and extracts it to the given path.
	// Only a limited set of options are supported; specficially none of path
	// options: IncludeFiles, ExcludePatterns, & RebaseNames are supported since
	// the caller doesn't know the path struct in the guest.
	ExtractArchive(input io.Reader, path string, options *archive.TarOptions) error

	// ArchivePath archives the given path (file or directory) and returns
	// the archive.
	// Only a limited set of options are supported; specficially none of path
	// options: IncludeFiles, ExcludePatterns, & RebaseNames are supported since
	// the caller doesn't know the path struct in the guest.
	ArchivePath(path string, options *archive.TarOptions) (io.ReadCloser, error)

	// Readlink interprets the path as a symlink and returns the file it's pointing to
	Readlink(name string) (string, error)

	// Stat & Lstat are equivalent to os.Stat & os.Lstat. The only difference
	// is that the FileInfo.Name() returns the absolute path instead of the base
	// file path.
	Stat(name string) (os.FileInfo, error)
	Lstat(name string) (os.FileInfo, error)

	// AbsPath returns the absolute path of the given string in scope of the
	// container.
	AbsPath(name string) string

	// WriteFile(filename string, data []byte, perm os.FileMode) error
	//Mkdir(name string, perm FileMode) error
	//MkdirAll(path string, perm os.FileMode) error
	//Remove(name string) error
	//RemoveAll(path string) error
	//Rename(oldpath, newpath string) error

	//Link(oldname, newname string) error
	//Symlink(oldname, newname string) error

	//Chmod(name string, mode os.FileMode) error
	//Chown(name string, uid, gid int) error
	//Chtimes(name string, atime time.Time, mtime time.Time) error

	//Lchmod(name string, mode os.FileMode) error
	//Lchown(name string, uid, gid int) error
	//Lchtimes(name string, atime time.Time, mtime time.Time) error

}

// NewFilesystemOperator returns a remote or local filesystem operator
func NewFilesystemOperator(isRemote bool, hostPath string) FilesystemOperator {
	if isRemote {
		return &remotefs{dummyPath: hostPath}
	}
	return &localfs{root: hostPath}
}
