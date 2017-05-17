package fs

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/scopedpath"
	"github.com/docker/docker/pkg/system"
)

// @gupta-ak. TODO: Implement this
type remotefs struct {
	// Service VM Client?
	dummyPath string
}

func (rfs *remotefs) Remote() bool {
	return true
}

func (rfs *remotefs) HostPathName() string {
	return rfs.dummyPath
}

func (rfs *remotefs) ExtractArchive(input io.Reader, path string, options *archive.TarOptions) error {
	logrus.Debugf("LCOW: remotefs.ExtractArchive(). Not implemented yet. path=%s", path)
	tr := tar.NewReader(input)
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		fmt.Printf("%+v\n", *hdr)
		io.Copy(os.Stdout, tr)
	}
	return nil
}
func (rfs *remotefs) ArchivePath(path string, options *archive.TarOptions) (io.ReadCloser, error) {
	logrus.Debugf("LCOW: remotefs.ArchivePath(). Not implemented yet. path=%s", path)

	// Dummy archive from https://golang.org/pkg/archive/tar/#example_
	// Code copied with slight modifications
	reader, writer := io.Pipe()
	go func() {
		tw := tar.NewWriter(writer)
		var files = []struct {
			Name, Body string
		}{
			{"readme.txt", "This archive contains some text files."},
			{"gopher.txt", "Gopher names:GeorgeGeoffreyGonzo"},
			{"todo.txt", "Get animal handling license."},
		}
		for _, file := range files {
			hdr := &tar.Header{
				Name: file.Name,
				Mode: 0600,
				Size: int64(len(file.Body)),
			}
			if err := tw.WriteHeader(hdr); err != nil {
				break
			}
			if _, err := tw.Write([]byte(file.Body)); err != nil {
				break
			}
		}
		tw.Close()
	}()
	return reader, nil
}

func (rfs *remotefs) Readlink(name string) (string, error) {
	logrus.Debugf("LCOW: remotefs.Readlink(). Not implemented yet. path=%s", name)
	return rfs.dummyPath, nil
}

func (rfs *remotefs) Stat(name string) (os.FileInfo, error) {
	logrus.Debugf("LCOW: remotefs.Stat(). Not implemented yet. path=%s", name)

	body := "Hello World!\n"
	typeflag := tar.TypeReg

	// Change to directory settings
	if len(name) > 0 && (name[len(name)-1] == '\\' || name[len(name)-1] == '/') {
		body = ""
		typeflag = tar.TypeDir
	}

	hdr := &tar.Header{
		Name:     name,
		Mode:     0755,
		Size:     int64(len(body)),
		Typeflag: byte(typeflag),
	}
	return hdr.FileInfo(), nil
}

func (rfs *remotefs) Lstat(name string) (os.FileInfo, error) {
	logrus.Debugf("LCOW: remotefs.Lstat(). Not implemented yet. path=%s", name)
	body := "Hello World!\n"
	typeflag := tar.TypeReg

	// Change to directory settings
	if len(name) > 0 && (name[len(name)-1] == '\\' || name[len(name)-1] == '/') {
		body = ""
		typeflag = tar.TypeDir
	}

	hdr := &tar.Header{
		Name:     name,
		Mode:     0755,
		Size:     int64(len(body)),
		Typeflag: byte(typeflag),
	}
	return hdr.FileInfo(), nil
}

func (rfs *remotefs) ResolvePath(name string) (string, string, error) {
	logrus.Debugf("LCOW: remotefs.ResolvePath(). Not implemented yet. path=%s", name)
	resolvedPath, absPath, err := scopedpath.EvalScopedPathAbs(name, rfs.dummyPath)
	resolvedPath, err = toUnix(resolvedPath)
	if err != nil {
		return "", "", err
	}
	absPath, err = toUnix(absPath)
	if err != nil {
		return "", "", err
	}
	resolvedPath, absPath = addSlash(name, resolvedPath), addSlash(name, absPath)
	logrus.Debugf("LCOW: remotefs.ResolvePath(). Converted paths: path=%s -> %s %s", name, resolvedPath, absPath)
	return resolvedPath, absPath, nil
}

func (rfs *remotefs) GetResourcePath(name string) (string, error) {
	logrus.Debugf("LCOW: remotefs.GetResourcePath(). Not implemented yet. path=%s", name)
	path, err := scopedpath.EvalScopedPath(name, rfs.dummyPath)
	if err != nil {
		return "", err
	}
	path, err = toUnix(path)
	if err != nil {
		return "", err
	}
	path = addSlash(name, path)
	logrus.Debugf("LCOW: remotefs.GetResourcePath(). Converted paths: path=%s -> %s", name, path)
	return path, nil
}

func toUnix(path string) (string, error) {
	path, err := system.CheckSystemDriveAndRemoveDriveLetter(path)
	if err != nil {
		return "", err
	}

	return filepath.ToSlash(path), nil
}

func addSlash(oldpath, newpath string) string {
	if len(oldpath) > 0 && oldpath[len(oldpath)-1] == '/' && newpath[len(newpath)-1] != '/' {
		newpath += "/"
	}
	return newpath
}
