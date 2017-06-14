// +build windows

package opengcs

// TODO @jhowardmsft - This will move to Microsoft/opengcs soon

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/rneugeba/virtsock/pkg/hvsock"
)

// TODO Make this configurable.
const (
	uvmTimeout = (2000 * time.Hour)
	socketID   = "E9447876-BA98-444F-8C14-6A2FFF773E87"

	// DefaultSandboxSizeMB is the size of the default sandbox size in MB
	DefaultSandboxSizeMB = 20 * 1024 * 1024
)

var (
	serviceVMId          hvsock.GUID
	serviceVMSocketID, _ = hvsock.GUIDFromString(socketID)
)

func init() {
	// TODO @jhowardmsft. Will require revisiting.  Get ID for hvsock. For now,
	// assume that it is always up. So, ignore the err for now.
	cmd := fmt.Sprintf("$(Get-ComputeProcess %s).Id", "LinuxServiceVM")
	result, _ := exec.Command("powershell", cmd).Output()
	serviceVMId, _ = hvsock.GUIDFromString(strings.TrimSpace(string(result)))
	logrus.Debugf("opengcs: init: utility VM hvsock serviceVMID %s", serviceVMId)
}
