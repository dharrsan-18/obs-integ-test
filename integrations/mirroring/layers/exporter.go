package layers

import (
	"context"
	"fmt"
	"log/slog"

	"mirroring/config"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

func InitExporter(ctx context.Context, suricataConfig *config.SuricataConfig, envConfig *config.EnvConfig) (*sdktrace.TracerProvider, error) {
	// Create OTLP exporter
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(suricataConfig.OtelCollectorEndpoint),
		otlptracegrpc.WithTimeout(envConfig.OTELExportTimeout),
		otlptracegrpc.WithRetry(otlptracegrpc.RetryConfig{
			Enabled:         true,
			InitialInterval: envConfig.OTELRetryInitInterval,
			MaxInterval:     envConfig.OTELRetryMaxInterval,
			MaxElapsedTime:  envConfig.OTELRetryMaxElapsed,
		}),
	)
	if err != nil {
		slog.Error("Failed to create OTLP exporter",
			"error", err,
			"endpoint", suricataConfig.OtelCollectorEndpoint)
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceVersion(SENSOR_VERSION),
		),
	)
	if err != nil {
		slog.Error("Failed to create resource",
			"error", err,
			"sensor_version", SENSOR_VERSION)
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create and set TracerProvider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(envConfig.OTELBatchTimeout),
			sdktrace.WithMaxExportBatchSize(envConfig.OTELMaxBatchSize),
			sdktrace.WithMaxQueueSize(envConfig.OTELMaxQueueSize),
		),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	slog.Info("Initialized OpenTelemetry exporter",
		slog.Group("exporter",
			"endpoint", suricataConfig.OtelCollectorEndpoint,
			"timeout", envConfig.OTELExportTimeout.String(),
			"retry_initial_interval", envConfig.OTELRetryInitInterval.String(),
			"retry_max_interval", envConfig.OTELRetryMaxInterval.String(),
			"retry_max_elapsed", envConfig.OTELRetryMaxElapsed.String()),
		slog.Group("tracer",
			"batch_timeout", envConfig.OTELBatchTimeout.String(),
			"max_batch_size", envConfig.OTELMaxBatchSize,
			"max_queue_size", envConfig.OTELMaxQueueSize,
			"sensor_version", SENSOR_VERSION))

	return tp, nil
}

func ExportFunc(ctx context.Context, ch *Channels, envConfig *config.EnvConfig) error {
	tracer := otel.GetTracerProvider().Tracer("http-monitor")

	for {
		select {
		case <-ctx.Done():
			slog.Debug("Context cancelled, stopping Exporter")
			return ctx.Err()
		case attrs, ok := <-ch.OtelAttributesChan:
			if !ok {
				return nil
			}

			// Create new span for each event
			_, span := tracer.Start(ctx, "http.request")

			// Convert struct fields to OTEL attributes
			attributes := []attribute.KeyValue{
				attribute.String("http.method", attrs.HTTPMethod),
				attribute.String("http.flavor", attrs.HTTPFlavor),
				attribute.String("http.target", attrs.HTTPTarget),
				attribute.String("http.host", attrs.HTTPHost),
				attribute.Int("http.status_code", attrs.HTTPStatusCode),
				attribute.String("http.scheme", attrs.HTTPScheme),
				attribute.Int("net.host.port", attrs.NetHostPort),
				attribute.String("net.peer.ip", attrs.NetPeerIP),
				attribute.Int("net.peer.port", attrs.NetPeerPort),
				attribute.String("sensor.version", attrs.SensorVersion),
				attribute.String("sensor.id", attrs.SensorID),
				attribute.String("http.request.body", attrs.RequestBody),
				attribute.String("http.request.headers", attrs.RequestHeaders),
				attribute.String("http.response.headers", attrs.ResponseHeaders),
				attribute.String("http.response.body", attrs.ResponseBody),
			}

			// Set attributes and end span
			span.SetAttributes(attributes...)
			slog.Debug("Created span with attributes",
				"method", attrs.HTTPMethod,
				"target", attrs.HTTPTarget,
				"host", attrs.HTTPHost,
				"status_code", attrs.HTTPStatusCode,
				"client_ip", attrs.NetPeerIP,
				slog.Group("request",
					"content_type", attrs.RequestHeaders, // This will contain Content-Type from headers
					"body_size", len(attrs.RequestBody)),
				slog.Group("response",
					"content_type", attrs.ResponseHeaders, // This will contain Content-Type from headers
					"body_size", len(attrs.ResponseBody)))
			span.End()
		}
	}
}
