<p align="center"><img src="images/logo.png" alt="Domain-harvester Logo"></p>

# Domain-harvester

[![Release](https://img.shields.io/github/release/shurshun/domain-harvester.svg)](https://github.com/shurshun/domain-harvester/releases/latest)
![Docker Pulls](https://img.shields.io/docker/pulls/shurshun/domain-harvester)
[![Build](https://github.com/shurshun/domain-harvester/workflows/code_lint_build_repeat/badge.svg?tags)](https://github.com/shurshun/domain-harvester/actions?query=workflow%3Acode_lint_build_repeat)
[![Go Report Card](https://goreportcard.com/badge/github.com/shurshun/domain-harvester)](https://goreportcard.com/report/github.com/shurshun/domain-harvester)
[![Powered By: GoReleaser](https://img.shields.io/badge/powered%20by-goreleaser-blue.svg)](https://github.com/goreleaser)

App collects domains from all Ingress resources in a Kubernetes cluster and provides its expiry information.

## Domain sources

* Kubernetes Ingress Resource
* Config file

## Metrics example
App provides 3 metrics per domain and 1 metric with total number of the requests to the whois servers.

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
```

## Installation

* **via binary**

Just download and run binary for your platform https://github.com/shurshun/domain-harvester/releases

* **via docker**

```
docker run --rm -it -v ~/.kube/config:/root/.kube/config -p 8080:8080 shurshun/domain-harvester
```

* **via helm**

Application could be installed using my own Helm chart [go-app](https://github.com/shurshun/go-app-chart)

```
helm repo add shurshun https://shurshun.github.com/helm-charts
helm repo update
helm upgrade --install domain-harverster shurshun/go-app -f https://raw.githubusercontent.com/shurshun/domain-harvester/master/.helm/values.yaml
```

## Configuration options

```
   --kubeconfig value        Path to kubernetes config [optional] [$KUBECONFIG]
   --config value, -c value  Path to config with domains [yaml] (default: "config.yml") [$CONFIG]
   --log-level value         info/error/debug (default: "debug") [$LOG_LEVEL]
   --metrics-addr value      Metrics address (default: ":8080") [$METRICS_ADDR]
   --help, -h                show help
   --version, -v             print the version
```

## Example of the optional config file

```
projects:
  google:
    - google.com

```

## Support

For any additional information, please, contact me via telegram [@shursh](https://t.me/shursh)

