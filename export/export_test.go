package export

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/monooso/gitboard/github"
	"github.com/monooso/gitboard/pinboard"
)

// Mock implementations for testing

type mockGitHubClient struct {
	repos []github.StarredRepo
	err   error
}

func (m *mockGitHubClient) GetStarredRepos(ctx context.Context) ([]github.StarredRepo, error) {
	return m.repos, m.err
}

type mockPinboardClient struct {
	addedBookmarks []pinboard.Bookmark
	existingURLs   map[string]bool
	err            error
	getErr         error
}

func (m *mockPinboardClient) AddBookmark(ctx context.Context, b pinboard.Bookmark) error {
	if m.err != nil {
		return m.err
	}
	m.addedBookmarks = append(m.addedBookmarks, b)
	return nil
}

func (m *mockPinboardClient) GetBookmarkURLsByTag(ctx context.Context, tag string) (map[string]bool, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.existingURLs == nil {
		return map[string]bool{}, nil
	}
	return m.existingURLs, nil
}

// Test NormaliseTags
func TestNormaliseTags(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "mixed case",
			input:    []string{"GoLang", "WebDev"},
			expected: []string{"golang", "webdev"},
		},
		{
			name:     "spaces to hyphens",
			input:    []string{"machine learning", "data science"},
			expected: []string{"machine-learning", "data-science"},
		},
		{
			name:     "already normalised",
			input:    []string{"golang", "web-development"},
			expected: []string{"golang", "web-development"},
		},
		{
			name:     "empty slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "mixed case with spaces",
			input:    []string{"Go Lang", "WEB Dev"},
			expected: []string{"go-lang", "web-dev"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormaliseTags(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d tags, got %d", len(tt.expected), len(result))
			}
			for i, tag := range result {
				if tag != tt.expected[i] {
					t.Errorf("expected tag %q, got %q", tt.expected[i], tag)
				}
			}
		})
	}
}

// Test RepoToBookmark with topics
func TestRepoToBookmark(t *testing.T) {
	repo := github.StarredRepo{
		FullName:    "golang/go",
		HTMLURL:     "https://github.com/golang/go",
		Description: "The Go programming language",
		Topics:      []string{"Language", "Compiler"},
		StarredAt:   time.Now(),
	}

	bookmark := RepoToBookmark(repo)

	if bookmark.URL != repo.HTMLURL {
		t.Errorf("expected URL %q, got %q", repo.HTMLURL, bookmark.URL)
	}
	if bookmark.Title != repo.FullName {
		t.Errorf("expected Title %q, got %q", repo.FullName, bookmark.Title)
	}
	if bookmark.Description != repo.Description {
		t.Errorf("expected Description %q, got %q", repo.Description, bookmark.Description)
	}
	if !bookmark.Private {
		t.Error("expected Private to be true")
	}
	if bookmark.ToRead {
		t.Error("expected ToRead to be false")
	}

	// Check tags: should have "github-repo" plus normalised topics
	expectedTags := []string{"github-repo", "language", "compiler"}
	if len(bookmark.Tags) != len(expectedTags) {
		t.Fatalf("expected %d tags, got %d", len(expectedTags), len(bookmark.Tags))
	}
	for i, tag := range bookmark.Tags {
		if tag != expectedTags[i] {
			t.Errorf("expected tag %q, got %q", expectedTags[i], tag)
		}
	}
}

// Test RepoToBookmark with empty topics
func TestRepoToBookmarkEmptyTopics(t *testing.T) {
	repo := github.StarredRepo{
		FullName:    "user/repo",
		HTMLURL:     "https://github.com/user/repo",
		Description: "A repository",
		Topics:      []string{},
		StarredAt:   time.Now(),
	}

	bookmark := RepoToBookmark(repo)

	// Should still have "github-repo" tag
	expectedTags := []string{"github-repo"}
	if len(bookmark.Tags) != len(expectedTags) {
		t.Fatalf("expected %d tags, got %d", len(expectedTags), len(bookmark.Tags))
	}
	if bookmark.Tags[0] != "github-repo" {
		t.Errorf("expected tag %q, got %q", "github-repo", bookmark.Tags[0])
	}
}

// Test RepoToBookmark with empty description
func TestRepoToBookmarkEmptyDescription(t *testing.T) {
	repo := github.StarredRepo{
		FullName:    "user/repo",
		HTMLURL:     "https://github.com/user/repo",
		Description: "",
		Topics:      []string{"topic"},
		StarredAt:   time.Now(),
	}

	bookmark := RepoToBookmark(repo)

	if bookmark.Description != "" {
		t.Errorf("expected empty Description, got %q", bookmark.Description)
	}
}

// Test Run with normal operation
func TestRun(t *testing.T) {
	mockRepos := []github.StarredRepo{
		{
			FullName:    "golang/go",
			HTMLURL:     "https://github.com/golang/go",
			Description: "The Go programming language",
			Topics:      []string{"language"},
			StarredAt:   time.Now(),
		},
		{
			FullName:    "user/repo",
			HTMLURL:     "https://github.com/user/repo",
			Description: "A test repository",
			Topics:      []string{"testing"},
			StarredAt:   time.Now(),
		},
	}

	ghClient := &mockGitHubClient{repos: mockRepos}
	pbClient := &mockPinboardClient{}
	exporter := NewExporter(ghClient, pbClient)

	ctx := context.Background()
	result, err := exporter.Run(ctx)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Added != 2 {
		t.Errorf("expected Added=2, got %d", result.Added)
	}

	// Should have called AddBookmark twice
	if len(pbClient.addedBookmarks) != 2 {
		t.Fatalf("expected AddBookmark called 2 times, got %d", len(pbClient.addedBookmarks))
	}

	// Verify first bookmark
	if pbClient.addedBookmarks[0].URL != mockRepos[0].HTMLURL {
		t.Errorf("expected first bookmark URL %q, got %q", mockRepos[0].HTMLURL, pbClient.addedBookmarks[0].URL)
	}
}

// Test Run with dry run
func TestRunDryRun(t *testing.T) {
	mockRepos := []github.StarredRepo{
		{
			FullName:    "golang/go",
			HTMLURL:     "https://github.com/golang/go",
			Description: "The Go programming language",
			Topics:      []string{"language"},
			StarredAt:   time.Now(),
		},
	}

	ghClient := &mockGitHubClient{repos: mockRepos}
	pbClient := &mockPinboardClient{}
	exporter := NewExporter(ghClient, pbClient)
	exporter.DryRun = true

	ctx := context.Background()
	result, err := exporter.Run(ctx)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Added != 1 {
		t.Errorf("expected Added=1, got %d", result.Added)
	}

	// Should NOT have called AddBookmark
	if len(pbClient.addedBookmarks) != 0 {
		t.Errorf("expected AddBookmark not called in dry run, but was called %d times", len(pbClient.addedBookmarks))
	}
}

// Test Run skips repos that already exist in Pinboard.
func TestRunSkipsExistingBookmarks(t *testing.T) {
	mockRepos := []github.StarredRepo{
		{
			FullName: "golang/go",
			HTMLURL:  "https://github.com/golang/go",
			Topics:   []string{"language"},
		},
		{
			FullName: "user/new-repo",
			HTMLURL:  "https://github.com/user/new-repo",
			Topics:   []string{"testing"},
		},
		{
			FullName: "user/also-existing",
			HTMLURL:  "https://github.com/user/also-existing",
			Topics:   []string{},
		},
	}

	ghClient := &mockGitHubClient{repos: mockRepos}
	pbClient := &mockPinboardClient{
		existingURLs: map[string]bool{
			"https://github.com/golang/go":          true,
			"https://github.com/user/also-existing": true,
		},
	}
	exporter := NewExporter(ghClient, pbClient)

	ctx := context.Background()
	result, err := exporter.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only add the one new repo
	if len(pbClient.addedBookmarks) != 1 {
		t.Fatalf("expected 1 bookmark added, got %d", len(pbClient.addedBookmarks))
	}
	if pbClient.addedBookmarks[0].URL != "https://github.com/user/new-repo" {
		t.Errorf("expected new-repo to be added, got %q", pbClient.addedBookmarks[0].URL)
	}

	// Result should report counts correctly
	if result.Total != 3 {
		t.Errorf("expected Total=3, got %d", result.Total)
	}
	if result.Skipped != 2 {
		t.Errorf("expected Skipped=2, got %d", result.Skipped)
	}
	if result.Added != 1 {
		t.Errorf("expected Added=1, got %d", result.Added)
	}
}

// Test Run calls the progress callback.
func TestRunProgress(t *testing.T) {
	mockRepos := []github.StarredRepo{
		{FullName: "a/one", HTMLURL: "https://github.com/a/one"},
		{FullName: "b/two", HTMLURL: "https://github.com/b/two"},
	}

	ghClient := &mockGitHubClient{repos: mockRepos}
	pbClient := &mockPinboardClient{}
	exporter := NewExporter(ghClient, pbClient)

	var progressCalls []Progress
	exporter.OnProgress = func(p Progress) {
		progressCalls = append(progressCalls, p)
	}

	ctx := context.Background()
	_, err := exporter.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(progressCalls) != 2 {
		t.Fatalf("expected 2 progress calls, got %d", len(progressCalls))
	}

	// First call
	if progressCalls[0].Current != 1 || progressCalls[0].Total != 2 {
		t.Errorf("expected progress 1/2, got %d/%d", progressCalls[0].Current, progressCalls[0].Total)
	}
	if progressCalls[0].RepoName != "a/one" {
		t.Errorf("expected repo name %q, got %q", "a/one", progressCalls[0].RepoName)
	}

	// Second call
	if progressCalls[1].Current != 2 || progressCalls[1].Total != 2 {
		t.Errorf("expected progress 2/2, got %d/%d", progressCalls[1].Current, progressCalls[1].Total)
	}
}

// Test Run with Pinboard GetBookmarkURLsByTag error.
func TestRunPinboardGetError(t *testing.T) {
	ghClient := &mockGitHubClient{repos: []github.StarredRepo{
		{FullName: "a/b", HTMLURL: "https://github.com/a/b"},
	}}
	pbClient := &mockPinboardClient{
		getErr: errors.New("pinboard fetch error"),
	}
	exporter := NewExporter(ghClient, pbClient)

	ctx := context.Background()
	_, err := exporter.Run(ctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "pinboard fetch error" {
		t.Errorf("expected pinboard fetch error, got %v", err)
	}
}

// Test Run with GitHub error
func TestRunGitHubError(t *testing.T) {
	expectedErr := errors.New("github API error")
	ghClient := &mockGitHubClient{err: expectedErr}
	pbClient := &mockPinboardClient{}
	exporter := NewExporter(ghClient, pbClient)

	ctx := context.Background()
	_, err := exporter.Run(ctx)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}
