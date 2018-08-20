package main

import (
	"log"

	"github.com/ninnemana/godns/service"
)

func main() {
	svc, err := service.New()
	if err != nil {
		log.Fatalf("failed to create service: %v", err)
	}

	if err := svc.Run(); err != nil {
		log.Fatalf("fell out: %v", err)
	}
}
