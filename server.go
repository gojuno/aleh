package aleh

import (
	"context"
	"net/http"

	"junolab.net/aleh/collectors"
	"junolab.net/aleh/storages"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Config struct {
	DockerDaemonSocket string `json:"docker_daemon_socket"`
	Endpoint           string `json:"endpoint"`
	MetricPrefix       string `json:"metric_prefix"`
}

type Server struct {
	mux *http.ServeMux
}

func New(ctx context.Context, c Config) *Server {
	s := &Server{mux: http.NewServeMux()}

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

	s.mux.Handle("/metrics", promhttp.Handler())
	s.mux.HandleFunc("/internal", containerListener.HttpHandler())

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
