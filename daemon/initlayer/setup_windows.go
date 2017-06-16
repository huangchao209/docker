// +build windows

package initlayer

import (
	"github.com/docker/docker/daemon/graphdriver"
)

// Setup populates a directory with mountpoints suitable
// for bind-mounting dockerinit into the container. The mountpoint is simply an
// empty file at /.dockerinit
//
// This extra layer is used by all containers as the top-most ro layer. It protects
// the container from unwanted side-effects on the rw layer.
func Setup(initLayer graphdriver.Mount, rootUID, rootGID int) error {
	return nil
}
