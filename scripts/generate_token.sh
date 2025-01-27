#!/bin/bash

# -----------------------------------------------------------------------------
# Script Name: generate_store_token.sh
# Description: Generates a token in the format tdt_<user_id>_<32_byte_random>
#              and stores it as a SecureString in AWS SSM Parameter Store.
# Usage: ./generate_store_token.sh <user_id>
# -----------------------------------------------------------------------------

# Exit immediately if a command exits with a non-zero status.
set -e

# Function to display usage information
usage() {
    echo "Usage: $0 <user_id>"
    echo "Example: $0 user123"
    exit 1
}

# Check if AWS CLI is installed
if ! command -v aws &> /dev/null
then
    echo "Error: AWS CLI is not installed. Please install it before running this script."
    exit 1
fi

# Check if user_id is provided
if [ "$#" -ne 1 ]; then
    echo "Error: Missing required argument <user_id>."
    usage
fi

USER_ID="$1"

# Validate user_id (optional: adjust the regex as per your requirements)
if [[ ! "$USER_ID" =~ ^[a-zA-Z0-9_-]+$ ]]; then
    echo "Error: Invalid <user_id>. Only alphanumeric characters, underscores, and hyphens are allowed."
    exit 1
fi

# Generate a 32-byte (256-bit) secure random string, base64-encoded
SECURE_RANDOM=$(openssl rand -base64 32 | tr -d '\n' | tr -d '=' | tr '/+' '_-')
if [ $? -ne 0 ]; then
    echo "Error: Failed to generate a secure random string."
    exit 1
fi

# Construct the token
TOKEN="tdt_${USER_ID}_${SECURE_RANDOM}"

# Define the SSM parameter name
SSM_PARAMETER_NAME="/techdetector/dev/users/${USER_ID}"

# Check if the parameter already exists
echo "Checking if the SSM parameter exists..."
if aws ssm get-parameter --name "${SSM_PARAMETER_NAME}" &>/dev/null; then
    echo "Parameter exists. Updating the value..."
    aws ssm put-parameter \
        --name "${SSM_PARAMETER_NAME}" \
        --value "${TOKEN}" \
        --type SecureString \
        --overwrite
else
    echo "Parameter does not exist. Creating a new parameter..."
    aws ssm put-parameter \
        --name "${SSM_PARAMETER_NAME}" \
        --value "${TOKEN}" \
        --type SecureString
fi

# Add tags separately after creation or update
echo "Tagging the parameter..."
aws ssm add-tags-to-resource \
    --resource-type "Parameter" \
    --resource-id "${SSM_PARAMETER_NAME}" \
    --tags Key=Project,Value=TechDetector

if [ $? -eq 0 ]; then
    echo "Success: Token stored and tagged successfully."
    echo "Token for user '${USER_ID}': ${TOKEN}"
    echo "Note: This token has been securely stored in SSM Parameter Store under '${SSM_PARAMETER_NAME}'."
else
    echo "Error: Failed to store and tag the token in SSM Parameter Store."
    exit 1
fi
