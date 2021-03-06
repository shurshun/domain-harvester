package cluster

import (
	"domain-harvester/internal/harvester/helpers"
	"domain-harvester/internal/harvester/types"
	"domain-harvester/pkg/k8s"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
)

const source = "k8s"

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

	watchlist := cache.NewListWatchFromClient(k8sClient.ExtensionsV1beta1().RESTClient(), "ingresses", v1.NamespaceAll, fields.Everything())

	iStore, iController := cache.NewInformer(
		watchlist,
		&v1beta1.Ingress{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    harvester.ingressCreated,
			UpdateFunc: harvester.ingressUpdated,
			DeleteFunc: harvester.ingressDeleted,
		},
	)

	go iController.Run(wait.NeverStop)

	harvester.ingressCache = iStore

	return harvester, nil
}

func (ch *ClusterHarverster) ingressCreated(obj interface{}) {
	ingress := obj.(*v1beta1.Ingress)

	log.WithFields(log.Fields{
		"name":   ingress.ObjectMeta.Name,
		"action": "create",
	}).Debug("Found new ingress")

	ch.domainCache.Update(source, ch.getDomains())
}

func (ch *ClusterHarverster) ingressUpdated(oldObj, newObj interface{}) {
	ingress := newObj.(*v1beta1.Ingress)

	log.WithFields(log.Fields{
		"name":   ingress.ObjectMeta.Name,
		"action": "update",
	}).Debug("Ingress has been updated")

	ch.domainCache.Update(source, ch.getDomains())
}

func (ch *ClusterHarverster) ingressDeleted(obj interface{}) {
	ingress := obj.(*v1beta1.Ingress)

	log.WithFields(log.Fields{
		"name":   ingress.ObjectMeta.Name,
		"action": "delete",
	}).Debug("Ingress was deleted")

	ch.domainCache.Update(source, ch.getDomains())
}

func (ch *ClusterHarverster) getDomains() []*types.Domain {
	var result []*types.Domain

	for _, obj := range ch.ingressCache.List() {
		ingress := obj.(*v1beta1.Ingress)

		for _, rule := range ingress.Spec.Rules {
			result = append(result, &types.Domain{
				Name:    helpers.EffectiveTLDPlusOne(rule.Host),
				Raw:     rule.Host,
				Source:  source,
				Ingress: ingress.ObjectMeta.Name,
				NS:      ingress.ObjectMeta.Namespace,
			})
		}
	}

	return result
}
