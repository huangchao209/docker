// +build windows

package opengcs

// TODO @jhowardmsft - This will move to Microsoft/opengcs soon

import (
	"fmt"
	"io"

	"github.com/Microsoft/hcsshim"
	"github.com/Sirupsen/logrus"
)

// TarToVhd streams a tarstream contained in an io.Reader to a fixed vhd file
func TarToVhd(uvm hcsshim.Container, targetVHDFile string, reader io.Reader) (int64, error) {
	logrus.Debugf("opengcs: TarToVhd: %s", targetVHDFile)

	if uvm == nil {
		return 0, fmt.Errorf("opengcs: TarToVhd: No utility VM was supplied")
	}

	process, err := createUtilsProcess(uvm, "tar2vhd")
	if err != nil {
		return 0, fmt.Errorf("opengcs: TarToVhd: %s: failed to create utils process tar2vhd: %s", targetVHDFile, err)
	}
	defer process.Process.Close()

	// Send the tarstream into the `tar2vhd`s stdin
	if _, err = copyWithTimeout(process.Stdin, reader, 0, uvmTimeout, fmt.Sprintf("send %s, to stdin of tar2vhd", targetVHDFile)); err != nil {
		return 0, fmt.Errorf("opengcs: TarToVhd: %s: failed to send to tar2vhd in uvm: %s", targetVHDFile, err)
	}

	// Don't need stdin now we've sent everything. This signals GCS that we are finished sending data.
	process.Process.CloseStdin()

	// Write stdout contents of `tar2vhd` to the VHD file
	payloadSize, err := writeFileFromReader(targetVHDFile, process.Stdout, fmt.Sprintf("output of tar2vhd to %s", targetVHDFile))
	if err != nil {
		return 0, fmt.Errorf("opengcs: TarToVhd: %s: failed writing VHD file: %s", targetVHDFile, err)
	}

	logrus.Debugf("opengcs: TarToVhd: %s created, %d bytes", targetVHDFile, payloadSize)
	return payloadSize, err
}
