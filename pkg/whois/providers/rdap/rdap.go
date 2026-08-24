// Package rdap looks up domain expiry via RDAP (RFC 7482/7483), the HTTPS/JSON
// successor to WHOIS. Most registries either require it or rate-limit port 43
// far more aggressively than RDAP.
package rdap

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/openrdap/rdap"

	"github.com/shurshun/domain-harvester/pkg/whois/types"
)

// DefaultTimeout bounds a single RDAP request (including bootstrap lookups).
const DefaultTimeout = 10 * time.Second

// expirationEvent is the RFC 7483 §4.5 eventAction value for domain expiry.
const expirationEvent = "expiration"

// Provider is the only "real" provider: every GetDomainData call results in
// one or more outbound requests, so it is meant to be wrapped by pkg/whois/cache.
type Provider struct {
	client *rdap.Client

	externalRequests atomic.Uint64
}

// New returns a provider whose requests (including bootstrap lookups) are
// bounded by timeout.
func New(timeout time.Duration) *Provider {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	return &Provider{
		client: &rdap.Client{HTTP: &http.Client{Timeout: timeout}},
	}
}

// GetExternalRequestsCnt reports how many outbound HTTP requests were made,
// including bootstrap registry lookups.
func (p *Provider) GetExternalRequestsCnt() uint64 {
	return p.externalRequests.Load()
}

// GetDomainData always returns a non-nil WhoisData: on failure it carries the
// error message in the Error field so the exporter can surface it as a metric.
func (p *Provider) GetDomainData(domain string) (*types.WhoisData, error) {
	wd := &types.WhoisData{Domain: domain, LastUpdated: time.Now()}

	resp, err := p.client.Do(&rdap.Request{Type: rdap.DomainRequest, Query: domain})

	requests := uint64(1)
	if resp != nil {
		requests = uint64(len(resp.HTTP))
	}

	p.externalRequests.Add(requests)

	if err != nil {
		return withError(wd, fmt.Errorf("rdap query for %q failed: %w", domain, err))
	}

	rdapDomain, ok := resp.Object.(*rdap.Domain)
	if !ok {
		return withError(wd, fmt.Errorf("rdap response for %q is not a domain object", domain))
	}

	expiry, err := expirationDate(rdapDomain)
	if err != nil {
		return withError(wd, fmt.Errorf("rdap response for %q: %w", domain, err))
	}

	wd.ExpiryDays = math.Floor(time.Until(expiry).Hours() / 24)

	return wd, nil
}

// expirationDate finds the RFC 7483 "expiration" event, factored out from
// GetDomainData so it's unit-testable against fixture *rdap.Domain values
// without a network round trip.
func expirationDate(d *rdap.Domain) (time.Time, error) {
	for _, ev := range d.Events {
		if ev.Action != expirationEvent {
			continue
		}

		t, err := time.Parse(time.RFC3339, ev.Date)
		if err != nil {
			return time.Time{}, fmt.Errorf("unparsable expiration date %q: %w", ev.Date, err)
		}

		return t, nil
	}

	return time.Time{}, errors.New("no expiration event in rdap response")
}

func withError(wd *types.WhoisData, err error) (*types.WhoisData, error) {
	wd.Error = err.Error()

	return wd, err
}
