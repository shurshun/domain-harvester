package harvester

import (
	"github.com/shurshun/domain-harvester/internal/cache"
	cluster_harvester "github.com/shurshun/domain-harvester/internal/harvester/modules/cluster"
	config_harvester "github.com/shurshun/domain-harvester/internal/harvester/modules/config"
	"github.com/shurshun/domain-harvester/internal/metrics"
	"github.com/shurshun/domain-harvester/pkg/whois"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func Init(c *cli.Context) error {
	setLogLevel(c.String("log-level"))

	whoisProvider, err := whois.Init()
	if err != nil {
		return err
	}

	domainCache, err := cache.Init(whoisProvider)
	if err != nil {
		return err
	}

	_, err = cluster_harvester.Init(c, domainCache)
	if err != nil {
		return err
	}

	_, err = config_harvester.Init(c, domainCache)
	if err != nil {
		log.Errorf("Can't load config file: %s", err.Error())
	}

	return metrics.Init(c, domainCache)
}

func setLogLevel(logLevel string) {
	ll, err := log.ParseLevel(logLevel)

	if err != nil {
		log.SetLevel(log.WarnLevel)
	} else {
		log.SetLevel(ll)
	}

	log.SetFormatter(&log.TextFormatter{DisableTimestamp: true})
}
