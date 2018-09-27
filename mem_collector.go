package main

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type memStatCollector struct {
	listener *containerListener
	desc     *prometheus.Desc
}

func newMemStatCollector(l *containerListener) *memStatCollector {
	return &memStatCollector{
		listener: l,
		desc:     prometheus.NewDesc("yauhen_cgroup_memory_stats", "Container memory statistic", []string{"stat", "service", "container", "container_id", "revisions"}, nil),
	}
}

// Describe prometheus.Collector interface implementation
func (ms *memStatCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- ms.desc
}

// Collect prometheus.Collector interface implementation
func (ms *memStatCollector) Collect(ch chan<- prometheus.Metric) {
	wg := sync.WaitGroup{}
	for _, c := range ms.listener.aliveContainers() {
		wg.Add(1)
		go func(c container) {
			defer wg.Done()
			loadMetric(c, ms.desc, ch)
		}(c)
	}
	wg.Wait()
}
