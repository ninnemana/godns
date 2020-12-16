package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/uber/jaeger-client-go"

	"github.com/ninnemana/drudge/telemetry"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	tracelog "github.com/opentracing/opentracing-go/log"
	"github.com/pkg/errors"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"

	//"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Interval time.Duration `json:"interval" yaml:"interval"`
	Hosts    []Host        `json:"hosts" yaml:"hosts"`
}

type Host struct {
	Host     string `json:"host" yaml:"host"`
	User     string `json:"user" yaml:"user"`
	Password string `json:"password" yaml:"password"`
}

type Service struct {
	config Config
	client *http.Client
	log    *zap.Logger

	count   *stats.Int64Measure
	errors  *stats.Int64Measure
	latency *stats.Float64Measure
}

// New validates that the required system settings
// have been configured for the service to run.
func New(configFile string, l *zap.Logger) (*Service, error) {
	// parse the config file
	file, err := os.Open(configFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open config file")
	}

	var config Config
	if err := yaml.NewDecoder(file).Decode(&config); err != nil {
		return nil, errors.Wrap(err, "failed to read config file")
	}

	return &Service{
		count: telemetry.Int64Measure("operation_count", "Number of times the service is ran", "1", []tag.Key{
			telemetry.ServiceTag,
		}, view.Count()),
		errors: telemetry.Int64Measure("operation_errors", "Number of times the service encounters an error", "1", []tag.Key{
			telemetry.ServiceTag,
			telemetry.ErrorTag,
		}, view.Count()),
		latency: telemetry.Float64Measure("operation_latency", "Latency when the service is ran", "ms", []tag.Key{
			telemetry.ServiceTag,
			telemetry.LatencyTag,
		}, telemetry.LatencyDistribution),
		config: config,
		client: &http.Client{
			Timeout: time.Second * 5,
		},
		log: l,
	}, nil
}

func (s *Service) Run(ctx context.Context) error {
	s.log.Debug("running on interval", zap.Duration("interval", s.config.Interval))

	c := time.NewTicker(s.config.Interval)
	for {
		span, ctx := opentracing.StartSpanFromContext(ctx, "service.Run")
		log := s.log.With(
			zap.String("traceID", traceID(ctx)),
			zap.String("spanID", spanID(ctx)),
		)

		log.Info("Checking DNS Mappings")

		if err := s.execute(ctx); err != nil {
			ext.Error.Set(span, true)
			span.LogFields(tracelog.Error(err))
			log.Error("failed to run DNS check", zap.Error(err))
		}

		span.Finish()
		<-c.C
	}
}

func (s *Service) execute(ctx context.Context) (err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "service.execute")

	log := s.log.With(
		zap.String("traceID", traceID(ctx)),
		zap.String("spanID", spanID(ctx)),
	)

	start := time.Now()

	defer func() {
		span.Finish()

		telemetry.MeasureFloat(ctx, s.latency, sinceInMilliseconds(start))

		if err != nil {
			telemetry.MeasureInt(ctx, s.errors, 1)
		}
	}()

	var ip string

	ip, err = externalIP()
	if err != nil {
		err := fmt.Errorf("failed to get local IP address: %w", err)
		log.Error("failed to get local IP address", zap.Error(err))
		ext.Error.Set(span, true)
		span.LogFields(tracelog.Error(err))
		return err
	}

	for _, host := range s.config.Hosts {
		if err := s.updateHost(ctx, host, ip); err != nil {
			log.Error("failed to update host", zap.Error(err), zap.String("host", host.Host))
			ext.Error.Set(span, true)
			span.LogFields(tracelog.Error(err), tracelog.String("host", host.Host))
		}
	}

	return nil
}

func (s *Service) updateHost(ctx context.Context, host Host, ip string) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "service.update")
	defer span.Finish()

	log := s.log.With(
		zap.String("traceID", traceID(ctx)),
		zap.String("spanID", spanID(ctx)),
	)

	span.SetTag("hostname", host.Host)
	log.Info("updating host", zap.String("host", host.Host), zap.String("ip", ip))

	// set the request parameters
	q := url.Values{}
	q.Add("hostname", host.Host)
	q.Add("myip", ip)

	req, err := http.NewRequest(
		http.MethodGet,
		"https://domains.google.com/nic/update",
		nil,
	)
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}

	req.URL.RawQuery = q.Encode()

	auth := base64.StdEncoding.EncodeToString([]byte(
		fmt.Sprintf("%s:%s", host.User, host.Password),
	))
	req.Header.Add("Authorization", fmt.Sprintf("Basic %s", auth))
	req.Header.Add("User-Agent", "Mozilla/5.0 (compatible; MSIE 9.0; Windows NT 6.1; Trident/5.0)")

	resp, err := s.client.Do(req.WithContext(ctx))
	if err != nil {
		span.LogFields(tracelog.Error(err))
		log.Error("failed to make request to DNS service", zap.Error(err))
		return errors.Wrap(err, "failed to make request to Dynamic DNS Service")
	}

	span.SetTag("statusCode", resp.StatusCode)

	if resp.StatusCode > 299 {
		span.LogFields(tracelog.Error(err), tracelog.Int("statusCode", resp.StatusCode))
		log.Error("failed to make request to DNS service", zap.Error(err))
		return errors.Errorf("failed to query Dynamic DNS Service, received '%d'", resp.StatusCode)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		const msg = "failed to decode result from Dynamic DNS Service"
		span.LogFields(tracelog.Error(err))
		log.Error(msg, zap.Error(err))
		return errors.Wrap(err, msg)
	}

	switch {
	case strings.Contains(string(data), "good"):
		log.Info("change was successfully applied", zap.String("host", host.Host), zap.String("ip", ip))
		span.SetTag("change", "good")
	case strings.Contains(string(data), "nochg"):
		log.Info("no change was recorded", zap.String("host", host.Host), zap.String("ip", ip))
		span.SetTag("change", "nochange")
	default:
		err := fmt.Errorf("received error code from Dynamic DNS service: %s", data)
		span.LogFields(tracelog.Error(err))
		log.Error("received error from Dynamic DNS service", zap.Error(err))
		return err
	}

	return nil
}

func sinceInMilliseconds(s time.Time) float64 {
	return float64(time.Since(s).Nanoseconds()) / 1e6
}

func traceID(ctx context.Context) string {
	sp := opentracing.SpanFromContext(ctx)
	if sp == nil {
		return ""
	}

	sctx, ok := sp.Context().(jaeger.SpanContext)
	if !ok {
		return ""
	}

	return sctx.TraceID().String()
}

func spanID(ctx context.Context) string {
	sp := opentracing.SpanFromContext(ctx)
	if sp == nil {
		return ""
	}

	sctx, ok := sp.Context().(jaeger.SpanContext)
	if !ok {
		return ""
	}

	return sctx.SpanID().String()
}
