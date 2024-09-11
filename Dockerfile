# Stage 1: Build Isolate and Go application
FROM golang:1.21 AS builder

# Install necessary packages and dependencies for Go and Isolate
RUN apt-get update && apt-get install -y \
    gcc \
    g++ \
    python3 \
#    openjdk-11-jdk \
    git \
    make \
    autoconf \
    automake \
    libtool \
    pkg-config \
    bash \
    libcap-dev \
    libsystemd-dev \
    asciidoc \
    && rm -rf /var/lib/apt/lists/*

# Clone and build Isolate with systemd support
RUN git clone https://github.com/ioi/isolate.git /tmp/isolate && \
    cd /tmp/isolate && \
    make && \
    make install && \
    cd / && rm -rf /tmp/isolate

# Set the Current Working Directory inside the container for Go application
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download all Go dependencies
RUN go mod download

# Copy the source code from the current directory to the Working Directory inside the container
COPY . .

# Build the Go application
RUN go build -o /app/judge cmd/judge/main.go

# Stage 2: Use a smaller base image for running the application
FROM debian:bookworm-slim

# Install libcap and Python to support Isolate runtime and Python code execution
RUN apt-get update && apt-get install -y \
    libcap2 \
    python3 \
    && rm -rf /var/lib/apt/lists/*

# Copy the built Go application from the builder stage
COPY --from=builder /app/judge /cmd/judge/main.go

# Copy Isolate binary and configuration from the builder stage
COPY --from=builder /usr/local/bin/isolate /usr/local/bin/isolate
COPY --from=builder /usr/local/etc/isolate /usr/local/etc/isolate

# Set the working directory to /app
WORKDIR /app

# Expose port 8080 for the Go application
EXPOSE 8080

# Run the Go application
CMD ["/cmd/judge/main.go"]
