#!/bin/bash

LAMBDA_URL="$1"

REPO_URL="https://github.com/reaandrew/techdetector.git"

curl -X POST \
  -H "Content-Type: application/json" \
  -d "{\"repo\": \"${REPO_URL}\"}" \
  "${LAMBDA_URL}"