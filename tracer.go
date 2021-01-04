package main

import (
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/metric/prometheus"
	"go.opentelemetry.io/otel/exporters/trace/jaeger"
	"go.opentelemetry.io/otel/label"
	"go.opentelemetry.io/otel/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// ErrInvalidConfiguration is an error to notify client to provide valid trace report agent or config server
var (
	ErrBlankTraceConfiguration = errors.New("no trace report agent, config server, or collector endpoint specified")
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

func initMeter() (metric.Meter, error) {
	exporter, err := prometheus.InstallNewPipeline(prometheus.Config{})
	if err != nil {
		return metric.NoopMeterProvider{}.Meter("godns"), fmt.Errorf("failed to initialize prometheus exporter: %w", err)
	}

	http.HandleFunc("/", exporter.ServeHTTP)
	go func() {
		_ = http.ListenAndServe(":2222", nil)
	}()

	return otel.GetMeterProvider().Meter("godns"), nil
}
