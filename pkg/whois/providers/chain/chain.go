// Package chain composes two WhoisHarverster implementations: a primary
// lookup, and a fallback used only when the primary fails. This is how the
// default stack tries RDAP first and falls back to raw WHOIS:43 for the
// (shrinking, but real) set of TLDs without RDAP support.
package chain

import (
	log "github.com/sirupsen/logrus"

	"github.com/shurshun/domain-harvester/pkg/whois/types"
)

type Provider struct {
	primary  types.WhoisHarverster
	fallback types.WhoisHarverster
}

// New returns a provider that tries primary first, falling back to fallback
// on any error.
func New(primary, fallback types.WhoisHarverster) *Provider {
	return &Provider{primary: primary, fallback: fallback}
}

func (p *Provider) GetDomainData(domain string) (*types.WhoisData, error) {
	wd, err := p.primary.GetDomainData(domain)
	if err == nil {
		return wd, nil
	}

	log.Debugf("primary whois lookup for %s failed (%s), falling back", domain, err)

	return p.fallback.GetDomainData(domain)
}

func (p *Provider) GetExternalRequestsCnt() uint64 {
	return p.primary.GetExternalRequestsCnt() + p.fallback.GetExternalRequestsCnt()
}
