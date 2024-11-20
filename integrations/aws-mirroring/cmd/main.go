package main

import (
	"context"
	"log"
	"log/slog"
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

func initLogger() *slog.Logger {
	opts := &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

func main() {
	logger := initLogger()
	logger.Info("Starting application", "version", "1.0.0")

	path := getProjectRoot()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eg, ctx := errgroup.WithContext(ctx)

	suricataConfig, err := config.LoadSuricataConfig(filepath.Join(path, "mirror-settings.json"))
	if err != nil {
		slog.Error("Failed to load Suricata config",
			"error", err,
			"path", filepath.Join(path, "mirror-settings.json"))
		os.Exit(1)
	}

	envConfig, err := config.LoadEnvConfig()
	if err != nil {
		slog.Error("Failed to load environment config",
			"error", err)
		os.Exit(1)
	}

	tp, err := layers.InitExporter(ctx, suricataConfig, envConfig)
	if err != nil {
		slog.Error("Failed to initialize exporter",
			"error", err,
			"collector_endpoint", suricataConfig.OtelCollectorEndpoint)
		os.Exit(1)
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
			slog.Warn("Received shutdown signal",
				"signal", sig,
				"action", "starting graceful shutdown")
			cancel()
			slog.Info("Waiting for graceful shutdown...")
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
		slog.Error("Error during shutdown",
			"error", err)
	}

	// Final cleanup
	if err := tp.Shutdown(ctx); err != nil && err != context.Canceled {
		slog.Error("Error shutting down tracer provider",
			"error", err)
	}

	slog.Info("Shutdown complete")
}
