package layer

import (
	"errors"

	"github.com/docker/docker/daemon/graphdriver"
)

// GetLayerPath returns the path to a layer
func GetLayerPath(s Store, layer ChainID) (graphdriver.Mount, error) {
	ls, ok := s.(*layerStore)
	if !ok {
		return graphdriver.DummyMount{""}, errors.New("unsupported layer store")
	}
	ls.layerL.Lock()
	defer ls.layerL.Unlock()

	rl, ok := ls.layerMap[layer]
	if !ok {
		return graphdriver.DummyMount{""}, ErrLayerDoesNotExist
	}

	path, err := ls.driver.Get(rl.cacheID, "")
	if err != nil {
		return graphdriver.DummyMount{""}, err
	}

	if err := ls.driver.Put(rl.cacheID); err != nil {
		return graphdriver.DummyMount{""}, err
	}

	return path, nil
}

func (ls *layerStore) mountID(name string) string {
	// windows has issues if container ID doesn't match mount ID
	return name
}
