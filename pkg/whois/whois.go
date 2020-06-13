package whois

import (
	"domain-harvester/pkg/whois/cache"
	"domain-harvester/pkg/whois/providers/local"
	"domain-harvester/pkg/whois/types"
)

func Init() (types.WhoisHarverster, error) {
	wp := &local.WhoisProvider{}

	wp.Init()

	return cache.Init(wp)
}
