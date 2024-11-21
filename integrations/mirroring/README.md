# Guide for each Integration

## 1. Mirroring Integration Guide

### Step 1: Pull the Docker Image

Pull the latest or specific version of the Mirroring Docker image:
```bash
docker pull getastra/mirroring:latest
# or
docker pull getastra/mirroring:v1.0.1
```

Verify the image is pulled:
```bash
docker images
```

### Step 2: Create `mirror-settings.json`

Create a `mirror-settings.json` file with the following structure:
```json
{
    "network-interface": "",
    "sensor-id": "660e8400-e29b-41d4-a716-446655440000",
    "otel-collector-endpoint": "54.80.167.215:4317",
    "accept-hosts": ["getastra.com", "time.jsontest.com"],
    "deny-content-type": [""]
}
```

#### Field Descriptions:
- **network-interface**: Specify the network interface to monitor (e.g., `eth0`).
- **sensor-id**: Unique identifier for the sensor instance.
- **otel-collector-endpoint**: OpenTelemetry collector endpoint (e.g., `54.80.167.215:4317`).
- **accept-hosts**: List of hostnames to accept traffic from (e.g., `["getastra.com", "time.jsontest.com"]`).
- **deny-content-type**: List of content types to deny (e.g., `["application/json"]`).

### Step 3: Create a `.env` File

Create a `.env` file to customize processor and exporter settings. Default values are:
```ini
# General Settings
ROUTINES=49
LOG_LEVEL=DEBUG

# OTEL Exporter Settings
OTEL_BATCH_TIMEOUT=5
OTEL_MAX_BATCH_SIZE=512
OTEL_MAX_QUEUE_SIZE=2048
OTEL_EXPORT_TIMEOUT=30

# OTEL Retry Settings
OTEL_RETRY_INITIAL_INTERVAL=1
OTEL_RETRY_MAX_INTERVAL=5
OTEL_RETRY_MAX_ELAPSED_TIME=30
```

Modify these values as needed and save them in the `.env` file.

### Step 4: Run the Docker Container

Run the container with the following command:
```bash
docker run -itd --net=host --cap-add=NET_ADMIN \
  -v <path>/mirror-settings.json:/root/obs-integ/mirror-settings.json \
  --env-file=<envPath> \
  --name <container-name> \
  <image-name or image-id>
```

### Step 5: Verify the Container is Running

Check if the container is running:
```bash
docker ps
```

### Step 6: View Logs

To view the logs of the running container:
```bash
docker logs <container-name or container-id>
```

### Step 7: Beautify Logs with `jq`

If `jq` is installed, you can beautify the logs:
```bash
docker logs <container-name or container-id> | jq
```

This guide provides a comprehensive setup for the Mirroring integration using Docker. Adjust the configurations as needed for your specific environment.