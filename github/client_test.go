package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestGetStarredRepos_SinglePage tests fetching a single page of starred repos.
func TestGetStarredRepos_SinglePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		if r.Method != http.MethodGet {
			t.Errorf("expected GET request, got %s", r.Method)
		}
		if r.URL.Path != "/user/starred" {
			t.Errorf("expected path /user/starred, got %s", r.URL.Path)
		}

		// Verify headers
		if got := r.Header.Get("Accept"); got != "application/vnd.github.star+json" {
			t.Errorf("expected Accept header application/vnd.github.star+json, got %s", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("expected Authorization header Bearer test-token, got %s", got)
		}

		// Verify query parameters
		if got := r.URL.Query().Get("per_page"); got != "100" {
			t.Errorf("expected per_page=100, got %s", got)
		}

		// Return a single page of results
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[
			{
				"starred_at": "2023-01-15T10:30:00Z",
				"repo": {
					"full_name": "owner/repo1",
					"html_url": "https://github.com/owner/repo1",
					"description": "First repository",
					"topics": ["go", "cli"]
				}
			},
			{
				"starred_at": "2023-02-20T14:45:00Z",
				"repo": {
					"full_name": "owner/repo2",
					"html_url": "https://github.com/owner/repo2",
					"description": "Second repository",
					"topics": ["python"]
				}
			}
		]`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.baseURL = server.URL

	ctx := context.Background()
	repos, err := client.GetStarredRepos(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}

	// Verify first repo
	expectedTime1, _ := time.Parse(time.RFC3339, "2023-01-15T10:30:00Z")
	if repos[0].FullName != "owner/repo1" {
		t.Errorf("expected FullName owner/repo1, got %s", repos[0].FullName)
	}
	if repos[0].HTMLURL != "https://github.com/owner/repo1" {
		t.Errorf("expected HTMLURL https://github.com/owner/repo1, got %s", repos[0].HTMLURL)
	}
	if repos[0].Description != "First repository" {
		t.Errorf("expected Description 'First repository', got %s", repos[0].Description)
	}
	if len(repos[0].Topics) != 2 || repos[0].Topics[0] != "go" || repos[0].Topics[1] != "cli" {
		t.Errorf("expected Topics [go cli], got %v", repos[0].Topics)
	}
	if !repos[0].StarredAt.Equal(expectedTime1) {
		t.Errorf("expected StarredAt %v, got %v", expectedTime1, repos[0].StarredAt)
	}

	// Verify second repo
	expectedTime2, _ := time.Parse(time.RFC3339, "2023-02-20T14:45:00Z")
	if repos[1].FullName != "owner/repo2" {
		t.Errorf("expected FullName owner/repo2, got %s", repos[1].FullName)
	}
	if repos[1].HTMLURL != "https://github.com/owner/repo2" {
		t.Errorf("expected HTMLURL https://github.com/owner/repo2, got %s", repos[1].HTMLURL)
	}
	if repos[1].Description != "Second repository" {
		t.Errorf("expected Description 'Second repository', got %s", repos[1].Description)
	}
	if len(repos[1].Topics) != 1 || repos[1].Topics[0] != "python" {
		t.Errorf("expected Topics [python], got %v", repos[1].Topics)
	}
	if !repos[1].StarredAt.Equal(expectedTime2) {
		t.Errorf("expected StarredAt %v, got %v", expectedTime2, repos[1].StarredAt)
	}
}

// TestGetStarredRepos_Pagination tests that the client follows pagination links.
func TestGetStarredRepos_Pagination(t *testing.T) {
	pageRequests := 0
	var serverURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageRequests++
		page := r.URL.Query().Get("page")

		w.Header().Set("Content-Type", "application/json")

		if page == "" || page == "1" {
			// First page - include Link header for next page
			w.Header().Set("Link", `<`+serverURL+`/user/starred?per_page=100&page=2>; rel="next"`)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[
				{
					"starred_at": "2023-01-01T00:00:00Z",
					"repo": {
						"full_name": "owner/page1",
						"html_url": "https://github.com/owner/page1",
						"description": "Page 1 repo",
						"topics": ["go"]
					}
				}
			]`))
		} else if page == "2" {
			// Second page - no Link header (last page)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[
				{
					"starred_at": "2023-02-01T00:00:00Z",
					"repo": {
						"full_name": "owner/page2",
						"html_url": "https://github.com/owner/page2",
						"description": "Page 2 repo",
						"topics": ["python"]
					}
				}
			]`))
		}
	}))
	defer server.Close()
	serverURL = server.URL

	client := NewClient("test-token")
	client.baseURL = server.URL

	ctx := context.Background()
	repos, err := client.GetStarredRepos(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pageRequests != 2 {
		t.Errorf("expected 2 page requests, got %d", pageRequests)
	}

	if len(repos) != 2 {
		t.Fatalf("expected 2 repos total, got %d", len(repos))
	}

	if repos[0].FullName != "owner/page1" {
		t.Errorf("expected first repo owner/page1, got %s", repos[0].FullName)
	}
	if repos[1].FullName != "owner/page2" {
		t.Errorf("expected second repo owner/page2, got %s", repos[1].FullName)
	}
}

// TestGetStarredRepos_EmptyResponse tests that an empty response returns an empty slice.
func TestGetStarredRepos_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.baseURL = server.URL

	ctx := context.Background()
	repos, err := client.GetStarredRepos(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repos == nil {
		t.Error("expected non-nil slice, got nil")
	}
	if len(repos) != 0 {
		t.Errorf("expected empty slice, got %d repos", len(repos))
	}
}

// TestGetStarredRepos_HTTPError tests that HTTP errors are returned as meaningful errors.
func TestGetStarredRepos_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"Bad credentials"}`))
	}))
	defer server.Close()

	client := NewClient("invalid-token")
	client.baseURL = server.URL

	ctx := context.Background()
	repos, err := client.GetStarredRepos(ctx)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if repos != nil {
		t.Errorf("expected nil repos on error, got %v", repos)
	}

	// Check that error message is meaningful
	expectedMsg := "GitHub API request failed with status 401"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}
