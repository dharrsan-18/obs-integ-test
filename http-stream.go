package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/google/gopacket"
	"github.com/google/gopacket/tcpassembly"
	"github.com/google/gopacket/tcpassembly/tcpreader"
)

// HttpStreamFactory implements tcpassembly.StreamFactory
type HttpStreamFactory struct{}

// HttpStream will handle the actual decoding of http requests and responses from the tcp stream.
type HttpStream struct {
	net, transport gopacket.Flow
	r              tcpreader.ReaderStream
}

func (h *HttpStreamFactory) New(net, transport gopacket.Flow) tcpassembly.Stream {
	hstream := &HttpStream{
		net:       net,
		transport: transport,
		r:         tcpreader.NewReaderStream(),
	}
	go hstream.run() // Important... we must guarantee that data from the reader stream is read.

	return &hstream.r
}

func (h *HttpStream) run() {
	buf := bufio.NewReader(&h.r)
	for {
		// Peek at the first few bytes to determine if it's a request or response
		data, err := buf.Peek(10)
		if err == io.EOF {
			return
		} else if err != nil {
			log.Println("Error peeking into stream", h.net, h.transport, ":", err)
			continue
		}

		if bytes.HasPrefix(data, []byte("HTTP/")) {
			// MAYBE RESPONSE
			resp, err := http.ReadResponse(buf, nil)
			if err != nil {
				if err == io.EOF {
					return
				}
				log.Println("Error parsing HTTP response", h.net, h.transport, ":", err)
				continue
			}
			// Process the response
			fmt.Println("HTTP Response:")
			fmt.Println(resp.Proto, resp.Status)
			for key, values := range resp.Header {
				for _, value := range values {
					fmt.Println(key+":", value)
				}
			}
			fmt.Println()

			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Println("Error reading response body", h.net, h.transport, ":", err)
			} else {
				fmt.Println(string(respBody))
			}
			fmt.Println("----------------------------------------------------------")
		} else {
			// MAYBE REQUEST
			req, err := http.ReadRequest(buf)
			if err != nil {
				if err == io.EOF {
					return
				}
				log.Println("Error parsing HTTP request", h.net, h.transport, ":", err)
				continue
			}
			fmt.Println("HTTP Request:")
			fmt.Println(req.Method, req.URL, req.Proto, req.Host, req.URL)
			for key, values := range req.Header {
				for _, value := range values {
					fmt.Println(key+":", value)
				}
			}
			fmt.Println()

			body, err := io.ReadAll(req.Body)
			if err != nil {
				log.Println("Error reading request body", h.net, h.transport, ":", err)
			} else {
				fmt.Println(string(body))
			}
			fmt.Println()
		}
	}
}
