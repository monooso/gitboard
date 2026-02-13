package pinboard

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Bookmark represents a Pinboard bookmark.
type Bookmark struct {
	URL         string
	Title       string
	Description string
	Tags        []string
	Private     bool
	ToRead      bool
}

// Client is a Pinboard v1 API client.
type Client struct {
	authToken  string
	baseURL    string
	httpClient *http.Client
	lastCall   time.Time
	minDelay   time.Duration
}

// NewClient creates a new Pinboard API client with the given auth token.
// It points at the Pinboard v1 API endpoint by default.
func NewClient(authToken string) *Client {
	return &Client{
		authToken:  authToken,
		baseURL:    "https://api.pinboard.in/v1",
		httpClient: &http.Client{},
		minDelay:   3 * time.Second,
	}
}

// GetBookmarkURLsByTag fetches all bookmark URLs that have the given tag.
// Returns a set of URLs (map[string]bool) for efficient lookups.
func (c *Client) GetBookmarkURLsByTag(ctx context.Context, tag string) (map[string]bool, error) {
	// Construct the URL for the posts/all endpoint
	apiURL := fmt.Sprintf("%s/posts/all", c.baseURL)

	// Prepare query parameters
	queryParams := url.Values{}
	queryParams.Set("auth_token", c.authToken)
	queryParams.Set("tag", tag)
	queryParams.Set("format", "json")

	// Construct the full URL with query parameters
	fullURL := fmt.Sprintf("%s?%s", apiURL, queryParams.Encode())

	// Create GET request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check for HTTP 200 status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse JSON response
	var bookmarks []struct {
		Href string `json:"href"`
	}
	if err := json.Unmarshal(body, &bookmarks); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Build the URL set
	urls := make(map[string]bool)
	for _, bookmark := range bookmarks {
		urls[bookmark.Href] = true
	}

	return urls, nil
}

// AddBookmark creates or updates a bookmark on Pinboard.
func (c *Client) AddBookmark(ctx context.Context, b Bookmark) error {
	// Enforce rate limiting, respecting context cancellation.
	if !c.lastCall.IsZero() {
		elapsed := time.Since(c.lastCall)
		if elapsed < c.minDelay {
			timer := time.NewTimer(c.minDelay - elapsed)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	c.lastCall = time.Now()

	// Construct the base URL for v1 posts/add endpoint
	apiURL := fmt.Sprintf("%s/posts/add", c.baseURL)

	// Prepare query parameters
	queryParams := url.Values{}
	queryParams.Set("auth_token", c.authToken)
	queryParams.Set("url", b.URL)
	queryParams.Set("description", b.Title)        // v1 API uses "description" for the title
	queryParams.Set("extended", b.Description)     // v1 API uses "extended" for the description
	queryParams.Set("tags", strings.Join(b.Tags, " "))
	queryParams.Set("replace", "yes")              // Always yes for idempotent exports
	queryParams.Set("format", "json")

	// Map boolean fields to v1 API format
	if b.Private {
		queryParams.Set("shared", "no")
	} else {
		queryParams.Set("shared", "yes")
	}

	if b.ToRead {
		queryParams.Set("toread", "yes")
	} else {
		queryParams.Set("toread", "no")
	}

	// Construct the full URL with query parameters
	fullURL := fmt.Sprintf("%s?%s", apiURL, queryParams.Encode())

	// Create GET request (v1 API uses GET, not POST)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check for HTTP 200 status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var result struct {
		ResultCode string `json:"result_code"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for success result_code
	if result.ResultCode != "done" {
		return fmt.Errorf("API error: result_code was %q", result.ResultCode)
	}

	return nil
}
