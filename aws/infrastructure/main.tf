terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 4.66.1"
    }
  }

  backend "s3" {
    bucket  = "reaandrew-techdetector-tf"
    key     = "state/reaandrew_techdetector.tfstate"
    region  = "eu-west-2"
    encrypt = true
  }
}

data "aws_region" "current" {}

# IAM Role for Lambda Execution
data "aws_iam_policy_document" "assume_lambda_role" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "lambda" {
  name               = "${var.environment}_TechDetector_AssumeLambdaRole"
  description        = "Role for Lambda execution"
  assume_role_policy = data.aws_iam_policy_document.assume_lambda_role.json

  tags = {
    Project = "reaandrew-techdetector"
  }
}

# IAM Policy for Lambda Logging
data "aws_iam_policy_document" "allow_lambda_logging" {
  statement {
    effect = "Allow"
    actions = [
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutLogEvents",
    ]
    resources = [
      "arn:aws:logs:*:*:*",
    ]
  }
}

resource "aws_iam_policy" "function_logging_policy" {
  name        = "${var.environment}_TechDetector_AllowLambdaLoggingPolicy"
  description = "Policy for Lambda CloudWatch logging"
  policy      = data.aws_iam_policy_document.allow_lambda_logging.json

  tags = {
    Project = "reaandrew-techdetector"
  }
}

resource "aws_iam_role_policy_attachment" "lambda_logging_policy_attachment" {
  role       = aws_iam_role.lambda.id
  policy_arn = aws_iam_policy.function_logging_policy.arn
}

# Lambda Function Deployment
resource "aws_lambda_function" "lambda_techdetector" {
  function_name = "lambda_techdetector"
  description   = "Lambda function for tech detector"
  role          = aws_iam_role.lambda.arn
  handler       = "bootstrap"
  runtime       = "provided.al2"
  timeout       = 900
  memory_size   = 6144
  filename      = "${path.module}/lambda_dist/bootstrap.zip"

  environment {
    variables = {
      ENVIRONMENT = var.environment
    }
  }

  tags = {
    Project = "reaandrew-techdetector"
  }
}

# Create a Lambda URL for Public Access
resource "aws_lambda_function_url" "lambda_url" {
  function_name      = aws_lambda_function.lambda_techdetector.function_name
  authorization_type = "NONE"  # Change to AWS_IAM if authentication is required

  cors {
    allow_credentials = true
    allow_origins     = ["*"]
    allow_methods     = ["*"]
    allow_headers     = ["date", "keep-alive"]
    expose_headers    = ["keep-alive", "date"]
    max_age           = 86400
  }

}

# Lambda Permissions to Allow Public Access
resource "aws_lambda_permission" "public_access" {
  statement_id  = "AllowPublicAccess"
  action        = "lambda:InvokeFunctionUrl"
  function_name = aws_lambda_function.lambda_techdetector.function_name
  principal     = "*"
  function_url_auth_type = "NONE"
  source_arn    = aws_lambda_function_url.lambda_url.function_arn  # Use 'id' instead of 'arn'
}

# Outputs
output "lambda_function_url" {
  description = "The URL to invoke the Lambda function"
  value       = aws_lambda_function_url.lambda_url.function_url
}

output "lambda_function_name" {
  description = "The name of the Lambda function"
  value       = aws_lambda_function.lambda_techdetector.function_name
}
