package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"junolab.net/aleh/collectors"
	"junolab.net/aleh/storages"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type config struct {
	DockerDaemonSocket string `json:"docker_daemon_socket"`
	Endpoint           string `json:"endpoint"`
	MetricPrefix       string `json:"metric_prefix"`
}

var configFile = flag.String("c", "config.json", "pass path to config file")

func main() {
	c := readConfig()
	log.Printf("starting with config %+v", c)

	ctx := context.Background()

	containerListener := storages.New(ctx, c.DockerDaemonSocket)

	// cpu
	cpuStatCollector := collectors.NewCPUCollector(c.MetricPrefix, containerListener)
	prometheus.MustRegister(cpuStatCollector)

	// mem
	memStatCollector := collectors.NewMemCollector(c.MetricPrefix, containerListener)
	prometheus.MustRegister(memStatCollector)

	// alive
	aliveCollector := collectors.NewAliveCollector(c.MetricPrefix, containerListener)
	prometheus.MustRegister(aliveCollector)

	// docker space
	spaceCollector := collectors.NewDockerSpaceCollector(c.MetricPrefix, c.DockerDaemonSocket)
	prometheus.MustRegister(spaceCollector)

	router := http.NewServeMux()
	router.Handle("/metrics", promhttp.Handler())
	router.Handle("/internal", containerListener.HttpHandler())

	if err := http.ListenAndServe(c.Endpoint, router); err != nil {
		log.Fatalf("metrics handler failed to connect to %s: %v", c.Endpoint, err)
	}
}

func readConfig() config {
	flag.Parse()
	cfg, err := os.Open(*configFile)
	if err != nil {
		log.Fatalf("failed to open config file %s: %v", *configFile, err)
	}
	defer cfg.Close()

	configData, err := ioutil.ReadAll(cfg)
	if err != nil {
		log.Fatalf("failed to read config file %s: %v", *configFile, err)
	}
	c := config{}
	if err := json.Unmarshal(configData, &c); err != nil {
		log.Fatalf("failed to unmarshal config %s file %s: %v", *configFile, string(configData), err)
	}

	if c.MetricPrefix == "" {
		c.MetricPrefix = "aleh_"
	}

	if c.DockerDaemonSocket == "" {
		c.DockerDaemonSocket = "/var/run/docker.sock"
	}

	if c.Endpoint == "" {
		c.Endpoint = "0.0.0.0:1234"
	}
	return c
}
