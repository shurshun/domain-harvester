package config

import (
	"domain-harvester/internal/harvester/helpers"
	"domain-harvester/internal/harvester/types"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
	"os"
	// log "github.com/sirupsen/logrus"
)

const source = "config"

type Config struct {
	Projects map[string][]string `yaml:"projects"`
}

type ConfigHarverster struct {
	// domains     []types.Domain
	config      Config
	domainCache types.DomainCache
}

func Init(c *cli.Context, domainCache types.DomainCache) (types.Harvester, error) {
	harvester := &ConfigHarverster{domainCache: domainCache}

	configPath := c.String("config")

	_, err := os.Stat(configPath)
	if err != nil {
		return harvester, err
	}

	f, err := os.Open(configPath)
	if err != nil {
		return harvester, err
	}
	defer f.Close()

	harvester.config = Config{}

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&harvester.config)
	if err != nil {
		return harvester, err
	}

	harvester.domainCache.Update(source, harvester.getDomains())

	return harvester, nil
}

func (ch *ConfigHarverster) getDomains() []*types.Domain {
	var result []*types.Domain

	for project, domains := range ch.config.Projects {
		for _, domain := range domains {
			result = append(result, &types.Domain{
				Name:    helpers.EffectiveTLDPlusOne(domain),
				Raw:     domain,
				Source:  source,
				Ingress: project,
				NS:      project,
			})
		}
	}

	return result
}
