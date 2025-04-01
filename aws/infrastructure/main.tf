terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 4.66.1"
    }
  }

  backend "s3" {
    bucket  = "techdetector-tf"
    key     = "state/techdetector.tfstate"
    region  = "eu-west-2"
    encrypt = true
  }
}

data "aws_region" "current" {}
data "aws_caller_identity" "current" {}

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

# IAM Policy Document for accessing the SSM parameters
data "aws_iam_policy_document" "allow_ssm_access" {
  statement {
    actions = [
      "ssm:GetParameter",
      "ssm:GetParameters",
    ]
    resources = [
      "arn:aws:ssm:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:parameter/techdetector/${var.environment}/users/*"
    ]
  }
}

# Create an IAM Policy for SSM Parameter Store access
resource "aws_iam_policy" "ssm_access_policy" {
  name        = "${var.environment}_TechDetector_SSMAccessPolicy"
  description = "Policy to allow Lambda to read authentication tokens from SSM Parameter Store"
  policy      = data.aws_iam_policy_document.allow_ssm_access.json

  tags = {
    Project = "reaandrew-techdetector"
  }
}

# Attach the SSM access policy to the Lambda IAM role
resource "aws_iam_role_policy_attachment" "lambda_ssm_access_policy_attachment" {
  role       = aws_iam_role.lambda.name
  policy_arn = aws_iam_policy.ssm_access_policy.arn
}

# Lambda Function Deployment
resource "aws_lambda_function" "lambda_techdetector" {
  function_name    = "lambda_techdetector"
  description      = "Lambda function for tech detector"
  role             = aws_iam_role.lambda.arn
  package_type     = "Image"  # This is a container image
  image_uri        = "${data.aws_caller_identity.current.account_id}.dkr.ecr.${data.aws_region.current.name}.amazonaws.com/${var.image_name}:${var.image_tag}"
  publish          = true

  ephemeral_storage {
    size = 6144
  }
  timeout     = 10
  memory_size = 5120

  environment {
    variables = {
      ENVIRONMENT          = var.environment
      SSM_PARAMETER_PREFIX = "/techdetector/${var.environment}/users/"
    }
  }

  tags = {
    Project = "reaandrew-techdetector"
  }
}

output "lambda_function_name" {
  description = "The name of the Lambda function"
  value       = aws_lambda_function.lambda_techdetector.function_name
}
