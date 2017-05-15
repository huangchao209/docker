package fs

import (
	"archive/tar"
	"io"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/scopedpath"
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
	return ""
}

func (rfs *remotefs) ExtractArchive(input io.Reader, path string, options *archive.TarOptions) error {
	logrus.Debugln("LCOW: remotefs.ExtractArchive(). Not implemented yet.")
	return nil
}
func (rfs *remotefs) ArchivePath(path string, options *archive.TarOptions) (io.ReadCloser, error) {
	logrus.Debugln("LCOW: remotefs.ArchivePath(). Not implemented yet.")

	// Dummy archive from https://golang.org/pkg/archive/tar/#example_
	// Code copied with slight modifications
	reader, writer := io.Pipe()
	go func() {
		tw := tar.NewWriter(writer)
		var files = []struct {
			Name, Body string
		}{
			{"readme.txt", "This archive contains some text files."},
			{"gopher.txt", "Gopher names:\nGeorge\nGeoffrey\nGonzo"},
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
	logrus.Debugln("LCOW: remotefs.Readlink(). Not implemented yet.")
	return rfs.dummyPath, nil
}

func (rfs *remotefs) Stat(name string) (os.FileInfo, error) {
	logrus.Debugln("LCOW: remotefs.Stat(). Not implemented yet.")
	return os.Stat(rfs.dummyPath)
}

func (rfs *remotefs) Lstat(name string) (os.FileInfo, error) {
	logrus.Debugln("LCOW: remotefs.Lstat(). Not implemented yet.")
	return os.Lstat(rfs.dummyPath)
}

func (rfs *remotefs) ResolvePath(name string) (string, string, error) {
	return scopedpath.EvalScopedPathAbs(name, rfs.dummyPath)
}
