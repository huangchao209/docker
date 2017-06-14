// +build windows

package lcow

// Maintainer: @jhowardmsft
// Graph-driver for Linux Containers On Windows (LCOW)

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Microsoft/hcsshim"
	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/opengcs"
	"github.com/docker/docker/pkg/system"
)

// init registers the LCOW driver to the register.
func init() {
	graphdriver.Register("lcow", InitLCOW)
}

// cacheType is our internal structure representing an item in our local cache
// of things that have been mounted.
type cacheType struct {
	uvmPath   string // Path in utility VM
	hostPath  string // Path on host
	refCount  int    // How many times its been mounted
	isSandbox bool   // True if a sandbox
}

// Driver represents an LCOW graph driver.
type Driver struct {
	// homeDir is the hostpath where we're storing everything
	homeDir string
	// cachedSandboxFile is the location of the local default-size cached sandbox
	cachedSandboxFile string
	// options are the graphdriver options we are initialised with
	options []string
	// JJH LIFETIME TODO - Remove this and move up to daemon. For now, a global service utility-VM
	uvm hcsshim.Container

	// it is safe for windows to use a cache here because it does not support
	// restoring containers when the daemon dies.

	// cacheMu is the mutex protection add/update/deletes to our cache
	cacheMu sync.Mutex
	// cache is the cache of all the IDs we've mounted/unmounted.
	cache map[string]cacheType
}

// InitLCOW returns a new LCOW storage driver.
func InitLCOW(home string, options []string, uidMaps, gidMaps []idtools.IDMap) (graphdriver.Driver, error) {
	logrus.Debugf("lcowdriver: init: %s", home)

	d := &Driver{
		homeDir:           home,
		options:           options,
		cachedSandboxFile: filepath.Join(home, `cache\sandbox.vhdx`),
		cache:             make(map[string]cacheType),
	}

	if err := idtools.MkdirAllAs(home, 0700, 0, 0); err != nil {
		return nil, fmt.Errorf("lcowdriver: init: failed to create '%s': %v", home, err)
	}

	// Cache directory for blank sandbox so don't have to pull it from the service VM each time
	if err := idtools.MkdirAllAs(filepath.Dir(d.cachedSandboxFile), 0700, 0, 0); err != nil {
		return nil, fmt.Errorf("lcowdriver: init: failed to create '%s': %v", home, err)
	}

	return d, nil
}

// startUvm starts the service utility VM if it isn't running.
// TODO @jhowardmsft. This will change before RS3 ships as we move to a model of one
// service VM globally to a service VM per container (or offline operation). However,
// for the initial bring-up of LCOW, this is acceptable.
func (d *Driver) startUvm() error {
	if d.uvm != nil {
		return nil
	}

	var err error
	d.uvm, err = system.StartUVM(d.options)
	if err != nil {
		return fmt.Errorf("lcowdriver: startUvm: failed to start utility VM: %s", err)
	}
	return nil
}

// String returns the string representation of a driver. This should match
// the name the graph driver has been registered with.
func (d *Driver) String() string {
	return "lcow"
}

// Status returns the status of the driver.
func (d *Driver) Status() [][2]string {
	return [][2]string{
		{"LCOW", ""},
	}
}

// Exists returns true if the given id is registered with this driver.
func (d *Driver) Exists(id string) bool {
	_, err := os.Lstat(d.dir(id))
	logrus.Debugf("lcowdriver: exists: id %s %t", id, err == nil)
	return err == nil
}

// CreateReadWrite creates a layer that is writable for use as a container
// file system. That equates to creating a sandbox VHDx.
func (d *Driver) CreateReadWrite(id, parent string, opts *graphdriver.CreateOpts) error {
	logrus.Debugf("lcowdriver: createreadwrite: id %s", id)

	if d.uvm == nil {
		if err := d.startUvm(); err != nil {
			return fmt.Errorf("lcowdriver: createreadwrite: failed to start service utility vm: %s", err)
		}
	}

	if err := d.Create(id, parent, opts); err != nil {
		return err
	}
	return opengcs.CreateSandbox(d.uvm, filepath.Join(d.dir(id), "sandbox.vhdx"), opengcs.DefaultSandboxSizeMB, d.cachedSandboxFile)
}

// Create creates a new read-only layer with the given id.
func (d *Driver) Create(id, parent string, opts *graphdriver.CreateOpts) error {
	logrus.Debugf("lcowdriver: create: id %s parent: %s", id, parent)

	parentChain, err := d.getLayerChain(parent)
	if err != nil {
		return err
	}

	var layerChain []string
	if parent != "" {
		if !d.Exists(parent) {
			return fmt.Errorf("lcowdriver: cannot create read-only layer with missing parent %s", parent)
		}
		layerChain = []string{d.dir(parent)}
	}
	layerChain = append(layerChain, parentChain...)

	layerPath := d.dir(id)
	logrus.Debugf("lcowdriver: create: id %s: layerPath %s", id, layerPath)
	// Make sure the layers are created with the correct ACL so that VMs can access them.
	if err := system.MkdirAllWithACL(layerPath, 755, system.SddlNtvmAdministratorsLocalSystem); err != nil {
		return err
	}

	if err := d.setLayerChain(id, layerChain); err != nil {
		if err2 := os.RemoveAll(layerPath); err2 != nil {
			logrus.Warnf("Failed to remove layer %s: %s", layerPath, err2)
		}
		return err
	}
	logrus.Debugf("lcowdriver: createreadwrite: id %s: success", id)
	return nil
}

// Remove unmounts and removes the dir information.
func (d *Driver) Remove(id string) error {
	logrus.Debugf("lcowdriver: remove: id %s", id)
	tmpID := fmt.Sprintf("%s-removing", id)
	tmpLayerPath := d.dir(tmpID)
	layerPath := d.dir(id)

	logrus.Debugf("lcowdriver: remove: id %s: layerPath %s", id, layerPath)
	if err := os.Rename(layerPath, tmpLayerPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	if err := os.RemoveAll(tmpLayerPath); err != nil {
		return err
	}

	logrus.Debugf("lcowdriver: remove: id %s: layerPath %s succeeded", id, layerPath)
	return nil
}

// Get returns the rootfs path for the id. It is reference counted and
// effectively can be thought of as a "mount the layer into the utility
// vm if it isn't already"
func (d *Driver) Get(id, mountLabel string) (string, error) {
	dir, _, _, err := d.getEx(id)
	return dir, err
}

// getEx is Get, but also returns the cache-entry and the size of the VHD
func (d *Driver) getEx(id string) (string, cacheType, int64, error) {
	logrus.Debugf("lcowdriver: getEx: %s", id)

	if d.uvm == nil {
		if err := d.startUvm(); err != nil {
			return "", cacheType{}, 0, fmt.Errorf("lcowdriver: getex: failed to start service utility vm: %s", err)
		}
	}

	// Work out what file we are working on
	vhdHandle, vhdSize, isSandbox, err := opengcs.GetVHDFile(filepath.Join(d.homeDir, id))
	if err != nil {
		return "", cacheType{}, 0, fmt.Errorf("lcowdriver: getEx: failed to open layer or sandbox VHD to open in %s: %s", d.dir(id), err)
	}
	defer vhdHandle.Close()
	logrus.Debugf("lcowdriver: getEx: %s, size %d, isSandbox %t", vhdHandle.Name(), vhdSize, isSandbox)

	hotAddRequired := false
	d.cacheMu.Lock()
	var cacheEntry cacheType
	if _, ok := d.cache[id]; !ok {
		// The item is not currently in the cache.
		//
		// Sandboxes need hot-adding in the case that there is a single global utility VM
		// This will change for multiple instances with the lifetime changes.
		if isSandbox {
			hotAddRequired = true
		}
		d.cache[id] = cacheType{
			uvmPath:   fmt.Sprintf("/mnt/%s", id),
			refCount:  1,
			isSandbox: isSandbox,
			hostPath:  vhdHandle.Name(),
		}
	} else {
		// Increment the reference counter in the cache.
		cacheEntry = d.cache[id]
		cacheEntry.refCount++
		d.cache[id] = cacheEntry
	}

	cacheEntry = d.cache[id]
	logrus.Debugf("lcowdriver: getEx: %s: isSandbox %t, refCount %d", id, cacheEntry.isSandbox, cacheEntry.refCount)
	d.cacheMu.Unlock()

	if hotAddRequired {
		if err := opengcs.HotAddVhd(vhdHandle, d.uvm, cacheEntry.uvmPath); err != nil {
			return "", cacheType{}, 0, fmt.Errorf("lcowdriver: getEx: hot add %s failed: %s", vhdHandle.Name(), err)
		}
	}

	logrus.Debugf("lcowdriver: getEx: %s success. %s: %+v: size %d", id, d.dir(id), cacheEntry, vhdSize)
	return d.dir(id), cacheEntry, vhdSize, nil
}

// Put does the reverse of get. If there are no more references to
// the layer, it unmounts it from the utility VM.
func (d *Driver) Put(id string) error {
	logrus.Debugf("lcowdriver: put: %s", id)

	if d.uvm == nil {
		if err := d.startUvm(); err != nil {
			return fmt.Errorf("lcowdriver: put: failed to start service utility vm: %s", err)
		}
	}

	d.cacheMu.Lock()
	// Bad-news if unmounting something that isn't in the cache.
	entry, ok := d.cache[id]
	if !ok {
		d.cacheMu.Unlock()
		return fmt.Errorf("lcowdriver: put: possible ref-count error, or invalid id was passed to the graphdriver. Cannot handle id %s as it's not in the cache", id)
	}

	// Are we just decrementing the reference count
	if entry.refCount > 1 {
		entry.refCount--
		d.cache[id] = entry
		logrus.Debugf("lcowdriver: put: %s: refCount decremented to %d", id, entry.refCount)
		d.cacheMu.Unlock()
		return nil
	}

	// No more references, so tear it down
	if err := opengcs.HotRemoveVhd(entry.hostPath, d.uvm); err != nil {
		d.cacheMu.Unlock()
		return fmt.Errorf("lcowdriver: put: failed to hot-remove %s from service utility VM: %s", entry.hostPath, err)
	}

	// Remove from the cache map.
	delete(d.cache, id)
	d.cacheMu.Unlock()

	logrus.Debugf("lcowdriver: put: %s: refCount 0. un-mounted %s (%s) from utility VM", id, entry.hostPath, entry.uvmPath)
	return nil
}

// Cleanup ensures the information the driver stores is properly removed.
// We use this opportunity to cleanup any -removing folders which may be
// still left if the daemon was killed while it was removing a layer.
func (d *Driver) Cleanup() error {
	logrus.Debugf("lcowdriver: cleanup")

	d.cacheMu.Lock()
	for k, v := range d.cache {
		logrus.Debugf("lcowdriver: cleanup: cache entry: %s: %+v", k, v)
		if v.refCount > 0 {
			logrus.Warnf("lcowdriver: cleanup: leaked %s: %+v", k, v)
		}
	}
	d.cacheMu.Unlock()

	items, err := ioutil.ReadDir(d.homeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// Note we don't return an error below - it's possible the files
	// are locked. However, next time around after the daemon exits,
	// we likely will be able to to cleanup successfully. Instead we log
	// warnings if there are errors.
	for _, item := range items {
		if item.IsDir() && strings.HasSuffix(item.Name(), "-removing") {
			if err := os.RemoveAll(filepath.Join(d.homeDir, item.Name())); err != nil {
				logrus.Warnf("Failed to cleanup %s: %s", item.Name(), err)
			} else {
				logrus.Infof("Cleaned up %s", item.Name())
			}
		}
	}

	return nil
}

// Diff takes a layer (and it's parent layer which may be null, but
// is ignored by this implementation below) and returns a reader for
// a tarstream representing the layers contents. The id could be
// a read-only "layer.vhd" or a read-write "sandbox.vhdx". The semantics
// of this function dictate that the layer is already mounted.
func (d *Driver) Diff(id, parent string) (io.ReadCloser, error) {
	logrus.Debugf("lcowdriver: diff: id %s", id)

	if d.uvm == nil {
		if err := d.startUvm(); err != nil {
			return nil, fmt.Errorf("lcowdriver: diff: failed to start service utility vm: %s", err)
		}
	}

	d.cacheMu.Lock()
	if _, ok := d.cache[id]; !ok {
		d.cacheMu.Unlock()
		return nil, fmt.Errorf("lcowdriver: diff: cannot diff as id %s is not in the cache")
	}
	cacheEntry := d.cache[id]
	d.cacheMu.Unlock()

	// Stat to get size
	fileInfo, err := os.Stat(cacheEntry.hostPath)
	if err != nil {
		return nil, fmt.Errorf("lcowdriver: diff: failed to stat %s: %s", cacheEntry.hostPath, err)
	}

	// Then obtain the tar stream for it
	logrus.Debugf("lcowdriver: diff: %s, size %d, isSandbox %t", cacheEntry.hostPath, fileInfo.Size(), cacheEntry.isSandbox)
	tarReadCloser, err := opengcs.VhdToTar(d.uvm, cacheEntry.hostPath, cacheEntry.uvmPath, cacheEntry.isSandbox, fileInfo.Size())
	if err != nil {
		return nil, fmt.Errorf("lcowdriver: diff: failed to export layer to tar stream for id: %s, parent: %s : %s", id, parent, err)
	}
	logrus.Debugf("lcowdriver: diff: id %s parent %s completed successfully", id, parent)
	return tarReadCloser, nil
}

// Changes produces a list of changes between the specified layer
// and its parent layer. If parent is "", then all changes will be ADD changes.
// The layer should not be mounted when calling this function.
func (d *Driver) Changes(id, parent string) ([]archive.Change, error) {
	logrus.Debugf("lcowdriver: changes: id %s parent %s", id, parent)
	// TODO @gupta-ak. graphdriver.Get() on the parent and then
	// Have the service vm take the difference between the two files.
	return nil, nil
}

// ApplyDiff extracts the changeset from the given diff into the
// layer with the specified id and parent, returning the size of the
// new layer in bytes. The layer should not be mounted when calling
// this function. Another way of describing this is that ApplyDiff writes
// to a new layer (a VHD in LCOW) the contents of a tarstream it's given.
func (d *Driver) ApplyDiff(id, parent string, diff io.Reader) (int64, error) {
	logrus.Debugf("lcowdriver: applydiff: id %s", id)

	if d.uvm == nil {
		if err := d.startUvm(); err != nil {
			return 0, fmt.Errorf("lcowdriver: applydiff: failed to start service utility vm: %s", err)
		}
	}

	return opengcs.TarToVhd(d.uvm, filepath.Join(d.homeDir, id, "layer.vhd"), diff)
}

// DiffSize calculates the changes between the specified layer
// and its parent and returns the size in bytes of the changes
// relative to its base filesystem directory.
func (d *Driver) DiffSize(id, parent string) (size int64, err error) {
	logrus.Debugf("lcowdriver: diffsize: id %s", id)
	// TODO @gupta-ak. graphdriver.Get() on the parent and then
	// Have the service vm take the difference between the two files.
	return 0, nil
}

// GetMetadata returns custom driver information.
func (d *Driver) GetMetadata(id string) (map[string]string, error) {
	logrus.Debugf("lcowdriver: getmetadata: id %s", id)
	m := make(map[string]string)
	m["dir"] = d.dir(id)
	return m, nil
}

// dir returns the absolute path to the layer.
func (d *Driver) dir(id string) string {
	return filepath.Join(d.homeDir, filepath.Base(id))
}

// getLayerChain returns the layer chain information.
func (d *Driver) getLayerChain(id string) ([]string, error) {
	jPath := filepath.Join(d.dir(id), "layerchain.json")
	logrus.Debugf("lcowdriver: getlayerchain: id %s json %s", id, jPath)
	content, err := ioutil.ReadFile(jPath)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("lcowdriver: getlayerchain: %s unable to read layerchain file %s: %s", id, jPath, err)
	}

	var layerChain []string
	err = json.Unmarshal(content, &layerChain)
	if err != nil {
		return nil, fmt.Errorf("lcowdriver: getlayerchain: %s failed to unmarshall layerchain file %s: %s", id, jPath, err)
	}
	return layerChain, nil
}

// setLayerChain stores the layer chain information on disk.
func (d *Driver) setLayerChain(id string, chain []string) error {
	content, err := json.Marshal(&chain)
	if err != nil {
		return fmt.Errorf("lcowdriver: setlayerchain: %s failed to marshall layerchain json: %s", id, err)
	}

	jPath := filepath.Join(d.dir(id), "layerchain.json")
	logrus.Debugf("lcowdriver: setlayerchain: id %s json %s", id, jPath)
	err = ioutil.WriteFile(jPath, content, 0600)
	if err != nil {
		return fmt.Errorf("lcowdriver: setlayerchaing: %s failed to write layerchain file: %s", id, err)
	}
	return nil
}
