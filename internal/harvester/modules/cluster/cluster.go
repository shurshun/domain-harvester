package cluster

import (
	"github.com/shurshun/domain-harvester/internal/harvester/helpers"
	"github.com/shurshun/domain-harvester/internal/harvester/types"
	"github.com/shurshun/domain-harvester/pkg/k8s"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
)

const source = "cluster"

type ClusterHarverster struct {
	ingressCache cache.Store
	domainCache  types.DomainCache
}

func Init(c *cli.Context, domainCache types.DomainCache) (types.Harvester, error) {
	harvester := &ClusterHarverster{domainCache: domainCache}

	k8sClient, err := k8s.Init(c)
	if err != nil {
		return harvester, err
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

	go iController.Run(wait.NeverStop)

	harvester.ingressCache = iStore

	return harvester, nil
}

func (ch *ClusterHarverster) ingressCreated(obj interface{}) {
	ingress := obj.(*networkingv1.Ingress)

	log.WithFields(log.Fields{
		"name":   ingress.ObjectMeta.Name,
		"action": "create",
	}).Debug("Found new ingress")

	ch.domainCache.Update(source, ch.getDomains())
}

func (ch *ClusterHarverster) ingressUpdated(oldObj, newObj interface{}) {
	ingress := newObj.(*networkingv1.Ingress)

	log.WithFields(log.Fields{
		"name":   ingress.ObjectMeta.Name,
		"action": "update",
	}).Debug("Ingress has been updated")

	ch.domainCache.Update(source, ch.getDomains())
}

func (ch *ClusterHarverster) ingressDeleted(obj interface{}) {
	ingress := obj.(*networkingv1.Ingress)

	log.WithFields(log.Fields{
		"name":   ingress.ObjectMeta.Name,
		"action": "delete",
	}).Debug("Ingress was deleted")

	ch.domainCache.Update(source, ch.getDomains())
}

func (ch *ClusterHarverster) getDomains() []*types.Domain {
	var result []*types.Domain

	for _, obj := range ch.ingressCache.List() {
		ingress := obj.(*networkingv1.Ingress)

		for _, rule := range ingress.Spec.Rules {
			result = append(result, &types.Domain{
				Name:        helpers.EffectiveTLDPlusOne(rule.Host),
				DisplayName: helpers.ToUnicode(helpers.EffectiveTLDPlusOne(rule.Host)),
				Raw:         rule.Host,
				Source:      source,
				Ingress:     ingress.ObjectMeta.Name,
				NS:          ingress.ObjectMeta.Namespace,
			})
		}
	}

	return result
}
