package helpers

import "testing"

func TestEffectiveTLDPlusOne(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"bare domain", "google.com", "google.com"},
		{"single subdomain", "www.google.com", "google.com"},
		{"deep subdomain", "a.b.c.google.com", "google.com"},
		{"multi-level tld", "www.example.co.uk", "example.co.uk"},
		{"already eTLD+1 multi-level tld", "example.co.uk", "example.co.uk"},
		{"punycode host", "www.xn--80ak6aa92e.com", "xn--80ak6aa92e.com"},
		{"unparsable input returned unchanged", "not a domain", "not a domain"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EffectiveTLDPlusOne(tt.in); got != tt.want {
				t.Errorf("EffectiveTLDPlusOne(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestToUnicode(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"ascii passthrough", "google.com", "google.com"},
		{"punycode decoded", "xn--80ak6aa92e.com", "аррӏе.com"},
		{"invalid punycode kept as-is", "xn--x", "xn--x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToUnicode(tt.in); got != tt.want {
				t.Errorf("ToUnicode(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
