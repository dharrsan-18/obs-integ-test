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
	imageName     string = "getastra/proxy"           // Replace with your Docker image name
	containerName string = "astra-proxy-service"      // Replace with your container name
	lockFilePath  string = "/tmp/astra-cli-tool.lock" // Path for the lock file

	EntryPointArgIdentifier string = "--entrypoint"
)

var (
	lockFile       *os.File
	entryPointArgs []string
)

// checkDockerAvailability checks if the Docker command is available
func checkDockerAvailability() {
	cmd := exec.Command("docker", "--version")
	if err := cmd.Run(); err != nil {
		log.Println("Docker is not installed or not found in the system PATH.")
		log.Println("Please ensure Docker is installed and available in your PATH.")
		log.Println("For installation instructions, visit: https://docs.docker.com/engine/install/")
		os.Exit(1)
	}
}

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

// quickStartContainer starts the Docker container with the provided flags
func quickStartContainer(flags []string) {
	args := []string{"run", "-d", "--name", containerName, "--network=host"}
	dockerArgs := []string{}
	mitmPort := ""

	for idx := 0; idx < len(flags); idx++ {
		if flags[idx] == "--listen-port" {
			mitmPort = flags[idx+1]
			idx++
		} else {
			dockerArgs = append(dockerArgs, flags[idx])
		}
	}

	args = append(args, dockerArgs...)
	args = append(args, "--entrypoint", "mitmdump", imageName, "-k", "-s", "/app/capture.py")

	// Check if the entrypoint is specified in the flags
	if len(mitmPort) > 0 {
		args = append(args, "--listen-port", mitmPort)
	}

	log.Println("Running command... docker", strings.Join(args, " "))
	cmd := exec.Command("docker", args...)

	// Capture both stdout and stderr
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		// Log both the error and the stderr output
		log.Printf("Failed to start container: %v\n", err)
		log.Printf("Docker error output: %s\n", errBuf.String())
		return
	}

	// Log both stdout and stderr output in case of a successful run
	log.Printf("Command: `docker %s` executed successfully.", strings.Join(args, " "))
	log.Printf("Docker output: %s\n", outBuf.String())
}

// startContainer starts the Docker container with the provided flags
func startContainer(flags []string) {
	args := []string{"run", "-d", "--name", containerName}

	// Append any additional flags provided by the user until "--entrypoint" argument is observed
	dockerArgs := flags
	mitmArgs := []string{}
	for idx, arg := range flags {
		if arg == EntryPointArgIdentifier {
			dockerArgs = flags[:idx]
			mitmArgs = flags[idx:]
		}
	}

	args = append(args, dockerArgs...)

	// Check if the entrypoint is specified in the flags
	if len(mitmArgs) > 1 {
		args = append(args, mitmArgs...)
	} else {
		// Use default entrypoint args
		//append mandatory entrypoint arg
		args = append(args, "--entrypoint", "mitmdump", imageName, "-k", "-s", "/app/capture.py")
	}

	log.Println("Running command... docker", strings.Join(args, " "))
	cmd := exec.Command("docker", args...)

	// Capture both stdout and stderr
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		// Log both the error and the stderr output
		log.Printf("Failed to start container: %v\n", err)
		log.Printf("Docker error output: %s\n", errBuf.String())
		return
	}

	// Log both stdout and stderr output in case of a successful run
	log.Printf("Command: `docker %s` executed successfully.", strings.Join(args, " "))
	log.Printf("Docker output: %s\n", outBuf.String())
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
func pullLatestImage(_ []string) {
	log.Println("Checking for new image...")

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
	cmd := exec.Command("docker", "ps", "-a", "--filter", fmt.Sprintf("name=%s", containerName))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Println("Checking container status...")
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to check container status: %v", err)
	}
}

// removeContainer removes a Docker container by its name
func removeContainer() {
	cmd := exec.Command("docker", "rm", containerName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Println("Removing container...")
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to remove container: %v", err)
	}
	log.Println("Container removed successfully.")
}

func main() {
	checkDockerAvailability()

	// Ensure single instance using file lock
	ensureSingleInstance()
	defer releaseLock() // Release the lock when the program exits

	var rootCmd = &cobra.Command{
		Use:   "cli-tool",
		Short: "CLI Tool for managing Docker container",
	}

	var proxyCmd = &cobra.Command{
		Use:   "proxy",
		Short: "Manage the Astra proxy container",
	}

	var quickStartCmd = &cobra.Command{
		Use:                "quickstart",
		Short:              "Start the Astra proxy container with env variable(s) and proxy port",
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			quickStartContainer(args)
		},
	}

	var startCmd = &cobra.Command{
		Use:                "start",
		Short:              "Start the Astra proxy container",
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			startContainer(args)
		},
	}

	var stopCmd = &cobra.Command{
		Use:                "stop",
		Short:              "Stop the Astra proxy container",
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			stopContainer()
		},
	}

	var upgradeCmd = &cobra.Command{
		Use:                "upgrade",
		Short:              "Upgrade the Astra proxy container image if a new image is available",
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			pullLatestImage(args)
		},
	}

	var logsCmd = &cobra.Command{
		Use:                "logs",
		Short:              "Stream the Astra proxy container logs",
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			streamLogs(args)
		},
	}

	var statusCmd = &cobra.Command{
		Use:   "status",
		Short: "Check the status of the Astra proxy container",
		Run: func(cmd *cobra.Command, args []string) {
			checkStatus()
		},
	}

	var removeCmd = &cobra.Command{
		Use:   "remove",
		Short: "Remove the Astra proxy container",
		Run: func(cmd *cobra.Command, args []string) {
			removeContainer()
		},
	}

	proxyCmd.AddCommand(quickStartCmd, startCmd, stopCmd, upgradeCmd, logsCmd, statusCmd, removeCmd)
	rootCmd.AddCommand(proxyCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
