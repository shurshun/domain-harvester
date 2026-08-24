package cluster

import (
	"sync"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/shurshun/domain-harvester/internal/harvester/types"
)

// fakeDomainCache records every Update call so tests can inspect what the
// harvester pushed, without a real internal/cache.DomainCache.
type fakeDomainCache struct {
	mu      sync.Mutex
	sources []string
	last    []*types.Domain
}

func (f *fakeDomainCache) GetDomains() []*types.Domain { return nil }

func (f *fakeDomainCache) Update(source string, domains []*types.Domain) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.sources = append(f.sources, source)
	f.last = domains
}

func (f *fakeDomainCache) GetExternalRequestsCnt() uint64 { return 0 }

func ingress(name, namespace string, hosts ...string) *networkingv1.Ingress {
	rules := make([]networkingv1.IngressRule, 0, len(hosts))
	for _, h := range hosts {
		rules = append(rules, networkingv1.IngressRule{Host: h})
	}

	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       networkingv1.IngressSpec{Rules: rules},
	}
}

func newTestHarvester(t *testing.T, objs ...*networkingv1.Ingress) (*ClusterHarverster, *fakeDomainCache) {
	t.Helper()

	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	for _, o := range objs {
		if err := store.Add(o); err != nil {
			t.Fatalf("store.Add() error = %v", err)
		}
	}

	dc := &fakeDomainCache{}

	return &ClusterHarverster{ingressCache: store, domainCache: dc}, dc
}

func TestGetDomains(t *testing.T) {
	ch, _ := newTestHarvester(t,
		ingress("web", "default", "www.example.com", "example.com"),
		ingress("empty-host", "default", ""),
		ingress("intl", "shop", "www.xn--80ak6aa92e.com"),
	)

	domains := ch.getDomains()

	byRaw := make(map[string]*types.Domain, len(domains))
	for _, d := range domains {
		byRaw[d.Raw] = d
	}

	if len(domains) != 3 {
		t.Fatalf("getDomains() returned %d domains, want 3 (empty-host rule skipped)", len(domains))
	}

	www, ok := byRaw["www.example.com"]
	if !ok {
		t.Fatal("www.example.com missing from result")
	}

	if www.Name != "example.com" {
		t.Errorf("Name = %q, want eTLD+1 %q", www.Name, "example.com")
	}

	if www.Source != source {
		t.Errorf("Source = %q, want %q", www.Source, source)
	}

	if www.Ingress != "web" || www.NS != "default" {
		t.Errorf("Ingress/NS = %q/%q, want %q/%q", www.Ingress, www.NS, "web", "default")
	}

	intl, ok := byRaw["www.xn--80ak6aa92e.com"]
	if !ok {
		t.Fatal("punycode host missing from result")
	}

	if intl.DisplayName == intl.Name {
		t.Errorf("DisplayName %q should be the IDN-decoded form, not equal to Name %q", intl.DisplayName, intl.Name)
	}
}

func TestOnChange_pushesTheFullStoreView(t *testing.T) {
	ch, dc := newTestHarvester(t, ingress("a", "default", "a.com"), ingress("b", "default", "b.com"))

	ch.ingressCreated(ingress("a", "default", "a.com"))

	dc.mu.Lock()
	defer dc.mu.Unlock()

	if len(dc.sources) != 1 || dc.sources[0] != source {
		t.Fatalf("Update called with sources %v, want a single call with %q", dc.sources, source)
	}

	if len(dc.last) != 2 {
		t.Errorf("pushed %d domains, want 2 (the whole store, not just the changed object)", len(dc.last))
	}
}

func TestIngressDeleted_unwrapsTombstone(t *testing.T) {
	ch, dc := newTestHarvester(t)

	ing := ingress("gone", "default", "gone.com")
	tombstone := cache.DeletedFinalStateUnknown{Key: "default/gone", Obj: ing}

	ch.ingressDeleted(tombstone)

	dc.mu.Lock()
	defer dc.mu.Unlock()

	if len(dc.sources) != 1 {
		t.Fatalf("Update called %d times, want 1", len(dc.sources))
	}
}

func TestIngressDeleted_garbageObjectDoesNotPanic(t *testing.T) {
	ch, dc := newTestHarvester(t)

	ch.ingressDeleted("not an ingress or a tombstone")

	dc.mu.Lock()
	defer dc.mu.Unlock()

	if len(dc.sources) != 1 {
		t.Fatalf("Update called %d times, want 1 (still pushes the current store view)", len(dc.sources))
	}
}

func TestSource(t *testing.T) {
	ch := &ClusterHarverster{}
	if got := ch.Source(); got != "cluster" {
		t.Errorf("Source() = %q, want %q", got, "cluster")
	}
}

func TestHasSynced_nilControllerIsFalse(t *testing.T) {
	ch := &ClusterHarverster{}
	if ch.HasSynced() {
		t.Error("HasSynced() = true with a nil controller, want false")
	}
}
