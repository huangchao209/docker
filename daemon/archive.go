package daemon

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/fs"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/symlink"
)

// ContainerCopy performs a deprecated operation of archiving the resource at
// the specified path in the container identified by the given name.
func (daemon *Daemon) ContainerCopy(name string, res string) (io.ReadCloser, error) {
	container, err := daemon.GetContainer(name)
	if err != nil {
		return nil, err
	}

	if res[0] == '/' || res[0] == '\\' {
		res = res[1:]
	}

	// Make sure an online file-system operation is permitted.
	if err := daemon.isOnlineFSOperationPermitted(container); err != nil {
		return nil, err
	}

	return daemon.containerCopy(container, res)
}

// ContainerStatPath stats the filesystem resource at the specified path in the
// container identified by the given name.
func (daemon *Daemon) ContainerStatPath(name string, path string) (stat *types.ContainerPathStat, err error) {
	container, err := daemon.GetContainer(name)
	if err != nil {
		return nil, err
	}

	// Make sure an online file-system operation is permitted.
	if err := daemon.isOnlineFSOperationPermitted(container); err != nil {
		return nil, err
	}

	return daemon.containerStatPath(container, path)
}

// ContainerArchivePath creates an archive of the filesystem resource at the
// specified path in the container identified by the given name. Returns a
// tar archive of the resource and whether it was a directory or a single file.
func (daemon *Daemon) ContainerArchivePath(name string, path string) (content io.ReadCloser, stat *types.ContainerPathStat, err error) {
	container, err := daemon.GetContainer(name)
	if err != nil {
		return nil, nil, err
	}

	// Make sure an online file-system operation is permitted.
	if err := daemon.isOnlineFSOperationPermitted(container); err != nil {
		return nil, nil, err
	}

	return daemon.containerArchivePath(container, path)
}

// ContainerExtractToDir extracts the given archive to the specified location
// in the filesystem of the container identified by the given name. The given
// path must be of a directory in the container. If it is not, the error will
// be ErrExtractPointNotDirectory. If noOverwriteDirNonDir is true then it will
// be an error if unpacking the given content would cause an existing directory
// to be replaced with a non-directory and vice versa.
func (daemon *Daemon) ContainerExtractToDir(name, path string, copyUIDGID, noOverwriteDirNonDir bool, content io.Reader) error {
	container, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}

	// Make sure an online file-system operation is permitted.
	if err := daemon.isOnlineFSOperationPermitted(container); err != nil {
		return err
	}

	return daemon.containerExtractToDir(container, path, copyUIDGID, noOverwriteDirNonDir, content)
}

// containerStatPath stats the filesystem resource at the specified path in this
// container. Returns stat info about the resource.
func (daemon *Daemon) containerStatPath(container *container.Container, path string) (stat *types.ContainerPathStat, err error) {
	container.Lock()
	defer container.Unlock()

	if err = daemon.Mount(container); err != nil {
		return nil, err
	}
	defer daemon.Unmount(container)

	err = daemon.mountVolumes(container)
	defer container.DetachAndUnmount(daemon.LogVolumeEvent)
	if err != nil {
		return nil, err
	}

	return containerStatPathNoLock(container, path)
}

func containerStatPathNoLock(container *container.Container, path string) (*types.ContainerPathStat, error) {
	placeholderGuptaAk := fs.NewFilesystemOperator(false, container.BaseFS)
	info, err := placeholderGuptaAk.Lstat(path)
	if err != nil {
		return nil, err
	}

	var linkTarget string
	if info.Mode()&os.ModeSymlink != 0 {
		linkTarget, err = placeholderGuptaAk.Readlink(path)
		if err != nil {
			return nil, err
		}
	}

	return &types.ContainerPathStat{
		Name:       info.Name(),
		Size:       info.Size(),
		Mode:       info.Mode(),
		Mtime:      info.ModTime(),
		LinkTarget: linkTarget,
	}, nil
}

// containerArchivePath creates an archive of the filesystem resource at the specified
// path in this container. Returns a tar archive of the resource and stat info
// about the resource.
func (daemon *Daemon) containerArchivePath(container *container.Container, path string) (content io.ReadCloser, stat *types.ContainerPathStat, err error) {
	container.Lock()

	defer func() {
		if err != nil {
			// Wait to unlock the container until the archive is fully read
			// (see the ReadCloseWrapper func below) or if there is an error
			// before that occurs.
			container.Unlock()
		}
	}()

	if err = daemon.Mount(container); err != nil {
		return nil, nil, err
	}

	defer func() {
		if err != nil {
			// unmount any volumes
			container.DetachAndUnmount(daemon.LogVolumeEvent)
			// unmount the container's rootfs
			daemon.Unmount(container)
		}
	}()

	if err = daemon.mountVolumes(container); err != nil {
		return nil, nil, err
	}

	stat, err = containerStatPathNoLock(container, path)
	if err != nil {
		return nil, nil, err
	}

	opts := &archive.TarOptions{
		Compression:      archive.Uncompressed,
		IncludeSourceDir: true,
	}
	placeholderGuptaAk := fs.NewFilesystemOperator(false, container.BaseFS)
	data, err := placeholderGuptaAk.ArchivePath(path, opts)
	if err != nil {
		return nil, nil, err
	}

	content = ioutils.NewReadCloserWrapper(data, func() error {
		err := data.Close()
		container.DetachAndUnmount(daemon.LogVolumeEvent)
		daemon.Unmount(container)
		container.Unlock()
		return err
	})

	daemon.LogContainerEvent(container, "archive-path")

	return content, stat, nil
}

// containerExtractToDir extracts the given tar archive to the specified location in the
// filesystem of this container. The given path must be of a directory in the
// container. If it is not, the error will be ErrExtractPointNotDirectory. If
// noOverwriteDirNonDir is true then it will be an error if unpacking the
// given content would cause an existing directory to be replaced with a non-
// directory and vice versa.
func (daemon *Daemon) containerExtractToDir(container *container.Container, path string, copyUIDGID, noOverwriteDirNonDir bool, content io.Reader) (err error) {
	container.Lock()
	defer container.Unlock()

	if err = daemon.Mount(container); err != nil {
		return err
	}
	defer daemon.Unmount(container)

	err = daemon.mountVolumes(container)
	defer container.DetachAndUnmount(daemon.LogVolumeEvent)
	if err != nil {
		return err
	}

	options := daemon.defaultTarCopyOptions(noOverwriteDirNonDir)
	if copyUIDGID {
		var err error
		// tarCopyOptions will appropriately pull in the right uid/gid for the
		// user/group and will set the options.
		options, err = daemon.tarCopyOptions(container, noOverwriteDirNonDir)
		if err != nil {
			return err
		}
	}

	placeholderGuptaAk := fs.NewFilesystemOperator(false, container.BaseFS)

	// TODO @gupta-ak. Think of what to do with volume mounts
	// Right now, fail the if the rootfs is readonly + remote
	if placeholderGuptaAk.Remote() {
		if container.HostConfig.ReadonlyRootfs {
			return ErrRootFSReadOnly
		}
	} else {
		absPath := placeholderGuptaAk.AbsPath(path)

		toVolume, err := checkIfPathIsInAVolume(container, absPath)
		if err != nil {
			return err
		}

		if !toVolume && container.HostConfig.ReadonlyRootfs {
			return ErrRootFSReadOnly
		}
	}

	if err := placeholderGuptaAk.ExtractArchive(content, path, options); err != nil {
		return err
	}

	daemon.LogContainerEvent(container, "extract-to-dir")

	return nil
}

func (daemon *Daemon) containerCopy(container *container.Container, resource string) (rc io.ReadCloser, err error) {
	container.Lock()

	defer func() {
		if err != nil {
			// Wait to unlock the container until the archive is fully read
			// (see the ReadCloseWrapper func below) or if there is an error
			// before that occurs.
			container.Unlock()
		}
	}()

	if err := daemon.Mount(container); err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			// unmount any volumes
			container.DetachAndUnmount(daemon.LogVolumeEvent)
			// unmount the container's rootfs
			daemon.Unmount(container)
		}
	}()

	if err := daemon.mountVolumes(container); err != nil {
		return nil, err
	}

	placeholderGuptaAk := fs.NewFilesystemOperator(false, container.BaseFS)
	opts := &archive.TarOptions{
		Compression: archive.Uncompressed,
	}
	archive, err := placeholderGuptaAk.ArchivePath(resource, opts)
	if err != nil {
		return nil, err
	}

	reader := ioutils.NewReadCloserWrapper(archive, func() error {
		err := archive.Close()
		container.DetachAndUnmount(daemon.LogVolumeEvent)
		daemon.Unmount(container)
		container.Unlock()
		return err
	})
	daemon.LogContainerEvent(container, "copy")
	return reader, nil
}

// CopyOnBuild copies/extracts a source FileInfo to a destination path inside a container
// specified by a container object.
// TODO: make sure callers don't unnecessarily convert destPath with filepath.FromSlash (Copy does it already).
// CopyOnBuild should take in abstract paths (with slashes) and the implementation should convert it to OS-specific paths.
func (daemon *Daemon) CopyOnBuild(cID, destPath, srcRoot, srcPath string, decompress bool) error {
	fullSrcPath, err := symlink.FollowSymlinkInScope(filepath.Join(srcRoot, srcPath), srcRoot)
	if err != nil {
		return err
	}

	destExists := true
	destDir := false
	rootUID, rootGID := daemon.GetRemappedUIDGID()

	fmt.Printf("Docker build: %s, %s, %s %s\n", cID, destPath, srcRoot, srcPath)

	// Work in daemon-local OS specific file paths
	destPath = filepath.FromSlash(destPath)

	c, err := daemon.GetContainer(cID)
	if err != nil {
		return err
	}
	err = daemon.Mount(c)
	if err != nil {
		return err
	}
	defer daemon.Unmount(c)

	dest, err := c.GetResourcePath(destPath)
	if err != nil {
		return err
	}

	// Preserve the trailing slash
	// TODO: why are we appending another path separator if there was already one?
	if strings.HasSuffix(destPath, string(os.PathSeparator)) || destPath == "." {
		destDir = true
		dest += string(os.PathSeparator)
	}

	destPath = dest

	destStat, err := os.Stat(destPath)
	if err != nil {
		if !os.IsNotExist(err) {
			//logrus.Errorf("Error performing os.Stat on %s. %s", destPath, err)
			return err
		}
		destExists = false
	}

	uidMaps, gidMaps := daemon.GetUIDGIDMaps()
	archiver := &archive.Archiver{
		Untar:   chrootarchive.Untar,
		UIDMaps: uidMaps,
		GIDMaps: gidMaps,
	}

	src, err := os.Stat(fullSrcPath)
	if err != nil {
		return err
	}

	if src.IsDir() {
		// copy as directory
		if err := archiver.CopyWithTar(fullSrcPath, destPath); err != nil {
			return err
		}
		return fixPermissions(fullSrcPath, destPath, rootUID, rootGID, destExists)
	}
	if decompress && archive.IsArchivePath(fullSrcPath) {
		// Only try to untar if it is a file and that we've been told to decompress (when ADD-ing a remote file)

		// First try to unpack the source as an archive
		// to support the untar feature we need to clean up the path a little bit
		// because tar is very forgiving.  First we need to strip off the archive's
		// filename from the path but this is only added if it does not end in slash
		tarDest := destPath
		if strings.HasSuffix(tarDest, string(os.PathSeparator)) {
			tarDest = filepath.Dir(destPath)
		}

		// try to successfully untar the orig
		err := archiver.UntarPath(fullSrcPath, tarDest)
		/*
			if err != nil {
				logrus.Errorf("Couldn't untar to %s: %v", tarDest, err)
			}
		*/
		return err
	}

	// only needed for fixPermissions, but might as well put it before CopyFileWithTar
	if destDir || (destExists && destStat.IsDir()) {
		destPath = filepath.Join(destPath, filepath.Base(srcPath))
	}

	if err := idtools.MkdirAllNewAs(filepath.Dir(destPath), 0755, rootUID, rootGID); err != nil {
		return err
	}
	if err := archiver.CopyFileWithTar(fullSrcPath, destPath); err != nil {
		return err
	}

	return fixPermissions(fullSrcPath, destPath, rootUID, rootGID, destExists)
}
