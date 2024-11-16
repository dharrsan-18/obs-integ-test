package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type SuricataConfig struct {
	NetworkInterface      string   `json:"network-interface"`
	SensorID              string   `json:"sensor-id"`
	ServiceName           string   `json:"service-name"`
	ServiceVersion        string   `json:"service-version"`
	OtelCollectorEndpoint string   `json:"otel-collector-endpoint"`
	AcceptHosts           []string `json:"accept-hosts"`
	DenyContentTypes      []string `json:"deny-content-type"`
}

type EnvConfig struct {
	Routines int
}

func LoadSuricataConfig(filename string) (*SuricataConfig, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var config SuricataConfig
	if err := json.Unmarshal(bytes, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func LoadEnvConfig(filename string) (*EnvConfig, error) {
	if err := godotenv.Load(filename); err != nil {
		return nil, fmt.Errorf("error loading .env file: %v", err)
	}

	routines, err := strconv.Atoi(os.Getenv("routines"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse routines: %v", err)
	}

	env := &EnvConfig{
		Routines: routines,
	}

	return env, nil
}
