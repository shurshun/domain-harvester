package helpers

import (
	"golang.org/x/net/publicsuffix"
)

func EffectiveTLDPlusOne(domain string) string {
	tLDPlusOne, err := publicsuffix.EffectiveTLDPlusOne(domain)
	if err != nil {
		return domain
	}

	return tLDPlusOne
}
