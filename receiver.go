package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// make pointers
type suricataHTTPEvent struct {
	Request  HTTPRequest  `json:"request"`
	Response HTTPResponse `json:"response"`
	Metadata HTTPMetadata `json:"metadata"`
}

type HTTPMetadata struct {
	Timestamp string `json:"timestamp"`
	SrcPort   int    `json:"src_port"`
	SrcIP     string `json:"src_ip"`
	DestPort  int    `json:"dest_port"`
	DestIP    string `json:"dest_ip"`
}

// make map[string]interface{}
type HTTPRequest struct {
	Header map[string]string `json:"header"`
	Body   string            `json:"body"`
}

type HTTPResponse struct {
	Header map[string]string `json:"header"`
	Body   string            `json:"body"`
}

func receiverFunc(ctx context.Context, ch *Channels) {

	cmd := exec.Command("stdbuf", "-oL", "suricata", "-c", "/etc/suricata/suricata.yaml", "-i", "eth0")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("Error creating StdoutPipe:", err)
		return
	}
	if err := cmd.Start(); err != nil {
		fmt.Println("Error starting command:", err)
		return
	}
	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			close(ch.LogsChan)
			return
		default:
			data := scanner.Bytes()
			event := suricataHTTPEvent{}
			err := json.Unmarshal(data, &event)
			if err == nil {
				ch.LogsChan <- event
			}
			// Check if data starts with { and ends with }
			// if len(data) > 0 && data[0] == '{' && data[len(data)-1] == '}' {
			// 	ch.LogsChan <-
			// }
		}
	}

}
