FROM python:3.12-slim

# Install mitmproxy
RUN pip install --no-cache-dir mitmproxy opentelemetry-api opentelemetry-sdk opentelemetry-exporter-otlp

# Copy your Python script into the container
COPY capture.py /app/capture.py

# Set the working directory
WORKDIR /app

# Expose the port mitmdump will use
EXPOSE 8080

# Command to run mitmdump with the Python script
ENTRYPOINT ["mitmdump", "-k", "-s", "/app/capture.py"]