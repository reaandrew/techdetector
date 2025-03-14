package utils

import (
	"context"
	"github.com/google/go-github/v50/github"
	"golang.org/x/oauth2"
	"os"
)

type GithubApi interface {
	ListRepositories(org string) ([]*github.Repository, error)
}

type GithubApiClient struct {
	client *github.Client
}

func NewGithubApiClient() GithubApiClient {
	ctx := context.Background()
	token := os.Getenv("GITHUB_TOKEN")
	if token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		tc := oauth2.NewClient(ctx, ts)
		return GithubApiClient{client: github.NewClient(tc)}
	}
	return GithubApiClient{client: github.NewClient(nil)}
}

func (apiClient GithubApiClient) ListRepositories(org string) ([]*github.Repository, error) {
	var allRepos []*github.Repository
	opt := &github.RepositoryListByOrgOptions{ListOptions: github.ListOptions{PerPage: 100}}
	for {
		repos, resp, err := apiClient.client.Repositories.ListByOrg(context.Background(), org, opt)
		if err != nil {
			return nil, err
		}
		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return allRepos, nil
}
