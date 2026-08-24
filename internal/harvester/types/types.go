package types

import (
	whois_types "github.com/shurshun/domain-harvester/pkg/whois/types"
)

// Domain carries the three name forms used across the app:
//   - Name is the eTLD+1 and doubles as the WHOIS lookup key and the dedup key
//   - Raw is the untouched host (ingress rule host or literal config entry)
//   - DisplayName is the IDN-decoded Name
type Domain struct {
	Name        string
	DisplayName string
	Raw         string
	Source      string
	Ingress     string
	NS          string
	WhoisData   *whois_types.WhoisData
}

// Harvester is one source of domains. Implementations push their full view into
// the DomainCache whenever it changes.
type Harvester interface {
	// Source is the cache key this harvester owns, also exported as the
	// "source" metric label.
	Source() string
	// HasSynced reports whether the initial view has been loaded. Readiness
	// stays false until every harvester has synced.
	HasSynced() bool
}

type DomainCache interface {
	GetDomains() []*Domain
	Update(source string, domains []*Domain)
	GetExternalRequestsCnt() uint64
}
