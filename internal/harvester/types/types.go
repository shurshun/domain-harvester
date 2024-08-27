package types

import (
	whois_types "github.com/shurshun/domain-harvester/pkg/whois/types"
)

type Domain struct {
	Name        string
	DisplayName string
	Raw         string
	Source      string
	Ingress     string
	NS          string
	WhoisData   *whois_types.WhoisData
}

type Harvester interface {
	// GetDomains() []Domain
}

type DomainCache interface {
	GetDomains() []*Domain
	Update(source string, domains []*Domain)
	GetExternalRequestsCnt() uint64
}
