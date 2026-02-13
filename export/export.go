package export

import (
	"context"
	"strings"

	"github.com/monooso/gitboard/github"
	"github.com/monooso/gitboard/pinboard"
)

// GitHubClient defines the interface for fetching starred repositories.
type GitHubClient interface {
	GetStarredRepos(ctx context.Context) ([]github.StarredRepo, error)
}

// PinboardClient defines the interface for interacting with Pinboard.
type PinboardClient interface {
	AddBookmark(ctx context.Context, b pinboard.Bookmark) error
	GetBookmarkURLsByTag(ctx context.Context, tag string) (map[string]bool, error)
}

// Progress reports the current state of an export operation.
type Progress struct {
	Current  int
	Total    int
	RepoName string
	Skipped  bool
}

// Result summarises a completed export operation.
type Result struct {
	Total   int
	Added   int
	Skipped int
}

// Exporter exports GitHub starred repositories to Pinboard bookmarks.
type Exporter struct {
	gh         GitHubClient
	pb         PinboardClient
	DryRun     bool
	OnProgress func(Progress)
}

// NewExporter creates a new Exporter with the provided clients.
func NewExporter(gh GitHubClient, pb PinboardClient) *Exporter {
	return &Exporter{
		gh: gh,
		pb: pb,
	}
}

// NormaliseTags normalises topic tags by converting to lowercase and replacing spaces with hyphens.
func NormaliseTags(topics []string) []string {
	if len(topics) == 0 {
		return []string{}
	}

	normalised := make([]string, len(topics))
	for i, topic := range topics {
		normalised[i] = strings.ToLower(strings.ReplaceAll(topic, " ", "-"))
	}
	return normalised
}

// RepoToBookmark converts a GitHub starred repository to a Pinboard bookmark.
func RepoToBookmark(repo github.StarredRepo) pinboard.Bookmark {
	tags := []string{"github-repo"}
	normalisedTopics := NormaliseTags(repo.Topics)
	tags = append(tags, normalisedTopics...)

	return pinboard.Bookmark{
		URL:         repo.HTMLURL,
		Title:       repo.FullName,
		Description: repo.Description,
		Tags:        tags,
		Private:     true,
		ToRead:      false,
	}
}

// Run fetches starred repositories and creates Pinboard bookmarks for any
// that don't already exist. It returns a Result summarising what happened.
func (e *Exporter) Run(ctx context.Context) (Result, error) {
	repos, err := e.gh.GetStarredRepos(ctx)
	if err != nil {
		return Result{}, err
	}

	// Fetch existing Pinboard bookmarks tagged "github-repo" for deduplication.
	existing, err := e.pb.GetBookmarkURLsByTag(ctx, "github-repo")
	if err != nil {
		return Result{}, err
	}

	total := len(repos)
	var added, skipped int

	for i, repo := range repos {
		bookmark := RepoToBookmark(repo)
		isSkipped := existing[bookmark.URL]

		if e.OnProgress != nil {
			e.OnProgress(Progress{
				Current:  i + 1,
				Total:    total,
				RepoName: repo.FullName,
				Skipped:  isSkipped,
			})
		}

		if isSkipped {
			skipped++
			continue
		}

		if !e.DryRun {
			if err := e.pb.AddBookmark(ctx, bookmark); err != nil {
				return Result{Total: total, Added: added, Skipped: skipped}, err
			}
		}
		added++
	}

	return Result{Total: total, Added: added, Skipped: skipped}, nil
}
