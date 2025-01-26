#!/bin/bash

LAMBDA_URL="$1"
TOKEN="$2"

REPO_URL="https://github.com/reaandrew/techdetector.git"

curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d @- "${LAMBDA_URL}" <<EOF
{
  "repo": "${REPO_URL}",
  "cutoff": "1 week ago"
}
EOF