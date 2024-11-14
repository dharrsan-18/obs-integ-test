package config

import (
	"encoding/json"
	"io"
	"os"
)

type Config struct {
	NetworkInterface      string   `json:"network-interface"`
	SensorID              string   `json:"sensor-id"`
	ServiceName           string   `json:"service-name"`
	ServiceVersion        string   `json:"service-version"`
	OtelCollectorEndpoint string   `json:"otel-collector-endpoint"`
	AcceptHosts           []string `json:"accept-hosts"`
	DenyContentTypes      []string `json:"deny-content-type"`
}

func LoadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(bytes, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
