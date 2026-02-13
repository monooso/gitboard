package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// StarredRepo represents a GitHub repository that has been starred.
type StarredRepo struct {
	FullName    string
	HTMLURL     string
	Description string
	Topics      []string
	StarredAt   time.Time
}

// Client is a GitHub API client.
type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new GitHub API client with the given token.
func NewClient(token string) *Client {
	return &Client{
		token:      token,
		baseURL:    "https://api.github.com",
		httpClient: &http.Client{},
	}
}

// starredRepoResponse represents the JSON response from the GitHub API
// when fetching starred repositories.
type starredRepoResponse struct {
	StarredAt string `json:"starred_at"`
	Repo      struct {
		FullName    string   `json:"full_name"`
		HTMLURL     string   `json:"html_url"`
		Description string   `json:"description"`
		Topics      []string `json:"topics"`
	} `json:"repo"`
}

// GetStarredRepos fetches all starred repositories for the authenticated user.
// It handles pagination automatically, following Link headers until all pages
// have been retrieved.
func (c *Client) GetStarredRepos(ctx context.Context) ([]StarredRepo, error) {
	var allRepos []StarredRepo
	url := fmt.Sprintf("%s/user/starred?per_page=100", c.baseURL)

	for url != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Accept", "application/vnd.github.star+json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("GitHub API request failed with status %d", resp.StatusCode)
		}

		var pageRepos []starredRepoResponse
		if err := json.NewDecoder(resp.Body).Decode(&pageRepos); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		resp.Body.Close()

		// Convert API response to our domain model
		for _, apiRepo := range pageRepos {
			starredAt, err := time.Parse(time.RFC3339, apiRepo.StarredAt)
			if err != nil {
				return nil, fmt.Errorf("failed to parse starred_at time: %w", err)
			}

			repo := StarredRepo{
				FullName:    apiRepo.Repo.FullName,
				HTMLURL:     apiRepo.Repo.HTMLURL,
				Description: apiRepo.Repo.Description,
				Topics:      apiRepo.Repo.Topics,
				StarredAt:   starredAt,
			}

			// Ensure Topics is never nil, use empty slice instead
			if repo.Topics == nil {
				repo.Topics = []string{}
			}

			allRepos = append(allRepos, repo)
		}

		// Check for next page in Link header
		url = extractNextURL(resp.Header.Get("Link"))
	}

	// Ensure we return an empty slice rather than nil
	if allRepos == nil {
		allRepos = []StarredRepo{}
	}

	return allRepos, nil
}

// extractNextURL parses the Link header and extracts the URL for the next page.
// Returns an empty string if there is no next page.
func extractNextURL(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}

	// Link header format: <url>; rel="next", <url>; rel="last"
	links := strings.Split(linkHeader, ",")
	for _, link := range links {
		parts := strings.Split(strings.TrimSpace(link), ";")
		if len(parts) != 2 {
			continue
		}

		url := strings.Trim(strings.TrimSpace(parts[0]), "<>")
		rel := strings.TrimSpace(parts[1])

		if rel == `rel="next"` {
			return url
		}
	}

	return ""
}
