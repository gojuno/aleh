package collectors

import (
	"github.com/gojuno/aleh/storages"
	"github.com/prometheus/client_golang/prometheus"
)

type ContainerInfo struct {
	SkipRunning *bool `edn:"skip-running"`
}

// AliveCollector reports to prometheus known containers that is alive.
type AliveCollector struct {
	desc           *prometheus.Desc
	storage        *storages.InmemoryStorage
	staticServices map[string]map[string]ContainerInfo
}

func NewAliveCollector(metricPrefix string, l *storages.InmemoryStorage, services map[string]map[string]ContainerInfo) *AliveCollector {
	return &AliveCollector{
		staticServices: services,
		storage:        l,
		desc:           prometheus.NewDesc(metricPrefix+"container_running", "Container is alive", []string{"service", "container", "container_id", "revisions"}, nil),
	}
}

// Describe prometheus.Collector interface implementation
func (ac *AliveCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- ac.desc
}

// Collect prometheus.Collector interface implementation
func (ac *AliveCollector) Collect(ch chan<- prometheus.Metric) {
	for _, c := range ac.storage.AliveECSContainers() {
		ch <- prometheus.MustNewConstMetric(ac.desc, prometheus.CounterValue, 1.0, c.Service, c.Container, c.ID, c.Revisions)
	}
	for serviceName, v := range ac.staticServices {
		for containerName, info := range v {
			if info.SkipRunning == nil || !*info.SkipRunning {
				ch <- prometheus.MustNewConstMetric(ac.desc, prometheus.CounterValue, 0, serviceName, containerName, "", "")
			}

		}
	}
}
