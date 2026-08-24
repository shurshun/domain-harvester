// Package config harvests domains from an optional YAML file. Unlike the
// cluster source it is a static, one-shot read: a missing file is not fatal.
package config

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v2"

	"github.com/shurshun/domain-harvester/internal/harvester/helpers"
	"github.com/shurshun/domain-harvester/internal/harvester/types"
)

const source = "config"

type Config struct {
	Projects map[string][]string `yaml:"projects"`
}

type ConfigHarverster struct {
	config      Config
	domainCache types.DomainCache
}

// Init reads the config file once and pushes its domains into the cache.
func Init(cmd *cli.Command, domainCache types.DomainCache) (types.Harvester, error) {
	harvester := &ConfigHarverster{domainCache: domainCache}

	configPath := cmd.String("config")

	f, err := os.Open(configPath) //nolint:gosec // path is operator-supplied by design
	if err != nil {
		return nil, fmt.Errorf("cannot open %q: %w", configPath, err)
	}

	defer func() { _ = f.Close() }()

	if err := yaml.NewDecoder(f).Decode(&harvester.config); err != nil {
		return nil, fmt.Errorf("cannot parse %q: %w", configPath, err)
	}

	harvester.domainCache.Update(source, harvester.getDomains())

	return harvester, nil
}

func (ch *ConfigHarverster) Source() string {
	return source
}

// HasSynced is always true: Init returns only after the file has been read.
func (ch *ConfigHarverster) HasSynced() bool {
	return true
}

func (ch *ConfigHarverster) getDomains() []*types.Domain {
	var result []*types.Domain

	for project, domains := range ch.config.Projects {
		for _, domain := range domains {
			name := helpers.EffectiveTLDPlusOne(domain)

			result = append(result, &types.Domain{
				Name:        name,
				DisplayName: helpers.ToUnicode(name),
				Raw:         domain,
				Source:      source,
				Ingress:     project,
				NS:          project,
			})
		}
	}

	return result
}
