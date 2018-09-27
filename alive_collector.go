package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

type aliveCollector struct {
	desc     *prometheus.Desc
	listener *containerListener
}

func newAliveCollector(l *containerListener) *aliveCollector {
	return &aliveCollector{
		listener: l,
		desc:     prometheus.NewDesc("yauhen_container_running", "Container is alive", []string{"service", "container", "container_id", "revisions"}, nil),
	}
}

// Describe prometheus.Collector interface implementation
func (ac *aliveCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- ac.desc
}

// Collect prometheus.Collector interface implementation
func (ac *aliveCollector) Collect(ch chan<- prometheus.Metric) {
	for _, c := range ac.listener.aliveContainers() {
		ch <- prometheus.MustNewConstMetric(ac.desc, prometheus.CounterValue, 1.0, c.Service, c.Container, c.ID, c.Revisions)
	}
}
