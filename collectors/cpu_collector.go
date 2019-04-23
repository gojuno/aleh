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

const (
	nanosecondsInSecond = 1000000000
	// The value comes from `C.sysconf(C._SC_CLK_TCK)`, and
	// on Linux it's a constant which is safe to be hard coded,
	// so we can avoid using cgo here.
	clockTicks    = 100
	cpuMultiplier = clockTicks / nanosecondsInSecond
)

// CPUCollector reports to prometheus CPU usage of known alive containers. Data is grabbed from cgroups pseudo cpu stat file.
type CPUCollector struct {
	storage *storages.InmemoryStorage
	desc    *prometheus.Desc
}

func NewCPUCollector(metricPrefix string, l *storages.InmemoryStorage) *CPUCollector {
	return &CPUCollector{
		storage: l,
		desc:    prometheus.NewDesc(metricPrefix+"cgroup_cpu_stats", "Container cpu usage percent", []string{"who", "service", "container", "container_id", "revisions"}, nil),
	}
}

// Describe prometheus.Collector interface implementation
func (cs *CPUCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- cs.desc
}

// Collect prometheus.Collector interface implementation
func (cs *CPUCollector) Collect(ch chan<- prometheus.Metric) {
	wg := sync.WaitGroup{}
	for _, c := range cs.storage.AliveECSContainers() {
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
			value, err := strconv.ParseUint(statValue[1], 10, 64)
			if err != nil {
				log.Printf("ERROR: corrupted stat %q in file %q cant parse value: %v", metric, filePath, err)
				continue
			}
			ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(value), statValue[0], c.Service, c.Container, c.ID, c.Revisions)
		}
	}
}
