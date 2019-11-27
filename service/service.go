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
	"github.com/pkg/errors"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	yaml "gopkg.in/yaml.v2"
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
	log    *zap.SugaredLogger

	count   *stats.Int64Measure
	errors  *stats.Int64Measure
	latency *stats.Float64Measure
}

// New validates that the required system settings
// have been configured for the service to run.
func New(configFile string, l *zap.SugaredLogger) (*Service, error) {

	// i'm choosing not to populate the
	// credentials and interval within the
	//environment variables to allow for configuration
	// swapping without restarting the service.

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
	for {
		s.log.Info("Checking DNS Mappings")
		if err := s.execute(ctx); err != nil {
			s.log.Errorf("failed to run DNS check: %v", err)
		}

		time.Sleep(time.Second * s.config.Interval)
	}
}

func (s *Service) execute(ctx context.Context) (err error) {
	ctx, span := trace.StartSpan(ctx, "execution")
	// telemetry
	telemetry.MeasureInt(ctx, s.count, 1)
	start := time.Now()
	defer func() {
		span.End()
		telemetry.MeasureFloat(ctx, s.latency, sinceInMilliseconds(start))
		if err != nil {
			telemetry.MeasureInt(ctx, s.errors, 1)
		}
	}()

	var ip string
	ip, err = externalIP()
	if err != nil {
		return errors.Wrap(err, "failed to get local IP address")
	}

	for _, host := range s.config.Hosts {
		if err := s.updateHost(ctx, host, ip); err != nil {
			s.log.Errorf("failed to update host: %v", err)
		}
	}

	return nil
}

func (s *Service) updateHost(ctx context.Context, host Host, ip string) error {
	ctx, span := trace.StartSpan(ctx, "update")
	defer span.End()

	span.AddAttributes(trace.StringAttribute("hostname", host.Host))
	s.log.Infow("updating host", "host", host.Host, "ip", ip)

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

	resp, err := s.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to make request to Dynamic DNS Service")
	}

	span.AddAttributes(trace.Int64Attribute("statusCode", int64(resp.StatusCode)))
	if resp.StatusCode > 299 {
		return errors.Errorf("failed to query Dynamic DNS Service, received '%d'", resp.StatusCode)
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to decode result from Dynamic DNS Service")
	}

	switch {
	case strings.Contains(string(data), "good"):
		s.log.Infow("change was successfully applied", "host", host.Host, "ip", ip)
		span.AddAttributes(trace.StringAttribute("change", "good"))
	case strings.Contains(string(data), "nochg"):
		s.log.Infow("no change was recorded", "host", host.Host, "ip", ip)
		span.AddAttributes(trace.StringAttribute("change", "nochange"))
	default:
		return errors.Errorf("received error code from Dynamic DNS service: %s", data)
	}

	return nil
}

func sinceInMilliseconds(s time.Time) float64 {
	return float64(time.Since(s).Nanoseconds()) / 1e6
}
