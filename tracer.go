package main

import (
	"fmt"
	"io"

	"github.com/opentracing/opentracing-go"

	"github.com/pkg/errors"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	jaegerprom "github.com/uber/jaeger-lib/metrics/prometheus"
)

// ErrInvalidConfiguration is an error to notify client to provide valid trace report agent or config server
var (
	ErrBlankTraceConfiguration = errors.New("no trace report agent, config server, or collector endpoint specified")
)

// initTracer creates a new trace provider instance and registers it as global trace provider.
func initTracer(serviceName string) (io.Closer, error) {
	cfg, err := jaegercfg.FromEnv()
	if err != nil {
		return nil, fmt.Errorf("could not load jaeger tracer configuration: %w", err)
	}

	if cfg.Sampler.SamplingServerURL == "" && cfg.Reporter.LocalAgentHostPort == "" && cfg.Reporter.CollectorEndpoint == "" {
		return nil, ErrBlankTraceConfiguration
	}

	metricsFactory := jaegerprom.New()

	cfg.ServiceName = serviceName
	cfg.Sampler = &jaegercfg.SamplerConfig{
		Type:  "const",
		Param: 1,
	}
	tracer, closer, err := cfg.NewTracer(jaegercfg.Metrics(metricsFactory))
	if err != nil {
		return nil, err
	}

	opentracing.SetGlobalTracer(tracer)

	return closer, nil
}

//
//func initMeter() (metric.Meter, error) {
//	exporter, err := prometheus.InstallNewPipeline(prometheus.Config{})
//	if err != nil {
//		return metric.NoopMeterProvider{}.Meter("godns"), fmt.Errorf("failed to initialize prometheus exporter: %w", err)
//	}
//
//	http.HandleFunc("/", exporter.ServeHTTP)
//	go func() {
//		_ = http.ListenAndServe(":2222", nil)
//	}()
//
//	return otel.GetMeterProvider().Meter("godns"), nil
//}
