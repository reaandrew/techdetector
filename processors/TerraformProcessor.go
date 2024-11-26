package processors

import (
	"fmt"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/reaandrew/techdetector/core"
	"github.com/zclconf/go-cty/cty"
	"math/big"
	"strings"
)

type TerraformBlock struct {
	Type       string
	Labels     []string
	Attributes map[string]interface{}
	Blocks     []*TerraformBlock
}

type TerraformBlockProcessor interface {
	Process(block *TerraformBlock, path string, repoName string) ([]reporters.Finding, error)
}

type ModuleBlockProcessor struct{}

func (m ModuleBlockProcessor) Process(block *TerraformBlock, path string, repoName string) ([]reporters.Finding, error) {
	matches := make([]reporters.Finding, 0)
	if block.Type == "module" && len(block.Attributes) > 0 {
		matches = append(matches, reporters.Finding{
			Name:     "TF Module",
			Type:     "TF Module Use",
			Category: "",
			Properties: map[string]interface{}{
				"source": block.Attributes["source"],
			},
			RepoName: repoName,
			Path:     path,
		})
	}

	return matches, nil
}

type TerraformProcessor struct {
	processors []TerraformBlockProcessor
}

func NewTerraformProcessor() *TerraformProcessor {
	return &TerraformProcessor{processors: []TerraformBlockProcessor{
		ModuleBlockProcessor{},
	}}
}

func (t TerraformProcessor) Supports(filePath string) bool {
	return strings.HasSuffix(filePath, ".tf")
}

func (t TerraformProcessor) Process(path string, repoName string, content string) ([]reporters.Finding, error) {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL([]byte(content), path)
	if diags.HasErrors() {
		return nil, nil
	}

	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil, fmt.Errorf("Failed to get body")
	}
	tfBlocks := ParseBody(body, []byte(content))

	matches := make([]reporters.Finding, 0)
	for _, tfBlock := range tfBlocks {
		for _, processor := range t.processors {
			match, err := processor.Process(tfBlock, path, repoName)
			if err != nil {
				return nil, err
			} else {
				matches = append(matches, match...)
			}
		}
	}
	return matches, nil
}

func ParseBody(body *hclsyntax.Body, src []byte) []*TerraformBlock {
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

			tfBlock.Attributes[name] = ConvertCtyValueToGo(val)
		}

		// Recursively parse nested blocks
		tfBlock.Blocks = ParseBody(block.Body, src)

		blocks = append(blocks, tfBlock)
	}

	return blocks
}

func ConvertCtyValueToGo(val cty.Value) interface{} {
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
			list = append(list, ConvertCtyValueToGo(elem))
		}
		return list
	case val.Type().IsMapType() || val.Type().IsObjectType():
		m := make(map[string]interface{})
		for key, v := range val.AsValueMap() {
			m[key] = ConvertCtyValueToGo(v)
		}
		return m
	default:
		// For other types, return the string representation
		return val.GoString()
	}
}
