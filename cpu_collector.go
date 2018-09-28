package main

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type cpuStatCollector struct {
	listener *containerListener
	desc     *prometheus.Desc
}

func newCPUStatCollector(l *containerListener) *memStatCollector {
	return &memStatCollector{
		listener: l,
		desc:     prometheus.NewDesc("aleh_cgroup_cpu_stats", "Container cpu usage ", []string{"who", "service", "container", "container_id", "revisions"}, nil),
	}
}

// Describe prometheus.Collector interface implementation
func (cs *cpuStatCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- cs.desc
}

// Collect prometheus.Collector interface implementation
func (cs *cpuStatCollector) Collect(ch chan<- prometheus.Metric) {
	wg := sync.WaitGroup{}
	for _, c := range cs.listener.aliveContainers() {
		wg.Add(1)
		go func(c container) {
			defer wg.Done()
			loadMetric(c, c.CPUStatsPath, cs.desc, ch)
		}(c)
	}
	wg.Wait()
}

func loadMetric(c container, files []string, desc *prometheus.Desc, ch chan<- prometheus.Metric) {
	for _, filePath := range files {
		file, err := os.Open(filePath)
		if err != nil {
			log.Printf("ERROR: failed to open stats file %s for container %+v: %v", filePath, c, err)
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
			value, err := strconv.Atoi(statValue[1])
			if err != nil {
				log.Printf("ERROR: corrupted stat %q in file %q cant parse value: %v", metric, filePath, err)
				continue
			}
			ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(value), statValue[0], c.Service, c.Container, c.ID, c.Revisions)
		}
	}
}
