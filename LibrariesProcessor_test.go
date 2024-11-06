package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

// TestSupports verifies if the LibrariesProcessor correctly identifies supported file types.
func TestSupports(t *testing.T) {
	processor := NewLibrariesProcessor()

	tests := []struct {
		filePath      string
		shouldSupport bool
	}{
		// Supported files
		{"pom.xml", true},
		{"build.gradle", true}, // Assuming build.gradle is supported
		{"go.mod", true},
		{"package.json", true},
		{"requirements.txt", true},
		{"pyproject.toml", true},
		{"example.csproj", true},
		// Unsupported files
		{"README.md", false},
		{"Dockerfile", false},
		{"main.go", false},
		{"setup.py", false},
		{"example.txt", false},
	}

	for _, tt := range tests {
		supports := processor.Supports(tt.filePath)
		if supports != tt.shouldSupport {
			t.Errorf("Supports(%s) = %v; want %v", tt.filePath, supports, tt.shouldSupport)
		}
	}
}

// TestParsePomXML tests the parsePomXML function with various scenarios.
func TestParsePomXML(t *testing.T) {
	processor := NewLibrariesProcessor()

	tests := []struct {
		name        string
		content     string
		repoName    string
		path        string
		expected    []Finding
		expectError bool
	}{
		{
			name: "Valid pom.xml with dependencies",
			content: `
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 
         http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>
    <dependencies>
        <dependency>
            <groupId>org.springframework</groupId>
            <artifactId>spring-core</artifactId>
            <version>5.3.8</version>
        </dependency>
        <dependency>
            <groupId>com.fasterxml.jackson.core</groupId>
            <artifactId>jackson-databind</artifactId>
            <version>2.12.3</version>
        </dependency>
    </dependencies>
</project>
`,
			repoName: "test-repo",
			path:     "sample/pom.xml",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "org.springframework:spring-core",
						Language: "Java",
						Version:  "5.3.8",
					},
					Repository: "test-repo",
					Filepath:   "sample/pom.xml",
				},
				{
					Library: &Library{
						Name:     "com.fasterxml.jackson.core:jackson-databind",
						Language: "Java",
						Version:  "2.12.3",
					},
					Repository: "test-repo",
					Filepath:   "sample/pom.xml",
				},
			},
			expectError: false,
		},
		{
			name:        "Invalid XML",
			content:     `<project><Invalid></project>`,
			repoName:    "test-repo",
			path:        "sample/pom.xml",
			expected:    nil, // Expecting an error
			expectError: true,
		},
		{
			name:        "pom.xml with no dependencies",
			content:     `<project></project>`,
			repoName:    "test-repo",
			path:        "sample/pom.xml",
			expected:    []Finding{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := processor.parsePomXML(tt.content, tt.repoName, tt.path)
			if tt.expectError {
				assert.Error(t, err, "Expected an error but got none")
				return
			}

			assert.NoError(t, err, "Did not expect an error but got one")

			findings = normalizeFindings(findings)
			expected := normalizeFindings(tt.expected)

			assert.True(t, findingsEqual(findings, expected), "Findings do not match expected results")
			if !findingsEqual(findings, expected) {
				t.Logf("Actual Findings:")
				for _, finding := range findings {
					t.Logf("%+v", finding)
				}
				t.Logf("Expected Findings:")
				for _, finding := range expected {
					t.Logf("%+v", finding)
				}
			}
		})
	}
}

// TestParseGoMod tests the parseGoMod function with various scenarios.
func TestParseGoMod(t *testing.T) {
	processor := NewLibrariesProcessor()

	tests := []struct {
		name        string
		content     string
		repoName    string
		path        string
		expected    []Finding
		expectError bool
	}{
		{
			name: "Valid go.mod with require statements",
			content: `
module github.com/example/project

go 1.16

require (
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.0
)
`,
			repoName: "test-repo",
			path:     "sample/go.mod",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "github.com/sirupsen/logrus",
						Language: "Go",
						Version:  "v1.8.1",
					},
					Repository: "test-repo",
					Filepath:   "sample/go.mod",
				},
				{
					Library: &Library{
						Name:     "github.com/stretchr/testify",
						Language: "Go",
						Version:  "v1.7.0",
					},
					Repository: "test-repo",
					Filepath:   "sample/go.mod",
				},
			},
			expectError: false,
		},
		{
			name:        "go.mod with no require statements",
			content:     `module github.com/example/project`,
			repoName:    "test-repo",
			path:        "sample/go.mod",
			expected:    []Finding{},
			expectError: false,
		},
		{
			name: "go.mod with malformed require line",
			content: `
module github.com/example/project

go 1.16

require github.com/sirupsen/logrus
`,
			repoName:    "test-repo",
			path:        "sample/go.mod",
			expected:    []Finding{}, // Malformed require line should be ignored
			expectError: false,
		},
		{
			name:        "Empty go.mod",
			content:     ``,
			repoName:    "test-repo",
			path:        "sample/go.mod",
			expected:    []Finding{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := processor.parseGoMod(tt.content, tt.repoName, tt.path)
			if (err != nil) != tt.expectError {
				t.Errorf("parseGoMod() error = %v, expectError = %v", err, tt.expectError)
				return
			}

			if tt.expectError {
				// If an error is expected, no further checks are needed
				return
			}

			// Normalize both slices to handle nil vs empty slices
			findings = normalizeFindings(findings)
			expected := normalizeFindings(tt.expected)

			if !findingsEqual(findings, expected) {
				t.Errorf("parseGoMod() got %+v, want %+v", findings, expected)
				// Detailed logging for each finding
				for i, finding := range findings {
					if i >= len(expected) {
						t.Logf("Unexpected Finding %d: %+v", i, finding)
						continue
					}
					exp := expected[i]
					if !findingsEqual([]Finding{finding}, []Finding{exp}) {
						t.Logf("Finding %d mismatch:\nGot: %+v\nWant: %+v", i, finding, exp)
					}
				}
			}
		})
	}
}

// TestParsePackageJSON tests the parsePackageJSON function with various scenarios.
func TestParsePackageJSON(t *testing.T) {
	processor := NewLibrariesProcessor()

	tests := []struct {
		name        string
		content     string
		repoName    string
		path        string
		expected    []Finding
		expectError bool
	}{
		{
			name: "Valid package.json with dependencies and devDependencies",
			content: `
{
	"name": "example-project",
	"version": "1.0.0",
	"dependencies": {
		"express": "^4.17.1",
		"lodash": "^4.17.21"
	},
	"devDependencies": {
		"jest": "^26.6.3",
		"nodemon": "^2.0.7"
	}
}
`,
			repoName: "test-repo",
			path:     "sample/package.json",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "express",
						Language: "Node.js",
						Version:  "^4.17.1",
					},
					Repository: "test-repo",
					Filepath:   "sample/package.json",
				},
				{
					Library: &Library{
						Name:     "lodash",
						Language: "Node.js",
						Version:  "^4.17.21",
					},
					Repository: "test-repo",
					Filepath:   "sample/package.json",
				},
				{
					Library: &Library{
						Name:     "jest",
						Language: "Node.js",
						Version:  "^26.6.3",
					},
					Repository: "test-repo",
					Filepath:   "sample/package.json",
				},
				{
					Library: &Library{
						Name:     "nodemon",
						Language: "Node.js",
						Version:  "^2.0.7",
					},
					Repository: "test-repo",
					Filepath:   "sample/package.json",
				},
			},
			expectError: false,
		},
		{
			name: "package.json with only dependencies",
			content: `
{
	"name": "example-project",
	"version": "1.0.0",
	"dependencies": {
		"react": "^17.0.2"
	}
}
`,
			repoName: "test-repo",
			path:     "sample/package.json",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "react",
						Language: "Node.js",
						Version:  "^17.0.2",
					},
					Repository: "test-repo",
					Filepath:   "sample/package.json",
				},
			},
			expectError: false,
		},
		{
			name: "package.json with only devDependencies",
			content: `
{
	"name": "example-project",
	"version": "1.0.0",
	"devDependencies": {
		"webpack": "^5.38.1"
	}
}
`,
			repoName: "test-repo",
			path:     "sample/package.json",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "webpack",
						Language: "Node.js",
						Version:  "^5.38.1",
					},
					Repository: "test-repo",
					Filepath:   "sample/package.json",
				},
			},
			expectError: false,
		},
		{
			name:        "Malformed JSON",
			content:     `{ "name": "example-project", "dependencies": { "express": "^4.17.1", } }`,
			repoName:    "test-repo",
			path:        "sample/package.json",
			expected:    nil, // Expecting an error
			expectError: true,
		},
		{
			name: "package.json with no dependencies",
			content: `
{
	"name": "example-project",
	"version": "1.0.0"
}
`,
			repoName:    "test-repo",
			path:        "sample/package.json",
			expected:    []Finding{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := processor.parsePackageJSON(tt.content, tt.repoName, tt.path)
			if (err != nil) != tt.expectError {
				t.Errorf("parsePackageJSON() error = %v, expectError = %v", err, tt.expectError)
				return
			}

			if tt.expectError {
				// If an error is expected, no further checks are needed
				return
			}

			// Normalize both slices to handle nil vs empty slices
			findings = normalizeFindings(findings)
			expected := normalizeFindings(tt.expected)

			if !findingsEqual(findings, expected) {
				t.Errorf("parsePackageJSON() got %+v, want %+v", findings, expected)
				// Detailed logging for each finding
				for i, finding := range findings {
					if i >= len(expected) {
						t.Logf("Unexpected Finding %d: %+v", i, finding)
						continue
					}
					exp := expected[i]
					if !findingsEqual([]Finding{finding}, []Finding{exp}) {
						t.Logf("Finding %d mismatch:\nGot: %+v\nWant: %+v", i, finding, exp)
					}
				}
			}
		})
	}
}

// TestParseRequirementsTXT tests the parseRequirementsTXT function with various scenarios.
func TestParseRequirementsTXT(t *testing.T) {
	processor := NewLibrariesProcessor()

	tests := []struct {
		name        string
		content     string
		repoName    string
		path        string
		expected    []Finding
		expectError bool
	}{
		{
			name: "requirements.txt with only library names",
			content: `
numpy
pandas
scipy
`,
			repoName: "test-repo",
			path:     "sample/requirements.txt",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "numpy",
						Language: "Python",
						Version:  "N/A",
					},
					Repository: "test-repo",
					Filepath:   "sample/requirements.txt",
				},
				{
					Library: &Library{
						Name:     "pandas",
						Language: "Python",
						Version:  "N/A",
					},
					Repository: "test-repo",
					Filepath:   "sample/requirements.txt",
				},
				{
					Library: &Library{
						Name:     "scipy",
						Language: "Python",
						Version:  "N/A",
					},
					Repository: "test-repo",
					Filepath:   "sample/requirements.txt",
				},
			},
			expectError: false,
		},
		{
			name:        "Empty requirements.txt",
			content:     ``,
			repoName:    "test-repo",
			path:        "sample/requirements.txt",
			expected:    []Finding{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := processor.parseRequirementsTXT(tt.content, tt.repoName, tt.path)
			if (err != nil) != tt.expectError {
				t.Errorf("parseRequirementsTXT() error = %v, expectError = %v", err, tt.expectError)
				return
			}

			if tt.expectError {
				// If an error is expected, no further checks are needed
				return
			}

			// Normalize both slices to handle nil vs empty slices
			findings = normalizeFindings(findings)
			expected := normalizeFindings(tt.expected)

			if !findingsEqual(findings, expected) {
				t.Errorf("parseRequirementsTXT() got %+v, want %+v", findings, expected)
				// Detailed logging for each finding
				for i, finding := range findings {
					if i >= len(expected) {
						t.Logf("Unexpected Finding %d: %+v", i, finding)
						continue
					}
					exp := expected[i]
					if !findingsEqual([]Finding{finding}, []Finding{exp}) {
						t.Logf("Finding %d mismatch:\nGot: %+v\nWant: %+v", i, finding, exp)
					}
				}
			}
		})
	}
}

// TestParsePyProjectToml tests the parsePyProjectToml function with various scenarios.
func TestParsePyProjectToml(t *testing.T) {
	processor := NewLibrariesProcessor()

	tests := []struct {
		name        string
		content     string
		repoName    string
		path        string
		expected    []Finding
		expectError bool
	}{
		{
			name: "Valid pyproject.toml with dependencies and dev-dependencies",
			content: `
[tool.poetry]
name = "example-project"
version = "1.0.0"

[tool.poetry.dependencies]
python = "^3.8"
requests = "^2.25.1"

[tool.poetry.dev-dependencies]
pytest = "^6.2.4"
flake8 = "^3.9.1"
`,
			repoName: "test-repo",
			path:     "sample/pyproject.toml",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "python",
						Language: "Python",
						Version:  "^3.8",
					},
					Repository: "test-repo",
					Filepath:   "sample/pyproject.toml",
				},
				{
					Library: &Library{
						Name:     "requests",
						Language: "Python",
						Version:  "^2.25.1",
					},
					Repository: "test-repo",
					Filepath:   "sample/pyproject.toml",
				},
				{
					Library: &Library{
						Name:     "pytest",
						Language: "Python",
						Version:  "^6.2.4",
					},
					Repository: "test-repo",
					Filepath:   "sample/pyproject.toml",
				},
				{
					Library: &Library{
						Name:     "flake8",
						Language: "Python",
						Version:  "^3.9.1",
					},
					Repository: "test-repo",
					Filepath:   "sample/pyproject.toml",
				},
			},
			expectError: false,
		},
		{
			name: "pyproject.toml with only dependencies",
			content: `
[tool.poetry]
name = "example-project"
version = "1.0.0"

[tool.poetry.dependencies]
django = "^3.2"
`,
			repoName: "test-repo",
			path:     "sample/pyproject.toml",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "django",
						Language: "Python",
						Version:  "^3.2",
					},
					Repository: "test-repo",
					Filepath:   "sample/pyproject.toml",
				},
			},
			expectError: false,
		},
		{
			name: "pyproject.toml with only dev-dependencies",
			content: `
[tool.poetry]
name = "example-project"
version = "1.0.0"

[tool.poetry.dev-dependencies]
mypy = "^0.812"
`,
			repoName: "test-repo",
			path:     "sample/pyproject.toml",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "mypy",
						Language: "Python",
						Version:  "^0.812",
					},
					Repository: "test-repo",
					Filepath:   "sample/pyproject.toml",
				},
			},
			expectError: false,
		},
		{
			name:        "pyproject.toml with malformed TOML",
			content:     `[tool.poetry name = "example-project" version = "1.0.0"`,
			repoName:    "test-repo",
			path:        "sample/pyproject.toml",
			expected:    nil, // Expecting an error
			expectError: true,
		},
		{
			name:        "Empty pyproject.toml",
			content:     ``,
			repoName:    "test-repo",
			path:        "sample/pyproject.toml",
			expected:    []Finding{},
			expectError: false,
		},
		{
			name: "pyproject.toml with no dependencies",
			content: `
[tool.poetry]
name = "example-project"
version = "1.0.0"
`,
			repoName:    "test-repo",
			path:        "sample/pyproject.toml",
			expected:    []Finding{},
			expectError: false,
		},
		{
			name: "pyproject.toml with non-string dependency versions",
			content: `
[tool.poetry]
name = "example-project"
version = "1.0.0"

[tool.poetry.dependencies]
requests = { version = "^2.25.1", extras = ["security"] }
`,
			repoName:    "test-repo",
			path:        "sample/pyproject.toml",
			expected:    []Finding{}, // Since version is not a string, it should be ignored
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := processor.parsePyProjectToml(tt.content, tt.repoName, tt.path)
			if (err != nil) != tt.expectError {
				t.Errorf("parsePyProjectToml() error = %v, expectError = %v", err, tt.expectError)
				return
			}

			if tt.expectError {
				// If an error is expected, no further checks are needed
				return
			}

			// Normalize both slices to handle nil vs empty slices
			findings = normalizeFindings(findings)
			expected := normalizeFindings(tt.expected)

			if !findingsEqual(findings, expected) {
				t.Errorf("parsePyProjectToml() got %+v, want %+v", findings, expected)
				// Detailed logging for each finding
				for i, finding := range findings {
					if i >= len(expected) {
						t.Logf("Unexpected Finding %d: %+v", i, finding)
						continue
					}
					exp := expected[i]
					if !findingsEqual([]Finding{finding}, []Finding{exp}) {
						t.Logf("Finding %d mismatch:\nGot: %+v\nWant: %+v", i, finding, exp)
					}
				}
			}
		})
	}
}

// TestParseCsProj tests the parseCsProj function with various scenarios.
func TestParseCsProj(t *testing.T) {
	processor := NewLibrariesProcessor()

	tests := []struct {
		name        string
		content     string
		repoName    string
		path        string
		expected    []Finding
		expectError bool
	}{
		{
			name: "Valid .csproj with PackageReference and Reference with embedded version",
			content: `
<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net5.0</TargetFramework>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Newtonsoft.Json" Version="13.0.1" />
    <PackageReference Include="Serilog" Version="2.10.0" />
  </ItemGroup>
  <ItemGroup>
    <Reference Include="System.Data" />
    <Reference Include="System.Xml, Version=4.0.0.0, Culture=neutral, PublicKeyToken=..." />
    <Reference Include="CuttingEdge.Conditions, Version=1.2.0.11174, Culture=neutral, PublicKeyToken=984cb50dea722e99, processorArchitecture=MSIL">
      <HintPath>..\packages\CuttingEdge.Conditions.1.2.0.0\lib\NET35\CuttingEdge.Conditions.dll</HintPath>
    </Reference>
  </ItemGroup>
</Project>
`,
			repoName: "test-repo",
			path:     "sample/example.csproj",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "Newtonsoft.Json",
						Language: "C#",
						Version:  "13.0.1",
					},
					Repository: "test-repo",
					Filepath:   "sample/example.csproj",
				},
				{
					Library: &Library{
						Name:     "Serilog",
						Language: "C#",
						Version:  "2.10.0",
					},
					Repository: "test-repo",
					Filepath:   "sample/example.csproj",
				},
				{
					Library: &Library{
						Name:     "System.Data",
						Language: "C#",
						Version:  "N/A",
					},
					Repository: "test-repo",
					Filepath:   "sample/example.csproj",
				},
				{
					Library: &Library{
						Name:     "System.Xml",
						Language: "C#",
						Version:  "4.0.0.0",
					},
					Repository: "test-repo",
					Filepath:   "sample/example.csproj",
				},
				{
					Library: &Library{
						Name:     "CuttingEdge.Conditions",
						Language: "C#",
						Version:  "1.2.0.11174",
					},
					Repository: "test-repo",
					Filepath:   "sample/example.csproj",
				},
			},
			expectError: false,
		},
		{
			name: "Valid .csproj with only PackageReferences",
			content: `
<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <PackageReference Include="NUnit" Version="3.12.0" />
  </ItemGroup>
</Project>
`,
			repoName: "test-repo",
			path:     "sample/only_packages.csproj",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "NUnit",
						Language: "C#",
						Version:  "3.12.0",
					},
					Repository: "test-repo",
					Filepath:   "sample/only_packages.csproj",
				},
			},
			expectError: false,
		},
		{
			name: "Valid .csproj with only References",
			content: `
<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <Reference Include="System.Xml, Version=4.0.0.0, Culture=neutral" />
    <Reference Include="AnotherLib, Version=1.0.0.0, Culture=neutral, PublicKeyToken=abcd1234" />
  </ItemGroup>
</Project>
`,
			repoName: "test-repo",
			path:     "sample/only_references.csproj",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "System.Xml",
						Language: "C#",
						Version:  "4.0.0.0",
					},
					Repository: "test-repo",
					Filepath:   "sample/only_references.csproj",
				},
				{
					Library: &Library{
						Name:     "AnotherLib",
						Language: "C#",
						Version:  "1.0.0.0",
					},
					Repository: "test-repo",
					Filepath:   "sample/only_references.csproj",
				},
			},
			expectError: false,
		},
		{
			name: "Valid .csproj with References having separate Version attributes",
			content: `
<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <Reference Include="System.Net.Http" Version="4.3.4" />
    <Reference Include="System.Data" />
  </ItemGroup>
</Project>
`,
			repoName: "test-repo",
			path:     "sample/separate_versions.csproj",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "System.Net.Http",
						Language: "C#",
						Version:  "4.3.4",
					},
					Repository: "test-repo",
					Filepath:   "sample/separate_versions.csproj",
				},
				{
					Library: &Library{
						Name:     "System.Data",
						Language: "C#",
						Version:  "N/A",
					},
					Repository: "test-repo",
					Filepath:   "sample/separate_versions.csproj",
				},
			},
			expectError: false,
		},
		{
			name:        "Malformed XML in .csproj",
			content:     `<Project><Invalid></Project>`,
			repoName:    "test-repo",
			path:        "sample/malformed.csproj",
			expected:    nil, // Expecting an error
			expectError: true,
		},
		{
			name: "Empty .csproj",
			content: `
<Project Sdk="Microsoft.NET.Sdk">
</Project>
`,
			repoName:    "test-repo",
			path:        "sample/empty.csproj",
			expected:    []Finding{},
			expectError: false,
		},
		{
			name: "Reference with no Version attribute and no embedded version",
			content: `
<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <Reference Include="System.Drawing" />
  </ItemGroup>
</Project>
`,
			repoName: "test-repo",
			path:     "sample/no_version.csproj",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "System.Drawing",
						Language: "C#",
						Version:  "N/A",
					},
					Repository: "test-repo",
					Filepath:   "sample/no_version.csproj",
				},
			},
			expectError: false,
		},
		{
			name: "Reference with embedded version and additional attributes",
			content: `
<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <Reference Include="Example.Lib, Version=1.0.0.0, Culture=neutral, PublicKeyToken=abcd1234">
      <HintPath>..\packages\Example.Lib\lib\Example.Lib.dll</HintPath>
      <Private>True</Private>
    </Reference>
  </ItemGroup>
</Project>
`,
			repoName: "test-repo",
			path:     "sample/multiple_attributes.csproj",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "Example.Lib",
						Language: "C#",
						Version:  "1.0.0.0",
					},
					Repository: "test-repo",
					Filepath:   "sample/multiple_attributes.csproj",
				},
			},
			expectError: false,
		},
		{
			name: "Reference with Version attribute and embedded version",
			content: `
<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <Reference Include="Conflicting.Lib, Version=2.0.0.0" Version="1.0.0.0" />
  </ItemGroup>
</Project>
`,
			repoName: "test-repo",
			path:     "sample/conflicting_versions.csproj",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "Conflicting.Lib",
						Language: "C#",
						Version:  "2.0.0.0", // Embedded version takes precedence
					},
					Repository: "test-repo",
					Filepath:   "sample/conflicting_versions.csproj",
				},
			},
			expectError: false,
		},
		{
			name: "Reference with only embedded version",
			content: `
<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <Reference Include="Only.Embedded.Lib, Version=3.0.0.0" />
  </ItemGroup>
</Project>
`,
			repoName: "test-repo",
			path:     "sample/only_embedded_version.csproj",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "Only.Embedded.Lib",
						Language: "C#",
						Version:  "3.0.0.0",
					},
					Repository: "test-repo",
					Filepath:   "sample/only_embedded_version.csproj",
				},
			},
			expectError: false,
		},
		{
			name: "Reference with no Name and only attributes",
			content: `
<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <Reference Include=", Version=1.0.0.0, Culture=neutral" />
  </ItemGroup>
</Project>
`,
			repoName: "test-repo",
			path:     "sample/no_name.csproj",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "",
						Language: "C#",
						Version:  "1.0.0.0",
					},
					Repository: "test-repo",
					Filepath:   "sample/no_name.csproj",
				},
			},
			expectError: false,
		},
		{
			name: "Reference with multiple Version attributes (ambiguous)",
			content: `
<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <Reference Include="Multi.Version.Lib, Version=1.0.0.0, Version=2.0.0.0" />
  </ItemGroup>
</Project>
`,
			repoName: "test-repo",
			path:     "sample/multiple_versions.csproj",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "Multi.Version.Lib",
						Language: "C#",
						Version:  "1.0.0.0", // First occurrence is taken
					},
					Repository: "test-repo",
					Filepath:   "sample/multiple_versions.csproj",
				},
			},
			expectError: false,
		},
		{
			name: "Reference with trailing spaces and inconsistent formatting",
			content: `
<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <Reference Include="    Trailing.Space.Lib  ,  Version=1.0.0.0 , Culture=neutral " />
  </ItemGroup>
</Project>
`,
			repoName: "test-repo",
			path:     "sample/trailing_spaces.csproj",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "Trailing.Space.Lib",
						Language: "C#",
						Version:  "1.0.0.0",
					},
					Repository: "test-repo",
					Filepath:   "sample/trailing_spaces.csproj",
				},
			},
			expectError: false,
		},
		{
			name: "Reference with no Include attribute",
			content: `
<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <Reference>
      <HintPath>..\packages\NoInclude.Lib.dll</HintPath>
    </Reference>
  </ItemGroup>
</Project>
`,
			repoName:    "test-repo",
			path:        "sample/no_include.csproj",
			expected:    []Finding{},
			expectError: false,
		},
		{
			name: "Reference with empty Include attribute",
			content: `
<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <Reference Include="" />
  </ItemGroup>
</Project>
`,
			repoName:    "test-repo",
			path:        "sample/empty_include.csproj",
			expected:    []Finding{},
			expectError: false,
		},
		{
			name: "Reference with additional child elements",
			content: `
<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <Reference Include="Complex.Lib, Version=1.0.0.0, Culture=neutral, PublicKeyToken=abcd1234">
      <HintPath>..\packages\Complex.Lib\lib\Complex.Lib.dll</HintPath>
      <Private>True</Private>
    </Reference>
  </ItemGroup>
</Project>
`,
			repoName: "test-repo",
			path:     "sample/complex_reference.csproj",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "Complex.Lib",
						Language: "C#",
						Version:  "1.0.0.0",
					},
					Repository: "test-repo",
					Filepath:   "sample/complex_reference.csproj",
				},
			},
			expectError: false,
		},
		{
			name: "Reference with multiple commas in embedded version",
			content: `
<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <Reference Include="Multi.Comma.Lib, Version=1.0.0.0, Culture=neutral, Description=Lib, with commas" />
  </ItemGroup>
</Project>
`,
			repoName: "test-repo",
			path:     "sample/multiple_commas.csproj",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "Multi.Comma.Lib",
						Language: "C#",
						Version:  "1.0.0.0",
					},
					Repository: "test-repo",
					Filepath:   "sample/multiple_commas.csproj",
				},
			},
			expectError: false,
		},
		{
			name: "Reference with no library name and only Version",
			content: `
<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <Reference Include=", Version=1.0.0.0" />
  </ItemGroup>
</Project>
`,
			repoName: "test-repo",
			path:     "sample/no_name_version.csproj",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "",
						Language: "C#",
						Version:  "1.0.0.0",
					},
					Repository: "test-repo",
					Filepath:   "sample/no_name_version.csproj",
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := processor.parseCsProj(tt.content, tt.repoName, tt.path)
			if (err != nil) != tt.expectError {
				t.Errorf("parseCsProj() error = %v, expectError = %v", err, tt.expectError)
				return
			}

			if tt.expectError {
				// If an error is expected, no further checks are needed
				return
			}

			// Normalize both slices to handle nil vs empty slices
			findings = normalizeFindings(findings)
			expected := normalizeFindings(tt.expected)

			if !findingsEqual(findings, expected) {
				t.Errorf("parseCsProj() got %+v, want %+v", findings, expected)
				// Detailed logging for each finding
				for i, finding := range findings {
					if i >= len(expected) {
						t.Logf("Unexpected Finding %d: %+v", i, finding)
						continue
					}
					exp := expected[i]
					if !findingsEqual([]Finding{finding}, []Finding{exp}) {
						t.Logf("Finding %d mismatch:\nGot: %+v\nWant: %+v", i, finding, exp)
					}
				}
			}
		})
	}
}

// TestProcess tests the Process function with various file types.
func TestProcess(t *testing.T) {
	processor := NewLibrariesProcessor()

	tests := []struct {
		name        string
		filePath    string
		content     string
		repoName    string
		expected    []Finding
		expectError bool
	}{
		{
			name:     "Process valid pom.xml",
			filePath: "sample/pom.xml",
			content: `
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 
         http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>
    <dependencies>
        <dependency>
            <groupId>org.springframework</groupId>
            <artifactId>spring-core</artifactId>
            <version>5.3.8</version>
        </dependency>
    </dependencies>
</project>
`,
			repoName: "test-repo",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "org.springframework:spring-core",
						Language: "Java",
						Version:  "5.3.8",
					},
					Repository: "test-repo",
					Filepath:   "sample/pom.xml",
				},
			},
			expectError: false,
		},
		{
			name:     "Process .csproj with embedded version",
			filePath: "sample/example.csproj",
			content: `
<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <Reference Include="CuttingEdge.Conditions, Version=1.2.0.11174, Culture=neutral, PublicKeyToken=984cb50dea722e99, processorArchitecture=MSIL">
      <HintPath>..\packages\CuttingEdge.Conditions.1.2.0.0\lib\NET35\CuttingEdge.Conditions.dll</HintPath>
    </Reference>
  </ItemGroup>
</Project>
`,
			repoName: "test-repo",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "CuttingEdge.Conditions",
						Language: "C#",
						Version:  "1.2.0.11174",
					},
					Repository: "test-repo",
					Filepath:   "sample/example.csproj",
				},
			},
			expectError: false,
		},
		{
			name:     "Process .csproj with separate Version attributes",
			filePath: "sample/separate_versions.csproj",
			content: `
<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <Reference Include="System.Net.Http" Version="4.3.4" />
    <Reference Include="System.Data" />
  </ItemGroup>
</Project>
`,
			repoName: "test-repo",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "System.Net.Http",
						Language: "C#",
						Version:  "4.3.4",
					},
					Repository: "test-repo",
					Filepath:   "sample/separate_versions.csproj",
				},
				{
					Library: &Library{
						Name:     "System.Data",
						Language: "C#",
						Version:  "N/A",
					},
					Repository: "test-repo",
					Filepath:   "sample/separate_versions.csproj",
				},
			},
			expectError: false,
		},
		{
			name:        "Process .csproj with invalid XML",
			filePath:    "sample/malformed.csproj",
			content:     `<Project><Invalid></Project>`,
			repoName:    "test-repo",
			expected:    nil, // Expecting an error
			expectError: true,
		},
		{
			name:     "Process empty .csproj",
			filePath: "sample/empty.csproj",
			content: `
<Project Sdk="Microsoft.NET.Sdk">
</Project>
`,
			repoName:    "test-repo",
			expected:    []Finding{},
			expectError: false,
		},
		{
			name:     "Process .csproj with multiple References",
			filePath: "sample/multiple_references.csproj",
			content: `
<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <Reference Include="LibraryOne, Version=1.0.0.0, Culture=neutral" />
    <Reference Include="LibraryTwo, Version=2.0.0.0, Culture=neutral" />
  </ItemGroup>
</Project>
`,
			repoName: "test-repo",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "LibraryOne",
						Language: "C#",
						Version:  "1.0.0.0",
					},
					Repository: "test-repo",
					Filepath:   "sample/multiple_references.csproj",
				},
				{
					Library: &Library{
						Name:     "LibraryTwo",
						Language: "C#",
						Version:  "2.0.0.0",
					},
					Repository: "test-repo",
					Filepath:   "sample/multiple_references.csproj",
				},
			},
			expectError: false,
		},
		{
			name:     "Process .csproj with Reference having separate Version attribute",
			filePath: "sample/with_separate_version.csproj",
			content: `
<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <Reference Include="Separate.Version.Lib" Version="1.1.1" />
  </ItemGroup>
</Project>
`,
			repoName: "test-repo",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "Separate.Version.Lib",
						Language: "C#",
						Version:  "1.1.1",
					},
					Repository: "test-repo",
					Filepath:   "sample/with_separate_version.csproj",
				},
			},
			expectError: false,
		},
		{
			name:     "Process .csproj with Reference having embedded version and separate Version attribute",
			filePath: "sample/embedded_and_separate_version.csproj",
			content: `
<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <Reference Include="Conflicting.Lib, Version=2.0.0.0" Version="1.0.0.0" />
  </ItemGroup>
</Project>
`,
			repoName: "test-repo",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "Conflicting.Lib",
						Language: "C#",
						Version:  "2.0.0.0", // Embedded version takes precedence
					},
					Repository: "test-repo",
					Filepath:   "sample/embedded_and_separate_version.csproj",
				},
			},
			expectError: false,
		},
		{
			name:     "Process .csproj with Reference having multiple commas in embedded version",
			filePath: "sample/multiple_commas.csproj",
			content: `
<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <Reference Include="Complex.Lib, Version=1.0.0.0, Culture=neutral, Description=Lib, with commas" />
  </ItemGroup>
</Project>
`,
			repoName: "test-repo",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "Complex.Lib",
						Language: "C#",
						Version:  "1.0.0.0",
					},
					Repository: "test-repo",
					Filepath:   "sample/multiple_commas.csproj",
				},
			},
			expectError: false,
		},
		{
			name:     "Process .csproj with Reference having no library name",
			filePath: "sample/no_library_name.csproj",
			content: `
<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <Reference Include=", Version=1.0.0.0, Culture=neutral" />
  </ItemGroup>
</Project>
`,
			repoName: "test-repo",
			expected: []Finding{
				{
					Library: &Library{
						Name:     "",
						Language: "C#",
						Version:  "1.0.0.0",
					},
					Repository: "test-repo",
					Filepath:   "sample/no_library_name.csproj",
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := processor.Process(tt.filePath, tt.repoName, tt.content)
			if (err != nil) != tt.expectError {
				t.Errorf("Process() error = %v, expectError = %v", err, tt.expectError)
				return
			}

			if tt.expectError {
				// If an error is expected, no further checks are needed
				return
			}

			// Normalize both slices to handle nil vs empty slices
			findings = normalizeFindings(findings)
			expected := normalizeFindings(tt.expected)

			if !findingsEqual(findings, expected) {
				t.Errorf("Process() got %+v, want %+v", findings, expected)
				// Detailed logging for each finding
				for i, finding := range findings {
					if i >= len(expected) {
						t.Logf("Unexpected Finding %d: %+v", i, finding)
						continue
					}
					exp := expected[i]
					if !findingsEqual([]Finding{finding}, []Finding{exp}) {
						t.Logf("Finding %d mismatch:\nGot: %+v\nWant: %+v", i, finding, exp)
					}
				}
			}
		})
	}
}

func normalizeFindings(findings []Finding) []Finding {
	if findings == nil {
		return []Finding{}
	}
	return findings
}

func findingsEqual(a, b []Finding) bool {
	if len(a) != len(b) {
		return false
	}

	// Create maps to track findings
	mapA := make(map[string]Library)
	mapB := make(map[string]Library)

	for _, finding := range a {
		key := finding.Repository + "|" + finding.Filepath + "|" + finding.Library.Name
		if finding.Library != nil {
			mapA[key] = *finding.Library
		} else {
			// Handle nil Library if necessary
			mapA[key] = Library{}
		}
	}

	for _, finding := range b {
		key := finding.Repository + "|" + finding.Filepath + "|" + finding.Library.Name
		if finding.Library != nil {
			mapB[key] = *finding.Library
		} else {
			// Handle nil Library if necessary
			mapB[key] = Library{}
		}
	}

	// Compare the maps
	for key, libA := range mapA {
		libB, exists := mapB[key]
		if !exists {
			return false
		}
		if libA.Name != libB.Name || libA.Language != libB.Language || libA.Version != libB.Version {
			return false
		}
	}

	return true
}
