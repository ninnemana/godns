package service

import (
	"context"
	"io/ioutil"
	"net/http"
	"strings"

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

func externalIP(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://ifconfig.me/ip", nil)
	if err != nil {
		return "", errors.Wrap(err, "failed to create HTTP request")
	}

	req.Header.Add("User-Agent", "Mozilla/5.0 (compatible; MSIE 9.0; Windows NT 6.1; Trident/5.0)")

	cl := &http.Client{}

	resp, err := cl.Do(req)
	if err != nil {
		return "", errors.Errorf("failed to execute HTTP request")
	}

	if resp.StatusCode >= http.StatusMultipleChoices {
		return "", errors.Errorf("failed to make IP lookup, failed with status code '%d'", resp.StatusCode)
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read body")
	}

	return strings.TrimSpace(string(data)), nil
}
