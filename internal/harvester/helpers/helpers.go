// Package helpers normalises host names into the forms carried by types.Domain.
package helpers

import (
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

// EffectiveTLDPlusOne reduces a host to its registrable domain, which is what
// WHOIS servers answer for. Unparsable input is returned unchanged so the
// domain still shows up in the metrics with an error.
func EffectiveTLDPlusOne(domain string) string {
	tLDPlusOne, err := publicsuffix.EffectiveTLDPlusOne(domain)
	if err != nil {
		return domain
	}

	return tLDPlusOne
}

// ToUnicode decodes a punycode host for display; on failure the input is kept.
func ToUnicode(name string) string {
	domain, err := idna.New().ToUnicode(name)
	if err != nil {
		return name
	}

	return domain
}
