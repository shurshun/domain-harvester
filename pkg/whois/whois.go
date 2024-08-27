package whois

import (
	"github.com/shurshun/domain-harvester/pkg/whois/cache"
	"github.com/shurshun/domain-harvester/pkg/whois/providers/local"
	"github.com/shurshun/domain-harvester/pkg/whois/types"
)

func Init() (types.WhoisHarverster, error) {
	wp := &local.WhoisProvider{}

	wp.Init()

	return cache.Init(wp)
}
