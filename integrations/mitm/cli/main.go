package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

const (
	imageName     = "library/nginx"            // Replace with your Docker image name
	containerName = "astra-proxy-service"      // Replace with your container name
	lockFilePath  = "/tmp/astra-cli-tool.lock" // Path for the lock file
)

var lockFile *os.File

// ensureSingleInstance ensures only one instance of the CLI is running using a file lock
func ensureSingleInstance() {
	var err error
	lockFile, err = os.OpenFile(lockFilePath, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		log.Fatalf("Failed to create/open lock file: %v", err)
	}

	// Try to obtain an exclusive lock
	err = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		log.Fatalf("Another instance of the CLI tool is already running.")
	}
}

// releaseLock releases the file lock and closes the file
func releaseLock() {
	if lockFile != nil {
		syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
		lockFile.Close()
	}
}

// startContainer starts the Docker container with the provided flags
func startContainer(flags []string) {
	args := append([]string{"run", "--rm", "-d", "--name", containerName}, flags...)
	args = append(args, imageName)

	fmt.Println("Command being execed: docker", strings.Join(args, " "))

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Println("Starting container...")
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to start container: %v", err)
	}
	log.Println("Container started successfully.")
}

// isContainerRunning checks if the Docker container is running
func isContainerRunning() bool {
	cmd := exec.Command("docker", "ps", "--filter", fmt.Sprintf("name=%s", containerName), "--format", "{{.Names}}")
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		log.Printf("Error checking container status: %v", err)
		return false
	}

	return strings.TrimSpace(out.String()) == containerName
}

// stopContainer stops the running Docker container
func stopContainer() {
	fmt.Println("Command being execed: docker stop", containerName)

	if !isContainerRunning() {
		log.Println("Container is not running, no need to stop.")
		return
	}

	cmd := exec.Command("docker", "stop", containerName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Println("Stopping container...")
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to stop container: %v", err)
	}
	log.Println("Container stopped successfully.")
}

// pullLatestImage pulls the latest image from docker repository
func pullLatestImage(flags []string) {
	log.Println("Checking for new image...")

	fmt.Println("Command being execed: docker pull", imageName)

	pullCmd := exec.Command("docker", "pull", imageName)
	pullCmd.Stdout = os.Stdout
	pullCmd.Stderr = os.Stderr

	if err := pullCmd.Run(); err != nil {
		log.Fatalf("Failed to pull image: %v", err)
	}

	/*
		// Check if the image is up to date
		inspectCmd := exec.Command("docker", "inspect", "--format", "{{.Id}}", imageName)
		inspectOutput, err := inspectCmd.Output()
		if err != nil {
			log.Fatalf("Failed to inspect image: %v", err)
		}
		currentImageID := string(inspectOutput)

		// Compare the current image ID with the running container's image ID
		containerInspectCmd := exec.Command("docker", "inspect", "--format", "{{.Image}}", containerName)
		containerInspectOutput, err := containerInspectCmd.Output()
		if err == nil && string(containerInspectOutput) == currentImageID {
			log.Println("The container is already running the latest image.")
			return false
		}

		log.Println("A new image version is available.")
		return true
	*/
}

/*
// upgradeContainer checks for a new image, pulls it.
func upgradeContainer(flags []string) {
		if checkAndPullLatestImage() {
			log.Println("Image pulled successfully. Stopping and restarting container...")
			stopContainer()
			startContainer(flags)
		} else {
			log.Println("No upgrade needed. The container is already running the latest version.")
		}
}
*/

// streamLogs streams the Docker container logs
func streamLogs(flags []string) {
	args := append([]string{"logs"}, flags...)
	args = append(args, containerName)

	fmt.Println("Command being execed: docker ", strings.Join(args, " "))

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Println("Streaming container logs...")
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to stream container logs: %v", err)
	}
}

// checkStatus checks the status of the Docker container
func checkStatus() {
	cmd := exec.Command("docker", "ps", "--filter", fmt.Sprintf("name=%s", containerName))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Println("Checking container status...")
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to check container status: %v", err)
	}
}

func main() {
	// Ensure single instance using file lock
	ensureSingleInstance()
	defer releaseLock() // Release the lock when the program exits

	var rootCmd = &cobra.Command{
		Use:   "cli-tool",
		Short: "CLI Tool for managing Docker container",
	}

	var proxyCmd = &cobra.Command{
		Use:   "proxy",
		Short: "Manage the Docker proxy container",
	}

	var startCmd = &cobra.Command{
		Use:                "start",
		Short:              "Start the Docker container",
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(args)
			startContainer(args)
		},
	}

	var stopCmd = &cobra.Command{
		Use:                "stop",
		Short:              "Stop the Docker container",
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			stopContainer()
		},
	}

	var upgradeCmd = &cobra.Command{
		Use:                "upgrade",
		Short:              "Upgrade the Docker container if a new image is available",
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			pullLatestImage(args)
		},
	}

	var logsCmd = &cobra.Command{
		Use:                "logs",
		Short:              "Stream the Docker container logs",
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			streamLogs(args)
		},
	}

	var statusCmd = &cobra.Command{
		Use:   "status",
		Short: "Check the status of the Docker container",
		Run: func(cmd *cobra.Command, args []string) {
			checkStatus()
		},
	}

	proxyCmd.AddCommand(startCmd, stopCmd, upgradeCmd, logsCmd, statusCmd)
	rootCmd.AddCommand(proxyCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
