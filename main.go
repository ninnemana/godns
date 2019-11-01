package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/ninnemana/drudge/telemetry"
	"github.com/ninnemana/godns/service"
	"go.uber.org/zap"
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

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to create structured logger: %v", err)
	}

	defer logger.Sync()
	l := logger.Sugar()

	if promPort == nil {
		promPort = &defaultPort
	}

	go func() {
		l.Infow("Starting Prometheus", zap.String("port", *promPort))
		if err := telemetry.StartPrometheus(fmt.Sprintf(":%s", *promPort)); err != nil {
			l.Fatalw("failed to start prometheus exporter", zap.Error(err))
		}
	}()

	//flush, err := telemetry.StackDriver(os.Getenv("GCE_PROJECT_ID"), "godns", os.Getenv("GCE_SERVICE_ACCOUNT"))
	//if err != nil {
	//	l.Fatalw("failed to start StackDriver exporter", zap.Error(err))
	//}
	//defer flush()

	svc, err := service.New(*configFile)
	if err != nil {
		l.Fatalw("failed to create service", zap.Error(err))
	}

	l.Info("Starting DNS Publisher")
	if err := svc.Run(context.Background()); err != nil {
		l.Fatalw("fell out", zap.Error(err))
	}
}
