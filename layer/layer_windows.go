package layer

import (
	"errors"

	"github.com/docker/docker/daemon/graphdriver"
)

// GetLayerPath returns the path to a layer
func GetLayerPath(s Store, layer ChainID) (string, error) {
	ls, ok := s.(*layerStore)
	if !ok {
		return "", errors.New("unsupported layer store")
	}
	ls.layerL.Lock()
	defer ls.layerL.Unlock()

	rl, ok := ls.layerMap[layer]
	if !ok {
		return "", ErrLayerDoesNotExist
	}

	layerGetter, ok := ls.driver.(graphdriver.LayerGetter)
	if ok {
		return layerGetter.GetLayerPath(rl.cacheID)
	}

	// TODO: @gupta-ak. Right now, assume that fs is local if driver.GetLayerPath is not implemented.
	path, err := ls.driver.Get(rl.cacheID, "")
	if err != nil {
		return "", err
	}

	if err := ls.driver.Put(rl.cacheID); err != nil {
		return "", err
	}

	return path.HostPathName(), nil
}

func (ls *layerStore) mountID(name string) string {
	// windows has issues if container ID doesn't match mount ID
	return name
}
