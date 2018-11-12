package collectors

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gojuno/aleh/storages"

	"github.com/prometheus/client_golang/prometheus"
)

// CPUCollector reports to prometheus CPU usage of known alive containers. Data is grabbed from cgroups pseudo cpu stat file.
type CPUCollector struct {
	listener *storages.InmemoryStorage
	desc     *prometheus.Desc
}

func NewCPUCollector(metricPrefix string, l *storages.InmemoryStorage) *CPUCollector {
	return &CPUCollector{
		listener: l,
		desc:     prometheus.NewDesc(metricPrefix+"cgroup_cpu_stats", "Container cpu usage ", []string{"who", "service", "Container", "container_id", "revisions"}, nil),
	}
}

// Describe prometheus.Collector interface implementation
func (cs *CPUCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- cs.desc
}

// Collect prometheus.Collector interface implementation
func (cs *CPUCollector) Collect(ch chan<- prometheus.Metric) {
	wg := sync.WaitGroup{}
	for _, c := range cs.listener.AliveContainers() {
		wg.Add(1)
		go func(c storages.Container) {
			defer wg.Done()
			loadMetric(c, c.CPUStatsPath, cs.desc, ch)
		}(c)
	}
	wg.Wait()
}

func loadMetric(c storages.Container, files []string, desc *prometheus.Desc, ch chan<- prometheus.Metric) {
	for _, filePath := range files {
		file, err := os.Open(filePath)
		if err != nil {
			log.Printf("ERROR: failed to open stats file %s for Container %+v: %v", filePath, c, err)
			continue
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		scanner.Buffer([]byte{}, 1024)

		for scanner.Scan() {
			metric := scanner.Text()
			statValue := strings.Split(metric, " ")
			if len(statValue) < 2 {
				log.Printf("ERROR: corrupted stat %q in file %q", metric, filePath)
				continue
			}
			value, err := strconv.ParseInt(statValue[1], 10, 64)
			if err != nil {
				log.Printf("ERROR: corrupted stat %q in file %q cant parse value: %v", metric, filePath, err)
				continue
			}
			ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(value), statValue[0], c.Service, c.Container, c.ID, c.Revisions)
		}
	}
}
