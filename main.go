package main

import (
	"flag"
	"log"

	"github.com/ninnemana/godns/service"
)

var (
	configFile = flag.String("config", "", "declares the path of the config file")
)

func main() {
	flag.Parse()
	if configFile == nil || *configFile == "" {
		log.Fatalf("invalid config file")
	}

	svc, err := service.New(*configFile)
	if err != nil {
		log.Fatalf("failed to create service: %v", err)
	}

	if err := svc.Run(); err != nil {
		log.Fatalf("fell out: %v", err)
	}
}
