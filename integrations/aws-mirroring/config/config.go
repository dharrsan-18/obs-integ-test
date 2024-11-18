package config

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

type SuricataConfig struct {
	NetworkInterface      string   `json:"network-interface"`
	SensorID              string   `json:"sensor-id"`
	OtelCollectorEndpoint string   `json:"otel-collector-endpoint"`
	AcceptHosts           []string `json:"accept-hosts"`
	DenyContentTypes      []string `json:"deny-content-type"`
}

type EnvConfig struct {
	Routines              int           `env:"routines,default=10"`
	OTELBatchTimeout      time.Duration `env:"OTEL_BATCH_TIMEOUT,default=5s"`
	OTELMaxBatchSize      int           `env:"OTEL_MAX_BATCH_SIZE,default=512"`
	OTELMaxQueueSize      int           `env:"OTEL_MAX_QUEUE_SIZE,default=2048"`
	OTELExportTimeout     time.Duration `env:"OTEL_EXPORT_TIMEOUT,default=30s"`
	OTELRetryInitInterval time.Duration `env:"OTEL_RETRY_INITIAL_INTERVAL,default=1s"`
	OTELRetryMaxInterval  time.Duration `env:"OTEL_RETRY_MAX_INTERVAL,default=5s"`
	OTELRetryMaxElapsed   time.Duration `env:"OTEL_RETRY_MAX_ELAPSED_TIME,default=30s"`
	SensorVersion         string        `env:"SENSOR_VERSION,required"`
	SuricataVersion       string        `env:"SURICATA_VERSION,required"`
}

func isValidUUID(uuidStr string) bool {
	_, err := uuid.Parse(uuidStr)
	return err == nil
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

	config := &SuricataConfig{}
	if err := json.Unmarshal(bytes, config); err != nil {
		return nil, err
	}

	if !isValidUUID(config.SensorID) {
		log.Fatalf("Invalid SensorID %s, dropping trace", config.SensorID)
	}

	return config, nil
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

	if env.Routines > 50 || env.Routines < 1 {
		return nil, fmt.Errorf("unsupported number of routines. must be between 1 - 50")
	}

	return env, nil
}
