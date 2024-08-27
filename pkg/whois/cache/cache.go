package cache

import (
	"sync"
	"time"

	"github.com/shurshun/domain-harvester/pkg/whois/types"
)

type WhoisCache struct {
	cache         sync.Map
	whoisProvider types.WhoisHarverster
}

func Init(whoisProvider types.WhoisHarverster) (types.WhoisHarverster, error) {
	wc := &WhoisCache{whoisProvider: whoisProvider}

	go wc.runInvalidator()

	return wc, nil
}

func shouldUpdate(wd *types.WhoisData) bool {
	return wd.ExpiryDays < 30 || time.Since(wd.LastUpdated).Minutes() > 60
}

func (wc *WhoisCache) GetExternalRequestsCnt() uint64 {
	return wc.whoisProvider.GetExternalRequestsCnt()
}

func (wc *WhoisCache) runInvalidator() {
	for {
		wc.cache.Range(func(k, v interface{}) bool {
			if shouldUpdate(v.(*types.WhoisData)) {
				wc.update(k.(string))
			}

			return true
		})

		time.Sleep(time.Minute * 1)
	}
}

func (wc *WhoisCache) update(domain string) (*types.WhoisData, error) {
	wd, err := wc.whoisProvider.GetDomainData(domain)

	wc.cache.Store(domain, wd)

	return wd, err
}

func (wc *WhoisCache) GetDomainData(domain string) (*types.WhoisData, error) {
	wd, ok := wc.cache.Load(domain)

	if ok {
		return wd.(*types.WhoisData), nil
	}

	return wc.update(domain)
}
