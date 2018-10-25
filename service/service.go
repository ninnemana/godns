package service

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

type Service struct{}

var (
	envUser     = "GDNS_USER"
	envPass     = "GDNS_PASSWD"
	envInterval = "GDNS_INTERVAL"
	envHostname = "GDNS_HOSTS"
)

// New validates that the required system settings
// have been configured for the service to run.
func New() (*Service, error) {

	if os.Getenv(envUser) == "" {
		return nil, errors.Errorf("expected a username for the servie, received '%s' using environment variable '%s'", os.Getenv(envUser), envUser)
	}

	if os.Getenv(envPass) == "" {
		return nil, errors.Errorf("expected a password for the service using environment variable '%s'", envPass)
	}

	if os.Getenv(envHostname) == "" {
		return nil, errors.Errorf("expected a pass for the service using environment variable '%s'", envHostname)
	}

	if i, _ := strconv.Atoi(os.Getenv(envInterval)); i == 0 {
		return nil, errors.Errorf("expected interval setting, received '%s' using environment variable '%s'", os.Getenv(envInterval), envInterval)
	}

	// i'm choosing not to populate the
	// credentials and interval within the
	//environment variables to allow for configuration
	// swapping without restarting the service.

	return &Service{}, nil
}

type Ping struct {
	Hostname string `json:"hostname"`
	IP       string `json:"myip"`
	Offline  string `json:"offline,omitempty"` // yes|no
}

func (s *Service) Run() error {
	client := &http.Client{
		Timeout: time.Second * 5,
	}

	for {
		ip, err := externalIP()
		if err != nil {
			return errors.Wrap(err, "failed to get local IP address")
		}

		for _, h := range strings.Split(envHostname, ",") {
			// set the request parameters
			q := url.Values{}
			q.Add("hostname", h)
			q.Add("myip", ip)

			req, err := http.NewRequest(
				http.MethodPost,
				"https://domains.google.com/nic/update",
				nil,
			)
			if err != nil {
				return errors.Wrap(err, "failed to create request")
			}

			req.URL.RawQuery = q.Encode()

			auth := base64.StdEncoding.EncodeToString([]byte(
				fmt.Sprintf("%s:%s", os.Getenv(envUser), os.Getenv(envPass)),
			))
			req.Header.Add("Authorization", fmt.Sprintf("Basic %s", auth))

			resp, err := client.Do(req)
			if err != nil {
				return errors.Wrap(err, "failed to make request to Dynamic DNS Service")
			}

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
			case strings.Contains(string(data), "nochg"):
			default:
				return errors.Errorf("received error code from Dynamic DNS service: %s", data)
			}
		}

		interval, err := strconv.Atoi(os.Getenv(envInterval))
		switch {
		case err != nil:
			return errors.Wrap(err, "failed to check interval setting")
		case interval == 0:
			interval = 60
			// return errors.New("interval must be greater than zero")
		}

		time.Sleep(time.Second * time.Duration(interval))
	}
}
