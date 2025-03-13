package utils

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	log "github.com/sirupsen/logrus"
	"os"
	"strings"
)

func SanitizeRepoName(fullName string) string {
	return strings.ReplaceAll(fullName, "/", "_")
}

func ExtractRepoName(repoURL string) (string, error) {
	var repoName string
	if strings.HasPrefix(repoURL, "git@") {
		parts := strings.Split(repoURL, ":")
		if len(parts) != 2 {
			return "", fmt.Errorf("unexpected repository URL format")
		}
		repoName = strings.TrimSuffix(parts[1], ".git")
	} else if strings.HasPrefix(repoURL, "https://") || strings.HasPrefix(repoURL, "http://") {
		parts := strings.Split(repoURL, "/")
		if len(parts) < 2 {
			return "", fmt.Errorf("unexpected repository URL format")
		}
		repoName = strings.TrimSuffix(parts[len(parts)-1], ".git")
	} else {
		return "", fmt.Errorf("unsupported repository URL format")
	}
	return repoName, nil
}

func CloneRepository(cloneURL, destination string, bare bool) error {
	return CloneRepositoryWithToken(cloneURL, destination, bare, "")
}

func CloneRepositoryWithToken(cloneURL, destination string, bare bool, token string) error {
	if _, err := os.Stat(destination); err == nil {
		log.Printf("Repository already cloned at '%s'. Skipping clone.", destination)
		return nil
	}

	cloneOptions := &git.CloneOptions{
		URL:      cloneURL,
		Progress: nil,
	}

	// If a token is provided, set up basic authentication.
	if token != "" {
		cloneOptions.Auth = &http.BasicAuth{
			Username: "oauth2", // GitLab requires "oauth2" as the username for token auth.
			Password: token,
		}
	}

	if bare {
		log.Printf("Performing a bare clone to '%s'.", destination)
		_, err := git.PlainClone(destination, true, cloneOptions) // true = bare clone
		if err != nil {
			return fmt.Errorf("git bare clone failed: %w", err)
		}
	} else {
		_, err := git.PlainClone(destination, false, cloneOptions)
		if err != nil {
			return fmt.Errorf("git clone failed: %w", err)
		}
	}

	return nil
}
