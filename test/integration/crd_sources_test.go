// Package integration runs the CRD-backed domain sources (Traefik
// IngressRoute, Gateway API HTTPRoute/GRPCRoute) against a real
// kube-apiserver started by envtest with the actual upstream CRDs installed
// (testdata/crds), rather than the fake dynamic client the unit tests in
// internal/harvester/modules/dynamicroute use. That catches anything a fake
// client's loose validation would miss — e.g. a field name that doesn't
// match the real CRD schema.
//
// Skipped unless KUBEBUILDER_ASSETS is set. To run locally:
//
//	go run sigs.k8s.io/controller-runtime/tools/setup-envtest@latest use -p env
//	source <(go run sigs.k8s.io/controller-runtime/tools/setup-envtest@latest use -p env)
//	go test ./test/integration/...
package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/shurshun/domain-harvester/internal/harvester/modules/grpcroute"
	"github.com/shurshun/domain-harvester/internal/harvester/modules/httproute"
	"github.com/shurshun/domain-harvester/internal/harvester/modules/traefik"
	"github.com/shurshun/domain-harvester/internal/harvester/types"
)

var (
	dynClient  dynamic.Interface
	skipReason string
)

func TestMain(m *testing.M) {
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		skipReason = "KUBEBUILDER_ASSETS not set; run `go run sigs.k8s.io/controller-runtime/tools/setup-envtest@latest use -p env` and source its output first"
		os.Exit(m.Run())
	}

	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{"testdata/crds"},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, "envtest start:", err)
		os.Exit(1)
	}

	dynClient, err = dynamic.NewForConfig(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dynamic client:", err)
		_ = testEnv.Stop()
		os.Exit(1)
	}

	code := m.Run()

	if err := testEnv.Stop(); err != nil {
		fmt.Fprintln(os.Stderr, "envtest stop:", err)
	}

	os.Exit(code)
}

// fakeDomainCache records every Update call so tests can inspect what the
// harvester pushed, without depending on internal/cache.
type fakeDomainCache struct {
	updates chan []*types.Domain
}

func newFakeDomainCache() *fakeDomainCache {
	return &fakeDomainCache{updates: make(chan []*types.Domain, 16)}
}

func (f *fakeDomainCache) GetDomains() []*types.Domain    { return nil }
func (f *fakeDomainCache) GetExternalRequestsCnt() uint64 { return 0 }

func (f *fakeDomainCache) Update(_ string, domains []*types.Domain) {
	f.updates <- domains
}

// waitForDomains polls Update calls until one carries exactly n domains, or
// fails the test after 10s — apiserver list-watch setup in envtest is slower
// than the in-memory fake client the unit tests use.
func waitForDomains(t *testing.T, dc *fakeDomainCache, n int) []*types.Domain {
	t.Helper()

	deadline := time.After(10 * time.Second)

	for {
		select {
		case domains := <-dc.updates:
			if len(domains) == n {
				return domains
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %d domain(s)", n)
			return nil
		}
	}
}

func TestTraefikIngressRoute_realCRD(t *testing.T) {
	if skipReason != "" {
		t.Skip(skipReason)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gvr := schema.GroupVersionResource{Group: "traefik.io", Version: "v1alpha1", Resource: "ingressroutes"}

	route := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "traefik.io/v1alpha1",
		"kind":       "IngressRoute",
		"metadata": map[string]any{
			"name":      "web",
			"namespace": "default",
		},
		"spec": map[string]any{
			"routes": []any{
				map[string]any{
					"kind":  "Rule",
					"match": "Host(`traefik-test.example.com`)",
				},
			},
		},
	}}

	if _, err := dynClient.Resource(gvr).Namespace("default").Create(ctx, route, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create(IngressRoute) error = %v", err)
	}

	dc := newFakeDomainCache()

	if _, err := traefik.Init(ctx, dynClient, dc); err != nil {
		t.Fatalf("traefik.Init() error = %v", err)
	}

	domains := waitForDomains(t, dc, 1)

	if domains[0].Name != "example.com" || domains[0].Raw != "traefik-test.example.com" {
		t.Errorf("got Name=%q Raw=%q, want Name=%q Raw=%q",
			domains[0].Name, domains[0].Raw, "example.com", "traefik-test.example.com")
	}
}

func TestGatewayHTTPRoute_realCRD(t *testing.T) {
	if skipReason != "" {
		t.Skip(skipReason)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gvr := schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}

	route := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "HTTPRoute",
		"metadata": map[string]any{
			"name":      "web",
			"namespace": "default",
		},
		"spec": map[string]any{
			"hostnames": []any{"httproute-test.example.com"},
		},
	}}

	if _, err := dynClient.Resource(gvr).Namespace("default").Create(ctx, route, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create(HTTPRoute) error = %v", err)
	}

	dc := newFakeDomainCache()

	if _, err := httproute.Init(ctx, dynClient, dc); err != nil {
		t.Fatalf("httproute.Init() error = %v", err)
	}

	domains := waitForDomains(t, dc, 1)

	if domains[0].Name != "example.com" || domains[0].Raw != "httproute-test.example.com" {
		t.Errorf("got Name=%q Raw=%q, want Name=%q Raw=%q",
			domains[0].Name, domains[0].Raw, "example.com", "httproute-test.example.com")
	}
}

func TestGatewayGRPCRoute_realCRD(t *testing.T) {
	if skipReason != "" {
		t.Skip(skipReason)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gvr := schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "grpcroutes"}

	route := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "GRPCRoute",
		"metadata": map[string]any{
			"name":      "web",
			"namespace": "default",
		},
		"spec": map[string]any{
			"hostnames": []any{"grpcroute-test.example.com"},
		},
	}}

	if _, err := dynClient.Resource(gvr).Namespace("default").Create(ctx, route, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create(GRPCRoute) error = %v", err)
	}

	dc := newFakeDomainCache()

	if _, err := grpcroute.Init(ctx, dynClient, dc); err != nil {
		t.Fatalf("grpcroute.Init() error = %v", err)
	}

	domains := waitForDomains(t, dc, 1)

	if domains[0].Name != "example.com" || domains[0].Raw != "grpcroute-test.example.com" {
		t.Errorf("got Name=%q Raw=%q, want Name=%q Raw=%q",
			domains[0].Name, domains[0].Raw, "example.com", "grpcroute-test.example.com")
	}
}
