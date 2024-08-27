package helpers

import (
	"golang.org/x/net/publicsuffix"
	"golang.org/x/net/idna"
)

func EffectiveTLDPlusOne(domain string) string {
	tLDPlusOne, err := publicsuffix.EffectiveTLDPlusOne(domain)
	if err != nil {
		return domain
	}

	return tLDPlusOne
}

func ToUnicode(name string) string {
	p := idna.New()
	domain, _ := p.ToUnicode(name)

	return domain
}