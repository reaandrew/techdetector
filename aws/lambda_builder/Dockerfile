# Use Amazon Linux 2 as base image
FROM amazonlinux:2


ARG GO_VERSION=1.22.4

# Install dependencies and Go
RUN yum install -y gcc gcc-c++ glibc-static tar gzip curl && \
    curl -OL https://golang.org/dl/go${GO_VERSION}.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz && \
    rm go${GO_VERSION}.linux-amd64.tar.gz

# Set Go environment variables
ENV PATH="/usr/local/go/bin:$PATH"
ENV GOPATH="/go"
ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=amd64

# Set working directory inside container
WORKDIR /app

# Default command
CMD ["go", "version"]