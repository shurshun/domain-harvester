// Package grpcroute harvests domains from Gateway API's GRPCRoute
// (gateway.networking.k8s.io/v1), the gRPC counterpart to HTTPRoute.
package grpcroute

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/shurshun/domain-harvester/internal/harvester/modules/dynamicroute"
	"github.com/shurshun/domain-harvester/internal/harvester/types"
)

const source = "grpcroute"

var gvr = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "grpcroutes"}

// Init checks for the GRPCRoute CRD and, if present, starts an all-namespace
// watch that stops with ctx. Returns a plain error — not necessarily fatal to
// the caller — if the CRD isn't installed. Note GRPCRoute only reached the v1
// API group in Gateway API v1.1; clusters running an older Gateway API
// release only have v1alpha2, which this GVR won't match either — same
// non-fatal path as a genuinely absent CRD.
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
