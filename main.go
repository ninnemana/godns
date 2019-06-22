package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ninnemana/drudge/telemetry"
	"github.com/ninnemana/godns/service"
)

var (
	configFile = flag.String("config", "", "declares the path of the config file")
	promPort   = flag.String("metric-port", "9090", "indicates the port for Prometheus metrics to be served")

	defaultPort = "9090"
)

func main() {
	flag.Parse()
	if configFile == nil || *configFile == "" {
		log.Fatal("invalid config file")
	}

	if promPort == nil {
		promPort = &defaultPort
	}

	go func() {
		if err := telemetry.StartPrometheus(fmt.Sprintf(":%s", *promPort)); err != nil {
			log.Fatalf("failed to start prometheus exporter: %v", err)
		}
	}()

	flush, err := telemetry.StackDriver(os.Getenv("GCE_PROJECT_ID"), "godns", os.Getenv("GCE_SERVICE_ACCOUNT"))
	if err != nil {
		log.Fatalf("failed to start StackDriver exporter: %v", err)
	}
	defer flush()

	svc, err := service.New(*configFile)
	if err != nil {
		log.Fatalf("failed to create service: %v", err)
	}

	if err := svc.Run(context.Background()); err != nil {
		log.Fatalf("fell out: %v", err)
	}
}
