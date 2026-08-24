// Package whois assembles the WHOIS lookup stack: a real provider wrapped in a
// refreshing cache.
package whois

import (
	"context"
	"time"

	"github.com/shurshun/domain-harvester/pkg/whois/cache"
	"github.com/shurshun/domain-harvester/pkg/whois/providers/chain"
	"github.com/shurshun/domain-harvester/pkg/whois/providers/local"
	"github.com/shurshun/domain-harvester/pkg/whois/providers/rdap"
	"github.com/shurshun/domain-harvester/pkg/whois/types"
)

// Options is the subset of knobs exposed on the command line.
type Options struct {
	// Timeout bounds a single outbound RDAP or WHOIS request.
	Timeout time.Duration
	// Cache controls refresh tiers; zero values fall back to defaults.
	Cache cache.Options
}

// Init returns a cached WHOIS harvester whose background refresh stops with
// ctx. Lookups try RDAP first (RFC 7482/7483, HTTPS/JSON) and fall back to
// raw WHOIS:43 for the TLDs that don't publish an RDAP bootstrap entry yet.
func Init(ctx context.Context, opts Options) (types.WhoisHarverster, error) {
	provider := chain.New(rdap.New(opts.Timeout), local.New(opts.Timeout))

	return cache.Init(ctx, provider, opts.Cache)
}
