// Package dynamicroute is the shared engine behind every optional,
// CRD-backed domain source (Traefik IngressRoute, Gateway API HTTPRoute,
// GRPCRoute, ...). Each of those is a thin wrapper supplying a GVR, a source
// name, and a function that pulls hostnames out of the resource's shape —
// this package handles watching it via the dynamic client (no generated
// typed client needed) and, critically, treating an absent CRD as
// "nothing to harvest" rather than a fatal error.
package dynamicroute

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"

	"github.com/shurshun/domain-harvester/internal/harvester/helpers"
	"github.com/shurshun/domain-harvester/internal/harvester/types"
)

// ExtractFunc pulls every hostname a resource routes for out of it. A nil or
// empty return is fine — the object just contributes no domains.
type ExtractFunc func(obj *unstructured.Unstructured) []string

type Harvester struct {
	source      string
	extract     ExtractFunc
	store       cache.Store
	controller  cache.Controller
	domainCache types.DomainCache
}

// Init checks that gvr is actually served by the cluster (a missing CRD
// surfaces as a 404 on this very check, and is returned as a plain error —
// callers are expected to treat that as non-fatal, same as a missing config
// file), then starts an all-namespace informer that stops with ctx.
func Init(
	ctx context.Context,
	dynClient dynamic.Interface,
	domainCache types.DomainCache,
	gvr schema.GroupVersionResource,
	source string,
	extract ExtractFunc,
) (types.Harvester, error) {
	resource := dynClient.Resource(gvr)

	if _, err := resource.List(ctx, metav1.ListOptions{Limit: 1}); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("%s (%s) is not installed in this cluster", source, gvr.String())
		}

		return nil, fmt.Errorf("checking for %s (%s): %w", source, gvr.String(), err)
	}

	harvester := &Harvester{source: source, extract: extract, domainCache: domainCache}

	watchlist := &cache.ListWatch{
		ListWithContextFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
			return resource.List(ctx, options)
		},
		WatchFuncWithContext: func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
			return resource.Watch(ctx, options)
		},
	}

	store, controller := cache.NewInformerWithOptions(
		cache.InformerOptions{
			// dynClient (not resource: the marker method lives on the
			// top-level client) lets the reflector detect a client that
			// doesn't support the newer streaming-list watch semantics —
			// true for the fake dynamic client used in tests, so this
			// isn't just a production nicety, it's what keeps those tests
			// from hanging on a bookmark event the fake client never sends.
			ListerWatcher: cache.ToListWatcherWithWatchListSemantics(watchlist, dynClient),
			ObjectType:    &unstructured.Unstructured{},
			ResyncPeriod:  0,
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    harvester.onChange,
				UpdateFunc: func(_, newObj any) { harvester.onChange(newObj) },
				DeleteFunc: harvester.onChange,
			},
		},
	)

	harvester.store = store
	harvester.controller = controller

	go controller.Run(ctx.Done())

	// AddFunc only fires for objects that exist, so a cluster/namespace with
	// none never calls domainCache.Update on its own — push once, explicitly,
	// right after the informer's initial list completes.
	go func() {
		if cache.WaitForCacheSync(ctx.Done(), controller.HasSynced) {
			harvester.domainCache.Update(harvester.source, harvester.getDomains())
		}
	}()

	return harvester, nil
}

func (h *Harvester) Source() string {
	return h.source
}

// HasSynced reports whether the initial list has been processed.
func (h *Harvester) HasSynced() bool {
	return h.controller != nil && h.controller.HasSynced()
}

func (h *Harvester) onChange(obj any) {
	// A delete hands over a tombstone rather than the object itself on a
	// missed event; irrelevant here since the full re-list below doesn't
	// look at obj's content at all.
	if u, ok := obj.(*unstructured.Unstructured); ok {
		log.WithFields(log.Fields{
			"source":    h.source,
			"name":      u.GetName(),
			"namespace": u.GetNamespace(),
		}).Debug("Found a change")
	}

	h.domainCache.Update(h.source, h.getDomains())
}

// getDomains re-lists the whole informer store: handlers push the full view
// rather than a delta, which keeps the domain cache a pure function of it.
func (h *Harvester) getDomains() []*types.Domain {
	var result []*types.Domain

	for _, obj := range h.store.List() {
		u, ok := obj.(*unstructured.Unstructured)
		if !ok {
			continue
		}

		for _, host := range h.extract(u) {
			if host == "" {
				continue
			}

			name := helpers.EffectiveTLDPlusOne(host)

			result = append(result, &types.Domain{
				Name:        name,
				DisplayName: helpers.ToUnicode(name),
				Raw:         host,
				Source:      h.source,
				Ingress:     u.GetName(),
				NS:          u.GetNamespace(),
			})
		}
	}

	return result
}
