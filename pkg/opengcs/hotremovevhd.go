// +build windows

package opengcs

// TODO @jhowardmsft - This will move to Microsoft/opengcs soon

import (
	"fmt"

	"github.com/Microsoft/hcsshim"
	"github.com/Sirupsen/logrus"
)

// HotRemoveVhd hot-removes a VHD from a utility VM. This is used in the global one-utility-VM-
// service-VM per host scenario.
func HotRemoveVhd(hostPath string, uvm hcsshim.Container) error {
	logrus.Debugf("opengcs: HotRemoveVhd: %s", hostPath)
	modification := &hcsshim.ResourceModificationRequestResponse{
		Resource: "MappedVirtualDisk",
		Data: hcsshim.MappedVirtualDisk{
			HostPath: hostPath,
		},
		Request: "Remove",
	}
	if err := uvm.Modify(modification); err != nil {
		return fmt.Errorf("opengcs: HotRemoveVhd: %s failed: %s", hostPath, err)
	}
	logrus.Debugf("opengcs: HotRemoveVhd: %s removed successfully", hostPath)
	return nil
}
