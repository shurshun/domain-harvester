package metrics

import (
	"domain-harvester/internal/harvester/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"net/http"
	"net/http/pprof"
)

func Init(c *cli.Context, domainCache types.DomainCache) error {
	r := http.NewServeMux()

	r.HandleFunc("/_liveness", livenessHandler)
	r.HandleFunc("/_readiness", readinessHandler)

	r.HandleFunc("/debug/pprof/", pprof.Index)
	r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.HandleFunc("/debug/pprof/trace", pprof.Trace)

	r.Handle("/metrics", promhttp.Handler())

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>Domain Harvester</title></head>
             <body>
             <h1>Domain Harvester</h1>
             <p><a href='/metrics'>Metrics</a></p>
             </body>
             </html>`))
	})

	prometheus.MustRegister(prometheus.NewBuildInfoCollector())
	prometheus.MustRegister(NewDomainExporter(domainCache))

	log.Infof("ready to handle requests at %s", c.String("metrics-addr"))

	return http.ListenAndServe(c.String("metrics-addr"), r)
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}

func livenessHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}
