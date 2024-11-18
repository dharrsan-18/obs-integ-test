package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"mirroring/config"
	"mirroring/layers"

	"golang.org/x/sync/errgroup"
)

func getProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	return dir
}

func main() {
	path := getProjectRoot()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eg, ctx := errgroup.WithContext(ctx)

	suricataConfig, err := config.LoadSuricataConfig(filepath.Join(path, "mirror-settings.json"))
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	envConfig, err := config.LoadEnvConfig(filepath.Join(path, ".env"))
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	tp, err := layers.InitExporter(ctx, suricataConfig, envConfig)
	if err != nil {
		log.Fatalf("Failed to initialize exporter: %v", err)
	}

	channels := &layers.Channels{
		LogsChan:           make(chan *layers.SuricataHTTPEvent, envConfig.Routines),
		OtelAttributesChan: make(chan *layers.OTELAttributes, envConfig.Routines),
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
			// Wait for 10 seconds
			time.Sleep(10 * time.Second)
			close(channels.LogsChan)
			close(channels.OtelAttributesChan)
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	// Start receiver
	eg.Go(func() error {
		return layers.ReceiverFunc(ctx, channels, suricataConfig.NetworkInterface)
	})

	// Start processors and exporters
	for i := 0; i < envConfig.Routines; i++ {
		eg.Go(func() error {
			return layers.ProcessorFunc(ctx, channels, suricataConfig, envConfig)
		})
		eg.Go(func() error {
			return layers.ExportFunc(ctx, channels, envConfig)
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
