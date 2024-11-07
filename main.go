package main

import (
	"context"
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

func main() {

	ctx, cancel := context.WithCancel(context.Background())
	tp, err := initExporter(ctx)
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
	go processorFunc(ctx, channels)
	go exportFunc(ctx, channels)

	<-signalChan
	cancel()

	// sleep for 10s to wait for existing routines to finish
	time.Sleep(10 * time.Second)
}
