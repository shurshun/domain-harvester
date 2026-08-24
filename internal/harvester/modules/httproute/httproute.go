// Package httproute harvests domains from Gateway API's HTTPRoute
// (gateway.networking.k8s.io/v1), for clusters using the Gateway API instead
// of, or alongside, plain Ingress.
package httproute

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/shurshun/domain-harvester/internal/harvester/modules/dynamicroute"
	"github.com/shurshun/domain-harvester/internal/harvester/types"
)

const source = "httproute"

var gvr = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}

// Init checks for the HTTPRoute CRD and, if present, starts an all-namespace
// watch that stops with ctx. Returns a plain error — not necessarily fatal to
// the caller — if the CRD isn't installed.
func Init(ctx context.Context, dynClient dynamic.Interface, domainCache types.DomainCache) (types.Harvester, error) {
	return dynamicroute.Init(ctx, dynClient, domainCache, gvr, source, extractHostnames)
}

func extractHostnames(obj *unstructured.Unstructured) []string {
	hostnames, _, err := unstructured.NestedStringSlice(obj.Object, "spec", "hostnames")
	if err != nil {
		return nil
	}

	return hostnames
}
