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

/*
https://docs.docker.com/reference/dockerfile/

Instruction	Description
ADD			Add local or remote files and directories.
ARG			Use build-time variables.
CMD			Specify default commands.
COPY		Copy files and directories.
ENTRYPOINT	Specify default executable.
ENV	Set 	environment variables.
EXPOSE		Describe which ports your application is listening on.
FROM		Create a new build stage from a base image.
HEALTHCHECK	Check a container's health on startup.
LABEL		Add metadata to an image.
MAINTAINER	Specify the author of an image.
ONBUILD		Specify instructions for when the image is used in a build.
RUN			Execute build commands.
SHELL		Set the default shell of an image.
STOPSIGNAL	Specify the system call signal for exiting a container.
USER		Set user and group ID.
VOLUME		Create volume mounts.
WORKDIR		Change working directory.

*/

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
	return filename == "Dockerfile" || strings.HasPrefix(filename, "Dockerfile.")
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
		if utils.Contains(handledInstructions, instruction.Directive) {
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
