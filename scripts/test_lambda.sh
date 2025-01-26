#!/bin/bash

LAMBDA_URL="$1"

REPO_URL="https://github.com/reaandrew/techdetector.git"

curl -X POST \
  -H "Content-Type: application/json" \
  -H "User-Agent: Custom User Agent String" \
  -H "x-amzn-trace-id: 1-67891233-abcdef012345678912345678" \
  -d "{\"repo\": \"${REPO_URL}\"}" \
  "${LAMBDA_URL}"