<p align="center"><img src="images/logo.png" alt="domain-harvester"></p>

<h1 align="center">domain-harvester</h1>

<p align="center">
  <a href="https://github.com/shurshun/domain-harvester/releases/latest"><img src="https://img.shields.io/github/release/shurshun/domain-harvester.svg" alt="Release"></a>
  <a href="https://github.com/shurshun/domain-harvester/actions/workflows/goreleaser.yml"><img src="https://github.com/shurshun/domain-harvester/actions/workflows/goreleaser.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/shurshun/domain-harvester/blob/master/go.mod"><img src="https://img.shields.io/github/go-mod/go-version/shurshun/domain-harvester" alt="Go version"></a>
  <a href="https://github.com/goreleaser"><img src="https://img.shields.io/badge/powered%20by-goreleaser-blue.svg" alt="Powered by GoReleaser"></a>
</p>

**domain-harvester** watches every Ingress in your Kubernetes cluster, works out which registrable domain each hostname belongs to, and looks up its expiry — so a Prometheus alert can tell you a domain is about to lapse before it actually does, instead of after.

## How it works

- **Discovery** — a Kubernetes informer watches Ingress resources across the cluster and extracts every hostname automatically. An optional static YAML file adds domains that aren't backed by an Ingress at all.
- **Lookup** — each hostname is reduced to its registrable domain (`shop.example.co.uk` → `example.co.uk`) and its expiry is resolved via [RDAP](https://en.wikipedia.org/wiki/Registration_Data_Access_Protocol) first, falling back to WHOIS on port 43 for the shrinking set of TLDs without an RDAP bootstrap entry. Lookups are cached and re-checked on a schedule that tightens automatically as a domain gets closer to expiring.
- **Export** — the result is exposed as Prometheus metrics, ready to scrape and alert on.

## Installation

### Helm (recommended)

The chart is published as an OCI artifact on every release:

```console
helm upgrade --install domain-harvester oci://ghcr.io/shurshun/charts/domain-harvester --version 2.0.0
```

or straight from a checkout of this repo:

```console
helm upgrade --install domain-harvester ./charts/domain-harvester
```

See [`charts/domain-harvester/values.yaml`](charts/domain-harvester/values.yaml) for the full set of options — RBAC scope, the optional config-file source, a `ServiceMonitor`/`PrometheusRule`, and a bundled Grafana dashboard `ConfigMap`. A plain-manifest [kustomize example](deploy/kustomize/) is also available for clusters that don't use Helm.

> The `.helm/values.yaml` file at the repo root is a **deprecated** values file for an older, externally hosted chart; it's kept only for existing installs and isn't updated for new flags or metrics.

### Docker

```console
docker run --rm -it -v ~/.kube/config:/root/.kube/config -p 8080:8080 ghcr.io/shurshun/domain-harvester:latest
```

### Binary

Download a prebuilt binary for your platform from the [releases page](https://github.com/shurshun/domain-harvester/releases).

## Configuration

Every option is a flag with a matching environment variable:

```
   --kubeconfig string                    Path to kubernetes config [optional] [$KUBECONFIG]
   --config string, -c string             Path to config with domains [yaml] (default: "config.yml") [$CONFIG]
   --log-level string                     info/error/debug (default: "debug") [$LOG_LEVEL]
   --log-format string                    text/json (default: "text") [$LOG_FORMAT]
   --metrics-addr string                  Metrics address (default: ":8080") [$METRICS_ADDR]
   --whois-concurrency int                Max parallel WHOIS lookups per cache rebuild (default: 16) [$WHOIS_CONCURRENCY]
   --whois-timeout duration               Timeout of a single WHOIS request (default: 10s) [$WHOIS_TIMEOUT]
   --whois-refresh-interval duration      How often a healthy domain is re-queried (default: 1h0m0s) [$WHOIS_REFRESH_INTERVAL]
   --whois-near-expiry-interval duration  How often a domain expiring within 30 days is re-queried (default: 10m0s) [$WHOIS_NEAR_EXPIRY_INTERVAL]
   --whois-error-retry-interval duration  How often a failed lookup is retried (default: 15m0s) [$WHOIS_ERROR_RETRY_INTERVAL]
   --rebuild-interval duration            Unconditional domain cache rebuild interval (default: 1m0s) [$REBUILD_INTERVAL]
   --enable-pprof                         Expose net/http/pprof on the metrics listener [$ENABLE_PPROF]
   --help, -h                             show help
   --version, -v                          print the version
```

### Optional config file

Domains that aren't backed by an Ingress can be added via `--config`:

```yaml
projects:
  google:
    - google.com
```

Each top-level key is just a label (exported as the `ingress`/`ingress_namespace` metric labels) grouping one or more domains.

## Metrics

Three metrics per domain, one counter for outbound WHOIS/RDAP requests, and two gauges describing the domain cache's own rebuild loop:

```
# HELP domain_expiry_days time in days until the domain expires
# TYPE domain_expiry_days gauge
domain_expiry_days{ingress="google",domain="google.com",ingress_namespace="google",fqdn="google.com",source="config"} 3014
domain_expiry_days{ingress="example",domain="example.com",ingress_namespace="default",fqdn="test.example.com",source="cluster"} 341
# HELP domain_last_updated last update of the domain
# TYPE domain_last_updated gauge
domain_last_updated{ingress="google",domain="google.com",ingress_namespace="google",fqdn="google.com",source="config"} 1.592078203e+09
domain_last_updated{ingress="example",domain="example.com"",ingress_namespace="default",fqdn="test.example.com",source="cluster"} 1.592078203e+09
# HELP domain_update_error error on domain update
# TYPE domain_update_error gauge
domain_update_error{ingress="google",domain="google.com",ingress_namespace="google",fqdn="google.com",source="config"} 0
domain_update_error{ingress="example",domain="example.com"",ingress_namespace="default",fqdn="test.example.com",source="cluster"} 0
# HELP domain_whois_requests requests to the whois servers
# TYPE domain_whois_requests counter
domain_whois_requests 2
# HELP domain_cache_last_rebuild_timestamp unix timestamp of the last completed domain cache rebuild
# TYPE domain_cache_last_rebuild_timestamp gauge
domain_cache_last_rebuild_timestamp 1.592078203e+09
# HELP domain_cache_rebuild_duration_seconds wall-clock time the last domain cache rebuild took
# TYPE domain_cache_rebuild_duration_seconds gauge
domain_cache_rebuild_duration_seconds 0.842
```

`domain_update_error` and `domain_cache_last_rebuild_timestamp` are what the chart's bundled `PrometheusRule` alerts on — see [`charts/domain-harvester/templates/prometheusrule.yaml`](charts/domain-harvester/templates/prometheusrule.yaml).

## Development

```console
go build ./...
go test -race ./...
golangci-lint run
go run ./cmd/domain-harvester --kubeconfig ~/.kube/config --log-level debug
```

Release artifacts (binaries, container images, SBOMs, signatures, and the Helm chart) are all built by [GoReleaser](https://goreleaser.com) on every `v*` tag — see [`.goreleaser.yaml`](.goreleaser.yaml) and [`.github/workflows/goreleaser.yml`](.github/workflows/goreleaser.yml).

## Support

For any additional information, please contact me via Telegram [@shursh](https://t.me/shursh).
