package main

import (
	"domain-harvester/internal/harvester"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"os"
)

var (
	Version = "0.1.0"
	cliApp  = cli.NewApp()
)

func init() {
	cliApp.Version = Version
	cliApp.Name = "domain-harvester"
	cliApp.Usage = "Collect domains from all ingress resources in the cluster"

	cliApp.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "kubeconfig",
			Usage:  "Path to K8S config [optional]",
			EnvVar: "KUBECONFIG",
		},
		cli.StringFlag{
			Name:   "config, c",
			Value:  "config.yml",
			Usage:  "Path to config with domains [yaml]",
			EnvVar: "CONFIG",
		},
		cli.StringFlag{
			Name:   "log-level",
			Value:  "debug",
			Usage:  "info/error/debug",
			EnvVar: "LOG_LEVEL",
		},
		cli.StringFlag{
			Name:   "metrics-addr",
			Value:  ":8081",
			Usage:  "Metrics addr",
			EnvVar: "METRICS_ADDR",
		},
	}
}

func main() {
	cliApp.Action = harvester.Init
	err := cliApp.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
