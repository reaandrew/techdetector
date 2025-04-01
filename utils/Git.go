package utils

import (
	"context"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type GitApi interface {
	CloneRepositoryWithContext(ctx context.Context, cloneURL, destination string, bare bool) error
	NewClone(ctx context.Context, cloneURL, destination string) Cloner
}

type GitApiClient struct{}

func (g GitApiClient) NewClone(ctx context.Context, cloneURL, destination string) Cloner {
	return NewCloneOptionsBuilder(ctx, cloneURL, destination)
}

func (g GitApiClient) CloneRepositoryWithContext(ctx context.Context, cloneURL, destination string, bare bool) error {
	if _, err := os.Stat(destination); err == nil {
		log.Printf("Repository already cloned at '%s'. Skipping clone.", destination)
		return nil
	}

	done := make(chan error, 1)

	go func() {
		_, err := git.PlainCloneContext(ctx, destination, bare, &git.CloneOptions{
			URL:      cloneURL,
			Progress: nil,
		})
		done <- err
	}()

	select {
	case <-ctx.Done():
		// Timeout or cancellation reached
		return fmt.Errorf("git clone timed out or cancelled for '%s': %w", cloneURL, ctx.Err())
	case err := <-done:
		if err != nil {
			return fmt.Errorf("git clone failed for '%s': %w", cloneURL, err)
		}
	}

	return nil
}

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

type GitCloneInfo struct {
	RepoPath     string
	BareRepoPath string
}

func (g GitApiClient) Clone(cloneBaseDir, repoUrl string) (*GitCloneInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	repoName, err := ExtractRepoName(repoUrl)
	if err != nil {
		return nil, err
	}
	repoPath := filepath.Join(cloneBaseDir, SanitizeRepoName(repoName))
	log.Infof("Cloning repository: %s\n", repoName)
	err = g.CloneRepositoryWithContext(ctx, repoUrl, repoPath, false)
	if err != nil {
		return nil, fmt.Errorf("failed to clone repository '%s': %v", repoName, err)
	}

	bareRepoPath := filepath.Join(cloneBaseDir, SanitizeRepoName(repoName)+"_bare")
	err = g.CloneRepositoryWithContext(ctx, repoUrl, bareRepoPath, true)
	if err != nil {
		return nil, fmt.Errorf("failed to perform bare clone for '%s': %v", repoName, err)
	}

	return &GitCloneInfo{
		RepoPath:     repoPath,
		BareRepoPath: bareRepoPath,
	}, nil
}

type CloneOptionsBuilder struct {
	ctx         context.Context
	cloneURL    string
	destination string
	bare        bool
	token       string
}

func NewCloneOptionsBuilder(ctx context.Context, cloneURL, destination string) *CloneOptionsBuilder {
	return &CloneOptionsBuilder{
		ctx:         ctx,
		cloneURL:    cloneURL,
		destination: destination,
	}
}

func (b *CloneOptionsBuilder) WithBare(bare bool) Cloner {
	b.bare = bare
	return b
}

func (b *CloneOptionsBuilder) WithToken(token string) Cloner {
	b.token = token
	return b
}

func (b *CloneOptionsBuilder) Clone() error {
	if _, err := os.Stat(b.destination); err == nil {
		log.Printf("Repository already cloned at '%s'. Skipping clone.", b.destination)
		return nil
	}

	cloneOptions := &git.CloneOptions{
		URL:      b.cloneURL,
		Progress: nil,
	}

	if b.token != "" {
		cloneOptions.Auth = &http.BasicAuth{
			Username: "oauth2",
			Password: b.token,
		}
	}

	done := make(chan error, 1)

	go func() {
		_, err := git.PlainCloneContext(b.ctx, b.destination, b.bare, cloneOptions)
		done <- err
	}()

	select {
	case <-b.ctx.Done():
		return fmt.Errorf("git clone timed out or cancelled for '%s': %w", b.cloneURL, b.ctx.Err())
	case err := <-done:
		if err != nil {
			return fmt.Errorf("git clone failed for '%s': %w", b.cloneURL, err)
		}
	}

	return nil
}

type Cloner interface {
	WithBare(bare bool) Cloner
	WithToken(token string) Cloner
	Clone() error
}
