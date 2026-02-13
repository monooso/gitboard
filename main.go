package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/monooso/gitboard/export"
	"github.com/monooso/gitboard/github"
	"github.com/monooso/gitboard/pinboard"
)

func main() {
	githubToken := flag.String("github-token", "", "GitHub personal access token (overrides GITHUB_TOKEN)")
	pinboardToken := flag.String("pinboard-token", "", "Pinboard API token (overrides PINBOARD_TOKEN)")
	dryRun := flag.Bool("dry-run", false, "Print what would be exported without creating bookmarks")
	flag.Parse()

	if *githubToken == "" {
		*githubToken = os.Getenv("GITHUB_TOKEN")
	}
	if *pinboardToken == "" {
		*pinboardToken = os.Getenv("PINBOARD_TOKEN")
	}

	if *githubToken == "" {
		log.Fatal("GitHub token is required: set GITHUB_TOKEN or use --github-token")
	}
	if *pinboardToken == "" {
		log.Fatal("Pinboard token is required: set PINBOARD_TOKEN or use --pinboard-token")
	}

	gh := github.NewClient(*githubToken)
	pb := pinboard.NewClient(*pinboardToken)

	exporter := export.NewExporter(gh, pb)
	exporter.DryRun = *dryRun
	exporter.OnProgress = func(p export.Progress) {
		action := "adding"
		if p.Skipped {
			action = "exists"
		} else if *dryRun {
			action = "would add"
		}

		bar := progressBar(p.Current, p.Total, 30)
		fmt.Fprintf(os.Stderr, "\r%s %d/%d %s: %s", bar, p.Current, p.Total, action, p.RepoName)

		// Pad with spaces to clear any leftover characters from longer previous lines.
		fmt.Fprintf(os.Stderr, "    ")
	}

	ctx := context.Background()
	result, err := exporter.Run(ctx)

	// Clear the progress line.
	fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", 80))

	if err != nil {
		log.Fatalf("Export failed: %v", err)
	}

	if *dryRun {
		fmt.Printf("Dry run: %d new, %d existing, %d total\n", result.Added, result.Skipped, result.Total)
	} else {
		fmt.Printf("Done: %d added, %d skipped, %d total\n", result.Added, result.Skipped, result.Total)
	}
}

// progressBar returns a simple text progress bar of the given width.
func progressBar(current, total, width int) string {
	if total == 0 {
		return "[" + strings.Repeat(" ", width) + "]"
	}

	filled := width * current / total
	empty := width - filled

	return "[" + strings.Repeat("=", filled) + strings.Repeat(" ", empty) + "]"
}
