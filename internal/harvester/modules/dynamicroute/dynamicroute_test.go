package dynamicroute

import (
	"context"
	"sync"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"

	"github.com/shurshun/domain-harvester/internal/harvester/types"
)

var testGVR = schema.GroupVersionResource{Group: "example.io", Version: "v1", Resource: "widgets"}

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

func (f *fakeDomainCache) domains() []*types.Domain {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.last
}

func newFakeClient(t *testing.T, objs ...runtime.Object) *fake.FakeDynamicClient {
	t.Helper()

	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{testGVR: "WidgetList"}

	return fake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, objs...)
}

func widget(name, namespace string, hostnames ...any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.io/v1",
		"kind":       "Widget",
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]any{
			"hostnames": hostnames,
		},
	}}
}

func extractHostnames(obj *unstructured.Unstructured) []string {
	hostnames, _, _ := unstructured.NestedStringSlice(obj.Object, "spec", "hostnames")

	return hostnames
}

func TestInit_missingCRDIsAnError(t *testing.T) {
	client := newFakeClient(t)
	client.PrependReactor("list", "widgets", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(testGVR.GroupResource(), "")
	})

	_, err := Init(context.Background(), client, &fakeDomainCache{}, testGVR, "widget", extractHostnames)
	if err == nil {
		t.Fatal("Init() error = nil, want an error when the CRD isn't installed")
	}
}

func TestInit_watchesAndPushesDomains(t *testing.T) {
	client := newFakeClient(t, widget("web", "default", "www.example.com"))
	dc := &fakeDomainCache{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h, err := Init(ctx, client, dc, testGVR, "widget", extractHostnames)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if got := h.Source(); got != "widget" {
		t.Errorf("Source() = %q, want %q", got, "widget")
	}

	waitFor(t, func() bool { return len(dc.domains()) == 1 })

	d := dc.domains()[0]

	if d.Name != "example.com" {
		t.Errorf("Name = %q, want eTLD+1 %q", d.Name, "example.com")
	}

	if d.Raw != "www.example.com" {
		t.Errorf("Raw = %q, want %q", d.Raw, "www.example.com")
	}

	if d.Source != "widget" || d.Ingress != "web" || d.NS != "default" {
		t.Errorf("Source/Ingress/NS = %q/%q/%q, want widget/web/default", d.Source, d.Ingress, d.NS)
	}

	waitFor(t, h.HasSynced)
}

func TestInit_reactsToLaterChanges(t *testing.T) {
	client := newFakeClient(t, widget("a", "default", "a.com"))
	dc := &fakeDomainCache{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if _, err := Init(ctx, client, dc, testGVR, "widget", extractHostnames); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	waitFor(t, func() bool { return len(dc.domains()) == 1 })

	if _, err := client.Resource(testGVR).Namespace("default").Create(ctx, widget("b", "default", "b.com"), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	waitFor(t, func() bool { return len(dc.domains()) == 2 })

	if err := client.Resource(testGVR).Namespace("default").Delete(ctx, "a", metav1.DeleteOptions{}); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	waitFor(t, func() bool {
		domains := dc.domains()
		return len(domains) == 1 && domains[0].Ingress == "b"
	})
}

func TestInit_skipsEmptyHostnames(t *testing.T) {
	client := newFakeClient(t, widget("web", "default", "", "www.example.com", ""))
	dc := &fakeDomainCache{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if _, err := Init(ctx, client, dc, testGVR, "widget", extractHostnames); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	waitFor(t, func() bool { return len(dc.domains()) == 1 })
}

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
