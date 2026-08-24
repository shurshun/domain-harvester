package chain

import (
	"errors"
	"testing"

	"github.com/shurshun/domain-harvester/pkg/whois/types"
)

type fakeProvider struct {
	calls   int
	wd      *types.WhoisData
	err     error
	extReqs uint64
}

func (f *fakeProvider) GetDomainData(domain string) (*types.WhoisData, error) {
	f.calls++

	return f.wd, f.err
}

func (f *fakeProvider) GetExternalRequestsCnt() uint64 {
	return f.extReqs
}

func TestProvider_GetDomainData(t *testing.T) {
	t.Run("primary success short-circuits the fallback", func(t *testing.T) {
		primary := &fakeProvider{wd: &types.WhoisData{Domain: "example.com", ExpiryDays: 10}}
		fallback := &fakeProvider{wd: &types.WhoisData{Domain: "example.com", ExpiryDays: 20}}

		p := New(primary, fallback)

		wd, err := p.GetDomainData("example.com")
		if err != nil {
			t.Fatalf("GetDomainData() error = %v", err)
		}

		if wd.ExpiryDays != 10 {
			t.Errorf("ExpiryDays = %v, want the primary's value (10)", wd.ExpiryDays)
		}

		if fallback.calls != 0 {
			t.Errorf("fallback called %d times, want 0", fallback.calls)
		}
	})

	t.Run("primary failure falls through to the fallback", func(t *testing.T) {
		primary := &fakeProvider{wd: &types.WhoisData{Domain: "example.com"}, err: errors.New("rdap: not found")}
		fallback := &fakeProvider{wd: &types.WhoisData{Domain: "example.com", ExpiryDays: 20}}

		p := New(primary, fallback)

		wd, err := p.GetDomainData("example.com")
		if err != nil {
			t.Fatalf("GetDomainData() error = %v", err)
		}

		if wd.ExpiryDays != 20 {
			t.Errorf("ExpiryDays = %v, want the fallback's value (20)", wd.ExpiryDays)
		}

		if fallback.calls != 1 {
			t.Errorf("fallback called %d times, want 1", fallback.calls)
		}
	})

	t.Run("both failing surfaces the fallback's error", func(t *testing.T) {
		fallbackErr := errors.New("whois: timeout")
		primary := &fakeProvider{wd: &types.WhoisData{}, err: errors.New("rdap: not found")}
		fallback := &fakeProvider{wd: &types.WhoisData{}, err: fallbackErr}

		p := New(primary, fallback)

		if _, err := p.GetDomainData("example.com"); !errors.Is(err, fallbackErr) {
			t.Errorf("GetDomainData() error = %v, want %v", err, fallbackErr)
		}
	})
}

func TestProvider_GetExternalRequestsCnt(t *testing.T) {
	primary := &fakeProvider{extReqs: 3}
	fallback := &fakeProvider{extReqs: 5}

	p := New(primary, fallback)

	if got := p.GetExternalRequestsCnt(); got != 8 {
		t.Errorf("GetExternalRequestsCnt() = %d, want 8 (sum of both)", got)
	}
}
