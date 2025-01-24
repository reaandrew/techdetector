# Makefile

# Define variables
LAMBDA_BINARY=bootstrap
DIST_DIR=aws/infrastructure/lambda_dist
QUERY_FILE=queries.yaml
TERRAFORM_DIR=./aws/infrastructure
ZIP_FILE=$(DIST_DIR)/bootstrap.zip
DOCKER_IMAGE=lambda-build-env

.PHONY: build_lambda
build_lambda:
	@echo "Building Lambda binary inside Docker..."
	# Build the Docker image
	docker build -t $(DOCKER_IMAGE) -f Dockerfile.build .
	# Create the DIST_DIR if it doesn't exist
	@mkdir -p $(DIST_DIR)
	# Run the Docker container to build the binary
	docker run --rm \
		-v $(shell pwd)/$(DIST_DIR):/output \
		-v $(shell pwd):/src \
		$(DOCKER_IMAGE) /src /output
	@echo "Lambda binary built successfully."

.PHONY: prepare_dist
prepare_dist: build_lambda
	@echo "Preparing deployment package..."
	# Remove redundant copy commands since build.sh already handled it
	# Only verify that bootstrap.zip exists
	@if [ -f $(ZIP_FILE) ]; then \
		echo "Lambda deployment package created: $(ZIP_FILE)"; \
	else \
		echo "Error: $(ZIP_FILE) not found."; \
		exit 1; \
	fi

.PHONY: deploy
deploy: prepare_dist
	@echo "Deploying infrastructure with Terraform..."
	@cd $(TERRAFORM_DIR) && terraform init && terraform apply -auto-approve
	@echo "Terraform deployment completed."

.PHONY: destroy
destroy:
	@echo "Destroying infrastructure with Terraform..."
	@cd $(TERRAFORM_DIR) && terraform init && terraform destroy -auto-approve
	@echo "Terraform destroy completed."

.PHONY: clean
clean:
	@echo "Cleaning build artifacts and deployment package..."
	@rm -f $(LAMBDA_BINARY)
	@rm -rf $(DIST_DIR)
	@echo "Cleaned successfully."

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build_lambda    - Build the Go Lambda function binary inside Docker"
	@echo "  prepare_dist    - Build and prepare deployment package"
	@echo "  deploy          - Deploy infrastructure using Terraform"
	@echo "  destroy         - Destroy infrastructure using Terraform"
	@echo "  clean           - Remove build artifacts and deployment package"
	@echo "  help            - Show available make targets"
