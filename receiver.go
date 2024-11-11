package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
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

func getFirstNonLoopbackInterface() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback == 0 && iface.Flags&net.FlagUp != 0 {
			return iface.Name, nil
		}
	}
	return "", fmt.Errorf("no suitable network interface found")
}
func receiverFunc(ctx context.Context, ch *Channels, iface string) {
	var err error
	if iface == "" {
		iface, err = getFirstNonLoopbackInterface() // Use '=' to update the existing 'iface' variable
		if err != nil {
			log.Fatalf("Failed to find a suitable network interface: %v", err)
		}
	}
	log.Printf("Using network interface: %s", iface)

	cmd := exec.Command("stdbuf", "-oL", "suricata", "-c", "/etc/suricata/suricata.yaml", "-i", iface)
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
		}
	}

}
