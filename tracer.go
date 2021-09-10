package main

import (
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel/exporters/metric/prometheus"
	"go.opentelemetry.io/otel/exporters/trace/jaeger"
	"go.opentelemetry.io/otel/label"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// initTracer creates a new trace provider instance and registers it as global trace provider.
func initTracer(serviceName string) (func(), error) {
	flush, err := jaeger.InstallNewPipeline(
		jaeger.WithCollectorEndpoint("", jaeger.WithCollectorEndpointOptionFromEnv()),
		jaeger.WithProcess(jaeger.Process{
			ServiceName: "godns",
			Tags: []label.KeyValue{
				label.String("exporter", "jaeger"),
			},
		}),
		jaeger.WithSDK(&sdktrace.Config{
			DefaultSampler: sdktrace.AlwaysSample(),
		}),
	)
	if err != nil {
		return nil, err
	}

	return flush, nil
}

func initMeter() error {
	exporter, err := prometheus.InstallNewPipeline(prometheus.Config{})
	if err != nil {
		return fmt.Errorf("failed to initialize prometheus exporter: %w", err)
	}

	http.HandleFunc("/", exporter.ServeHTTP)
	go func() {
		_ = http.ListenAndServe(":"+*promPort, nil)
	}()

	return nil
}
