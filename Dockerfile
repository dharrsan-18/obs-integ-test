# Use the Go base image for building the Go application
FROM golang:1.22-bullseye AS builder
WORKDIR /app

# Copy the application source files
COPY . .

# Initialize a new module and tidy up dependencies
RUN go mod init myapp && go mod tidy

# Build the Go application
RUN go build -o main .

# Use Amazon Linux 2 as the base image
FROM amazonlinux:2

# Update system and install basic dependencies
RUN yum update -y && \
    yum install -y \
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
    libmaxminddb-devel && \
    yum clean all

# Install Rust and Cargo
RUN yum install -y rustc cargo

# Install Lua JSON
RUN yum install -y lua-json

# Set the working directory
WORKDIR /root

# Download and build Suricata (following exact steps)
RUN curl -LO https://www.openinfosecfoundation.org/download/suricata-7.0.7.tar.gz && \
    tar xzvf suricata-7.0.7.tar.gz && \
    cd suricata-7.0.7 && \
    ./configure --enable-lua && \
    make && \
    make install

# Create the obs-integ directory
WORKDIR /root/obs-integ

# Copy the compiled Go binary from the builder stage
COPY --from=builder /app/main /root/obs-integ/main

# Copy Suricata configuration and Lua script files
COPY suricata.yaml /root/suricata-7.0.7/suricata.yaml
COPY http.lua /root/suricata-7.0.7/lua/http.lua

# Set permissions
RUN chmod +x /root/obs-integ/main

# Command to run the application
CMD ["/root/obs-integ/main"]