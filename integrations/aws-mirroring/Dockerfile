# Use a multi-stage build to compile the Go application
FROM golang:1.22-bullseye AS builder
WORKDIR /app

# Copy necessary files for building
COPY go.* ./
COPY *.go ./

# Build the Go application
RUN go mod init myapp && \
    go mod tidy && \
    go build -o main .

# Use Amazon Linux 2023 as the base image
FROM amazonlinux:2023

# Update system and install basic dependencies
RUN dnf update -y && \
    dnf clean all && \
    dnf install -y --allowerasing \
    gcc \
    make \
    pcre2-devel \
    libyaml-devel \
    libcap-ng-devel \
    file-devel \
    jansson-devel \
    nss-devel \
    lua-devel \
    zlib-devel \
    lz4-devel \
    libmaxminddb \
    libmaxminddb-devel \
    rustc \
    cargo \
    tar \
    gzip \
    curl \
    which \
    libpcap-devel \
    libnet-devel \
    libnetfilter_queue-devel && \
    dnf clean all && \
    rm -rf /var/cache/dnf/*

RUN yum install lua-json -y

WORKDIR /root

# Copy the JSON configuration file directly from the host
COPY env.json /root/env.json

# Read the Suricata version from the JSON file
ARG SURICATA_VERSION
RUN SURICATA_VERSION=$(jq -r '.suricata_version' /root/env.json) && \
    curl -LO https://www.openinfosecfoundation.org/download/suricata-${SURICATA_VERSION}.tar.gz && \
    tar xzvf suricata-${SURICATA_VERSION}.tar.gz && \
    cd suricata-${SURICATA_VERSION} && \
    ./configure --enable-lua && \
    make && \
    make install && \
    cd .. && \
    rm -rf suricata-${SURICATA_VERSION}.tar.gz

RUN mkdir -p /var/log/suricata && \
    mkdir -p /var/run/suricata && \
    mkdir -p /etc/suricata/rules && \
    chmod -R 755 /var/log/suricata && \
    chmod -R 755 /var/run/suricata && \
    chmod -R 755 /etc/suricata

# Create the obs-integ directory
WORKDIR /root/obs-integ

# Copy the compiled Go binary from the builder stage
COPY --from=builder /app/main /root/obs-integ/main

# Copy the env.json file to the obs-integ directory
COPY env.json /root/obs-integ/env.json

# Copy Suricata configuration and Lua script files
COPY suricata.yaml /root/suricata-${SURICATA_VERSION}/suricata.yaml
COPY http.lua /root/suricata-${SURICATA_VERSION}/lua/http.lua

# Set executable permission
RUN chmod +x /root/obs-integ/main

# Command to run the application
CMD ["/root/obs-integ/main"]