// Package traefik harvests domains from Traefik's IngressRoute CRD
// (traefik.io/v1alpha1), for clusters that route with Traefik instead of, or
// alongside, plain Ingress.
package traefik

import (
	"context"
	"regexp"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/shurshun/domain-harvester/internal/harvester/modules/dynamicroute"
	"github.com/shurshun/domain-harvester/internal/harvester/types"
)

const source = "ingressroute"

var gvr = schema.GroupVersionResource{Group: "traefik.io", Version: "v1alpha1", Resource: "ingressroutes"}

// hostMatcherRe finds every Host(...) matcher call; backtickRe then pulls the
// backtick-quoted hostnames out of each one. Traefik's matcher rule syntax
// (e.g. `Host(`a.com`,`b.com`) && PathPrefix(`/api`)`) is not otherwise
// parsed — domains are extracted regardless of the surrounding && / ||
// structure, since any Host() mention is a domain worth tracking.
var (
	hostMatcherRe = regexp.MustCompile(`Host\(([^)]*)\)`)
	backtickRe    = regexp.MustCompile("`([^`]*)`")
)

// Init checks for the IngressRoute CRD and, if present, starts an
// all-namespace watch that stops with ctx. Returns a plain error — not
// necessarily fatal to the caller — if the CRD isn't installed.
func Init(ctx context.Context, dynClient dynamic.Interface, domainCache types.DomainCache) (types.Harvester, error) {
	return dynamicroute.Init(ctx, dynClient, domainCache, gvr, source, extractHosts)
}

func extractHosts(obj *unstructured.Unstructured) []string {
	routes, _, err := unstructured.NestedSlice(obj.Object, "spec", "routes")
	if err != nil {
		return nil
	}

	var hosts []string

	for _, route := range routes {
		routeMap, ok := route.(map[string]any)
		if !ok {
			continue
		}

		match, _, err := unstructured.NestedString(routeMap, "match")
		if err != nil {
			continue
		}

		hosts = append(hosts, parseHostMatchers(match)...)
	}

	return hosts
}

func parseHostMatchers(match string) []string {
	var hosts []string

	for _, call := range hostMatcherRe.FindAllStringSubmatch(match, -1) {
		for _, quoted := range backtickRe.FindAllStringSubmatch(call[1], -1) {
			hosts = append(hosts, quoted[1])
		}
	}

	return hosts
}
