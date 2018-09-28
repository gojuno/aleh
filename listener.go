package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
)

const eventsPath = "/events"

type Event struct {
	Message string `json:"message"`
	Status  string `json:"status"`
	ID      string `json:"id"`
	Action  string `json:"action"`
	Type    string `json:"type"`
}

type containerListener struct {
	alive map[string]container
	mu    sync.RWMutex
	httpc http.Client
}

func newListener(socketPath string) *containerListener {
	return &containerListener{
		alive: map[string]container{},
		httpc: httpSocketClient(socketPath),
	}
}

func (m *containerListener) listenEvents() {
	dockerEventsPath := "http://localhost" + eventsPath
	req, err := http.NewRequest("GET", dockerEventsPath, nil)
	if err != nil {
		log.Printf("ERROR: failed to build http req for events stream%s: %v", dockerEventsPath, err)
	}

	resp, err := m.httpc.Do(req)
	if err != nil {
		log.Printf("ERROR: failed to do http req to %s: %v", dockerEventsPath, err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024), 1024)

	for scanner.Scan() {
		chunkBytes := bytes.TrimRight(scanner.Bytes(), "\r\n")

		if scanner.Err() != nil {
			break
		}

		e := Event{}
		if err := json.Unmarshal(chunkBytes, &e); err != nil {
			panic(err.Error())
		}
		go m.handleEvent(e)
	}
}

func (m *containerListener) httpHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cs := m.aliveContainers()
		body, err := json.Marshal(cs)
		if err != nil {
			log.Printf("failed to marshal alive containers %+v: %v", cs, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write(body)
	}
}

func (m *containerListener) handleEvent(event Event) {
	switch event.Status {
	case "start":
		m.loadContainer(event.ID)
	case "kill", "die", "stop":
		m.removeContainer(event.ID)
	}
}

type ContainerConfig struct {
	Labels map[string]string `json:"Labels"`
}

type Network struct {
	IPAddress string `json:"IPAddress"`
}

type NetworkSettings struct {
	// key is "bridge"
	Networks map[string]Network `json:"Networks"`
}

type HostConfig struct {
	CgroupParent string `json:"CgroupParent"`
}

type ContainerInfo struct {
	ID              string          `json:"Id"`
	Config          ContainerConfig `json:"config"`
	NetworkSettings NetworkSettings `json:"NetworkSettings"`
	HostConfig      HostConfig      `json:"HostConfig"`
}

func (m *containerListener) aliveContainers() map[string]container {
	m.mu.RLock()
	res := make(map[string]container, len(m.alive))
	for k, v := range m.alive {
		res[k] = v
	}
	m.mu.RUnlock()
	return res
}

func (m *containerListener) removeContainer(containerID string) {
	m.mu.Lock()
	delete(m.alive, containerID)
	m.mu.Unlock()
}

func (m *containerListener) loadContainer(containerID string) {
	resp, err := m.httpc.Get(containerUrl(containerID))
	if err != nil {
		panic(err.Error())
	}
	defer resp.Body.Close()

	respJson, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err.Error())
	}

	ci := ContainerInfo{}
	if err := json.Unmarshal(respJson, &ci); err != nil {
		panic(err.Error())
	}

	c := container{
		ID:        containerID,
		Container: ci.Config.Labels["com.amazonaws.ecs.container-name"],
		Service:   ci.Config.Labels["com.amazonaws.ecs.task-definition-family"],
		Address:   "172.17.42.1",
	}
	c.Ecs = c.Container != "" && c.Service != ""

	if bridge, ok := ci.NetworkSettings.Networks["bridge"]; ok && bridge.IPAddress != "" {
		c.Address = bridge.IPAddress
	}

	revisions := []string{}
	for label, revision := range ci.Config.Labels {
		if !strings.HasPrefix(label, "net.junolab.revision") {
			continue
		}

		parts := strings.Split(label, ".")
		revisionName := parts[len(parts)-1]
		revisions = append(revisions, revisionName+"="+revision)
	}
	if len(revisions) > 0 {
		c.Revisions = strings.Join(revisions, " ")
	}

	c.MemoryStatsPath = append(c.MemoryStatsPath, fmt.Sprintf("/mnt/cgroup/memory/docker/%s/memory.stat", c.ID))
	if ci.HostConfig.CgroupParent != "" {
		c.MemoryStatsPath = append(c.MemoryStatsPath,
			fmt.Sprintf("/mnt/cgroup/memory%s/%s/memory.stat", ci.HostConfig.CgroupParent, c.ID))
	}

	c.CPUStatsPath = append(c.CPUStatsPath, fmt.Sprintf("/mnt/cgroup/cpuacct/docker/%s/cpuacct.stat", c.ID))
	if ci.HostConfig.CgroupParent != "" {
		c.CPUStatsPath = append(c.CPUStatsPath,
			fmt.Sprintf("/mnt/cgroup/cpuacct%s/%s/cpuacct.stat", ci.HostConfig.CgroupParent, c.ID))
	}

	m.mu.Lock()
	m.alive[containerID] = c
	m.mu.Unlock()
}

func containerUrl(containerID string) string {
	return fmt.Sprintf("http://localhost/containers/%s/json", containerID)
}
