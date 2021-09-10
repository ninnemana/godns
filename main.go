package main

import (
	"context"
	"flag"
	"log"

	clog "github.com/ninnemana/godns/log"
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

	logConfig := zap.NewDevelopmentConfig()
	logConfig.Encoding = "json"

	logger, err := logConfig.Build()
	if err != nil {
		log.Fatalf("failed to create structured logger: %v", err)
	}

	defer func() {
		_ = logger.Sync()
	}()

	if promPort == nil {
		promPort = &defaultPort
	}

	flush, err := initTracer("godns")
	if err != nil {
		logger.Fatal("failed to start tracer", zap.Error(err))
	}
	defer func() {
		_ = flush(context.Background())
	}()

	logger.Info("starting metric collector", zap.String("service", "godns"), zap.String("port", *promPort))
	if err := initMeter("godns", *promPort); err != nil {
		logger.Fatal("failed to start metric meter", zap.Error(err))
	}

	svc, err := service.New(*configFile, &clog.Contextual{
		Logger: logger,
	})
	if err != nil {
		logger.Fatal("failed to create service", zap.Error(err))
	}

	logger.Info("Starting DNS Publisher")

	if err := svc.Run(context.Background()); err != nil {
		logger.Fatal("fell out", zap.Error(err))
	}
}
