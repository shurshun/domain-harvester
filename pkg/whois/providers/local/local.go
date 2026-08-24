// Package local queries WHOIS servers directly over port 43 and extracts the
// domain expiry date from the parsed response.
package local

import (
	"errors"
	"fmt"
	"math"
	"sync/atomic"
	"time"

	"github.com/likexian/whois"
	whoisparser "github.com/likexian/whois-parser"

	"github.com/shurshun/domain-harvester/pkg/whois/types"
)

// DefaultTimeout bounds a single WHOIS request.
const DefaultTimeout = 10 * time.Second

// WhoisProvider is the only "real" provider: every GetDomainData call results
// in an outbound request, so it is meant to be wrapped by pkg/whois/cache.
type WhoisProvider struct {
	client *whois.Client

	externalRequests atomic.Uint64
}

// New returns a provider whose requests are bounded by timeout.
func New(timeout time.Duration) *WhoisProvider {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	client := whois.NewClient().
		SetTimeout(timeout).
		SetDisableStats(true)

	return &WhoisProvider{client: client}
}

// GetExternalRequestsCnt reports how many outbound WHOIS requests were made.
func (wp *WhoisProvider) GetExternalRequestsCnt() uint64 {
	return wp.externalRequests.Load()
}

// GetDomainData always returns a non-nil WhoisData: on failure it carries the
// error message in the Error field so the exporter can surface it as a metric.
func (wp *WhoisProvider) GetDomainData(domain string) (*types.WhoisData, error) {
	wd := &types.WhoisData{Domain: domain, LastUpdated: time.Now()}

	wp.externalRequests.Add(1)

	raw, err := wp.client.Whois(domain)
	if err != nil {
		return withError(wd, fmt.Errorf("whois query for %q failed: %w", domain, err))
	}

	return parse(wd, raw)
}

// parse fills wd's expiry from a raw WHOIS response. Split out from
// GetDomainData so it can be unit-tested against fixtures without a network
// round trip.
func parse(wd *types.WhoisData, raw string) (*types.WhoisData, error) {
	info, err := whoisparser.Parse(raw)
	if err != nil {
		if errors.Is(err, whoisparser.ErrNotFoundDomain) {
			return withError(wd, fmt.Errorf("domain %q is not registered", wd.Domain))
		}

		return withError(wd, fmt.Errorf("cannot parse whois response for %q: %w", wd.Domain, err))
	}

	if info.Domain == nil || info.Domain.ExpirationDateInTime == nil {
		return withError(wd, fmt.Errorf("whois response for %q carries no expiration date", wd.Domain))
	}

	wd.ExpiryDays = math.Floor(time.Until(*info.Domain.ExpirationDateInTime).Hours() / 24)

	return wd, nil
}

func withError(wd *types.WhoisData, err error) (*types.WhoisData, error) {
	wd.Error = err.Error()

	return wd, err
}
