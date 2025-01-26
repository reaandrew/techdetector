#!/bin/bash

LAMBDA_URL="$1"
TOKEN="$2"

REPO_URL="https://github.com/reaandrew/techdetector.git"

curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"repo\": \"${REPO_URL}\"}" \
  "${LAMBDA_URL}"