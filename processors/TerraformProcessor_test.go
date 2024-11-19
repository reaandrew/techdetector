package processors

import (
	"fmt"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"os"
	"testing"
)

func TestSomethingParserTerraform(t *testing.T) {
	data := `

module "pfs-software-terraform-modules" {
  source     = "./modules/repository-collaborators"
  repository = "pfs-software-terraform-modules"
  collaborators = [
    {
      github_user  = "nathanials"
      permission   = "admin"
      name         = "Nathanials Stewart"
      email        = "n.stewart@kainos.com"
      org          = "Kainos"
      reason       = "Kainos is working on transfering code from a kainos owned repo to an MOJ owned repo for Jenkins"
      added_by     = "jonathan.houston@justice.gov.uk"
      review_after = "2025-01-26"
    },
    {
      github_user  = "simongivan"
      permission   = "admin"
      name         = "Simon Givan"
      email        = "s.givan@kainos.com"
      org          = "Kainos"
      reason       = "Kainos is working on transfering code from a kainos owned repo to an MOJ owned repo"
      added_by     = "jonathan.houston@justice.gov.uk"
      review_after = "2025-01-26"
    },
    {
      github_user  = "dmeehankainos"
      permission   = "admin"
      name         = "Darren Meehan"
      email        = "darren.meehan@kainos.com"
      org          = "Kainos"
      reason       = "Kainos is working on new modernization platform for Unilink services"
      added_by     = "federico.staiano1@justice.gov.uk"
      review_after = "2025-01-26"
    },
  ]
}

module "lambda_AuthenticateFunction" {
  source      = "./terraform-modules/golang-lambda-function"
  lambda_function_description = "Authenticate Function"
  lambda_function_name = "AuthenticateFunction"
  lambda_path = "lambdas/AuthenticateFunction"
  lambda_role_arn = aws_iam_role.lambda.arn
  lambda_permission_enabled   = true
  lambda_permission_principal = "apigateway.amazonaws.com"
  lambda_permission_source_arn = "${aws_apigatewayv2_api.example_http_api.execution_arn}/*/*/*"
  lambda_environment = var.environment
  lambda_env_vars = {
    JWT_SIGNING_SECRET_NAME = "/stackscanner/${var.environment}/jwt_signing_secret_key"
  }
}

# Create an SSH key pair for accessing the EC2 instance
resource "aws_key_pair" "this" {
  public_key = "${file("${var.ssh_public_key_path}")}"
}

# Create our default security group to access the instance, over specific protocols
resource "aws_security_group" "this" {
  vpc_id = "${data.aws_vpc.this.id}"
  tags   = "${merge(var.tags, map("Name", "${var.hostname}"))}"
}

# Incoming SSH & outgoing ANY needs to be allowed for provisioning to work

resource "aws_security_group_rule" "outgoing_any" {
  security_group_id = "${aws_security_group.this.id}"
  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
}

resource "aws_security_group_rule" "incoming_ssh" {
  security_group_id = "${aws_security_group.this.id}"
  type              = "ingress"
  from_port         = 22
  to_port           = 22
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
}

# The rest of the security rules are opt-in

resource "aws_security_group_rule" "incoming_http" {
  count             = "${var.allow_incoming_http ? 1 : 0}"
  security_group_id = "${aws_security_group.this.id}"
  type              = "ingress"
  from_port         = 80
  to_port           = 80
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
}

resource "aws_security_group_rule" "incoming_https" {
  count             = "${var.allow_incoming_https ? 1 : 0}"
  security_group_id = "${aws_security_group.this.id}"
  type              = "ingress"
  from_port         = 443
  to_port           = 443
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
}

resource "aws_security_group_rule" "incoming_dns_tcp" {
  count             = "${var.allow_incoming_dns ? 1 : 0}"
  security_group_id = "${aws_security_group.this.id}"
  type              = "ingress"
  from_port         = 53
  to_port           = 53
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
}

resource "aws_security_group_rule" "incoming_dns_udp" {
  count             = "${var.allow_incoming_dns ? 1 : 0}"
  security_group_id = "${aws_security_group.this.id}"
  type              = "ingress"
  from_port         = 53
  to_port           = 53
  protocol          = "udp"
  cidr_blocks       = ["0.0.0.0/0"]
}
`

	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL([]byte(data), "config.tf")
	if diags.HasErrors() {
		fmt.Println("Failed to parse HCL:", diags)
		os.Exit(1)
	}

	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		fmt.Println("Failed to cast body to hclsyntax.Body")
		os.Exit(1)
	}

	tfBlocks := ParseBody(body, []byte(data))

	for _, block := range tfBlocks {
		PrintBlock(block, 0)
	}

	fmt.Println("FINISHED")
}

func PrintBlock(block *TerraformBlock, indent int) {
	indentStr := ""
	for i := 0; i < indent; i++ {
		indentStr += "  "
	}

	fmt.Printf("%sType: %s\n", indentStr, block.Type)
	if len(block.Labels) > 0 {
		fmt.Printf("%sLabels: %v\n", indentStr, block.Labels)
	}
	if len(block.Attributes) > 0 {
		fmt.Printf("%sAttributes:\n", indentStr)
		for k, v := range block.Attributes {
			fmt.Printf("%s  %s = %v\n", indentStr, k, v)
		}
	}
	for _, nestedBlock := range block.Blocks {
		PrintBlock(nestedBlock, indent+1)
	}
}