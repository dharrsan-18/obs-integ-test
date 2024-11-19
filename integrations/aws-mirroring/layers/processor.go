package layers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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

func getRequestInfo(event *SuricataHTTPEvent) string {
	scheme := "http"
	if event.Metadata.DestPort == 443 {
		scheme = "https"
	}

	method := ""
	path := ""
	if requestLine, ok := event.Request.Header["request-line"].(string); ok {
		parts := strings.Split(requestLine, " ")
		if len(parts) >= 2 {
			method = parts[0]
			path = parts[1]
		}
	}

	return fmt.Sprintf("scheme=%s method=%s path=%s", scheme, method, path)
}

func ProcessorFunc(ctx context.Context, ch *Channels, suricataConfig *config.SuricataConfig, envConfig *config.EnvConfig) error {
	acceptSet := make(map[string]struct{})
	for _, host := range suricataConfig.AcceptHosts {
		acceptSet[host] = struct{}{}
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-ch.LogsChan:
			if !ok {
				return nil
			}

			// Check if the hostname is accepted
			if host, ok := event.Request.Header["Host"].(string); ok {
				if _, hostAccepted := acceptSet[host]; !hostAccepted {
					log.Printf("Host %s is not accepted, skipping event: %s", host, getRequestInfo(event))
					continue
				}
			} else {
				log.Printf("Host is not a string, skipping event")
				continue
			}

			// Deny if the request or response content types contain any denied substring
			reqContentType, _ := event.Request.Header["Content-Type"].(string)
			respContentType, _ := event.Response.Header["Content-Type"].(string)
			if isContentTypeDenied(reqContentType, suricataConfig.DenyContentTypes) ||
				isContentTypeDenied(respContentType, suricataConfig.DenyContentTypes) {
				log.Printf("Content-Type is denied, skipping event: %s", getRequestInfo(event))
				continue
			}

			if len(event.Request.Body) > maxBodySize || len(event.Response.Body) > maxBodySize {
				log.Printf("Request or response body exceeds 1MB, skipping event: %s", getRequestInfo(event))
				continue
			}

			otelAttrs := mapEventToOTEL(event)
			if otelAttrs == nil {
				log.Printf("Invalid event data, skipping event: %s", getRequestInfo(event))
				continue
			}
			// Populate service fields from config
			otelAttrs.SensorVersion = envConfig.SensorVersion
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
		log.Printf("Missing HTTPMethod in request-line: %+v", event.Request.Header["request-line"])
		missingFields = append(missingFields, "HTTPMethod")
	}

	if attrs.HTTPTarget == "" {
		log.Printf("Missing HTTPTarget in request-line: %+v", event.Request.Header["request-line"])
		missingFields = append(missingFields, "HTTPTarget")
	}

	if attrs.HTTPHost == "" {
		log.Printf("Missing HTTPHost in headers: %+v", event.Request.Header["Host"])
		missingFields = append(missingFields, "HTTPHost")
	}

	if attrs.HTTPStatusCode == 0 {
		log.Printf("Missing or invalid HTTPStatusCode in response-line: %+v", event.Response.Header["response-line"])
		missingFields = append(missingFields, "HTTPStatusCode")
	}

	if attrs.NetPeerIP == "" {
		log.Printf("Missing NetPeerIP in metadata: SrcIP=%v", event.Metadata.SrcIP)
		missingFields = append(missingFields, "NetPeerIP")
	}

	if len(missingFields) > 0 {
		return nil
	}

	return &attrs
}
