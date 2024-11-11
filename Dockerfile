FROM golang:1.22-bullseye AS builder
WORKDIR /app

# Copy the application source files
COPY . .

# Initialize a new module and tidy up dependencies
RUN go mod init myapp && go mod tidy

# Build the Go application
RUN go build -o main .

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

# Download and build Suricata
RUN curl -LO https://www.openinfosecfoundation.org/download/suricata-7.0.7.tar.gz && \
    tar xzvf suricata-7.0.7.tar.gz && \
    cd suricata-7.0.7 && \
    ./configure --enable-lua && \
    make && \
    make install && \
    cd .. && \
    rm -rf suricata-7.0.7.tar.gz

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
COPY --from=builder /app/env.json /root/obs-integ/env.json

# Copy Suricata configuration and Lua script files
COPY suricata.yaml /root/suricata-7.0.7/suricata.yaml
COPY http.lua /root/suricata-7.0.7/lua/http.lua

# Set executable perimission
RUN chmod +x /root/obs-integ/main

# Command to run the application
CMD ["/root/obs-integ/main"]
