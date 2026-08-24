package metrics

import (
	"github.com/shurshun/domain-harvester/internal/harvester/types"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace = "domain"
	subsystem = ""
)

// rebuildObserver is implemented by *cache.DomainCache but kept out of
// types.DomainCache so other implementations (tests, future caches) aren't
// forced to carry rebuild bookkeeping that's specific to this one's design.
type rebuildObserver interface {
	LastRebuildUnix() int64
	LastRebuildDurationSeconds() float64
}

// Exporter reads the domain cache on every scrape; nothing is precomputed.
type Exporter struct {
	domainCache types.DomainCache

	expiryDays      *prometheus.Desc
	lastUpdated     *prometheus.Desc
	updateError     *prometheus.Desc
	whoisRequests   *prometheus.Desc
	lastRebuild     *prometheus.Desc
	rebuildDuration *prometheus.Desc
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(e, ch)
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	domains := e.domainCache.GetDomains()

	for _, d := range domains {
		if d.WhoisData == nil {
			continue
		}

		ch <- prometheus.MustNewConstMetric(e.expiryDays, prometheus.GaugeValue, d.WhoisData.ExpiryDays, d.DisplayName, d.Raw, d.Source, d.Ingress, d.NS)
	}

	for _, d := range domains {
		if d.WhoisData == nil {
			continue
		}

		ch <- prometheus.MustNewConstMetric(e.lastUpdated, prometheus.GaugeValue, float64(d.WhoisData.LastUpdated.Unix()), d.DisplayName, d.Raw, d.Source, d.Ingress, d.NS)
	}

	for _, d := range domains {
		if d.WhoisData == nil {
			continue
		}

		var err float64

		if d.WhoisData.Error != "" {
			err = 1
		} else {
			err = 0
		}

		ch <- prometheus.MustNewConstMetric(e.updateError, prometheus.GaugeValue, err, d.DisplayName, d.Raw, d.Source, d.Ingress, d.NS)
	}

	ch <- prometheus.MustNewConstMetric(e.whoisRequests, prometheus.CounterValue, float64(e.domainCache.GetExternalRequestsCnt()))

	if ro, ok := e.domainCache.(rebuildObserver); ok {
		ch <- prometheus.MustNewConstMetric(e.lastRebuild, prometheus.GaugeValue, float64(ro.LastRebuildUnix()))
		ch <- prometheus.MustNewConstMetric(e.rebuildDuration, prometheus.GaugeValue, ro.LastRebuildDurationSeconds())
	}
}

func NewDomainExporter(domainCache types.DomainCache) *Exporter {
	e := &Exporter{
		domainCache: domainCache,
		expiryDays: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "expiry_days"),
			"time in days until the domain expires",
			[]string{"domain", "fqdn", "source", "ingress", "ingress_namespace"},
			nil,
		),
		lastUpdated: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "last_updated"),
			"last update of the domain",
			[]string{"domain", "fqdn", "source", "ingress", "ingress_namespace"},
			nil,
		),
		updateError: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "update_error"),
			"error on domain update",
			[]string{"domain", "fqdn", "source", "ingress", "ingress_namespace"},
			nil,
		),
		whoisRequests: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "whois_requests"),
			"requests to the whois servers",
			nil,
			nil,
		),
		lastRebuild: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "cache", "last_rebuild_timestamp"),
			"unix timestamp of the last completed domain cache rebuild",
			nil,
			nil,
		),
		rebuildDuration: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "cache", "rebuild_duration_seconds"),
			"wall-clock time the last domain cache rebuild took",
			nil,
			nil,
		),
	}

	return e
}
