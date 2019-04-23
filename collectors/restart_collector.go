package collectors

import (
	"sync"

	"github.com/gojuno/aleh/storages"
	"github.com/prometheus/client_golang/prometheus"
)

// RestartCollector reports to prometheus service start's amount.
type RestartCollector struct {
	mu       sync.Mutex
	desc     *prometheus.Desc
	storage  *storages.InmemoryStorage
	listener chan storages.Container
	services map[string]float64
}

func NewRestartCollector(metricPrefix string, l *storages.InmemoryStorage) *RestartCollector {
	rc := &RestartCollector{
		services: map[string]float64{},
		listener: make(chan storages.Container),
		storage:  l,
		desc:     prometheus.NewDesc(metricPrefix+"service_starts", "Amount of service starts", []string{"service"}, nil),
	}
	l.AddContainerListener(rc.listener)

	go rc.countServices()
	return rc
}

func (rc *RestartCollector) countServices() {
	for c := range rc.listener {
		rc.mu.Lock()
		rc.services[c.Service] = rc.services[c.Service] + 1
		rc.mu.Unlock()
	}
}

// Describe prometheus.Collector interface implementation
func (rc *RestartCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- rc.desc
}

// Collect prometheus.Collector interface implementation
func (rc *RestartCollector) Collect(ch chan<- prometheus.Metric) {
	rc.mu.Lock()
	for s, c := range rc.services {
		ch <- prometheus.MustNewConstMetric(rc.desc, prometheus.CounterValue, c, s)
	}
	rc.mu.Unlock()
}
