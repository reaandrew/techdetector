package utils

import (
	"context"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"go.etcd.io/bbolt"
	"os"
	"path/filepath"
)

const CacheDirName = ".techdetector_cache"
const BucketName = "Projects"

type GitlabApi interface {
	ListAllProjects() ([]*gitlab.Project, error)
	Token() string
	BaseURL() string
}

type GitlabApiClient struct {
	client  *gitlab.Client
	baseUrl string
	token   string
	noCache bool
}

func (g GitlabApiClient) Token() string {
	return g.token
}

func (g GitlabApiClient) BaseURL() string {
	return g.baseUrl
}

func NewGitlabApiClient(gitlabToken string, gitlabBaseURL string, noCache bool) *GitlabApiClient {
	if gitlabToken == "" {
		log.Fatal("GitLab token is required (provide via --gitlab-token flag)")
	}
	client, err := gitlab.NewClient(gitlabToken, gitlab.WithBaseURL(gitlabBaseURL))
	if err != nil {
		log.Fatalf("Failed to create GitLab client: %v", err)
	}
	return &GitlabApiClient{
		client:  client,
		baseUrl: gitlabBaseURL,
		token:   gitlabToken,
		noCache: noCache,
	}
}

func (g GitlabApiClient) fetchAllProjects() ([]*gitlab.Project, error) {
	var allProjects []*gitlab.Project
	ctx := context.Background()
	opts := &gitlab.ListProjectsOptions{
		ListOptions: gitlab.ListOptions{
			Page:    1,
			PerPage: 100,
		},
	}

	for {
		projects, resp, err := g.client.Projects.ListProjects(opts, gitlab.WithContext(ctx))
		if err != nil {
			return nil, fmt.Errorf("failed to list projects: %w", err)
		}

		allProjects = append(allProjects, projects...)
		if err := g.saveProjectsToCache(g.baseUrl, allProjects); err != nil {
			log.Printf("Failed to save to cache: %v", err)
		}

		if resp.NextPage == 0 {
			break
		}

		fmt.Fprintf(os.Stderr, "Loaded %d projects \n", len(allProjects))
		opts.Page = resp.NextPage
		log.Printf("Fetched %d projects, total so far: %d\n", len(projects), len(allProjects))
	}

	log.Printf("Number of projects found: %v\n", len(allProjects))
	return allProjects, nil
}

func (g GitlabApiClient) ListAllProjects() ([]*gitlab.Project, error) {
	if g.noCache {
		return g.fetchAllProjects()
	}

	projects, err := g.loadProjectsFromCache(g.baseUrl)

	if err != nil {
		log.Printf("Failed to load from cache, proceeding with API fetch: %v", err)
	} else {
		log.Printf("Loaded %d projects from cache.\n", len(projects))
	}

	return projects, err
}

func (g GitlabApiClient) loadProjectsFromCache(baseURL string) ([]*gitlab.Project, error) {
	cacheFile, err := g.getCacheFile(baseURL)
	if err != nil {
		return nil, err
	}

	db, err := bbolt.Open(cacheFile, 0666, nil)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var projects []*gitlab.Project
	err = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(BucketName))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}

		return b.ForEach(func(k, v []byte) error {
			var project gitlab.Project
			if err := json.Unmarshal(v, &project); err != nil {
				return err
			}
			projects = append(projects, &project)
			return nil
		})
	})
	return projects, err
}

func (g GitlabApiClient) saveProjectsToCache(baseURL string, projects []*gitlab.Project) error {
	cacheFile, err := g.getCacheFile(baseURL)
	if err != nil {
		return err
	}

	db, err := bbolt.Open(cacheFile, 0666, nil)
	if err != nil {
		return err
	}
	defer db.Close()

	return db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(BucketName))
		if err != nil {
			return fmt.Errorf("create bucket: %w", err)
		}

		for _, project := range projects {
			data, _ := json.Marshal(project)
			if err := b.Put([]byte(project.PathWithNamespace), data); err != nil {
				return err
			}
		}
		return nil
	})
}

func (g GitlabApiClient) getCacheFile(baseURL string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	cacheDir := filepath.Join(homeDir, CacheDirName)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}

	cacheFileName := fmt.Sprintf("%s_projects_cache.db", Sanitize(baseURL))
	return filepath.Join(cacheDir, cacheFileName), nil
}
