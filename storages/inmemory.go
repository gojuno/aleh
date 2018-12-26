package storages

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gojuno/aleh/httpclient"
	"github.com/pkg/errors"
)

type InmemoryStorage struct {
	alive map[string]Container
	mu    sync.RWMutex
	httpc http.Client
}

type containerID struct {
	ID string `json:"Id"`
}

type event struct {
	Message string `json:"message"`
	Status  string `json:"status"`
	ID      string `json:"id"`
	Action  string `json:"action"`
	Type    string `json:"type"`
}

func New(ctx context.Context, socketPath string) *InmemoryStorage {
	inmemoryStorage := &InmemoryStorage{
		alive: map[string]Container{},
		httpc: httpclient.SocketClient(socketPath),
	}

	go inmemoryStorage.listenEvents(ctx)
	go inmemoryStorage.loadContainers(ctx, socketPath)

	return inmemoryStorage
}

func (m *InmemoryStorage) loadContainers(ctx context.Context, socketPath string) {
	dockerContainersPath := "http://localhost" + "/containers/json"
	req, err := http.NewRequest("GET", dockerContainersPath, nil)
	if err != nil {
		log.Printf("ERROR: failed to build http req for containers list%s: %v", dockerContainersPath, err)
		return
	}

	req = req.WithContext(ctx)
	resp, err := m.httpc.Do(req)
	if err != nil {
		log.Printf("ERROR: failed to do http req to %s: %v", dockerContainersPath, err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("ERROR: failed to read containers/json resp: %v", err)
		return
	}

	ids := []containerID{}
	if err := json.Unmarshal(body, &ids); err != nil {
		log.Printf("ERROR: failed to unmarshall containers/json body %s: %v", string(body), err)
		return
	}

	for _, id := range ids {
		go func(id string) {
			m.loadContainer(ctx, id)
		}(id.ID)
	}
}

func (m *InmemoryStorage) listenEvents(ctx context.Context) {
	dockerEventsPath := "http://localhost" + "/events"
	req, err := http.NewRequest("GET", dockerEventsPath, nil)
	if err != nil {
		log.Printf("ERROR: failed to build http req for events stream%s: %v", dockerEventsPath, err)
	}

	req = req.WithContext(ctx)
	resp, err := m.httpc.Do(req)
	if err != nil {
		log.Printf("ERROR: failed to do http req to %s: %v", dockerEventsPath, err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024), 1024)

	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}

		chunkBytes := bytes.TrimRight(scanner.Bytes(), "\r\n")

		if scanner.Err() != nil {
			break
		}

		e := event{}
		if err := json.Unmarshal(chunkBytes, &e); err != nil {
			panic(err.Error())
		}
		go m.handleEvent(ctx, e)
	}
}

func (m *InmemoryStorage) HttpHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cs := m.AliveECSContainers()
		body, err := json.Marshal(cs)
		if err != nil {
			log.Printf("failed to marshal alive containers %+v: %v", cs, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write(body)
	}
}

func (m *InmemoryStorage) handleEvent(ctx context.Context, event event) {
	switch event.Status {
	case "start":
		m.loadContainer(ctx, event.ID)
	case "kill", "die", "stop":
		m.removeContainer(event.ID)
	}
}

type containerConfig struct {
	Labels map[string]string `json:"Labels"`
}

type network struct {
	IPAddress string `json:"IPAddress"`
}

type networkSettings struct {
	// key is "bridge"
	Networks map[string]network `json:"Networks"`
}

type hostConfig struct {
	CgroupParent string `json:"CgroupParent"`
}

type containerInfo struct {
	ID              string          `json:"Id"`
	Config          containerConfig `json:"config"`
	NetworkSettings networkSettings `json:"NetworkSettings"`
	HostConfig      hostConfig      `json:"HostConfig"`
}

func (m *InmemoryStorage) AliveECSContainers() map[string]Container {
	m.mu.RLock()
	res := make(map[string]Container, len(m.alive))
	for k, v := range m.alive {
		if v.Ecs {
			res[k] = v
		}
	}
	m.mu.RUnlock()
	return res
}

func (m *InmemoryStorage) removeContainer(containerID string) {
	m.mu.Lock()
	delete(m.alive, containerID)
	m.mu.Unlock()
}

func (m *InmemoryStorage) loadContainer(ctx context.Context, containerID string) {

	info, err := m.load(ctx, containerID)
	if err != nil {
		log.Printf("ERROR: failed to load container: %v", err.Error())
	}

	container := m.parse(containerID, info)

	m.mu.Lock()
	m.alive[containerID] = container
	m.mu.Unlock()
}

func (m *InmemoryStorage) load(ctx context.Context, containerID string) (info containerInfo, err error) {
	containerJSONPath := "http://localhost/containers/%s/json"
	req, err := http.NewRequest("GET", fmt.Sprintf(containerJSONPath, containerID), nil)
	if err != nil {
		return info, errors.Wrapf(err, "failed to build http req %s", containerJSONPath)
	}

	req = req.WithContext(ctx)
	resp, err := m.httpc.Do(req)
	if err != nil {
		panic(err.Error())
		return info, errors.Wrapf(err, "failed to get container %s json", containerID)
	}
	defer resp.Body.Close()

	respJson, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return info, errors.Wrapf(err, "failed to read container %s json resp body", containerID)
	}

	if err := json.Unmarshal(respJson, &info); err != nil {
		return info, errors.Wrapf(err, "failed to unmarshall container %s", containerID)
	}

	return info, nil
}

func (m *InmemoryStorage) parse(containerID string, ci containerInfo) Container {
	c := Container{
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
	if ci.HostConfig.CgroupParent != "" {
		c.CPUStatsPath = append(c.CPUStatsPath,
			fmt.Sprintf("/mnt/cgroup/cpuacct%s/%s/cpuacct.stat", ci.HostConfig.CgroupParent, c.ID))
	}
	return c
}
