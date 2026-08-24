// Package harvester wires the WHOIS stack, the domain cache, every domain
// source and the metrics server together.
package harvester

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"

	"github.com/shurshun/domain-harvester/internal/cache"
	cluster_harvester "github.com/shurshun/domain-harvester/internal/harvester/modules/cluster"
	config_harvester "github.com/shurshun/domain-harvester/internal/harvester/modules/config"
	"github.com/shurshun/domain-harvester/internal/harvester/types"
	"github.com/shurshun/domain-harvester/internal/metrics"
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

	return metrics.Run(ctx, cmd, domainCache, harvesters)
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
