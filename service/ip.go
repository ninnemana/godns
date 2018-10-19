package service

import (
	"io/ioutil"
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
	resp, err := http.Get("http://ifconfig.me/ip")
	if err != nil {
		return "", err
	}

	if resp.StatusCode > 299 {
		return "", errors.Errorf("failed to make IP lookup, failed with status code '%d'", resp.StatusCode)
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read body")
	}

	return string(data), nil
}
