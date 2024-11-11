package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// pointer for suricataHTTPEvent
type Channels struct {
	LogsChan           chan suricataHTTPEvent
	OtelAttributesChan chan OTELAttributes
}

type Config struct {
	ServiceName           string   `json:"service_name"`
	ServiceVersion        string   `json:"service_version"`
	OtelCollectorEndpoint string   `json:"otel-collector-endpoint"`
	AcceptHosts           []string `json:"accept-hosts"`
	DenyContentTypes      []string `json:"deny-content-type"`
}

func loadConfig(filename string) (*Config, error) {
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

func main() {

	ctx, cancel := context.WithCancel(context.Background())

	config, err := loadConfig("env.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	tp, err := initExporter(ctx, config)
	if err != nil {
		log.Fatalf("Failed to initialize exporter: %v", err)
	}
	defer func() {
		if err := tp.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	defer cancel()

	channels := &Channels{
		LogsChan:           make(chan suricataHTTPEvent, 1),
		OtelAttributesChan: make(chan OTELAttributes, 1),
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	go receiverFunc(ctx, channels)
	go processorFunc(ctx, channels, config)
	go exportFunc(ctx, channels)

	<-signalChan
	cancel()

	// sleep for 10s to wait for existing routines to finish
	time.Sleep(10 * time.Second)
}
