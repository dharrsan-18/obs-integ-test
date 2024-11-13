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

	"golang.org/x/sync/errgroup"
)

// pointer for suricataHTTPEvent
type Channels struct {
	LogsChan           chan *suricataHTTPEvent
	OtelAttributesChan chan OTELAttributes
}

type Config struct {
	NetworkInterface      string   `json:"network-interface"`
	SensorID              string   `json:"sensor-id"`
	ServiceName           string   `json:"service-name"`
	ServiceVersion        string   `json:"service-version"`
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
	defer cancel()

	eg, ctx := errgroup.WithContext(ctx)

	config, err := loadConfig("env.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	tp, err := initExporter(ctx, config)
	if err != nil {
		log.Fatalf("Failed to initialize exporter: %v", err)
	}

	channels := &Channels{
		LogsChan:           make(chan *suricataHTTPEvent, 5),
		OtelAttributesChan: make(chan OTELAttributes, 5),
	}

	// Handle shutdown signals
	eg.Go(func() error {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
		select {
		case sig := <-signalChan:
			log.Printf("Received signal %v, starting graceful shutdown...", sig)
			cancel()

			// Close channels
			close(channels.LogsChan)
			close(channels.OtelAttributesChan)

			// Wait for 10 seconds
			time.Sleep(10 * time.Second)
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	// Start receiver
	eg.Go(func() error {
		return receiverFunc(ctx, channels, config.NetworkInterface)
	})

	// Start processors and exporters
	for i := 0; i < 5; i++ {
		eg.Go(func() error {
			return processorFunc(ctx, channels, config)
		})
		eg.Go(func() error {
			return exportFunc(ctx, channels)
		})
	}

	// Wait for all goroutines to complete
	if err := eg.Wait(); err != nil && err != context.Canceled {
		log.Printf("Error during shutdown: %v", err)
	}

	// Final cleanup
	if err := tp.Shutdown(ctx); err != nil && err != context.Canceled {
		log.Printf("Error shutting down tracer provider: %v", err)
	}

	log.Println("Shutdown complete")
}
