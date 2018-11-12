package collectors

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strconv"

	"github.com/gojuno/aleh/httpclient"

	"github.com/prometheus/client_golang/prometheus"
)

const infoPath = "/info"

type dockerInfo struct {
	DriverStatus [][]string `json:"DriverStatus"`
}

// DockerSpaceCollector reports to prometheus current docker disk space usage.
type DockerSpaceCollector struct {
	httpc http.Client
	descs map[string]*prometheus.Desc
}

func NewDockerSpaceCollector(metricPrefix, socketPath string) *DockerSpaceCollector {
	return &DockerSpaceCollector{
		httpc: httpclient.SocketClient(socketPath),
		descs: map[string]*prometheus.Desc{
			"Data Space Available":         prometheus.NewDesc(metricPrefix+"docker_data_space_available", "Data Space Available", nil, nil),
			"Metadata Space Available":     prometheus.NewDesc(metricPrefix+"docker_metadata_space_available", "Metadata Space Available", nil, nil),
			"Thin Pool Minimum Free Space": prometheus.NewDesc(metricPrefix+"docker_thin_pool_minimum_free_space", "Thin Pool Minimum Free Space", nil, nil),
			"Data Space Used":              prometheus.NewDesc(metricPrefix+"docker_data_space_used", "Data Space Used", nil, nil),
			"Metadata Space Used":          prometheus.NewDesc(metricPrefix+"docker_metadata_space_used", "Metadata Space Used", nil, nil),
			"Data Space Total":             prometheus.NewDesc(metricPrefix+"docker_data_space_total", "Data Space Total", nil, nil),
			"Metadata Space Total":         prometheus.NewDesc(metricPrefix+"docker_metadata_space_total", "Metadata Space Total", nil, nil),
		},
	}
}

// Describe prometheus.Collector interface implementation
func (s *DockerSpaceCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range s.descs {
		ch <- desc
	}
}

// Collect prometheus.Collector interface implementation
func (s *DockerSpaceCollector) Collect(ch chan<- prometheus.Metric) {
	dockerInfoPath := "http://localhost" + infoPath
	resp, err := s.httpc.Get(dockerInfoPath)
	if err != nil {
		log.Printf("ERROR: failed to do http req to %s: %v", dockerInfoPath, err)
	}
	defer resp.Body.Close()

	bodyJson, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("ERROR: failed to read body from docker info request: %v", err)
		return
	}
	di := dockerInfo{}
	if err := json.Unmarshal(bodyJson, &di); err != nil {
		log.Printf("ERROR: failed to unmarshall body `%s` from docker info request: %v", bodyJson, err)
		return
	}

	for _, info := range di.DriverStatus {
		if len(info) < 2 {
			continue
		}

		desc, ok := s.descs[info[0]]
		if !ok {
			continue
		}
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(humanReadableToBytes(info[len(info)-1])))
	}
}

const kb = 1024

var bytesMap = map[string]int64{
	"kB": kb,
	"MB": kb * kb,
	"GB": kb * kb * kb,
	"TB": kb * kb * kb * kb,
}

var sizeRegexp = regexp.MustCompile(`([\d.]+)([kMGT]B)$`)

func humanReadableToBytes(size string) int64 {
	m := sizeRegexp.FindStringSubmatch(size)
	if len(m) != 3 {
		log.Printf("ERROR: failed to parse size %s", size)
	}
	number, suffix := m[1], m[2]

	rawSize, err := strconv.ParseFloat(number, 64)
	if err != nil {
		log.Printf("ERROR: failed to convert %s to int from raw %s: %v", number, size, err)
	}
	multiplier := int64(1000)
	if m := bytesMap[suffix]; m != 0 {
		multiplier = m
	}
	return int64(rawSize * float64(multiplier))
}
