// Package cache decorates a WhoisHarverster with an in-memory cache so that
// repeated lookups for the same domain do not hit a WHOIS server every time.
package cache

import (
	"context"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/shurshun/domain-harvester/pkg/whois/types"
)

// Options controls how aggressively cached entries are refreshed. Entries fall
// into three tiers so that healthy, far-from-expiry domains are queried rarely.
type Options struct {
	// RefreshInterval applies to healthy entries.
	RefreshInterval time.Duration
	// NearExpiryInterval applies to entries expiring within NearExpiryDays.
	NearExpiryInterval time.Duration
	// ErrorRetryInterval applies to entries whose last lookup failed.
	ErrorRetryInterval time.Duration
	// NearExpiryDays is the threshold for the near-expiry tier.
	NearExpiryDays float64
	// TickInterval is how often the invalidator scans the cache.
	TickInterval time.Duration
}

// DefaultOptions mirrors the tiers documented in the README.
func DefaultOptions() Options {
	return Options{
		RefreshInterval:    60 * time.Minute,
		NearExpiryInterval: 10 * time.Minute,
		ErrorRetryInterval: 15 * time.Minute,
		NearExpiryDays:     30,
		TickInterval:       time.Minute,
	}
}

func (o Options) withDefaults() Options {
	d := DefaultOptions()

	if o.RefreshInterval <= 0 {
		o.RefreshInterval = d.RefreshInterval
	}

	if o.NearExpiryInterval <= 0 {
		o.NearExpiryInterval = d.NearExpiryInterval
	}

	if o.ErrorRetryInterval <= 0 {
		o.ErrorRetryInterval = d.ErrorRetryInterval
	}

	if o.NearExpiryDays <= 0 {
		o.NearExpiryDays = d.NearExpiryDays
	}

	if o.TickInterval <= 0 {
		o.TickInterval = d.TickInterval
	}

	return o
}

type WhoisCache struct {
	cache         sync.Map
	whoisProvider types.WhoisHarverster
	opts          Options
}

// Init wraps whoisProvider and starts a background invalidator that stops with ctx.
func Init(ctx context.Context, whoisProvider types.WhoisHarverster, opts Options) (types.WhoisHarverster, error) {
	wc := &WhoisCache{whoisProvider: whoisProvider, opts: opts.withDefaults()}

	go wc.runInvalidator(ctx)

	return wc, nil
}

// shouldUpdate reports whether a cached entry is due for a refresh at now.
func (o Options) shouldUpdate(wd *types.WhoisData, now time.Time) bool {
	age := now.Sub(wd.LastUpdated)

	switch {
	case wd.Error != "":
		return age >= o.ErrorRetryInterval
	case wd.ExpiryDays < o.NearExpiryDays:
		return age >= o.NearExpiryInterval
	default:
		return age >= o.RefreshInterval
	}
}

func (wc *WhoisCache) runInvalidator(ctx context.Context) {
	ticker := time.NewTicker(wc.opts.TickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			wc.cache.Range(func(k, v any) bool {
				if wc.opts.shouldUpdate(v.(*types.WhoisData), now) {
					if _, err := wc.update(k.(string)); err != nil {
						log.Debugf("Error refreshing %s: %s", k.(string), err)
					}
				}

				return ctx.Err() == nil
			})
		}
	}
}

func (wc *WhoisCache) update(domain string) (*types.WhoisData, error) {
	wd, err := wc.whoisProvider.GetDomainData(domain)

	wc.cache.Store(domain, wd)

	return wd, err
}

// GetDomainData returns the cached entry, querying the provider on a miss.
func (wc *WhoisCache) GetDomainData(domain string) (*types.WhoisData, error) {
	if wd, ok := wc.cache.Load(domain); ok {
		return wd.(*types.WhoisData), nil
	}

	return wc.update(domain)
}

func (wc *WhoisCache) GetExternalRequestsCnt() uint64 {
	return wc.whoisProvider.GetExternalRequestsCnt()
}
