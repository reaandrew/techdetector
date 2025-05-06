# TechDetector

**TechDetector** is a powerful command-line interface (CLI) tool designed to scan Git repositories or GitHub organizations for various technologies, libraries, frameworks, and Docker configurations. By analyzing your codebase, TechDetector helps you identify the technologies in use, enabling better insights, compliance, and maintenance of your projects.

## Table of Contents

- [Features](#features)
- [Installation](#installation)
    - [Install from Source](#install-from-source)
    - [Download Latest Release](#download-latest-release)
- [Usage](#usage)
    - [Scanning a Single Repository](#scanning-a-single-repository)
    - [Scanning a GitHub Organization](#scanning-a-github-organization)
- [Report Formats](#report-formats)
- [Supported Technologies](#supported-technologies)
    - [Applications](#applications)
    - [Cloud Services](#cloud-services)
    - [Frameworks](#frameworks)
    - [Libraries](#libraries)
    - [Docker Directives](#docker-directives)
- [Pattern Files](#pattern-files)
- [Contributing](#contributing)
- [License](#license)
- [Contact](#contact)

## Features

- **Comprehensive Scanning**: Analyze entire repositories or all repositories within a GitHub organization.

- **Multi-Language Support**: Detect technologies across various programming languages including Go, Python, Java, JavaScript, C#, and more.

- **Cloud Service Detection**: Identify cloud services from AWS, Azure, and GCP based on code patterns.
    - Detect AWS and Azure resources in Terraform files
    - Detect Azure resources in Bicep files
    - Detect AWS resources in CloudFormation files

- **Framework Identification**: Recognize popular frameworks such as Spring Boot, Django, Express.js, and more.

- **Library Extraction**: Parse package files to extract library dependencies.

- **Dockerfile Analysis**: Analyze Dockerfiles to identify used directives and configurations.

- **Customizable Reports**: Generate detailed reports in XLSX format to visualize the detected technologies.

## Installation

TechDetector can be installed either by building from source or by downloading the latest release from GitHub. Choose the method that best fits your needs.

### Install from Source

Follow these steps to build and install TechDetector from source. This method requires [Go](https://golang.org/dl/) to be installed on your machine.

1. **Prerequisites**

    - **Go**: Ensure you have Go installed. You can download it from [Go's official website](https://golang.org/dl/).

2. **Clone the Repository**

   ```bash
   git clone https://github.com/yourusername/techdetector.git
   cd techdetector
   ```

3. **Build the CLI**

   ```bash
   go build -o techdetector
   ```

   This command compiles the `techdetector` binary.

4. **(Optional) Install Globally**

   To make `techdetector` accessible from anywhere in your terminal, move the binary to a directory that's in your `PATH`, such as `/usr/local/bin`.

   ```bash
   sudo mv techdetector /usr/local/bin/
   ```

5. **Verify Installation**

   ```bash
   techdetector --help
   ```

   You should see the help message displaying available commands and options.

### Download Latest Release

If you prefer not to build TechDetector from source, you can download the pre-compiled binaries from the GitHub Releases page.

1. **Navigate to the Releases Page**

   Visit the [TechDetector Releases](https://github.com/yourusername/techdetector/releases) page on GitHub.

2. **Download the Appropriate Binary**

   Find the latest release and download the binary that matches your operating system and architecture. For example:

    - `techdetector-linux-amd64`
    - `techdetector-windows-amd64.exe`
    - `techdetector-macos-amd64`

3. **Make the Binary Executable (Linux/macOS)**

   After downloading, you may need to make the binary executable.

   ```bash
   chmod +x techdetector-linux-amd64
   ```

4. **Move the Binary to Your PATH**

   To use `techdetector` from any location in your terminal, move it to a directory that's included in your `PATH`, such as `/usr/local/bin`.

   ```bash
   sudo mv techdetector-linux-amd64 /usr/local/bin/techdetector
   ```

5. **Verify Installation**

   ```bash
   techdetector --help
   ```

   You should see the help message displaying available commands and options.

## Usage

TechDetector provides a straightforward CLI with commands to scan individual repositories or entire GitHub organizations.

**IMPORTANT!** : Ensure your GITHUB_TOKEN environment variable is set.

### Scanning a Single Repository

To scan a specific Git repository:

```bash
techdetector scan repo <REPO_URL> --report=<FORMAT>
```

**Parameters:**

- `<REPO_URL>`: The URL of the Git repository you want to scan.
- `--report`: *(Optional)* The format of the generated report. Supported formats: `xlsx` (default).

**Example:**

```bash
techdetector scan repo https://github.com/yourusername/yourrepo.git --report=xlsx
```

### Scanning a GitHub Organization

To scan all repositories within a GitHub organization:

```bash
techdetector scan github_org <ORG_NAME> --report=<FORMAT>
```

**Parameters:**

- `<ORG_NAME>`: The name of the GitHub organization you want to scan.
- `--report`: *(Optional)* The format of the generated report. Supported formats: `xlsx` (default).

**Example:**

```bash
techdetector scan github_org yourorganization --report=xlsx
```

## Report Formats

Currently, TechDetector supports generating reports in the following format:

- **XLSX**: Excel format providing a comprehensive overview of detected technologies, libraries, frameworks, and Docker configurations.

Future updates may include additional formats based on user demand.

## Supported Technologies

TechDetector utilizes pattern files to detect a wide range of technologies across different languages and platforms. Below is an overview of the supported categories and examples.

### Applications

Detect various applications based on file names or specific patterns within configuration files.

**Example Patterns:**

- **Apache HTTP Server (httpd)**
    - **File Names**: `httpd`, `apache2`, `apache`, `httpd.conf`

### Cloud Services

Identify cloud services from major providers like AWS, Azure, and GCP by analyzing file extensions and content patterns.

**AWS Examples:**

- **Amazon Access Analyzer**
    - **File Extensions**: `.cs`, `.go`, `.java`, `.js`, `.py`
    - **Content Patterns**:
        - C#: `Amazon.AccessAnalyzer`
        - Go: `github.com/aws/aws-sdk-go-v2/service/accessanalyzer`
        - Java: `com.amazonaws.services.applicationautoscaling`
        - JavaScript: `@aws-sdk/client-accessanalyzer`
        - Python: `boto3.client('accessanalyzer')`

**Azure Examples:**

- **Azure AI AnomalyDetector**
    - **File Extensions**: `.cs`, `.go`, `.java`, `.js`, `.py`
    - **Content Patterns**:
        - C#: `using Azure.AI.AnomalyDetector`
        - Go: `github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets`
        - Java: `import com.azure.core`
        - JavaScript: `@azure/abort-controller`
        - Python: `import azure-core`

**GCP Examples:**

- **AccessApproval**
    - **File Extensions**: `.cs`, `.go`, `.java`, `.js`, `.py`
    - **Content Patterns**:
        - C#: `Google.Ads.AdManager.V1`
        - Go: `cloud.google.com/go/accessapproval`
        - Java: `com.google.cloud.accessapproval.v1`
        - JavaScript: `@google-cloud/access-approval`
        - Python: `from google.cloud import compute_v1`

### Frameworks

Detect popular frameworks based on file names and content patterns.

**Example Patterns:**

- **Spring Boot (Java)**
    - **File Names**: `pom.xml`
    - **Content Patterns**: `<groupId>org.springframework.boot</groupId>`

- **Django (Python)**
    - **File Names**: `requirements.txt`
    - **Content Patterns**: `django==`

- **Express.js (Node.js)**
    - **File Names**: `package.json`
    - **Content Patterns**: `"express":`

### Libraries

Parse various package files to extract library dependencies across multiple languages.

**Supported Files:**

- `pom.xml` (Java - Maven)
- `go.mod` (Go)
- `package.json` (Node.js)
- `requirements.txt` & `pyproject.toml` (Python)
- `*.csproj` (C#)

**Example Processing:**

- **Java (`pom.xml`)**
    - Extract `groupId`, `artifactId`, and `version`.

- **Go (`go.mod`)**
    - Extract module dependencies and versions.

- **Node.js (`package.json`)**
    - Extract dependencies and devDependencies.

- **Python (`requirements.txt`)**
    - Extract package names and versions.

- **C# (`*.csproj`)**
    - Extract `PackageReference` and `Reference` entries.

### Docker Directives

Analyze Dockerfiles to identify used directives such as `FROM`, `RUN`, `ENV`, `EXPOSE`, etc.

**Example Directives Detected:**

- `FROM`
- `EXPOSE`
- `LABEL`
- `MAINTAINER`

## Pattern Files

TechDetector uses JSON pattern files located in the `data/patterns/` directory to detect various technologies. Each pattern file corresponds to a specific technology category and language.

**Example Pattern File Structure:**

```json
[
  {
    "type": "Cloud Service",
    "name": "Amazon Access Analyzer",
    "category": "Security",
    "file_extensions": ["go"],
    "content_patterns": [
      "github.com/aws/aws-sdk-go-v2/service/accessanalyzer"
    ]
  }
]
```

**Pattern File Categories:**

- **Applications**: Detect specific applications based on file names.
- **Cloud Services**: Identify cloud services from AWS, Azure, GCP.
- **Frameworks**: Recognize development frameworks.
- **Libraries**: Parse package files to extract library dependencies.

**Customizing Patterns:**

You can add or modify pattern files to extend TechDetector's capabilities. Ensure that new patterns follow the existing JSON structure for consistency.

## Contributing

Contributions are welcome! Whether it's reporting bugs, suggesting features, or submitting pull requests, your help is appreciated.

### Steps to Contribute

1. **Fork the Repository**

   Click the "Fork" button at the top-right corner of the repository page to create your own fork.

2. **Clone Your Fork**

   ```bash
   git clone https://github.com/yourusername/techdetector.git
   cd techdetector
   ```

3. **Create a New Branch**

   ```bash
   git checkout -b feature/your-feature-name
   ```

4. **Make Your Changes**

   Implement your feature, fix bugs, or update documentation.

5. **Commit Your Changes**

   ```bash
   git commit -m "Description of your changes"
   ```

6. **Push to Your Fork**

   ```bash
   git push origin feature/your-feature-name
   ```

7. **Create a Pull Request**

   Navigate to the original repository and click "Compare & pull request" to submit your changes.

### Guidelines

- **Code Quality**: Ensure your code follows Go's best practices and is properly formatted.
- **Documentation**: Update or add documentation where necessary.
- **Tests**: Write unit tests for new features or bug fixes.
- **Commit Messages**: Write clear and descriptive commit messages.

## License

This project is licensed under the [MIT License](LICENSE).


## Contact

For any questions, suggestions, or support, feel free to reach out:

- **Email**: email@andrewrea.co.uk
- **GitHub Issues**: [https://github.com/reaandrew/techdetector/issues](https://github.com/yourusername/techdetector/issues)

