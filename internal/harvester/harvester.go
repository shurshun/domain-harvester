// Package harvester wires the WHOIS stack, the domain cache, every domain
// source and the metrics server together.
package harvester

import (
	"context"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
	"k8s.io/client-go/dynamic"

	"github.com/shurshun/domain-harvester/internal/cache"
	cluster_harvester "github.com/shurshun/domain-harvester/internal/harvester/modules/cluster"
	config_harvester "github.com/shurshun/domain-harvester/internal/harvester/modules/config"
	"github.com/shurshun/domain-harvester/internal/harvester/modules/grpcroute"
	"github.com/shurshun/domain-harvester/internal/harvester/modules/httproute"
	"github.com/shurshun/domain-harvester/internal/harvester/modules/traefik"
	"github.com/shurshun/domain-harvester/internal/harvester/types"
	"github.com/shurshun/domain-harvester/internal/metrics"
	"github.com/shurshun/domain-harvester/pkg/k8s"
	"github.com/shurshun/domain-harvester/pkg/whois"
	whois_cache "github.com/shurshun/domain-harvester/pkg/whois/cache"
)

// Run is the cli action. It blocks in the metrics server until ctx is cancelled.
func Run(ctx context.Context, cmd *cli.Command) error {
	setLogLevel(cmd.String("log-level"))
	setLogFormat(cmd.String("log-format"))

	whoisProvider, err := whois.Init(ctx, whois.Options{
		Timeout: cmd.Duration("whois-timeout"),
		Cache: whois_cache.Options{
			RefreshInterval:    cmd.Duration("whois-refresh-interval"),
			NearExpiryInterval: cmd.Duration("whois-near-expiry-interval"),
			ErrorRetryInterval: cmd.Duration("whois-error-retry-interval"),
		},
	})
	if err != nil {
		return err
	}

	domainCache, err := cache.Init(ctx, whoisProvider, cache.Options{
		Concurrency:     cmd.Int("whois-concurrency"),
		RebuildInterval: cmd.Duration("rebuild-interval"),
		SourcePriority:  parseSourcePriority(cmd.String("source-priority")),
	})
	if err != nil {
		return err
	}

	var harvesters []types.Harvester

	clusterHarvester, err := cluster_harvester.Init(ctx, cmd, domainCache)
	if err != nil {
		return err
	}

	harvesters = append(harvesters, clusterHarvester)

	// The config file is optional: its absence must not take the app down.
	configHarvester, err := config_harvester.Init(cmd, domainCache)
	if err != nil {
		log.Errorf("Can't load config file: %s", err.Error())
	} else {
		harvesters = append(harvesters, configHarvester)
	}

	optionalHarvesters, err := initOptionalSources(ctx, cmd, domainCache)
	if err != nil {
		return err
	}

	harvesters = append(harvesters, optionalHarvesters...)

	return metrics.Run(ctx, cmd, domainCache, harvesters)
}

// optionalSource is a CRD-backed domain source that's off by default and
// opt-in via its own flag; each shares the exact Init signature so they can
// be driven from one table instead of three near-identical if-blocks.
type optionalSource struct {
	flag  string
	label string
	init  func(context.Context, dynamic.Interface, types.DomainCache) (types.Harvester, error)
}

// initOptionalSources builds a dynamic client only if at least one optional
// source is enabled, and treats a missing CRD as non-fatal — same as a
// missing config file — logging and simply not adding that harvester.
func initOptionalSources(ctx context.Context, cmd *cli.Command, domainCache types.DomainCache) ([]types.Harvester, error) {
	sources := []optionalSource{
		{"enable-traefik-ingressroute", "Traefik IngressRoute", traefik.Init},
		{"enable-gateway-httproute", "Gateway API HTTPRoute", httproute.Init},
		{"enable-gateway-grpcroute", "Gateway API GRPCRoute", grpcroute.Init},
	}

	var (
		harvesters []types.Harvester
		dynClient  dynamic.Interface
	)

	for _, s := range sources {
		if !cmd.Bool(s.flag) {
			continue
		}

		if dynClient == nil {
			var err error

			dynClient, err = k8s.InitDynamic(cmd)
			if err != nil {
				return nil, err
			}
		}

		h, err := s.init(ctx, dynClient, domainCache)
		if err != nil {
			log.Errorf("Can't watch %s: %s", s.label, err.Error())
			continue
		}

		harvesters = append(harvesters, h)
	}

	return harvesters, nil
}

// parseSourcePriority splits a comma-separated flag value into the list
// internal/cache.Options.SourcePriority expects, trimming whitespace and
// dropping empty entries (so "" or a stray trailing comma falls back to the
// cache package's own default rather than producing an empty-string source).
func parseSourcePriority(raw string) []string {
	var priority []string

	for _, s := range strings.Split(raw, ",") {
		if s = strings.TrimSpace(s); s != "" {
			priority = append(priority, s)
		}
	}

	return priority
}

func setLogLevel(logLevel string) {
	ll, err := log.ParseLevel(logLevel)
	if err != nil {
		log.SetLevel(log.WarnLevel)
	} else {
		log.SetLevel(ll)
	}
}

// setLogFormat defaults to plain text; "json" switches to structured logs
// for log aggregators that parse JSON rather than logfmt-ish text.
func setLogFormat(logFormat string) {
	if logFormat == "json" {
		log.SetFormatter(&log.JSONFormatter{})
		return
	}

	log.SetFormatter(&log.TextFormatter{DisableTimestamp: true})
}
