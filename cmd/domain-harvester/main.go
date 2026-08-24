package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"

	"github.com/shurshun/domain-harvester/internal/harvester"
)

// Version is overwritten at build time via -ldflags "-X main.Version=...".
var Version = "0.1.0"

func main() {
	cmd := &cli.Command{
		Name:    "domain-harvester",
		Usage:   "Collect domains from all ingress resources in the cluster",
		Version: Version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "kubeconfig",
				Usage:   "Path to kubernetes config [optional]",
				Sources: cli.EnvVars("KUBECONFIG"),
			},
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Value:   "config.yml",
				Usage:   "Path to config with domains [yaml]",
				Sources: cli.EnvVars("CONFIG"),
			},
			&cli.StringFlag{
				Name:    "log-level",
				Value:   "debug",
				Usage:   "info/error/debug",
				Sources: cli.EnvVars("LOG_LEVEL"),
			},
			&cli.StringFlag{
				Name:    "log-format",
				Value:   "text",
				Usage:   "text/json",
				Sources: cli.EnvVars("LOG_FORMAT"),
			},
			&cli.StringFlag{
				Name:    "metrics-addr",
				Value:   ":8080",
				Usage:   "Metrics address",
				Sources: cli.EnvVars("METRICS_ADDR"),
			},
			&cli.IntFlag{
				Name:    "whois-concurrency",
				Value:   16,
				Usage:   "Max parallel WHOIS lookups per cache rebuild",
				Sources: cli.EnvVars("WHOIS_CONCURRENCY"),
			},
			&cli.DurationFlag{
				Name:    "whois-timeout",
				Value:   10 * time.Second,
				Usage:   "Timeout of a single WHOIS request",
				Sources: cli.EnvVars("WHOIS_TIMEOUT"),
			},
			&cli.DurationFlag{
				Name:    "whois-refresh-interval",
				Value:   60 * time.Minute,
				Usage:   "How often a healthy domain is re-queried",
				Sources: cli.EnvVars("WHOIS_REFRESH_INTERVAL"),
			},
			&cli.DurationFlag{
				Name:    "whois-near-expiry-interval",
				Value:   10 * time.Minute,
				Usage:   "How often a domain expiring within 30 days is re-queried",
				Sources: cli.EnvVars("WHOIS_NEAR_EXPIRY_INTERVAL"),
			},
			&cli.DurationFlag{
				Name:    "whois-error-retry-interval",
				Value:   15 * time.Minute,
				Usage:   "How often a failed lookup is retried",
				Sources: cli.EnvVars("WHOIS_ERROR_RETRY_INTERVAL"),
			},
			&cli.DurationFlag{
				Name:    "rebuild-interval",
				Value:   time.Minute,
				Usage:   "Unconditional domain cache rebuild interval",
				Sources: cli.EnvVars("REBUILD_INTERVAL"),
			},
			&cli.StringFlag{
				Name:    "source-priority",
				Value:   "cluster,config,ingressroute,httproute,grpcroute",
				Usage:   "Comma-separated source names, highest priority first, breaking ties when the same domain comes from more than one enabled source",
				Sources: cli.EnvVars("SOURCE_PRIORITY"),
			},
			&cli.BoolFlag{
				Name:    "enable-pprof",
				Value:   false,
				Usage:   "Expose net/http/pprof on the metrics listener",
				Sources: cli.EnvVars("ENABLE_PPROF"),
			},
			&cli.BoolFlag{
				Name:    "enable-traefik-ingressroute",
				Value:   false,
				Usage:   "Watch Traefik IngressRoute (traefik.io/v1alpha1) for domains; non-fatal if the CRD isn't installed",
				Sources: cli.EnvVars("ENABLE_TRAEFIK_INGRESSROUTE"),
			},
			&cli.BoolFlag{
				Name:    "enable-gateway-httproute",
				Value:   false,
				Usage:   "Watch Gateway API HTTPRoute (gateway.networking.k8s.io/v1) for domains; non-fatal if the CRD isn't installed",
				Sources: cli.EnvVars("ENABLE_GATEWAY_HTTPROUTE"),
			},
			&cli.BoolFlag{
				Name:    "enable-gateway-grpcroute",
				Value:   false,
				Usage:   "Watch Gateway API GRPCRoute (gateway.networking.k8s.io/v1) for domains; non-fatal if the CRD isn't installed",
				Sources: cli.EnvVars("ENABLE_GATEWAY_GRPCROUTE"),
			},
		},
		Action: harvester.Run,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	if err := cmd.Run(ctx, os.Args); err != nil {
		stop()
		log.Fatal(err)
	}

	stop()
}
