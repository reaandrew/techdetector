package main

import (
	"encoding/json"
	"fmt"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsimple"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TerraformConfig represents the structure of Terraform files.
type TerraformConfig struct {
	Resources []Resource `hcl:"resource,block"`
	Variables []Variable `hcl:"variable,block"`
	Outputs   []Output   `hcl:"output,block"`
}

// Resource represents a Terraform resource block.
type Resource struct {
	Type string                 `hcl:"type,label"` // Resource type (e.g., "aws_s3_bucket")
	Name string                 `hcl:"name,label"` // Resource name (e.g., "example")
	Body map[string]interface{} `hcl:",remain"`    // Resource body as a map
}

// Variable represents a Terraform variable block.
type Variable struct {
	Name string                 `hcl:"name,label"` // Variable name
	Body map[string]interface{} `hcl:",remain"`    // Variable body as a map
}

// Output represents a Terraform output block.
type Output struct {
	Name string                 `hcl:"name,label"` // Output name
	Body map[string]interface{} `hcl:",remain"`    // Output body as a map
}

func TestSomethingTerraform(t *testing.T) {
	data := `
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

	config := TerraformConfig{}

	err := hclsimple.Decode("something.hcl", []byte(data), nil, &config)
	if err != nil {
		fmt.Println(err)
	}

	configJson, _ := json.MarshalIndent(config, "", "  ")
	fmt.Println(string(configJson))
}

func TestSomethingParserTerraform(t *testing.T) {
	data := `
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

	config := TerraformConfig{}

	parser := hclparse.NewParser()

	body, diags := parser.ParseHCL([]byte(data), "something.hcl")

	if !diags.HasErrors(){
		body.
	}
}

func TestFileWalkerSomething(t *testing.T) {
	rootDir := "/tmp/terraform-examples"
	filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// If there's an error accessing the path, skip it
			fmt.Fprintf(os.Stderr, "error accessing path %q: %v\n", path, err)
			return nil
		}

		// If it's a file and has a .tf extension
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".tf") {

			hcl_path := strings.ReplaceAll(path, ".tf", ".hcl")
			content, _ := os.ReadFile(path)

			config := TerraformConfig{}

			err := hclsimple.Decode(hcl_path, []byte(content), nil, &config)

			if err != nil {
				configJson, _ := json.MarshalIndent(config, "", "  ")
				fmt.Println(string(configJson))
			} else {
				log.Fatal(err)
			}
		}
		return nil
	})
}
