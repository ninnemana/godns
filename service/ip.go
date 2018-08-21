package service

import (
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"
)

type NetworkResult struct {
	IP       string `json:"ip"`
	Type     string `json:"type"`
	Subtype  string `json:"subtype"`
	Via      string `json:"via"`
	Padding  string `json:"padding"`
	Asn      string `json:"asn"`
	Asnlist  string `json:"asnlist"`
	AsnName  string `json:"asn_name"`
	Country  string `json:"country"`
	Protocol string `json:"protocol"`
}

func externalIP() (string, error) {
	resp, err := http.Get("http://ipv6.lookup.test-ipv6.com/ip/")
	if err != nil {
		return "", err
	}

	if resp.StatusCode > 299 {
		return "", errors.Errorf("failed to make IP lookup, failed with status code '%d'", resp.StatusCode)
	}

	var res NetworkResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", errors.Wrap(err, "failed to decode result from IP lookups")
	}

	return res.IP, nil
}
