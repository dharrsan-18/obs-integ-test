package main

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"strings"
)

type OTELAttributes struct {
	HTTPMethod      string
	HTTPFlavor      string
	HTTPTarget      string
	HTTPHost        string
	HTTPStatusCode  int
	HTTPScheme      string
	NetHostPort     int
	NetPeerIP       string
	NetPeerPort     int
	SensorVersion   string
	SensorID        string
	RequestBody     string
	RequestHeaders  string
	ResponseHeaders string
	ResponseBody    string
}

func processorFunc(ctx context.Context, ch *Channels, config *Config) {
	acceptSet := make(map[string]struct{})
	for _, host := range config.AcceptHosts {
		acceptSet[host] = struct{}{}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-ch.LogsChan:
			if !ok {
				return
			}
			if _, accepted := acceptSet[event.Request.Header["Host"]]; accepted {
				otelAttrs := mapEventToOTEL(event)

				// Populate service fields from config
				otelAttrs.SensorVersion = config.ServiceVersion
				otelAttrs.SensorID = config.ServiceName

				// Send to OTEL layer
				ch.OtelAttributesChan <- otelAttrs
			} else {
				log.Printf("Host %s is not accepted, skipping event", event.Request.Header["Host"])
			}
		}
	}
}

func mapEventToOTEL(event suricataHTTPEvent) OTELAttributes {
	attrs := OTELAttributes{}

	// Extract HTTP method, target, and flavor from request-line
	if requestLine, ok := event.Request.Header["request-line"]; ok {
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
	attrs.HTTPHost = event.Request.Header["Host"]

	// Extract status code from response-line
	if responseLine, ok := event.Response.Header["response-line"]; ok {
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

	return attrs
}
