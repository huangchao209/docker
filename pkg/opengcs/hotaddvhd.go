// +build windows

package opengcs

// TODO @jhowardmsft - This will move to Microsoft/opengcs soon

import (
	"fmt"
	"os"

	"github.com/Microsoft/hcsshim"
	"github.com/Sirupsen/logrus"
)

// HotAddVhd hot-adds a VHD to a utility VM. This is used in the global one-utility-VM-
// service-VM per host scenario. In order to do a graphdriver `Diff`, we hot-add the
// sandbox to /mnt/<id> so that we can run `exportSandbox` inside the utility VM to
// get a tar-stream of the sandboxes contents back to the daemon.
func HotAddVhd(vhdHandle *os.File, uvm hcsshim.Container, containerPath string) error {
	logrus.Debugf("opengcs: HotAddVhd: %s", vhdHandle.Name())
	modification := &hcsshim.ResourceModificationRequestResponse{
		Resource: "MappedVirtualDisk",
		Data: hcsshim.MappedVirtualDisk{
			HostPath:          vhdHandle.Name(),
			ContainerPath:     containerPath,
			CreateInUtilityVM: true,
			//ReadOnly:          true,
		},
		Request: "Add",
	}
	logrus.Debugf("opengcs: HotAddVhd: %s to %s", vhdHandle.Name(), containerPath)
	if err := uvm.Modify(modification); err != nil {
		return fmt.Errorf("opengcs: HotAddVhd: failed: %s", err)
	}
	logrus.Debugf("opengcs: HotAddVhd: %s added successfully", vhdHandle.Name())
	return nil
}
