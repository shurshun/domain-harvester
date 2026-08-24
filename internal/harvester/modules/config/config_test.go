package config

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/urfave/cli/v3"

	"github.com/shurshun/domain-harvester/internal/harvester/types"
)

// fakeDomainCache records every Update call so tests can inspect what the
// harvester pushed, without a real internal/cache.DomainCache.
type fakeDomainCache struct {
	mu      sync.Mutex
	sources []string
	last    []*types.Domain
}

func (f *fakeDomainCache) GetDomains() []*types.Domain { return nil }

func (f *fakeDomainCache) Update(source string, domains []*types.Domain) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.sources = append(f.sources, source)
	f.last = domains
}

func (f *fakeDomainCache) GetExternalRequestsCnt() uint64 { return 0 }

// runInit drives Init the same way main.go does: through a parsed *cli.Command,
// so cmd.String("config") reflects a real --config flag value.
func runInit(t *testing.T, configPath string, domainCache types.DomainCache) (types.Harvester, error) {
	t.Helper()

	var (
		harvester types.Harvester
		initErr   error
	)

	cmd := &cli.Command{
		Flags: []cli.Flag{&cli.StringFlag{Name: "config"}},
		Action: func(_ context.Context, cmd *cli.Command) error {
			harvester, initErr = Init(cmd, domainCache)

			return nil
		},
	}

	args := []string{"domain-harvester"}
	if configPath != "" {
		args = append(args, "--config", configPath)
	}

	if err := cmd.Run(context.Background(), args); err != nil {
		t.Fatalf("cmd.Run() error = %v", err)
	}

	return harvester, initErr
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	return path
}

func TestInit_missingFileIsAnError(t *testing.T) {
	dc := &fakeDomainCache{}

	_, err := runInit(t, filepath.Join(t.TempDir(), "does-not-exist.yml"), dc)
	if err == nil {
		t.Fatal("Init() error = nil, want an error for a missing file")
	}

	dc.mu.Lock()
	defer dc.mu.Unlock()

	if len(dc.sources) != 0 {
		t.Error("domainCache.Update was called despite Init failing")
	}
}

func TestInit_emptyConfigFlagIsAnError(t *testing.T) {
	dc := &fakeDomainCache{}

	// No --config given and no default set on the flag: cmd.String("config")
	// is "", which os.Open correctly rejects rather than reading a directory
	// or some other surprise.
	if _, err := runInit(t, "", dc); err == nil {
		t.Fatal("Init() error = nil, want an error for an empty config path")
	}
}

func TestInit_invalidYAMLIsAnError(t *testing.T) {
	dc := &fakeDomainCache{}
	path := writeTempFile(t, "not: valid: yaml: [")

	if _, err := runInit(t, path, dc); err == nil {
		t.Fatal("Init() error = nil, want an error for invalid YAML")
	}
}

func TestInit_validConfigPushesDomainsOnce(t *testing.T) {
	dc := &fakeDomainCache{}
	path := writeTempFile(t, "projects:\n  google:\n    - google.com\n")

	harvester, err := runInit(t, path, dc)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if harvester == nil {
		t.Fatal("Init() returned a nil harvester alongside a nil error")
	}

	if !harvester.HasSynced() {
		t.Error("HasSynced() = false right after Init, want true (the read is synchronous)")
	}

	dc.mu.Lock()
	defer dc.mu.Unlock()

	if len(dc.sources) != 1 || dc.sources[0] != source {
		t.Fatalf("Update called with sources %v, want a single call with %q", dc.sources, source)
	}

	if len(dc.last) != 1 || dc.last[0].Raw != "google.com" {
		t.Errorf("pushed domains = %+v, want a single google.com entry", dc.last)
	}
}

func TestGetDomains(t *testing.T) {
	ch := &ConfigHarverster{
		config: Config{
			Projects: map[string][]string{
				"google": {"google.com", "www.google.com"},
				"shop":   {"xn--80ak6aa92e.com"},
			},
		},
	}

	domains := ch.getDomains()

	if len(domains) != 3 {
		t.Fatalf("getDomains() returned %d domains, want 3", len(domains))
	}

	byRaw := make(map[string]*types.Domain, len(domains))
	for _, d := range domains {
		byRaw[d.Raw] = d
	}

	www, ok := byRaw["www.google.com"]
	if !ok {
		t.Fatal("www.google.com missing from result")
	}

	if www.Name != "google.com" {
		t.Errorf("Name = %q, want eTLD+1 %q", www.Name, "google.com")
	}

	if www.Source != source || www.Ingress != "google" || www.NS != "google" {
		t.Errorf("Source/Ingress/NS = %q/%q/%q, want %q/%q/%q", www.Source, www.Ingress, www.NS, source, "google", "google")
	}

	intl, ok := byRaw["xn--80ak6aa92e.com"]
	if !ok {
		t.Fatal("punycode entry missing from result")
	}

	if intl.DisplayName == intl.Name {
		t.Errorf("DisplayName %q should be the IDN-decoded form, not equal to Name %q", intl.DisplayName, intl.Name)
	}
}

func TestSource(t *testing.T) {
	ch := &ConfigHarverster{}
	if got := ch.Source(); got != "config" {
		t.Errorf("Source() = %q, want %q", got, "config")
	}
}
