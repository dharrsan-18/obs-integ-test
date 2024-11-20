package layers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"strings"
)

const originalSuricataYamlPath string = "/root/obs-integ/suricata.yaml"
const tempSuricataYamlPath string = "/root/obs-integ/temp-suricata.yaml"

func getFirstNonLoopbackInterface() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		slog.Error("Failed to get network interfaces", "error", err)
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
		slog.Error("Failed to open original Suricata config",
			"error", err,
			"path", originalSuricataYamlPath)
		return fmt.Errorf("error opening original suricata.yaml: %v", err)
	}
	defer srcFile.Close()

	destFile, err := os.Create(tempSuricataYamlPath)
	if err != nil {
		slog.Error("Failed to create temporary Suricata config",
			"error", err,
			"path", tempSuricataYamlPath)
		return fmt.Errorf("error creating temp-suricata.yaml: %v", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		slog.Error("Failed to copy Suricata config",
			"error", err,
			"source", originalSuricataYamlPath,
			"destination", tempSuricataYamlPath)
		return fmt.Errorf("error copying suricata.yaml to temp-suricata.yaml: %v", err)
	}

	slog.Info("Successfully copied Suricata config",
		"source", originalSuricataYamlPath,
		"destination", tempSuricataYamlPath)
	return nil
}

func updateSuricataConfig(iface string) error {
	content, err := os.ReadFile(tempSuricataYamlPath)
	if err != nil {
		slog.Error("Failed to read temporary Suricata config",
			"error", err,
			"path", tempSuricataYamlPath)
		return fmt.Errorf("error reading temp-suricata.yaml: %v", err)
	}

	updatedContent := strings.ReplaceAll(string(content), "${NETWORK_INTERFACE}", iface)

	err = os.WriteFile(tempSuricataYamlPath, []byte(updatedContent), 0644)
	if err != nil {
		slog.Error("Failed to write updated Suricata config",
			"error", err,
			"path", tempSuricataYamlPath)
		return fmt.Errorf("error writing to temp-suricata.yaml: %v", err)
	}

	slog.Info("Updated Suricata config with network interface",
		"interface", iface,
		"path", tempSuricataYamlPath)
	return nil
}

func ReceiverFunc(ctx context.Context, ch *Channels, iface string) error {
	var err error
	if iface == "" {
		iface, err = getFirstNonLoopbackInterface()
		if err != nil {
			slog.Error("Failed to find suitable network interface", "error", err)
			return fmt.Errorf("failed to find suitable network interface: %v", err)
		}
	}
	slog.Info("Using network interface", "interface", iface)

	if err := copySuricataConfig(); err != nil {
		slog.Error("Failed to copy Suricata config", "error", err)
		return fmt.Errorf("failed to copy suricata config: %v", err)
	}

	if err := updateSuricataConfig(iface); err != nil {
		slog.Error("Failed to update Suricata config", "error", err)
		return fmt.Errorf("failed to update suricata config: %v", err)
	}

	cmd := exec.Command("stdbuf", "-oL", "suricata", "-c", tempSuricataYamlPath, "-i", iface)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		slog.Error("Failed to create stdout pipe", "error", err)
		return fmt.Errorf("error creating StdoutPipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		slog.Error("Failed to start Suricata", "error", err)
		return fmt.Errorf("error starting command: %v", err)
	}

	defer func() {
		if err := cmd.Process.Kill(); err != nil {
			slog.Error("Failed to kill Suricata process", "error", err)
		}
	}()

	slog.Info("Started Suricata process",
		"interface", iface,
		"config_path", tempSuricataYamlPath)

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			slog.Info("Context cancelled, stopping receiver")
			return ctx.Err()
		default:
			data := scanner.Bytes()
			event := &SuricataHTTPEvent{}
			if err := json.Unmarshal(data, event); err == nil {
				slog.Debug("Received HTTP event",
					"client_ip", event.Metadata.SrcIP,
					"server_ip", event.Metadata.DestIP)
				ch.LogsChan <- event
			}
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Error("Scanner error", "error", err)
		return fmt.Errorf("scanner error: %v", err)
	}

	return nil
}
