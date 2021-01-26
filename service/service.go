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

	"github.com/ninnemana/godns/log"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/label"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/unit"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v2"
)

const (
	serviceName = "godns"
	httpTimeout = time.Second * 5
)

var (
	tracer = otel.Tracer(serviceName)
	meter  = otel.GetMeterProvider().Meter(serviceName)
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
	log    *log.Contextual

	latency metric.Float64ValueRecorder
	count   metric.Int64Counter
	errors  metric.Int64Counter
}

// New validates that the required system settings
// have been configured for the service to run.
func New(configFile string, l *log.Contextual) (*Service, error) {
	// parse the config file
	file, err := os.Open(configFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open config file")
	}

	var config Config
	if err := yaml.NewDecoder(file).Decode(&config); err != nil {
		return nil, errors.Wrap(err, "failed to read config file")
	}

	latency, err := meter.NewFloat64ValueRecorder(
		"operation_latency",
		metric.WithDescription("Latency when the service is ran"),
		metric.WithUnit(unit.Milliseconds),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create operation latency metric: %w", err)
	}

	count, err := meter.NewInt64Counter(
		"operation_count",
		metric.WithDescription("Number of times the service is ran"),
		metric.WithUnit(unit.Dimensionless),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create operation count metric: %w", err)
	}

	errRecorder, err := meter.NewInt64Counter(
		"operation_count",
		metric.WithDescription("Number of times the service encounters an error"),
		metric.WithUnit(unit.Dimensionless),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create operation error count metric: %w", err)
	}

	return &Service{
		count:   count,
		errors:  errRecorder,
		latency: latency,
		config:  config,
		client: &http.Client{
			Transport:     http.DefaultClient.Transport,
			CheckRedirect: http.DefaultClient.CheckRedirect,
			Jar:           http.DefaultClient.Jar,
			Timeout:       httpTimeout,
		},
		log: l,
	}, nil
}

func (s *Service) Run(ctx context.Context) error {
	s.log.Debug("running on interval", zap.Duration("interval", s.config.Interval))

	c := time.NewTicker(s.config.Interval)

	for {
		ctx, span := tracer.Start(ctx, "service.Run")

		s.log.Info(ctx, "Checking DNS Mappings")

		if err := s.execute(ctx); err != nil {
			s.log.Error(ctx, "failed to run DNS check", zap.Error(err))
		}

		span.End()
		<-c.C
	}
}

func (s *Service) execute(ctx context.Context) (err error) {
	s.count.Add(ctx, 1)
	ctx, span := tracer.Start(ctx, "service.execute")

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
		s.log.Error(ctx, "failed to get local IP address", zap.Error(err))

		return err
	}

	grp, _ := errgroup.WithContext(ctx)
	for _, host := range s.config.Hosts {
		grp.Go(func() error {
			if err := s.updateHost(ctx, host, ip); err != nil {
				s.log.Error(
					ctx,
					"failed to update host",
					zap.Error(err),
					zap.String("host", host.Host),
				)
			}
			return err
		})
	}

	return grp.Wait()
}

func (s *Service) updateHost(ctx context.Context, host Host, ip string) error {
	ctx, span := tracer.Start(ctx, "service.update")
	defer span.End()

	s.log.Info(ctx, "updating host", zap.String("host", host.Host), zap.String("ip", ip))

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
		s.log.Error(ctx, "failed to create request", zap.Error(err))

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
		s.log.Error(ctx, "failed to make request to DNS service", zap.Error(err))

		return errors.Wrap(err, "failed to make request to Dynamic DNS Service")
	}

	span.SetAttributes(label.Int("statusCode", resp.StatusCode))

	if resp.StatusCode >= http.StatusMultipleChoices {
		s.log.Error(ctx, "failed to make request to DNS service", zap.Error(err))

		return errors.Errorf("failed to query Dynamic DNS Service, received '%d'", resp.StatusCode)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		const msg = "failed to decode result from Dynamic DNS Service"

		s.log.Error(ctx, msg, zap.Error(err))

		return errors.Wrap(err, msg)
	}

	switch {
	case strings.Contains(string(data), "good"):
		s.log.Info(ctx, "change was successfully applied", zap.String("host", host.Host), zap.String("ip", ip))
	case strings.Contains(string(data), "nochg"):
		s.log.Info(ctx, "no change was recorded", zap.String("host", host.Host), zap.String("ip", ip))
	default:
		s.log.Error(ctx, "received error from Dynamic DNS service", zap.Error(errors.Wrap(errDNSUpdate, string(data))))

		return errors.Wrap(errDNSUpdate, string(data))
	}

	return nil
}

var (
	errDNSUpdate = fmt.Errorf("received error code from Dynamic DNS service")
)

func sinceInMilliseconds(s time.Time) float64 {
	return float64(time.Since(s).Nanoseconds()) / 1e6
}
