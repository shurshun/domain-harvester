// Package cache holds the harvested domains and enriches them with WHOIS data.
package cache

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bep/debounce"
	log "github.com/sirupsen/logrus"

	"github.com/shurshun/domain-harvester/internal/harvester/types"
	whois_types "github.com/shurshun/domain-harvester/pkg/whois/types"
)

// Options tunes the rebuild loop.
type Options struct {
	// Concurrency caps how many WHOIS lookups run at once during a rebuild.
	// Without a cap a large cluster would open one socket per unique domain.
	Concurrency int
	// RebuildInterval is the unconditional periodic rebuild.
	RebuildInterval time.Duration
	// DebounceInterval collapses a burst of Update calls into one rebuild.
	DebounceInterval time.Duration
	// SourcePriority orders sources when the same domain is seen more than
	// once; earlier entries win. Sources not listed sort last, by name, so
	// the resulting labels are stable across scrapes.
	SourcePriority []string
}

// maxSaneConcurrency is the point past which WHOIS servers start rate-limiting.
const maxSaneConcurrency = 256

// DefaultOptions returns the built-in tuning.
func DefaultOptions() Options {
	return Options{
		Concurrency:      16,
		RebuildInterval:  time.Minute,
		DebounceInterval: time.Second,
		SourcePriority:   []string{"cluster", "config"},
	}
}

func (o Options) withDefaults() Options {
	d := DefaultOptions()

	if o.Concurrency <= 0 {
		o.Concurrency = d.Concurrency
	}

	if o.Concurrency > maxSaneConcurrency {
		log.Warnf("whois concurrency %d is very high, WHOIS servers are likely to rate-limit", o.Concurrency)
	}

	if o.RebuildInterval <= 0 {
		o.RebuildInterval = d.RebuildInterval
	}

	if o.DebounceInterval <= 0 {
		o.DebounceInterval = d.DebounceInterval
	}

	if len(o.SourcePriority) == 0 {
		o.SourcePriority = d.SourcePriority
	}

	return o
}

type DomainCache struct {
	rawCache      sync.Map
	intCache      *internalCache
	whoisProvider whois_types.WhoisHarverster
	opts          Options
	priority      map[string]int
	ctx           context.Context
	debounced     func(f func())
	rebuilds      atomic.Uint64
	// everUpdated tracks whether Update has ever been called, regardless of
	// whether the pushed view was empty. See HasSynced.
	everUpdated atomic.Bool
	// lastRebuildUnix and lastRebuildMillis back the two observability
	// metrics in internal/metrics/exporter.go.
	lastRebuildUnix   atomic.Int64
	lastRebuildMillis atomic.Int64
	// rebuildMu serializes rebuildDomainCache: the debounced Update path and
	// the periodic ticker can otherwise fire close together and run two
	// rebuilds concurrently, and since a rebuild takes as long as the
	// slowest WHOIS lookup, the two can finish out of order — the older,
	// staler one publishing last and clobbering fresher data.
	rebuildMu sync.Mutex
}

// Init starts the rebuild loop; it stops with ctx.
func Init(ctx context.Context, whoisProvider whois_types.WhoisHarverster, opts Options) (types.DomainCache, error) {
	opts = opts.withDefaults()

	priority := make(map[string]int, len(opts.SourcePriority))
	for i, source := range opts.SourcePriority {
		priority[source] = i
	}

	dc := &DomainCache{
		intCache:      &internalCache{},
		whoisProvider: whoisProvider,
		opts:          opts,
		priority:      priority,
		ctx:           ctx,
		debounced:     debounce.New(opts.DebounceInterval),
	}

	go dc.runCacheInvalidator(ctx)

	return dc, nil
}

func (dc *DomainCache) runCacheInvalidator(ctx context.Context) {
	ticker := time.NewTicker(dc.opts.RebuildInterval)
	defer ticker.Stop()

	dc.rebuildDomainCache(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dc.rebuildDomainCache(ctx)
		}
	}
}

func (dc *DomainCache) GetDomains() []*types.Domain {
	return dc.intCache.Get()
}

func (dc *DomainCache) GetExternalRequestsCnt() uint64 {
	return dc.whoisProvider.GetExternalRequestsCnt()
}

// Update replaces this source's whole view and schedules a debounced rebuild.
func (dc *DomainCache) Update(source string, domains []*types.Domain) {
	dc.rawCache.Store(source, domains)
	dc.everUpdated.Store(true)

	dc.debounced(func() {
		dc.rebuildDomainCache(dc.ctx)
	})
}

// sourceLess orders sources by configured priority, then by name, so that
// deduplication is deterministic regardless of sync.Map iteration order.
func (dc *DomainCache) sourceLess(a, b string) bool {
	pa, oka := dc.priority[a]
	pb, okb := dc.priority[b]

	switch {
	case oka && okb:
		return pa < pb
	case oka:
		return true
	case okb:
		return false
	default:
		return a < b
	}
}

func (dc *DomainCache) getUniqDomains() []*types.Domain {
	bySource := map[string][]*types.Domain{}

	dc.rawCache.Range(func(k, v any) bool {
		bySource[k.(string)] = v.([]*types.Domain)

		return true
	})

	sources := make([]string, 0, len(bySource))
	for source := range bySource {
		sources = append(sources, source)
	}

	sort.Slice(sources, func(i, j int) bool { return dc.sourceLess(sources[i], sources[j]) })

	var (
		seen   = map[string]bool{}
		result []*types.Domain
	)

	for _, source := range sources {
		for _, domain := range bySource[source] {
			if seen[domain.Name] {
				continue
			}

			seen[domain.Name] = true

			result = append(result, domain)
		}
	}

	return result
}

func (dc *DomainCache) rebuildDomainCache(ctx context.Context) {
	// Held for the whole rebuild (including the wg.Wait() below, and before
	// even snapshotting rawCache) so rebuilds serialize instead of racing
	// each other; see the rebuildMu doc comment.
	dc.rebuildMu.Lock()
	defer dc.rebuildMu.Unlock()

	startTime := time.Now()

	// Recorded on every exit path (including the empty-cache early return
	// below) so domain_cache_last_rebuild_timestamp reflects "is the rebuild
	// loop still alive", not just "did it last find something to enrich".
	defer func() {
		dc.lastRebuildUnix.Store(time.Now().Unix())
		dc.lastRebuildMillis.Store(time.Since(startTime).Milliseconds())
	}()

	uniqDomains := dc.getUniqDomains()

	if len(uniqDomains) == 0 {
		return
	}

	log.Debugf("Start rebuilding cache for %d domains...", len(uniqDomains))

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		newCache = make([]*types.Domain, 0, len(uniqDomains))
	)

	sem := make(chan struct{}, dc.opts.Concurrency)

	for _, domain := range uniqDomains {
		if ctx.Err() != nil {
			break
		}

		wg.Add(1)

		go func(domain *types.Domain) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			wd, err := dc.whoisProvider.GetDomainData(domain.Name)
			if err != nil {
				log.Debugf("Error on update %s domain: %s", domain.Name, err)
			}

			if wd == nil {
				// The exporter dereferences WhoisData without a nil check;
				// keep that invariant by dropping unpopulated entries.
				return
			}

			// uniqDomains points at the same *types.Domain objects the
			// harvesters keep in rawCache, which the exporter's Collect()
			// may be reading concurrently off the previously published
			// cache — copy rather than mutate them in place.
			clone := *domain
			clone.WhoisData = wd

			mu.Lock()
			newCache = append(newCache, &clone)
			mu.Unlock()
		}(domain)
	}

	wg.Wait()

	dc.intCache.Update(newCache)
	dc.rebuilds.Add(1)

	log.Debugf("Domain cache has been updated in %s", time.Since(startTime))
}

// HasSynced reports whether the cache reflects a real view of the world: either
// a rebuild has actually enriched and published some domains, or every source
// has reported in and there is genuinely nothing to show (an empty cluster
// with no config file is fully synced, not stuck).
func (dc *DomainCache) HasSynced() bool {
	if dc.rebuilds.Load() > 0 {
		return true
	}

	return dc.everUpdated.Load() && len(dc.getUniqDomains()) == 0
}

// LastRebuildUnix is the unix timestamp of the last completed rebuild pass
// (even one that found nothing to enrich), or 0 before the first one.
func (dc *DomainCache) LastRebuildUnix() int64 {
	return dc.lastRebuildUnix.Load()
}

// LastRebuildDurationSeconds is the wall-clock time the last rebuild pass
// took, including its WHOIS fan-out.
func (dc *DomainCache) LastRebuildDurationSeconds() float64 {
	return float64(dc.lastRebuildMillis.Load()) / 1000
}
