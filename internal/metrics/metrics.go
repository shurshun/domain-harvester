// Package metrics serves the Prometheus endpoint, health probes and (opt-in)
// pprof. Run blocks and is the application's main loop.
package metrics

import (
	"context"
	"errors"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"

	"github.com/shurshun/domain-harvester/internal/harvester/types"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 60 * time.Second
	idleTimeout       = 60 * time.Second
	shutdownTimeout   = 10 * time.Second
)

// syncer is implemented by the domain cache; readiness waits for the first
// successful rebuild on top of every harvester being synced.
type syncer interface {
	HasSynced() bool
}

// Run starts the HTTP server and blocks until ctx is cancelled or it fails.
func Run(ctx context.Context, cmd *cli.Command, domainCache types.DomainCache, harvesters []types.Harvester) error {
	registry := prometheus.NewRegistry()

	registry.MustRegister(
		collectors.NewBuildInfoCollector(),
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		NewDomainExporter(domainCache),
	)

	syncers := make([]syncer, 0, len(harvesters)+1)
	for _, h := range harvesters {
		syncers = append(syncers, h)
	}

	if s, ok := domainCache.(syncer); ok {
		syncers = append(syncers, s)
	}

	r := http.NewServeMux()

	r.HandleFunc("/_liveness", okHandler)
	r.HandleFunc("/_readiness", readinessHandler(syncers))
	r.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{Registry: registry}))
	r.HandleFunc("/", indexHandler)

	// pprof is opt-in: the metrics port is usually reachable from anywhere in
	// the cluster, and the profiling handlers are a cheap DoS surface.
	if cmd.Bool("enable-pprof") {
		log.Warn("pprof endpoints are enabled on the metrics listener")

		r.HandleFunc("/debug/pprof/", pprof.Index)
		r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		r.HandleFunc("/debug/pprof/profile", pprof.Profile)
		r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		r.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}

	srv := &http.Server{
		Addr:              cmd.String("metrics-addr"),
		Handler:           r,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	errCh := make(chan error, 1)

	go func() {
		log.Infof("ready to handle requests at %s", srv.Addr)

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}

		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutting down...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		return srv.Shutdown(shutdownCtx)
	}
}

// readinessHandler reports 503 until every source has produced its first view,
// so that a rolling update never sends traffic to a pod with empty metrics.
func readinessHandler(syncers []syncer) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		for _, s := range syncers {
			if !s.HasSynced() {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte("syncing"))

				return
			}
		}

		_, _ = w.Write([]byte("OK"))
	}
}

func okHandler(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("OK"))
}

func indexHandler(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte(`<html>
<head><title>Domain Harvester</title></head>
<body>
<h1>Domain Harvester</h1>
<p><a href='/metrics'>Metrics</a></p>
</body>
</html>`))
}
