# Gitboard

A command-line tool that exports your GitHub starred repositories to Pinboard bookmarks.

Each bookmark is tagged with `github-repo` plus any topics from the repository (normalised to lowercase with hyphens). All bookmarks are created as private.

Exports are incremental: gitboard checks which starred repos already have Pinboard bookmarks and only adds the missing ones. This makes subsequent runs fast, even with hundreds of stars.

## Requirements

- Go 1.25+
- A [GitHub personal access token](https://github.com/settings/tokens) (no special scopes required)
- A [Pinboard API token](https://pinboard.in/settings/password)

## Installation

```sh
go install github.com/monooso/gitboard@latest
```

Or build from source:

```sh
git clone https://github.com/monooso/gitboard.git
cd gitboard
go build -o gitboard .
```

## Usage

Provide your tokens via environment variables:

```sh
export GITHUB_TOKEN=ghp_xxxxxxxxxxxx
export PINBOARD_TOKEN=user:xxxxxxxxxxxx
gitboard
```

Or pass them as flags:

```sh
gitboard --github-token ghp_xxxxxxxxxxxx --pinboard-token user:xxxxxxxxxxxx
```

Flags override environment variables.

### Dry run

Preview what would be exported without creating any bookmarks:

```sh
gitboard --dry-run
```

### Flags

| Flag | Environment variable | Description |
|---|---|---|
| `--github-token` | `GITHUB_TOKEN` | GitHub personal access token |
| `--pinboard-token` | `PINBOARD_TOKEN` | Pinboard API token |
| `--dry-run` | | Preview changes without exporting |

## Licence

[AGPL-3.0](LICENSE)
