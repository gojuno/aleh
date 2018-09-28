package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type config struct {
	DockerDaemonSocket string `json:"docker_daemon_socket"`
	Endpoint           string `json:"endpoint"`
}

type container struct {
	ID              string
	Ecs             bool
	Container       string
	Service         string
	Address         string
	Revisions       string
	MemoryStatsPath []string
	CPUStatsPath    []string
}

var configFile = flag.String("c", "config.json", "pass path to config file")

func main() {
	c := readConfig()

	listener := newListener(c.DockerDaemonSocket)
	go listener.listenEvents()

	// cpu
	cpuStatCollector := newCPUStatCollector(listener)
	prometheus.MustRegister(cpuStatCollector)

	// mem
	memStatCollector := newMemStatCollector(listener)
	prometheus.MustRegister(memStatCollector)

	// alive
	aliveCollector := newAliveCollector(listener)
	prometheus.MustRegister(aliveCollector)

	// docker space
	spaceCollector := newSpaceCollector(c.DockerDaemonSocket)
	prometheus.MustRegister(spaceCollector)

	router := http.NewServeMux()
	router.Handle("/metrics", promhttp.Handler())
	router.Handle("/internal", listener.httpHandler())

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
	return c
}
