// +build windows

package opengcs

// TODO @jhowardmsft - This will move to Microsoft/opengcs soon

import (
	"fmt"
	"os"
	"sync"

	"github.com/Microsoft/hcsshim"
	"github.com/Sirupsen/logrus"
)

var sandboxCacheLock sync.Mutex

// CreateSandbox does what it says on the tin. This is done by copying a prebuilt-sandbox from the ServiceVM
// TODO: @jhowardmsft maxSizeInMB isn't hooked up in GCS. Needs a platform change which is in flight.
func CreateSandbox(uvm hcsshim.Container, destFile string, maxSizeInMB uint32, cacheFile string) error {
	// Smallest we can accept is the default sandbox size as we can't size down, only expand.
	if maxSizeInMB < DefaultSandboxSizeMB {
		maxSizeInMB = DefaultSandboxSizeMB
	}

	logrus.Debugf("opengcs: CreateSandbox: %s size:%dMB cache:%s", destFile, maxSizeInMB, cacheFile)

	// Retrieve from cache if the default size and already on disk
	if maxSizeInMB == DefaultSandboxSizeMB {
		sandboxCacheLock.Lock()
		if _, err := os.Stat(cacheFile); err == nil {
			if err := copyFile(cacheFile, destFile); err != nil {
				sandboxCacheLock.Unlock()
				return fmt.Errorf("opengcs: CreateSandbox: Failed to copy cached sandbox '%s' to '%s': %s", cacheFile, destFile, err)
			}
			sandboxCacheLock.Unlock()
			logrus.Debugf("opengcs: CreateSandbox: %s fulfilled from cache", destFile)
			return nil
		}
		sandboxCacheLock.Unlock()
	}

	if uvm == nil {
		return fmt.Errorf("opengcs: CreateSandbox: No utility VM was supplied")
	}

	process, err := createUtilsProcess(uvm, "createSandbox") //fmt.Sprintf("createSandbox -size %d", maxSizeInMB))
	if err != nil {
		return fmt.Errorf("opengcs: CreateSandbox: %s: failed to create utils process: %s", destFile, err)
	}

	defer func() {
		process.Process.Close()
	}()

	logrus.Debugf("opengcs: CreateSandbox: %s: writing from stduout", destFile)
	// Get back the sandbox VHDx stream from the service VM and write it to file
	resultSize, err := writeFileFromReader(destFile, process.Stdout, fmt.Sprintf("createSandbox %s", destFile))
	if err != nil {
		return fmt.Errorf("opengcs: CreateSandbox: %s: failed writing %d bytes to target file: %s", destFile, resultSize, err)
	}

	// Populate the cache
	if maxSizeInMB == DefaultSandboxSizeMB {
		sandboxCacheLock.Lock()
		if err := copyFile(destFile, cacheFile); err != nil {
			sandboxCacheLock.Unlock()
			return fmt.Errorf("opengcs: CreateSandbox: Failed to seed sandbox cache '%s' from '%s': %s", destFile, cacheFile, err)
		}
		sandboxCacheLock.Unlock()
	}

	logrus.Debugf("opengcs: CreateSandbox: %s created (non-cache)", destFile)
	return nil
}
