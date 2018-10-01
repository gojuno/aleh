package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"junolab.net/aleh"
)

var configFile = flag.String("c", "Config.json", "pass path to Config file")

func main() {
	c := readConfig()
	log.Printf("starting with Config %+v", c)

	ctx := context.Background()

	httpServer := &http.Server{
		Addr:    c.Endpoint,
		Handler: aleh.New(ctx, c),
	}

	go func() {
		log.Printf("Listening on %s\n", c.Endpoint)
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	graceful(ctx, httpServer)
}

func graceful(ctx context.Context, httpServer *http.Server) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		log.Println("Server stopped")
	}
}

func readConfig() aleh.Config {
	flag.Parse()
	cfg, err := os.Open(*configFile)
	if err != nil {
		log.Fatalf("failed to open Config file %s: %v", *configFile, err)
	}
	defer cfg.Close()

	configData, err := ioutil.ReadAll(cfg)
	if err != nil {
		log.Fatalf("failed to read Config file %s: %v", *configFile, err)
	}
	c := aleh.Config{}
	if err := json.Unmarshal(configData, &c); err != nil {
		log.Fatalf("failed to unmarshal Config %s file %s: %v", *configFile, string(configData), err)
	}

	if c.MetricPrefix == "" {
		c.MetricPrefix = "aleh_"
	}

	if c.DockerDaemonSocket == "" {
		c.DockerDaemonSocket = "/var/run/docker.sock"
	}

	if c.Endpoint == "" {
		c.Endpoint = "0.0.0.0:1234"
	}
	return c
}
