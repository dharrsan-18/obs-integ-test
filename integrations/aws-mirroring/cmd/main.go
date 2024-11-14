package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/getastra/obs-integrations/integrations/aws-mirroring/config"
	"github.com/getastra/obs-integrations/integrations/aws-mirroring/layers"
	"golang.org/x/sync/errgroup"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eg, ctx := errgroup.WithContext(ctx)

	config, err := config.LoadConfig("../env.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	tp, err := layers.InitExporter(ctx, config)
	if err != nil {
		log.Fatalf("Failed to initialize exporter: %v", err)
	}

	channels := &layers.Channels{
		LogsChan:           make(chan *layers.SuricataHTTPEvent, 5),
		OtelAttributesChan: make(chan *layers.OTELAttributes, 5),
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
		return layers.ReceiverFunc(ctx, channels, config.NetworkInterface)
	})

	// Start processors and exporters
	for i := 0; i < 5; i++ {
		eg.Go(func() error {
			return layers.ProcessorFunc(ctx, channels, config)
		})
		eg.Go(func() error {
			return layers.ExportFunc(ctx, channels)
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
