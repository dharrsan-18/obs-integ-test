package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
)

const originalSuricataYamlPath string = "/root/suricata-7.0.7/suricata.yaml"
const tempSuricataYamlPath string = "/root/suricata-7.0.7/temp-suricata.yaml"

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

func copySuricataConfig() error {
	srcFile, err := os.Open(originalSuricataYamlPath)
	if err != nil {
		return fmt.Errorf("error opening original suricata.yaml: %v", err)
	}
	defer srcFile.Close()

	destFile, err := os.Create(tempSuricataYamlPath)
	if err != nil {
		return fmt.Errorf("error creating temp-suricata.yaml: %v", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return fmt.Errorf("error copying suricata.yaml to temp-suricata.yaml: %v", err)
	}

	return nil
}

// Update the interface in temp-suricata.yaml
func updateSuricataConfig(iface string) error {
	content, err := os.ReadFile(tempSuricataYamlPath)
	if err != nil {
		return fmt.Errorf("error reading temp-suricata.yaml: %v", err)
	}

	updatedContent := strings.ReplaceAll(string(content), "${NETWORK_INTERFACE}", iface)

	err = os.WriteFile(tempSuricataYamlPath, []byte(updatedContent), 0644)
	if err != nil {
		return fmt.Errorf("error writing to temp-suricata.yaml: %v", err)
	}

	log.Printf("Updated temp-suricata.yaml with network interface: %s", iface)
	return nil
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

	// Copy the original config file to temp-suricata.yaml before each run
	if err := copySuricataConfig(); err != nil {
		log.Fatalf("Failed to copy suricata.yaml to temp-suricata.yaml: %v", err)
	}

	// Update the temporary config with the interface
	if err := updateSuricataConfig(iface); err != nil {
		log.Fatalf("Failed to update temp-suricata.yaml: %v", err)
	}
	fmt.Println("stdbuf", "-oL", "suricata", "-c", tempSuricataYamlPath, "-i", iface)
	cmd := exec.Command("stdbuf", "-oL", "suricata", "-c", tempSuricataYamlPath, "-i", iface)
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
