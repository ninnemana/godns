package service

import (
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"
)

type IPResult struct {
	IP string `json:"ip"`
}

func externalIP() (string, error) {
	resp, err := http.Get("https://api.ipify.org?format=json")
	if err != nil {
		return "", err
	}

	if resp.StatusCode > 299 {
		return "", errors.Errorf("failed to make IP lookup, failed with status code '%d'", resp.StatusCode)
	}

	var res IPResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", errors.Wrap(err, "failed to decode result from IP lookups")
	}

	return res.IP, nil
}
