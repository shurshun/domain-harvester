package metrics

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/shurshun/domain-harvester/internal/harvester/types"
	whois_types "github.com/shurshun/domain-harvester/pkg/whois/types"
)

// fakeDomainCache implements types.DomainCache with a fixed, test-controlled view.
type fakeDomainCache struct {
	domains       []*types.Domain
	externalCalls uint64
}

func (f *fakeDomainCache) GetDomains() []*types.Domain    { return f.domains }
func (f *fakeDomainCache) Update(string, []*types.Domain) {}
func (f *fakeDomainCache) GetExternalRequestsCnt() uint64 { return f.externalCalls }

func TestExporter_Collect(t *testing.T) {
	lastUpdated := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	cache := &fakeDomainCache{
		externalCalls: 7,
		domains: []*types.Domain{
			{
				Name:        "example.com",
				DisplayName: "example.com",
				Raw:         "www.example.com",
				Source:      "cluster",
				Ingress:     "example",
				NS:          "default",
				WhoisData: &whois_types.WhoisData{
					Domain:      "example.com",
					ExpiryDays:  42,
					LastUpdated: lastUpdated,
				},
			},
			{
				Name:        "broken.com",
				DisplayName: "broken.com",
				Raw:         "broken.com",
				Source:      "config",
				Ingress:     "broken-project",
				NS:          "broken-project",
				WhoisData: &whois_types.WhoisData{
					Domain:      "broken.com",
					ExpiryDays:  0,
					LastUpdated: lastUpdated,
					Error:       "whois query failed",
				},
			},
			{
				// Not yet enriched by a cache rebuild: must not panic and
				// must not appear in the exported metrics.
				Name:      "pending.com",
				WhoisData: nil,
			},
		},
	}

	exporter := NewDomainExporter(cache)

	want := fmt.Sprintf(`
# HELP domain_expiry_days time in days until the domain expires
# TYPE domain_expiry_days gauge
domain_expiry_days{domain="broken.com",fqdn="broken.com",ingress="broken-project",ingress_namespace="broken-project",source="config"} 0
domain_expiry_days{domain="example.com",fqdn="www.example.com",ingress="example",ingress_namespace="default",source="cluster"} 42
# HELP domain_last_updated last update of the domain
# TYPE domain_last_updated gauge
domain_last_updated{domain="broken.com",fqdn="broken.com",ingress="broken-project",ingress_namespace="broken-project",source="config"} %[1]d
domain_last_updated{domain="example.com",fqdn="www.example.com",ingress="example",ingress_namespace="default",source="cluster"} %[1]d
# HELP domain_update_error error on domain update
# TYPE domain_update_error gauge
domain_update_error{domain="broken.com",fqdn="broken.com",ingress="broken-project",ingress_namespace="broken-project",source="config"} 1
domain_update_error{domain="example.com",fqdn="www.example.com",ingress="example",ingress_namespace="default",source="cluster"} 0
# HELP domain_whois_requests requests to the whois servers
# TYPE domain_whois_requests counter
domain_whois_requests 7
`, lastUpdated.Unix())

	if err := testutil.CollectAndCompare(exporter, strings.NewReader(want)); err != nil {
		t.Errorf("unexpected collected metrics:\n%s", err)
	}
}

// fakeDomainCacheWithRebuildStats additionally implements rebuildObserver,
// exercising the optional cache metrics path in Collect.
type fakeDomainCacheWithRebuildStats struct {
	fakeDomainCache

	lastRebuildUnix     int64
	rebuildDurationSecs float64
}

func (f *fakeDomainCacheWithRebuildStats) LastRebuildUnix() int64 {
	return f.lastRebuildUnix
}

func (f *fakeDomainCacheWithRebuildStats) LastRebuildDurationSeconds() float64 {
	return f.rebuildDurationSecs
}

func TestExporter_Collect_withRebuildStats(t *testing.T) {
	cache := &fakeDomainCacheWithRebuildStats{
		lastRebuildUnix:     1700000000,
		rebuildDurationSecs: 2.5,
	}

	exporter := NewDomainExporter(cache)

	want := `
# HELP domain_cache_last_rebuild_timestamp unix timestamp of the last completed domain cache rebuild
# TYPE domain_cache_last_rebuild_timestamp gauge
domain_cache_last_rebuild_timestamp 1.7e+09
# HELP domain_cache_rebuild_duration_seconds wall-clock time the last domain cache rebuild took
# TYPE domain_cache_rebuild_duration_seconds gauge
domain_cache_rebuild_duration_seconds 2.5
# HELP domain_whois_requests requests to the whois servers
# TYPE domain_whois_requests counter
domain_whois_requests 0
`

	if err := testutil.CollectAndCompare(exporter, strings.NewReader(want)); err != nil {
		t.Errorf("unexpected collected metrics:\n%s", err)
	}
}
