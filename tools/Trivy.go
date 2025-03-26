package tools

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/reaandrew/techdetector/core"
	log "github.com/sirupsen/logrus"
)

// TrivyVersion is the pinned version you want to install.
// Adjust as needed. Also consider how you'd verify checksums, etc.
var TrivyVersion = "v0.60.0"

// TrivyResult helps unmarshal Trivy's JSON output. You can expand
// or trim fields depending on what you care about.
type TrivyResult struct {
	Results []struct {
		Target          string `json:"Target"`
		Class           string `json:"Class"`
		Type            string `json:"Type"`
		Vulnerabilities []struct {
			VulnerabilityID  string `json:"VulnerabilityID"`
			PkgName          string `json:"PkgName"`
			InstalledVersion string `json:"InstalledVersion"`
			FixedVersion     string `json:"FixedVersion"`
			Severity         string `json:"Severity"`
			Title            string `json:"Title"`
			Description      string `json:"Description"`
		} `json:"Vulnerabilities"`
	} `json:"Results"`
}

// Initialize ensures Trivy is installed (downloading if needed) and updates
// the local vulnerability database so scanning is current.
func InitializeTrivy() error {
	log.Debug("Initializing Trivy...")

	// 1) Ensure binary is installed
	if err := ensureTrivyInstalled(); err != nil {
		return fmt.Errorf("failed to install or locate Trivy: %w", err)
	}

	// 2) Update DB so scans are current
	if err := updateTrivyDB(); err != nil {
		return fmt.Errorf("failed to update Trivy DB: %w", err)
	}

	log.Debug("Trivy initialization complete.")
	return nil
}

// Scan runs "trivy repo --format json" against the given path, parses, filters for
// CRITICAL or HIGH vulnerabilities with no fixed version, and returns them as Findings.
// You can adjust filtering logic or add flags to the Trivy command as needed.
func TrivyScan(repoPath, repoName string) ([]core.Finding, error) {
	// 1) Create a temp file to capture JSON output
	tmpFile, err := os.CreateTemp("", "trivy-output-*.json")
	if err != nil {
		return nil, fmt.Errorf("cannot create temp file for trivy output: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// 2) Execute Trivy in "repo" mode with JSON output
	cmd := exec.Command("trivy",
		"fs",
		"--format", "json",
		"--output", tmpFile.Name(),
		repoPath,
	)
	// Add flags here if you want to skip updates, ignore unfixed, etc.
	// e.g., "--ignore-unfixed", "--skip-db-update", etc.

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("trivy scan failed: %w", err)
	}

	// 3) Read the JSON output
	outputBytes, err := os.ReadFile(tmpFile.Name())
	log.Debugf("Output: %v", string(outputBytes))

	if err != nil {
		return nil, fmt.Errorf("failed to read trivy output: %w", err)
	}

	// 4) Parse the JSON
	var trivyOutput TrivyResult
	if err := json.Unmarshal(outputBytes, &trivyOutput); err != nil {
		return nil, fmt.Errorf("failed to parse trivy JSON: %w", err)
	}

	// 5) Filter and transform into our standard Finding struct
	var findings []core.Finding
	log.Infof("Found %d trivy results", len(trivyOutput.Results))
	for _, result := range trivyOutput.Results {
		result_type := result.Type
		target := result.Target
		for _, vuln := range result.Vulnerabilities {
			findings = append(findings, core.Finding{
				Name:     vuln.VulnerabilityID,
				Type:     "VULNERABILITY_SCAN",
				Category: "Trivy Filesystem",
				Properties: map[string]interface{}{
					"title":             vuln.Title,
					"severity":          vuln.Severity,
					"installed_version": vuln.InstalledVersion,
					"pkg_name":          vuln.PkgName,
					"target":            target,
					"result_type":       result_type,
				},
				RepoName: repoName,
			})
		}
	}

	for _, finding := range findings {
		log.Debugf("Finding %v", finding)
	}
	return findings, nil
}

// ----------------------------------------------------------------------------
// Internal helpers
// ----------------------------------------------------------------------------

// ensureTrivyInstalled checks if `trivy` is present in PATH. If not, we download
// and extract the pinned version. For Linux/macOS, we handle tar.gz; Windows zip
// logic can be added if desired.
func ensureTrivyInstalled() error {
	if _, err := exec.LookPath("trivy"); err == nil {
		log.Debug("Trivy is already installed or available in PATH.")
		return nil
	}
	log.Debug("Trivy not found; downloading...")

	downloadURL, fileName, err := getTrivyDownloadURL()
	if err != nil {
		return fmt.Errorf("failed to build Trivy download URL: %w", err)
	}

	// Download the archive
	if err := downloadFile(downloadURL, fileName); err != nil {
		return err
	}

	// Extract it
	if strings.HasSuffix(fileName, ".tar.gz") {
		if err := extractTarGz(fileName); err != nil {
			return fmt.Errorf("failed to extract tar.gz: %w", err)
		}
		// Move to /usr/local/bin (requires privileges)
		if err := os.Rename("trivy", "/usr/local/bin/trivy"); err != nil {
			return fmt.Errorf("failed to move trivy binary: %w", err)
		}
		if err := os.Chmod("/usr/local/bin/trivy", 0755); err != nil {
			return fmt.Errorf("failed to chmod trivy binary: %w", err)
		}
		log.Debugf("Trivy installed to /usr/local/bin/trivy")
	} else if strings.HasSuffix(fileName, ".zip") {
		// Implement Windows logic if needed
		return fmt.Errorf("zip extraction not implemented in this example")
	} else {
		return fmt.Errorf("unknown archive format: %s", fileName)
	}

	log.Debug("Trivy installation complete.")
	return nil
}

// updateTrivyDB runs "trivy db update" to ensure the local DB is current.
// Adjust or skip if you prefer other behaviors (e.g., offline DB).
func updateTrivyDB() error {
	log.Debug("Updating Trivy database...")
	cmd := exec.Command("trivy", "db", "update")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("trivy db update failed: %w", err)
	}
	log.Debug("Trivy database updated.")
	return nil
}

// getTrivyDownloadURL builds the direct download URL for the pinned version
// according to the OS/architecture. Adjust as needed for official release asset names.
func getTrivyDownloadURL() (string, string, error) {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	var fileSuffix string
	switch {
	case osName == "linux" && arch == "amd64":
		fileSuffix = "Linux-64bit.tar.gz"
	case osName == "darwin" && arch == "amd64":
		fileSuffix = "macOS-64bit.tar.gz"
	case osName == "darwin" && arch == "arm64":
		fileSuffix = "macOS-ARM64.tar.gz"
	case osName == "windows" && arch == "amd64":
		fileSuffix = "Windows-64bit.zip"
	default:
		return "", "", fmt.Errorf("unsupported platform: %s/%s", osName, arch)
	}

	fileName := fmt.Sprintf("trivy_%s_%s", strings.TrimPrefix(TrivyVersion, "v"), fileSuffix)
	url := fmt.Sprintf("https://github.com/aquasecurity/trivy/releases/download/%s/%s", TrivyVersion, fileName)
	return url, fileName, nil
}

// downloadFile fetches the given URL and writes to a local file with the specified name.
func downloadFile(url, fileName string) error {
	log.Debugf("Downloading Trivy from %s ...", url)
	out, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("unable to create file for download: %w", err)
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download trivy: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status code downloading trivy: %d", resp.StatusCode)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("failed to save trivy archive: %w", err)
	}
	return nil
}

// extractTarGz untars a .tar.gz file into the current working directory.
func extractTarGz(tarFile string) error {
	f, err := os.Open(tarFile)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tarReader := tar.NewReader(gz)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(header.Name, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := os.Create(header.Name)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		default:
			// handle other types if needed
		}
	}
	return nil
}
