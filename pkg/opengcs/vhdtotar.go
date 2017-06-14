// +build windows

package opengcs

// TODO @jhowardmsft - This will move to Microsoft/opengcs soon

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Microsoft/hcsshim"
	"github.com/Sirupsen/logrus"
)

// VhdToTar does what is says - it exports a VHD in a specified
// folder (either a read-only layer.vhd, or a read-write sandbox.vhd) to a
// ReadCloser containing a tar-stream of the layers contents.
func VhdToTar(uvm hcsshim.Container, vhdFile string, uvmMountPath string, isSandbox bool, vhdSize int64) (io.ReadCloser, error) {
	logrus.Debugf("opengcs: VhdToTar: %s isSandbox: %t", vhdFile, isSandbox)

	vhdHandle, err := os.Open(vhdFile)
	if err != nil {
		return nil, fmt.Errorf("opengcs: VhdToTar: failed to open %s: %s", vhdFile, err)
	}
	defer vhdHandle.Close()
	logrus.Debugf("opengcs: VhdToTar: exporting %s, size %d, isSandbox %t", vhdHandle.Name(), vhdSize, isSandbox)

	// Different binary depending on whether a RO layer or a RW sandbox
	command := "vhd2tar"
	if isSandbox {
		command = fmt.Sprintf("exportSandbox -path %s", uvmMountPath)
	}

	// Start the binary in the utility VM
	process, err := createUtilsProcess(uvm, command)
	if err != nil {
		return nil, fmt.Errorf("opengcs: VhdToTar: %s: failed to create utils process %s: %s", vhdHandle.Name(), command, err)
	}

	if !isSandbox {
		// Send the VHD contents to the utility VM processes stdin handle if not a sandbox
		logrus.Debugf("opengcs: VhdToTar: copying the layer VHD into the utility VM")
		if _, err = copyWithTimeout(process.Stdin, vhdHandle, vhdSize, uvmTimeout, fmt.Sprintf("vhdtotarstream: sending %s to %s", vhdHandle.Name(), command)); err != nil {
			process.Process.Close()
			return nil, fmt.Errorf("opengcs: VhdToTar: %s: failed to copyWithTimeout on the stdin pipe (to utility VM): %s", vhdHandle.Name(), err)
		}
	}

	// Start a goroutine which copies the stdout (ie the tar stream)
	reader, writer := io.Pipe()
	go func() {
		defer writer.Close()
		defer process.Process.Close()
		logrus.Debugf("opengcs: VhdToTar: copying tar stream back from the utility VM")
		bytes, err := copyWithTimeout(writer, process.Stdout, vhdSize, uvmTimeout, fmt.Sprintf("vhdtotarstream: copy tarstream from %s", command))
		if err != nil {
			logrus.Errorf("opengcs: VhdToTar: %s:  copyWithTimeout on the stdout pipe (from utility VM) failed: %s", vhdHandle.Name(), err)
		}
		logrus.Debugf("opengcs: VhdToTar: copied %d bytes of the tarstream of %s from the utility VM", bytes, vhdHandle.Name())
	}()

	// Return the read-side of the pipe connected to the goroutine which is reading from the stdout of the process in the utility VM
	return reader, nil

}

// BUGBUG @jhowardmsft - DOn't do the open here. Need a seperate call. Do stat instead

// GetVHDFile is a utility for getting a file handle and the size to
// a VHD(x) in a folder. A read-only layer will be layer.vhd. A read-write
// layer will be sandbox.vhdx.
func GetVHDFile(folder string) (*os.File, int64, bool, error) {
	vhdHandle, err := os.Open(filepath.Join(folder, "layer.vhd"))
	defer vhdHandle.Close()
	if err == nil {
		fileInfo, err := vhdHandle.Stat()
		if err != nil {
			return nil, 0, false, err
		}
		return vhdHandle, fileInfo.Size(), false, nil
	} else if err != nil && !os.IsNotExist(err) {
		return nil, 0, false, err
	}

	// Try the sandbox path
	vhdHandle, err = os.Open(filepath.Join(folder, "sandbox.vhdx"))
	if err != nil {
		return nil, 0, false, err
	}
	fileInfo, err := vhdHandle.Stat()
	if err != nil {
		return nil, 0, false, err
	}
	return vhdHandle, fileInfo.Size(), true, nil
}
