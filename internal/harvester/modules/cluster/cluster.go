// Package cluster harvests domains from Ingress resources via a client-go informer.
package cluster

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"

	"github.com/shurshun/domain-harvester/internal/harvester/helpers"
	"github.com/shurshun/domain-harvester/internal/harvester/types"
	"github.com/shurshun/domain-harvester/pkg/k8s"
)

const source = "cluster"

type ClusterHarverster struct {
	ingressCache cache.Store
	controller   cache.Controller
	domainCache  types.DomainCache
}

// Init starts an all-namespace Ingress informer that stops with ctx.
func Init(ctx context.Context, cmd *cli.Command, domainCache types.DomainCache) (types.Harvester, error) {
	harvester := &ClusterHarverster{domainCache: domainCache}

	k8sClient, err := k8s.Init(cmd)
	if err != nil {
		return nil, err
	}

	watchlist := cache.NewListWatchFromClient(k8sClient.NetworkingV1().RESTClient(), "ingresses", v1.NamespaceAll, fields.Everything())

	iStore, iController := cache.NewInformerWithOptions(
		cache.InformerOptions{
			ListerWatcher: watchlist,
			ObjectType:    &networkingv1.Ingress{},
			ResyncPeriod:  0,
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    harvester.ingressCreated,
				UpdateFunc: harvester.ingressUpdated,
				DeleteFunc: harvester.ingressDeleted,
			},
		},
	)

	harvester.ingressCache = iStore
	harvester.controller = iController

	go iController.Run(ctx.Done())

	// AddFunc only fires for Ingresses that exist, so a cluster/namespace
	// with none never calls domainCache.Update on its own — the domain
	// cache would then wait forever for a view that's never coming. Push
	// once, explicitly, right after the informer's initial list completes.
	go func() {
		if cache.WaitForCacheSync(ctx.Done(), iController.HasSynced) {
			harvester.domainCache.Update(source, harvester.getDomains())
		}
	}()

	return harvester, nil
}

func (ch *ClusterHarverster) Source() string {
	return source
}

// HasSynced reports whether the initial Ingress list has been processed.
func (ch *ClusterHarverster) HasSynced() bool {
	return ch.controller != nil && ch.controller.HasSynced()
}

func (ch *ClusterHarverster) ingressCreated(obj any) {
	ch.onChange(obj, "create", "Found new ingress")
}

func (ch *ClusterHarverster) ingressUpdated(_, newObj any) {
	ch.onChange(newObj, "update", "Ingress has been updated")
}

func (ch *ClusterHarverster) ingressDeleted(obj any) {
	// On a missed delete the informer hands over a tombstone rather than the
	// object itself; the full re-list below is correct either way.
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}

	ch.onChange(obj, "delete", "Ingress was deleted")
}

func (ch *ClusterHarverster) onChange(obj any, action, msg string) {
	if ingress, ok := obj.(*networkingv1.Ingress); ok {
		log.WithFields(log.Fields{
			"name":      ingress.Name,
			"namespace": ingress.Namespace,
			"action":    action,
		}).Debug(msg)
	}

	ch.domainCache.Update(source, ch.getDomains())
}

// getDomains re-lists the whole informer store: handlers push the full view
// rather than a delta, which keeps the domain cache a pure function of it.
func (ch *ClusterHarverster) getDomains() []*types.Domain {
	var result []*types.Domain

	for _, obj := range ch.ingressCache.List() {
		ingress, ok := obj.(*networkingv1.Ingress)
		if !ok {
			continue
		}

		for _, rule := range ingress.Spec.Rules {
			if rule.Host == "" {
				log.WithFields(log.Fields{
					"name":   ingress.Name,
					"action": "skip",
				}).Debug("Ingress rule has no host")

				continue
			}

			name := helpers.EffectiveTLDPlusOne(rule.Host)

			result = append(result, &types.Domain{
				Name:        name,
				DisplayName: helpers.ToUnicode(name),
				Raw:         rule.Host,
				Source:      source,
				Ingress:     ingress.Name,
				NS:          ingress.Namespace,
			})
		}
	}

	return result
}
