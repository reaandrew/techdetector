# Use Amazon Linux 2 as base image
FROM amazonlinux:2

# Install dependencies and Go
RUN yum install -y gcc gcc-c++ glibc-static tar gzip curl && \
    curl -OL https://golang.org/dl/go1.21.5.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz && \
    rm go1.21.5.linux-amd64.tar.gz

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