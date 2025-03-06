
# syntax=docker/dockerfile:1

############
# 1) Build #
############
FROM amazonlinux:2 AS builder

# Adjust Go version as needed
ARG GO_VERSION=1.21.5

# Install dependencies: gcc and glibc for CGO, plus git if you need it
RUN yum install -y \
    gcc \
    gcc-c++ \
    glibc-static \
    tar \
    gzip \
    curl \
    git

# Download and install Go
RUN curl -OL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" && \
    tar -C /usr/local -xzf "go${GO_VERSION}.linux-amd64.tar.gz" && \
    rm "go${GO_VERSION}.linux-amd64.tar.gz"

# Set Go environment
ENV PATH="/usr/local/go/bin:${PATH}"
ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=amd64

WORKDIR /app

# Copy in Go modules first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of your source
COPY . .

# Build the binary (using CGO)
RUN go build -ldflags="-s -w" -o techdetector .

#########################
# 2) Final Runtime Image#
#########################
FROM amazonlinux:2

# Optionally remove extra packages to shrink image (this is just a runtime)
# But glibc must remain, so minimal removal is possible without breakage
RUN yum remove -y \
    gcc \
    gcc-c++ \
    glibc-static \
    tar \
    gzip \
    curl \
    git \
  || true

# Copy the compiled binary from the builder
COPY --from=builder /app/techdetector /usr/local/bin/techdetector

# Optionally create a non-root user
# RUN useradd -ms /bin/bash appuser
# USER appuser

WORKDIR /app
ENTRYPOINT ["/usr/local/bin/techdetector"]
CMD ["scan", "--help"]  # Provide a default command if you want