<p align="center"><img src="images/logo.png" alt="domain-harvester"></p>

# domain-harvester

[![Release](https://img.shields.io/github/release/shurshun/domain-harvester.svg)](https://github.com/shurshun/domain-harvester/releases/latest)
[![Go Report Card](https://goreportcard.com/badge/github.com/shurshun/domain-harvester)](https://goreportcard.com/report/github.com/shurshun/domain-harvester)
[![Powered By: GoReleaser](https://img.shields.io/badge/powered%20by-goreleaser-blue.svg)](https://github.com/goreleaser)

App collects domains from all Ingress resources in a Kubernetes cluster and provides its expiry information.

## Domain sources

* Kubernetes Ingress Resource
* Config file

## Metrics example
App provides 3 metrics per domain, 1 metric with the total number of requests to WHOIS/RDAP servers, and 2 metrics about the domain cache's own rebuild loop.

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

Domain expiry is looked up via RDAP first (RFC 7482/7483 — the HTTPS/JSON successor to WHOIS), falling back to raw WHOIS on port 43 for the shrinking set of TLDs without an RDAP bootstrap entry.

## Installation

* **via binary**

Just download and run binary for your platform https://github.com/shurshun/domain-harvester/releases

* **via docker**

```
docker run --rm -it -v ~/.kube/config:/root/.kube/config -p 8080:8080 ghcr.io/shurshun/domain-harvester:1.4.0
```

* **via helm**

The maintained chart lives in this repo, at `charts/domain-harvester`:

```
helm upgrade --install domain-harvester ./charts/domain-harvester
```

See `charts/domain-harvester/values.yaml` for the full set of options (RBAC scope, the optional config-file source, ServiceMonitor/PrometheusRule, a bundled Grafana dashboard ConfigMap). A plain-manifest kustomize example is also available under `deploy/kustomize/`.

The `.helm/values.yaml` file at the repo root is a **deprecated** values file for an older, externally hosted chart (`shurshun/go-app`); it's kept only for existing installs.

## Configuration options

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

## Example of the optional config file

```
projects:
  google:
    - google.com

```

## Support

For any additional information, please, contact me via telegram [@shursh](https://t.me/shursh)

