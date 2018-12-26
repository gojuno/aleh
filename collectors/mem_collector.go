package collectors

import (
	"sync"

	"github.com/gojuno/aleh/storages"

	"github.com/prometheus/client_golang/prometheus"
)

// MemCollector reports to prometheus memory usage of known alive containers. Data is grabbed from cgroups pseudo memory stat file.
type MemCollector struct {
	listener *storages.InmemoryStorage
	desc     *prometheus.Desc
}

func NewMemCollector(metricPrefix string, l *storages.InmemoryStorage) *MemCollector {
	return &MemCollector{
		listener: l,
		desc:     prometheus.NewDesc(metricPrefix+"cgroup_memory_stats", "Container memory statistic", []string{"stat", "service", "container", "container_id", "revisions"}, nil),
	}
}

// Describe prometheus.Collector interface implementation
func (ms *MemCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- ms.desc
}

// Collect prometheus.Collector interface implementation
func (ms *MemCollector) Collect(ch chan<- prometheus.Metric) {
	wg := sync.WaitGroup{}
	for _, c := range ms.listener.AliveECSContainers() {
		wg.Add(1)
		go func(c storages.Container) {
			defer wg.Done()
			loadMetric(c, c.MemoryStatsPath, ms.desc, ch)
		}(c)
	}
	wg.Wait()
}
