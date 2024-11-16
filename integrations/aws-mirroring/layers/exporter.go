package layers

import (
	"context"
	"fmt"
	"log"

	"mirroring/config"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

func InitExporter(ctx context.Context, SuricataConfig *config.SuricataConfig) (*sdktrace.TracerProvider, error) {
	// Create OTLP exporter
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(SuricataConfig.OtelCollectorEndpoint), // Adjust endpoint as needed
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(SuricataConfig.ServiceName),
			semconv.ServiceVersion(SuricataConfig.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create and set TracerProvider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return tp, nil
}

func ExportFunc(ctx context.Context, ch *Channels) error {
	tracer := otel.GetTracerProvider().Tracer("http-monitor")

	for {
		select {
		case <-ctx.Done():
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
			log.Println("Span created and attributes set:", attributes)
			span.End()
		}
	}
}
