package main

import (
	"fmt"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"math/big"
	"os"
	"testing"
)

type TerraformBlock struct {
	Type       string
	Labels     []string
	Attributes map[string]interface{}
	Blocks     []*TerraformBlock
}

func TestSomethingParserTerraform(t *testing.T) {
	data := `

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

	// Use hclparse to parse the HCL data
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL([]byte(data), "config.tf")
	if diags.HasErrors() {
		fmt.Println("Failed to parse HCL:", diags)
		os.Exit(1)
	}

	// Assert that the file body is of type *hclsyntax.Body
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		fmt.Println("Failed to cast body to hclsyntax.Body")
		os.Exit(1)
	}

	tfBlocks := parseBody(body, []byte(data))

	// Now you can work with tfBlocks programmatically
	for _, block := range tfBlocks {
		printBlock(block, 0)
	}

	fmt.Println("FINISHED")
}

// parseBody recursively parses HCL syntax body into TerraformBlock structures.
func parseBody(body *hclsyntax.Body, src []byte) []*TerraformBlock {
	var blocks []*TerraformBlock

	// Parse blocks
	for _, block := range body.Blocks {
		tfBlock := &TerraformBlock{
			Type:       block.Type,
			Labels:     block.Labels,
			Attributes: make(map[string]interface{}),
		}

		// Parse attributes within the block
		for name, attr := range block.Body.Attributes {
			val, diags := attr.Expr.Value(nil)
			if diags.HasErrors() {
				// If unable to evaluate (due to variables, functions, etc.), store the expression as a string
				rng := attr.Expr.Range()
				exprSrc := string(src[rng.Start.Byte:rng.End.Byte])
				tfBlock.Attributes[name] = exprSrc
				continue
			}

			tfBlock.Attributes[name] = convertCtyValueToGo(val)
		}

		// Recursively parse nested blocks
		tfBlock.Blocks = parseBody(block.Body, src)

		blocks = append(blocks, tfBlock)
	}

	return blocks
}

// convertCtyValueToGo converts a cty.Value to a Go interface{}, handling different types appropriately.
func convertCtyValueToGo(val cty.Value) interface{} {
	if !val.IsKnown() || val.IsNull() {
		return nil
	}

	switch {
	case val.Type().Equals(cty.String):
		return val.AsString()
	case val.Type().Equals(cty.Bool):
		return val.True()
	case val.Type().Equals(cty.Number):
		bf := val.AsBigFloat()
		if i, acc := bf.Int64(); acc == big.Exact {
			return i
		} else if f, _ := bf.Float64(); true {
			return f
		} else {
			return bf.String()
		}
	case val.Type().IsListType() || val.Type().IsTupleType():
		elems := val.AsValueSlice()
		var list []interface{}
		for _, elem := range elems {
			list = append(list, convertCtyValueToGo(elem))
		}
		return list
	case val.Type().IsMapType() || val.Type().IsObjectType():
		m := make(map[string]interface{})
		for key, v := range val.AsValueMap() {
			m[key] = convertCtyValueToGo(v)
		}
		return m
	default:
		// For other types, return the string representation
		return val.GoString()
	}
}

// printBlock prints the TerraformBlock content with indentation for readability.
func printBlock(block *TerraformBlock, indent int) {
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
		printBlock(nestedBlock, indent+1)
	}
}
