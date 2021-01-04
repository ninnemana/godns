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

	"github.com/ninnemana/drudge/telemetry"
	"github.com/ninnemana/godns/log"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.uber.org/zap"
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
	log    *log.Contextual

	count   *stats.Int64Measure
	errors  *stats.Int64Measure
	latency *stats.Float64Measure
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
		span, ctx := opentracing.StartSpanFromContext(ctx, "service.Run")

		s.log.Info(ctx, "Checking DNS Mappings")

		if err := s.execute(ctx); err != nil {
			s.log.Error(ctx, "failed to run DNS check", zap.Error(err))
		}

		span.Finish()
		<-c.C
	}
}

func (s *Service) execute(ctx context.Context) (err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "service.execute")

	start := time.Now()

	defer func() {
		span.Finish()

		telemetry.MeasureFloat(ctx, s.latency, sinceInMilliseconds(start))

		if err != nil {
			telemetry.MeasureInt(ctx, s.errors, 1)
		}
	}()

	var ip string

	ip, err = externalIP(ctx)
	if err != nil {
		err := fmt.Errorf("failed to get local IP address: %w", err)
		s.log.Error(ctx, "failed to get local IP address", zap.Error(err))

		return err
	}

	for _, host := range s.config.Hosts {
		if err := s.updateHost(ctx, host, ip); err != nil {
			s.log.Error(
				ctx,
				"failed to update host",
				zap.Error(err),
				zap.String("host", host.Host),
			)
		}
	}

	return nil
}

func (s *Service) updateHost(ctx context.Context, host Host, ip string) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "service.update")
	defer span.Finish()

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

	span.SetTag("statusCode", resp.StatusCode)

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
