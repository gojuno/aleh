package collectors

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/gojuno/aleh/storages"
)

// AliveCollector reports to prometheus known containers that is alive.
type AliveCollector struct {
	desc     *prometheus.Desc
	listener *storages.InmemoryStorage
}

func NewAliveCollector(metricPrefix string, l *storages.InmemoryStorage) *AliveCollector {
	return &AliveCollector{
		listener: l,
		desc:     prometheus.NewDesc(metricPrefix+"container_running", "Container is alive", []string{"service", "container", "container_id", "revisions"}, nil),
	}
}

// Describe prometheus.Collector interface implementation
func (ac *AliveCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- ac.desc
}

// Collect prometheus.Collector interface implementation
func (ac *AliveCollector) Collect(ch chan<- prometheus.Metric) {
	for _, c := range ac.listener.AliveECSContainers() {
		ch <- prometheus.MustNewConstMetric(ac.desc, prometheus.CounterValue, 1.0, c.Service, c.Container, c.ID, c.Revisions)
	}
}
