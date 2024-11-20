package layers

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"

	"mirroring/config"
)

const maxBodySize = 1 * 1024 * 1024 // 1MB in bytes

func isContentTypeDenied(contentType string, denyContentTypes []string) bool {
	for _, denyContentType := range denyContentTypes {
		if denyContentType == "" {
			continue // Skip empty deny content types
		}
		if strings.Contains(contentType, denyContentType) {
			return true
		}
	}
	return false
}

func ProcessorFunc(ctx context.Context, ch *Channels, suricataConfig *config.SuricataConfig, envConfig *config.EnvConfig) error {
	acceptSet := make(map[string]struct{})
	for _, host := range suricataConfig.AcceptHosts {
		acceptSet[host] = struct{}{}
	}

	for {
		select {
		case <-ctx.Done():
			slog.Debug("Context cancelled, stopping processor")
			return ctx.Err()
		case event, ok := <-ch.LogsChan:
			if !ok {
				slog.Debug("Logs channel closed, stopping processor")
				return nil
			}

			// Host validation
			if host, ok := event.Request.Header["Host"].(string); ok {
				if _, hostAccepted := acceptSet[host]; !hostAccepted {
					slog.Warn("Denied trace: non-accepted host",
						"client_ip", event.Metadata.SrcIP,
						"server_ip", event.Metadata.DestIP,
						"host", host,
						"request_line", event.Request.Header["request-line"])
					continue
				}
			} else {
				slog.Warn("Denied trace: invalid host in request headers",
					"client_ip", event.Metadata.SrcIP,
					"server_ip", event.Metadata.DestIP,
					"request_line", event.Request.Header["request-line"])
				continue
			}

			// Content type validation
			reqContentType, _ := event.Request.Header["Content-Type"].(string)
			respContentType, _ := event.Response.Header["Content-Type"].(string)
			if isContentTypeDenied(reqContentType, suricataConfig.DenyContentTypes) ||
				isContentTypeDenied(respContentType, suricataConfig.DenyContentTypes) {
				slog.Warn("Denied trace: denied content type",
					"client_ip", event.Metadata.SrcIP,
					"server_ip", event.Metadata.DestIP,
					"host", event.Request.Header["Host"],
					"request_line", event.Request.Header["request-line"],
					"request_content_type", reqContentType,
					"response_content_type", respContentType)
				continue
			}

			// Size validation
			if len(event.Request.Body) > maxBodySize || len(event.Response.Body) > maxBodySize {
				slog.Warn("Denied trace: oversized payload",
					"client_ip", event.Metadata.SrcIP,
					"server_ip", event.Metadata.DestIP,
					"host", event.Request.Header["Host"],
					"request_line", event.Request.Header["request-line"],
					"request_body_size", len(event.Request.Body),
					"response_body_size", len(event.Response.Body),
					"max_allowed_size", maxBodySize)
				continue
			}

			otelAttrs := mapEventToOTEL(event)
			if otelAttrs == nil {
				slog.Warn("Denied trace: invalid event data",
					"client_ip", event.Metadata.SrcIP,
					"server_ip", event.Metadata.DestIP,
					"host", event.Request.Header["Host"],
					"request_line", event.Request.Header["request-line"])
				continue
			}
			// Populate service fields from config
			otelAttrs.SensorVersion = SENSOR_VERSION
			otelAttrs.SensorID = suricataConfig.SensorID

			// Send to OTEL layer
			ch.OtelAttributesChan <- otelAttrs

		}
	}
}

func mapEventToOTEL(event *SuricataHTTPEvent) *OTELAttributes {
	attrs := OTELAttributes{}

	// Extract HTTP method, target, and flavor from request-line
	if requestLine, ok := event.Request.Header["request-line"].(string); ok {
		parts := strings.Split(requestLine, " ")
		if len(parts) >= 2 {
			attrs.HTTPMethod = parts[0] // GET, POST, etc.
			attrs.HTTPTarget = parts[1] // /path
			if len(parts) >= 3 {
				attrs.HTTPFlavor = strings.TrimPrefix(parts[2], "HTTP/") // 1.1, 2.0, etc.
			}
		}
	}

	// Get host from headers
	if host, ok := event.Request.Header["Host"].(string); ok {
		attrs.HTTPHost = host
	}

	// Extract status code from response-line
	if responseLine, ok := event.Response.Header["response-line"].(string); ok {
		parts := strings.Split(responseLine, " ")
		if len(parts) >= 2 {
			if statusCode, err := strconv.Atoi(parts[1]); err == nil {
				attrs.HTTPStatusCode = statusCode
			}
		}
	}

	// Set scheme based on port
	attrs.HTTPScheme = "http"
	if event.Metadata.DestPort == 443 {
		attrs.HTTPScheme = "https"
	}

	// Network attributes from metadata
	attrs.NetHostPort = event.Metadata.DestPort
	attrs.NetPeerIP = event.Metadata.SrcIP
	attrs.NetPeerPort = event.Metadata.SrcPort

	// Bodies
	attrs.RequestBody = event.Request.Body
	attrs.ResponseBody = event.Response.Body

	// Convert headers to JSON strings
	if reqHeadersBytes, err := json.Marshal(event.Request.Header); err == nil {
		attrs.RequestHeaders = string(reqHeadersBytes)
	}
	if respHeadersBytes, err := json.Marshal(event.Response.Header); err == nil {
		attrs.ResponseHeaders = string(respHeadersBytes)
	}

	var missingFields []string
	if attrs.HTTPMethod == "" {
		missingFields = append(missingFields, "HTTPMethod")
	}
	if attrs.HTTPTarget == "" {
		missingFields = append(missingFields, "HTTPTarget")
	}
	if attrs.HTTPHost == "" {
		missingFields = append(missingFields, "HTTPHost")
	}
	if attrs.HTTPStatusCode == 0 {
		missingFields = append(missingFields, "HTTPStatusCode")
	}
	if attrs.NetPeerIP == "" {
		missingFields = append(missingFields, "NetPeerIP")
	}

	if len(missingFields) > 0 {
		slog.Warn("Denied trace: missing required fields",
			"client_ip", event.Metadata.SrcIP,
			"server_ip", event.Metadata.DestIP,
			"host", event.Request.Header["Host"],
			"request_line", event.Request.Header["request-line"],
			"missing_fields", missingFields)
		return nil
	}

	return &attrs
}
