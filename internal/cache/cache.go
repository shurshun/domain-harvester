package cache

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/bep/debounce"
	"github.com/shurshun/domain-harvester/internal/harvester/types"
	whois_types "github.com/shurshun/domain-harvester/pkg/whois/types"
	log "github.com/sirupsen/logrus"
)

type DomainCache struct {
	rawCache        sync.Map
	intCache        *internalCache
	whoisProvider   whois_types.WhoisHarverster
	debounceCounter uint64
	debounced       func(f func())
}

func Init(whoisProvider whois_types.WhoisHarverster) (types.DomainCache, error) {
	dc := &DomainCache{
		intCache:      &internalCache{},
		whoisProvider: whoisProvider,
		debounced:     debounce.New(1 * time.Second),
	}

	go dc.runCacheInvalidator()

	return dc, nil
}

func (dc *DomainCache) runCacheInvalidator() {
	for {
		dc.rebuildDomainCache()

		time.Sleep(time.Minute * 1)
	}
}

func (dc *DomainCache) GetDomains() []*types.Domain {
	return dc.intCache.Get()
}

func (dc *DomainCache) GetExternalRequestsCnt() uint64 {
	return dc.whoisProvider.GetExternalRequestsCnt()
}

func (dc *DomainCache) Update(source string, domains []*types.Domain) {
	dc.rawCache.Store(source, domains)

	f := func() {
		atomic.AddUint64(&dc.debounceCounter, 1)
		dc.rebuildDomainCache()
	}

	dc.debounced(f)
}

func (dc *DomainCache) getUniqDomains() (result []*types.Domain) {
	domains := make(map[string]bool)

	dc.rawCache.Range(func(k, v interface{}) bool {
		for _, domain := range v.([]*types.Domain) {
			if _, ok := domains[domain.Name]; !ok {
				domains[domain.Name] = true
				result = append(result, domain)
			}
		}

		return true
	})

	return result
}

func (dc *DomainCache) rebuildDomainCache() {
	uniqDomains := dc.getUniqDomains()

	if len(uniqDomains) == 0 {
		return
	}

	log.Debugf("Start rebuilding cache for %d domains...", len(uniqDomains))

	var newCache []*types.Domain

	var wg sync.WaitGroup

	startTime := time.Now()

	queue := make(chan *types.Domain, len(uniqDomains))

	for _, domain := range uniqDomains {
		wg.Add(1)
		go func(wg *sync.WaitGroup, whoisProvider whois_types.WhoisHarverster, domain *types.Domain, queue chan *types.Domain) {
			defer wg.Done()

			wd, err := whoisProvider.GetDomainData(domain.Name)
			if err != nil {
				log.Debugf("Error on update %s domain: %s", wd.Domain, wd.Error)
			}

			domain.WhoisData = wd

			queue <- domain
		}(&wg, dc.whoisProvider, domain, queue)
	}

	go func() {
		defer close(queue)
		wg.Wait()
	}()

	for domain := range queue {
		newCache = append(newCache, domain)
	}

	dc.intCache.Update(newCache)

	elapsedTime := time.Since(startTime)

	log.Debugf("Domain cache has been updated in %s", elapsedTime)
}
