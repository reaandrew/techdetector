#!/bin/bash
set -e
set -x

SRC_DIR=$1
OUTPUT_DIR=$2

if [ -z "$SRC_DIR" ] || [ -z "$OUTPUT_DIR" ]; then
    echo "Usage: build.sh <source_directory> <output_directory>"
    exit 1
fi

# Verify that /src contains go.mod
if [ ! -f "$SRC_DIR/go.mod" ]; then
    echo "Error: go.mod not found in $SRC_DIR"
    exit 1
fi

echo "Copying source files from $SRC_DIR to /go/src/lambda..."
# Copy contents of /src into /go/src/lambda without nesting
cp -r "$SRC_DIR/." /go/src/lambda

# Navigate to the project directory
cd /go/src/lambda

echo "Current directory contents:"
ls -la

echo "Building the Go binary with static linking..."
# Build the Go binary with static linking to avoid GLIBC dependencies
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -mod=readonly -ldflags="-s -w" -o bootstrap

# Verify the binary was created
if [ ! -f bootstrap ]; then
    echo "Error: Failed to build bootstrap binary."
    exit 1
fi

echo "Copying bootstrap and queries.yaml to /lambda_dist..."
# Prepare the deployment package
mkdir -p /lambda_dist
cp bootstrap /lambda_dist/
cp queries.yaml /lambda_dist/

echo "Zipping the deployment package..."
cd /lambda_dist
zip bootstrap.zip bootstrap queries.yaml

echo "Copying bootstrap.zip to $OUTPUT_DIR..."
# Copy the deployment package to the output directory
cp bootstrap.zip "$OUTPUT_DIR/"

echo "Build and packaging completed successfully."
