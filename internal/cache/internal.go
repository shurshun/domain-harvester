package cache

import (
	"domain-harvester/internal/harvester/types"
	"sync"
)

type internalCache struct {
	mx    sync.RWMutex
	cache []*types.Domain
}

func (c *internalCache) Get() []*types.Domain {
	c.mx.RLock()
	defer c.mx.RUnlock()

	return c.cache
}

func (c *internalCache) Update(domains []*types.Domain) {
	c.mx.Lock()
	defer c.mx.Unlock()

	c.cache = domains
}
