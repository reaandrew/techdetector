package processors

import (
	"bufio"
	"fmt"
	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/utils"
	"io"
	"path/filepath"
	"strings"
)

type DockerInstruction struct {
	Directive string
	Arguments string
}

func ParseDockerfile(reader io.Reader) ([]DockerInstruction, error) {
	var instructions []DockerInstruction
	var currentCommand strings.Builder

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)

		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			continue
		}

		if strings.HasSuffix(trimmedLine, "\\") {
			trimmedLine = strings.TrimRight(trimmedLine, "\\")
			currentCommand.WriteString(trimmedLine + " ")
			continue
		} else {
			currentCommand.WriteString(trimmedLine)
			commandStr := currentCommand.String()
			if commandStr != "" {
				instruction, err := parseInstruction(commandStr)
				if err != nil {
					return nil, err
				}
				instructions = append(instructions, instruction)
			}
			currentCommand.Reset()
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if currentCommand.Len() > 0 {
		commandStr := currentCommand.String()
		if commandStr != "" {
			instruction, err := parseInstruction(commandStr)
			if err != nil {
				return nil, err
			}
			instructions = append(instructions, instruction)
		}
	}

	return instructions, nil
}

func parseInstruction(line string) (DockerInstruction, error) {
	commentIndex := strings.Index(line, "#")
	if commentIndex != -1 {
		line = strings.TrimSpace(line[:commentIndex])
	}

	parts := strings.Fields(line)
	if len(parts) == 0 {
		return DockerInstruction{}, fmt.Errorf("empty instruction")
	}

	directive := strings.ToUpper(parts[0])

	arguments := strings.TrimSpace(line[len(parts[0]):])

	return DockerInstruction{
		Directive: directive,
		Arguments: arguments,
	}, nil
}

type DockerProcessor struct {
}

func (d DockerProcessor) Supports(filePath string) bool {
	filename := filepath.Base(filePath)
	supported := filename == "Dockerfile" || strings.HasPrefix(filename, "Dockerfile.")
	return supported
}

func (d DockerProcessor) Process(path string, repoName string, content string) ([]core.Finding, error) {
	var matches []core.Finding
	reader := strings.NewReader(content)
	instructions, err := ParseDockerfile(reader)
	if err != nil {
		return nil, err
	}
	handledInstructions := []string{"MAINTAINER", "LABEL", "FROM", "EXPOSE"}

	for _, instruction := range instructions {
		if instruction.Directive == "FROM" {
			// Handle multi-stage FROM directives with AS
			imageParts, alias := parseFromDirective(instruction.Arguments)
			properties := map[string]interface{}{
				"owner":   imageParts["owner"],
				"image":   imageParts["image"],
				"version": imageParts["version"],
			}
			if alias != "" {
				properties["alias"] = alias
			}

			matches = append(matches, core.Finding{
				Name:       instruction.Directive,
				Type:       "Docker Directive",
				Category:   "",
				Properties: properties,
				Path:       path,
				RepoName:   repoName,
			})
		} else if utils.Contains(handledInstructions, instruction.Directive) {
			matches = append(matches, core.Finding{
				Name:     instruction.Directive,
				Type:     "Docker Directive",
				Category: "",
				Properties: map[string]interface{}{
					"arguments": instruction.Arguments,
				},
				Path:     path,
				RepoName: repoName,
			})
		}
	}
	return matches, nil
}

// Helper function to parse the FROM directive
func parseFromDirective(arguments string) (map[string]string, string) {
	alias := ""
	if strings.Contains(arguments, " AS ") {
		parts := strings.SplitN(arguments, " AS ", 2)
		arguments = parts[0]
		alias = strings.TrimSpace(parts[1])
	}

	imageParts := splitDockerImage(arguments)
	return imageParts, alias
}

// Helper function to split a Docker image string into components
func splitDockerImage(image string) map[string]string {
	parts := strings.Split(image, "/")
	result := map[string]string{
		"owner":   "",
		"image":   "",
		"version": "",
	}

	if len(parts) == 2 {
		result["owner"] = parts[0]
		imageWithVersion := parts[1]
		imageVersionParts := strings.SplitN(imageWithVersion, ":", 2)
		result["image"] = imageVersionParts[0]
		if len(imageVersionParts) == 2 {
			result["version"] = imageVersionParts[1]
		}
	} else if len(parts) == 1 {
		imageVersionParts := strings.SplitN(parts[0], ":", 2)
		result["image"] = imageVersionParts[0]
		if len(imageVersionParts) == 2 {
			result["version"] = imageVersionParts[1]
		}
	}

	return result
}
