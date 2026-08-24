package local

import (
	"strings"
	"testing"
	"time"

	"github.com/shurshun/domain-harvester/pkg/whois/types"
)

// A trimmed but real-shaped verisign-style WHOIS response.
const rawWithExpiry = `Domain Name: EXAMPLE.COM
Registry Domain ID: 2336799_DOMAIN_COM-VRSN
Registrar WHOIS Server: whois.example-registrar.com
Registrar URL: http://www.example-registrar.com
Updated Date: 2024-08-14T07:01:44Z
Creation Date: 1995-08-14T04:00:00Z
Registry Expiry Date: 2030-08-13T04:00:00Z
Registrar: Example Registrar, LLC
Domain Status: clientDeleteProhibited https://icann.org/epp#clientDeleteProhibited
Name Server: NS1.EXAMPLE.COM
Name Server: NS2.EXAMPLE.COM
DNSSEC: signedDelegation
>>> Last update of WHOIS database: 2026-08-24T00:00:00Z <<<
`

const rawNoExpiry = `Domain Name: EXAMPLE.COM
Registrar: Example Registrar, LLC
Domain Status: clientDeleteProhibited
Name Server: NS1.EXAMPLE.COM
`

func TestParse(t *testing.T) {
	t.Run("valid response yields a positive expiry", func(t *testing.T) {
		wd, err := parse(&types.WhoisData{Domain: "example.com"}, rawWithExpiry)
		if err != nil {
			t.Fatalf("parse() error = %v", err)
		}

		if wd.Error != "" {
			t.Errorf("wd.Error = %q, want empty", wd.Error)
		}

		if wd.ExpiryDays <= 0 {
			t.Errorf("wd.ExpiryDays = %v, want > 0 (fixture expires in 2030)", wd.ExpiryDays)
		}
	})

	t.Run("response without an expiry date is an error", func(t *testing.T) {
		wd, err := parse(&types.WhoisData{Domain: "example.com"}, rawNoExpiry)
		if err == nil {
			t.Fatal("parse() error = nil, want an error")
		}

		if wd.Error == "" {
			t.Error("wd.Error is empty, want the error message recorded on WhoisData")
		}
	})

	t.Run("unparsable garbage does not panic and records an error", func(t *testing.T) {
		wd, err := parse(&types.WhoisData{Domain: "example.com"}, "not a whois response at all")
		if err == nil {
			t.Fatal("parse() error = nil, want an error")
		}

		if wd.Error == "" {
			t.Error("wd.Error is empty, want the error message recorded on WhoisData")
		}
	})

	t.Run("not-found domain is reported as an error, not a crash", func(t *testing.T) {
		// whois-parser recognizes this exact phrasing as "domain not found".
		wd, err := parse(&types.WhoisData{Domain: "example.com"}, "No matching record.")
		if err == nil {
			t.Fatal("parse() error = nil, want an error")
		}

		if !strings.Contains(wd.Error, "not registered") {
			t.Errorf("wd.Error = %q, want it to mention the domain is not registered", wd.Error)
		}
	})
}

func TestNew(t *testing.T) {
	// New must apply DefaultTimeout when given a non-positive value, rather
	// than leaving the client to block indefinitely.
	if wp := New(0); wp == nil || wp.client == nil {
		t.Fatal("New(0) did not build a usable provider")
	}

	if wp := New(-time.Second); wp == nil || wp.client == nil {
		t.Fatal("New(negative) did not build a usable provider")
	}
}
