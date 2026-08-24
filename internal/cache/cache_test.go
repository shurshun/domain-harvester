package cache

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/shurshun/domain-harvester/internal/harvester/types"
	whois_types "github.com/shurshun/domain-harvester/pkg/whois/types"
)

// fakeWhois answers every lookup deterministically, and can be told to fail
// specific domains to exercise the "cache rebuild survives one bad lookup" path.
type fakeWhois struct {
	mu      sync.Mutex
	calls   uint64
	failFor map[string]bool
}

func (f *fakeWhois) GetDomainData(domain string) (*whois_types.WhoisData, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()

	if f.failFor[domain] {
		return &whois_types.WhoisData{Domain: domain, Error: "boom"}, errors.New("boom")
	}

	return &whois_types.WhoisData{Domain: domain, ExpiryDays: 100, LastUpdated: time.Now()}, nil
}

func (f *fakeWhois) GetExternalRequestsCnt() uint64 {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.calls
}

func newTestCache(t *testing.T, whois whois_types.WhoisHarverster, opts Options) *DomainCache {
	t.Helper()

	// RebuildInterval way beyond any test's lifetime: the only rebuilds that
	// should fire are the ones Update() debounces, plus the one harmless
	// no-op rebuildDomainCache call Init always makes against an empty
	// cache. Real callers rely on that immediate call so the first tick
	// isn't a full RebuildInterval away; it's a no-op here only because no
	// Update() has landed yet when it runs.
	if opts.RebuildInterval == 0 {
		opts.RebuildInterval = time.Hour
	}

	dc, err := Init(context.Background(), whois, opts)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	return dc.(*DomainCache)
}

func TestUpdate_dedupesAcrossSourcesByPriority(t *testing.T) {
	whois := &fakeWhois{}
	dc := newTestCache(t, whois, Options{
		DebounceInterval: time.Millisecond,
		SourcePriority:   []string{"cluster", "config"},
	})

	dc.Update("config", []*types.Domain{{Name: "shared.com", Ingress: "from-config"}})
	dc.Update("cluster", []*types.Domain{{Name: "shared.com", Ingress: "from-cluster"}, {Name: "only-cluster.com"}})

	waitFor(t, func() bool { return len(dc.GetDomains()) == 2 })

	domains := indexByName(dc.GetDomains())

	shared, ok := domains["shared.com"]
	if !ok {
		t.Fatal("shared.com missing from the merged cache")
	}

	if shared.Ingress != "from-cluster" {
		t.Errorf("shared.com kept Ingress %q, want %q (cluster has priority over config)", shared.Ingress, "from-cluster")
	}

	if _, ok := domains["only-cluster.com"]; !ok {
		t.Error("only-cluster.com missing from the merged cache")
	}
}

func TestUpdate_dedupeIsDeterministicRegardlessOfInsertionOrder(t *testing.T) {
	whois := &fakeWhois{}

	// Insert config before cluster: priority order must still win over
	// insertion order, since sync.Map iteration order is not guaranteed.
	dc := newTestCache(t, whois, Options{
		DebounceInterval: time.Millisecond,
		SourcePriority:   []string{"cluster", "config"},
	})

	dc.Update("config", []*types.Domain{{Name: "shared.com", Ingress: "from-config"}})
	waitFor(t, func() bool { return len(dc.GetDomains()) == 1 })

	dc.Update("cluster", []*types.Domain{{Name: "shared.com", Ingress: "from-cluster"}})

	// The cluster push doesn't change the domain count (still one, deduped
	// name), so wait for the content itself rather than a count change.
	waitFor(t, func() bool {
		d, ok := indexByName(dc.GetDomains())["shared.com"]
		return ok && d.Ingress == "from-cluster"
	})
}

func TestRebuild_survivesAFailedLookup(t *testing.T) {
	whois := &fakeWhois{failFor: map[string]bool{"broken.com": true}}
	dc := newTestCache(t, whois, Options{
		DebounceInterval: time.Millisecond,
	})

	dc.Update("config", []*types.Domain{{Name: "broken.com"}, {Name: "fine.com"}})

	waitFor(t, func() bool { return len(dc.GetDomains()) == 2 })

	domains := indexByName(dc.GetDomains())

	if domains["broken.com"].WhoisData.Error == "" {
		t.Error("broken.com should carry the lookup error")
	}

	if domains["fine.com"].WhoisData.Error != "" {
		t.Error("fine.com should not be affected by broken.com's failure")
	}
}

func TestHasSynced(t *testing.T) {
	whois := &fakeWhois{}
	dc := newTestCache(t, whois, Options{DebounceInterval: time.Millisecond})

	if dc.HasSynced() {
		t.Error("HasSynced() = true before any domains were ever pushed, want false")
	}

	dc.Update("config", []*types.Domain{{Name: "example.com"}})

	waitFor(t, dc.HasSynced)
}

func TestHasSynced_emptyClusterIsSyncedNotStuck(t *testing.T) {
	whois := &fakeWhois{}
	dc := newTestCache(t, whois, Options{DebounceInterval: time.Millisecond})

	// A source pushing an explicitly empty view (e.g. a cluster with zero
	// Ingresses) must count as synced: rebuildDomainCache's early-return on
	// an empty merged view would otherwise never increment dc.rebuilds, and
	// readiness would wait forever for data that will never arrive.
	dc.Update("cluster", nil)

	waitFor(t, dc.HasSynced)

	if got := len(dc.GetDomains()); got != 0 {
		t.Errorf("GetDomains() = %d entries, want 0", got)
	}
}

// waitFor polls cond until it's true or 2s pass, whichever comes first. Used
// instead of counting internal rebuilds: this package's rebuild cadence is
// deliberately not part of its contract (see the note in newTestCache), so
// tests wait for the outcome a caller can actually observe.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)

	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(time.Millisecond)
	}

	t.Fatal("timed out waiting for condition")
}

func indexByName(domains []*types.Domain) map[string]*types.Domain {
	result := make(map[string]*types.Domain, len(domains))
	for _, d := range domains {
		result[d.Name] = d
	}

	return result
}
