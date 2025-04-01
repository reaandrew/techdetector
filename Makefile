LAMBDA_BINARY=bootstrap
DIST_DIR=./aws/infrastructure/lambda_dist
QUERY_FILE=queries.yaml
TERRAFORM_DIR=./aws/infrastructure
ZIP_FILE=$(DIST_DIR)/bootstrap.zip
DOCKER_IMAGE=go-lambda-builder

.PHONY: build
build:
	go build -a -ldflags '-X "main.Version=$(shell git describe)"' -o ./build/techdetector-linux-amd64

.PHONY: build_lambda
build_lambda:
	@mkdir -p $(DIST_DIR)
	@docker run --rm -v $(PWD):/app -w /app $(DOCKER_IMAGE) \
        go build -tags lambda.norpc -ldflags='-s -w' -o $(DIST_DIR)/$(LAMBDA_BINARY)
	@echo "Lambda binary built successfully."

.PHONY: prepare_dist
prepare_dist: build_lambda
	@echo "Preparing deployment package..."
	@cp $(QUERY_FILE) $(DIST_DIR)/
	@cd $(DIST_DIR) && zip -r bootstrap.zip $(LAMBDA_BINARY) $(QUERY_FILE)
	@echo "Lambda deployment package created: $(ZIP_FILE)"

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
	@echo "Cleaning build artifacts and deployment directory..."
	@rm -f $(LAMBDA_BINARY)
	@rm -rf $(DIST_DIR)
	@echo "Cleaned successfully."

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build_lambda    - Build the Go Lambda function binary"
	@echo "  prepare_dist    - Build and prepare deployment package"
	@echo "  deploy          - Deploy infrastructure using Terraform"
	@echo "  destroy         - Destroy infrastructure using Terraform"
	@echo "  clean           - Remove build artifacts and deployment package"
	@echo "  help            - Show available make targets"
