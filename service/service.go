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

	"go.opentelemetry.io/otel/trace"

	"github.com/ninnemana/tracelog"
	"github.com/pkg/errors"
	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/unit"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v2"
)

const (
	httpTimeout = time.Second * 5
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
	log    *tracelog.TraceLogger
	tracer trace.Tracer
	meter  metric.Meter

	latency  metric.Float64Histogram
	count    metric.Int64Counter
	errors   metric.Int64Counter
	endpoint string
}

type Option func(*Service) error

var ErrInvalidService = errors.New("invalid service definition")

// WithEndpoint defines the HTTP address to be called when the service
// is attempting to update the DNS configuration.
func WithEndpoint(addr string) Option {
	return func(svc *Service) error {
		if svc == nil {
			return ErrInvalidService
		}

		svc.endpoint = addr

		return nil
	}
}

func WithLogger(l *tracelog.TraceLogger) Option {
	return func(svc *Service) error {
		if svc == nil {
			return ErrInvalidService
		}

		svc.log = l

		return nil
	}
}

func WithTracer(tr trace.Tracer) Option {
	return func(svc *Service) error {
		if svc == nil {
			return ErrInvalidService
		}

		svc.tracer = tr

		return nil
	}
}

func WithMeter(meter metric.Meter) Option {
	return func(svc *Service) error {
		svc.meter = meter
		var err error

		svc.latency, err = svc.meter.NewFloat64Histogram(
			"operation_latency",
			metric.WithDescription("Latency when the service is ran"),
			metric.WithUnit(unit.Milliseconds),
		)
		if err != nil {
			return fmt.Errorf("failed to create operation latency metric: %w", err)
		}

		svc.count, err = svc.meter.NewInt64Counter(
			"operation_count",
			metric.WithDescription("Number of times the service is ran"),
			metric.WithUnit(unit.Dimensionless),
		)
		if err != nil {
			return fmt.Errorf("failed to create operation count metric: %w", err)
		}

		svc.errors, err = svc.meter.NewInt64Counter(
			"operation_count",
			metric.WithDescription("Number of times the service encounters an error"),
			metric.WithUnit(unit.Dimensionless),
		)
		if err != nil {
			return fmt.Errorf("failed to create operation error count metric: %w", err)
		}

		return nil
	}
}

func WithConfig(path string) Option {
	return func(svc *Service) error {
		// parse the config file
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open config file: %w", err)
		}

		if err := yaml.NewDecoder(file).Decode(&svc.config); err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}

		return nil
	}
}

// New validates that the required system settings
// have been configured for the service to run.
func New(options ...Option) (*Service, error) {
	svc := &Service{
		config:   Config{},
		endpoint: "https://domains.google.com/nic/update",
		tracer:   trace.NewNoopTracerProvider().Tracer("service"),
		meter:    metric.NoopMeterProvider{}.Meter("service"),
		client: &http.Client{
			Transport:     http.DefaultClient.Transport,
			CheckRedirect: http.DefaultClient.CheckRedirect,
			Jar:           http.DefaultClient.Jar,
			Timeout:       httpTimeout,
		},
	}

	for _, opt := range options {
		if err := opt(svc); err != nil {
			return nil, err
		}
	}

	return svc, nil
}

func (s *Service) Run(ctx context.Context) error {
	log := s.log.SetContext(ctx)

	log.Debug("running on interval", zap.Duration("interval", s.config.Interval))

	c := time.NewTicker(s.config.Interval)

	for {
		ctx, span := s.tracer.Start(ctx, "service.Run")
		log = log.SetContext(ctx)

		log.Info("Checking DNS Mappings")

		if err := s.execute(ctx); err != nil {
			log.Error("failed to run DNS check", zap.Error(err))
		}

		span.End()
		select {
		case <-c.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *Service) execute(ctx context.Context) (err error) {
	s.count.Add(ctx, 1)
	ctx, span := s.tracer.Start(ctx, "service.execute")

	log := s.log.SetContext(ctx)

	start := time.Now()

	defer func() {
		span.End()

		s.latency.Record(ctx, sinceInMilliseconds(start))

		if err != nil {
			s.errors.Add(ctx, 1)
		}
	}()

	var ip string

	ip, err = externalIP(ctx)
	if err != nil {
		err := fmt.Errorf("failed to get local IP address: %w", err)
		log.Error("failed to get local IP address", zap.Error(err))

		return err
	}

	grp, _ := errgroup.WithContext(ctx)

	for _, h := range s.config.Hosts {
		host := h

		grp.Go(func() error {
			if err := s.updateHost(ctx, host, ip); err != nil {
				log.Error(
					"failed to update host",
					zap.Error(err),
					zap.String("host", host.Host),
				)
			}

			return err
		})
	}

	if err := grp.Wait(); err != nil {
		return fmt.Errorf("host updates failed: %w", err)
	}

	return nil
}

func (s *Service) updateHost(ctx context.Context, host Host, ip string) error {
	ctx, span := s.tracer.Start(ctx, "service.update")
	defer span.End()

	log := s.log.SetContext(ctx)

	log.Info("updating host", zap.String("host", host.Host), zap.String("ip", ip))

	// set the request parameters
	q := url.Values{}
	q.Add("hostname", host.Host)
	q.Add("myip", ip)

	req, err := http.NewRequest(
		http.MethodGet,
		s.endpoint,
		nil,
	)
	if err != nil {
		log.Error("failed to create request", zap.Error(err))

		return errors.Wrap(err, "failed to create request")
	}

	req.URL.RawQuery = q.Encode()

	auth := base64.StdEncoding.EncodeToString([]byte(
		fmt.Sprintf("%s:%s", host.User, host.Password),
	))
	req.Header.Add("Authorization", fmt.Sprintf("Basic %s", auth))
	req.Header.Add("User-Agent", "Mozilla/5.0 (compatible; MSIE 9.0; Windows NT 6.1; Trident/5.0)")

	resp, err := s.client.Do(log.WithRequest(ctx, req))
	if err != nil {
		log.Error("failed to make request to DNS service", zap.Error(err))

		return errors.Wrap(err, "failed to make request to Dynamic DNS Service")
	}

	span.SetAttributes(otelhttptrace.HTTPStatus.Int(resp.StatusCode))

	if resp.StatusCode >= http.StatusMultipleChoices {
		log.Error("failed to make request to DNS service", zap.Error(err))

		return errors.Errorf("failed to query Dynamic DNS Service, received '%d'", resp.StatusCode)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		const msg = "failed to decode result from Dynamic DNS Service"

		log.Error(msg, zap.Error(err))

		return errors.Wrap(err, msg)
	}

	switch {
	case strings.Contains(string(data), "good"):
		log.Info("change was successfully applied", zap.String("host", host.Host), zap.String("ip", ip))
	case strings.Contains(string(data), "nochg"):
		log.Info("no change was recorded", zap.String("host", host.Host), zap.String("ip", ip))
	default:
		log.Error("received error from Dynamic DNS service", zap.Error(errors.Wrap(errDNSUpdate, string(data))))

		return errors.Wrap(errDNSUpdate, string(data))
	}

	return nil
}

var errDNSUpdate = fmt.Errorf("received error code from Dynamic DNS service")

func sinceInMilliseconds(s time.Time) float64 {
	return float64(time.Since(s).Nanoseconds()) / 1e6
}
