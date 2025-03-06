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
  handler          = "bootstrap"
  runtime          = "provided.al2"
  ephemeral_storage {
    size = 6144
  }
  timeout          = 900
  memory_size      = 5120
  filename         = "${path.module}/lambda_dist/bootstrap.zip"
  source_code_hash = filebase64sha256("${path.module}/lambda_dist/bootstrap.zip")

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


# # Create a Lambda URL for Public Access
# resource "aws_lambda_function_url" "lambda_url" {
#   function_name      = aws_lambda_function.lambda_techdetector.function_name
#   authorization_type = "NONE"  # Authentication is handled within the Lambda function
#
#   cors {
#     allow_credentials = true
#     allow_origins     = ["*"]
#     allow_methods     = ["*"]
#     allow_headers     = ["Content-Type", "Authorization", "date", "keep-alive"]
#     expose_headers    = ["keep-alive", "date"]
#     max_age           = 86400
#   }
# }

# # Lambda Permissions to Allow Public Access
# resource "aws_lambda_permission" "public_access" {
#   statement_id           = "AllowPublicAccess"
#   action                 = "lambda:InvokeFunctionUrl"
#   function_name          = aws_lambda_function.lambda_techdetector.arn  # Use ARN instead of name
#   principal              = "*"  # Allows public access
#   function_url_auth_type = "NONE"
# }

# # Outputs
# output "lambda_function_url" {
#   description = "The URL to invoke the Lambda function"
#   value       = aws_lambda_function_url.lambda_url.function_url
# }


output "lambda_function_name" {
  description = "The name of the Lambda function"
  value       = aws_lambda_function.lambda_techdetector.function_name
}
