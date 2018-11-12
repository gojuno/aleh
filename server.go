package aleh

import (
	"context"
	"net/http"

	"github.com/gojuno/aleh/collectors"
	"github.com/gojuno/aleh/storages"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Config struct {
	DockerDaemonSocket string `edn:"docker_daemon_socket"`
	Endpoint           string `edn:"endpoint"`
	MetricPrefix       string `edn:"metric_prefix"`
}

// Server implements net/http.Handler
// It registers all needed prometheus collectors
// and handles http GET /metrics for prometheus
// and http GET /internal for debug purposes
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
