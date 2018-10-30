package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
)

type Config struct {
	Interval time.Duration `json:"interval"`
	Hosts    []Host        `json:"hosts"`
}

type Host struct {
	Host     string `json:"host"`
	User     string `json:"user"`
	Password string `json:"password"`
}

type Service struct {
	config Config
}

var (
	envInterval = "GDNS_INTERVAL"
)

// New validates that the required system settings
// have been configured for the service to run.
func New(configFile string) (*Service, error) {

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
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return nil, errors.Wrap(err, "failed to read config file")
	}

	return &Service{
		config: config,
	}, nil
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

		for _, host := range s.config.Hosts {
			fmt.Println(host.Host, host.User, host.Password)

			// set the request parameters
			q := url.Values{}
			q.Add("hostname", host.Host)
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
				fmt.Sprintf("%s:%s", host.User, host.Password),
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

		time.Sleep(time.Second * s.config.Interval)
	}
}
