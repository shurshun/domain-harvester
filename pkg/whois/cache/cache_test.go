package cache

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/shurshun/domain-harvester/pkg/whois/types"
)

func TestOptions_shouldUpdate(t *testing.T) {
	opts := DefaultOptions()
	now := time.Now()

	tests := []struct {
		name string
		wd   *types.WhoisData
		want bool
	}{
		{
			name: "healthy entry, fresh",
			wd:   &types.WhoisData{ExpiryDays: 365, LastUpdated: now.Add(-time.Minute)},
			want: false,
		},
		{
			name: "healthy entry, older than refresh interval",
			wd:   &types.WhoisData{ExpiryDays: 365, LastUpdated: now.Add(-opts.RefreshInterval - time.Second)},
			want: true,
		},
		{
			name: "near-expiry entry, within its own tighter interval",
			wd:   &types.WhoisData{ExpiryDays: opts.NearExpiryDays - 1, LastUpdated: now.Add(-time.Minute)},
			want: false,
		},
		{
			name: "near-expiry entry, older than its tighter interval but younger than the healthy one",
			wd: &types.WhoisData{
				ExpiryDays:  opts.NearExpiryDays - 1,
				LastUpdated: now.Add(-opts.NearExpiryInterval - time.Second),
			},
			want: true,
		},
		{
			name: "errored entry, within retry interval",
			wd:   &types.WhoisData{Error: "boom", LastUpdated: now.Add(-time.Minute)},
			want: false,
		},
		{
			name: "errored entry, older than retry interval",
			wd:   &types.WhoisData{Error: "boom", LastUpdated: now.Add(-opts.ErrorRetryInterval - time.Second)},
			want: true,
		},
		{
			name: "an error takes priority over how far expiry is, even if not near",
			wd:   &types.WhoisData{Error: "boom", ExpiryDays: 365, LastUpdated: now.Add(-opts.ErrorRetryInterval - time.Second)},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := opts.shouldUpdate(tt.wd, now); got != tt.want {
				t.Errorf("shouldUpdate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// fakeProvider counts calls per domain so tests can assert refresh behavior.
type fakeProvider struct {
	mu    sync.Mutex
	calls map[string]uint64
}

func newFakeProvider() *fakeProvider {
	return &fakeProvider{calls: map[string]uint64{}}
}

func (f *fakeProvider) GetDomainData(domain string) (*types.WhoisData, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls[domain]++

	return &types.WhoisData{Domain: domain, ExpiryDays: 365, LastUpdated: time.Now()}, nil
}

func (f *fakeProvider) GetExternalRequestsCnt() uint64 {
	f.mu.Lock()
	defer f.mu.Unlock()

	var total uint64
	for _, n := range f.calls {
		total += n
	}

	return total
}

func (f *fakeProvider) callCount(domain string) uint64 {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.calls[domain]
}

func TestWhoisCache_GetDomainData(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	provider := newFakeProvider()

	wc, err := Init(ctx, provider, Options{})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// First call is a miss and hits the provider.
	if _, err := wc.GetDomainData("example.com"); err != nil {
		t.Fatalf("GetDomainData() error = %v", err)
	}

	if got := provider.callCount("example.com"); got != 1 {
		t.Fatalf("provider called %d times, want 1", got)
	}

	// Second call is a cache hit: no additional provider call.
	if _, err := wc.GetDomainData("example.com"); err != nil {
		t.Fatalf("GetDomainData() error = %v", err)
	}

	if got := provider.callCount("example.com"); got != 1 {
		t.Errorf("provider called %d times after a cache hit, want still 1", got)
	}

	if got := wc.GetExternalRequestsCnt(); got != 1 {
		t.Errorf("GetExternalRequestsCnt() = %d, want 1", got)
	}
}
