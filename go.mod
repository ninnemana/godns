module github.com/ninnemana/godns

go 1.15

require (
	github.com/ninnemana/drudge v0.0.0-20190528151411-cf451f7cdf4c
	github.com/opentracing/opentracing-go v1.2.0
	github.com/pkg/errors v0.8.1
	github.com/uber/jaeger-client-go v2.25.0+incompatible
	github.com/uber/jaeger-lib v2.4.0+incompatible
	go.opencensus.io v0.22.4
	go.opentelemetry.io/otel v0.14.0
	go.opentelemetry.io/otel/exporters/metric/prometheus v0.14.0
	go.opentelemetry.io/otel/exporters/trace/jaeger v0.14.0
	go.opentelemetry.io/otel/sdk v0.14.0
	go.uber.org/zap v1.12.0
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/yaml.v2 v2.2.5
)
