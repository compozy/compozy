# Compozy Release CLI

`compozy-release` is an internal command-line tool designed to automate and standardize the release process for the Compozy project. It handles everything from version calculation and changelog generation to creating release branches, tagging, and publishing packages.

## Overview

This tool leverages conventional commit messages to automatically determine the next semantic version, generates detailed changelogs, and orchestrates the necessary Git and GitHub operations for a new release. It ensures that releases are consistent, predictable, and require minimal manual intervention.

## Features

- **Automatic Version Calculation**: Determines the next version (patch, minor, major) based on commit history since the last tag.
- **Changelog Generation**: Uses `git-cliff` to generate a formatted changelog for the new release.
- **Git Operations**: Automates the creation of release branches and Git tags.
- **NPM Package Management**: Updates versions in `package.json` files and publishes them to the NPM registry.
- **Pull Request Preparation**: Generates a formatted body for the release pull request.
- **Configuration-Driven**: Uses a simple YAML file for configuration.

## Prerequisites

- **Go**: The tool is written in Go.
- **git-cliff**: Required for version calculation and changelog generation. Make sure it is installed and available in your `PATH`.

## Configuration

The tool is configured via a `.compozy-release.yaml` file in the root of the repository.

```yaml
# .compozy-release.yaml

# GitHub token with repository access (read/write).
# Can also be provided via the GITHUB_TOKEN environment variable.
github_token: "your-github-token"

# Directory containing NPM packages to be updated and published.
# Defaults to "tools".
tools_dir: "tools"

# (Optional) GitHub repository owner.
# Defaults to "compozy".
github_owner: "compozy"

# (Optional) GitHub repository name.
# Defaults to "compozy".
github_repo: "compozy"
```

## Commands

The CLI is built using `cobra` and provides several commands to manage the release workflow.

| Command                   | Description                                                                 |
| ------------------------- | --------------------------------------------------------------------------- |
| `check-changes`           | Checks if there are pending changes for a new release since the last tag.   |
| `calculate-version`       | Calculates the next semantic version based on conventional commit messages. |
| `generate-changelog`      | Generates a changelog for a specific version.                               |
| `create-release-branch`   | Creates and pushes a new release branch.                                    |
| `create-git-tag`          | Creates and pushes a new Git tag.                                           |
| `update-package-versions` | Updates the version for all NPM packages in the `tools/` directory.         |
| `publish-npm-packages`    | Publishes all NPM packages in the `tools/` directory.                       |
| `prepare-pr-body`         | Prepares a formatted body for a release pull request.                       |
| `update-main-changelog`   | Prepends the generated changelog to the main `CHANGELOG.md` file.           |

### Usage Examples

```bash
# Check for pending changes
go run ./pkg/release check-changes

# Calculate the next version
go run ./pkg/release calculate-version

# Generate a changelog for a version
go run ./pkg/release generate-changelog --version v1.2.3

# Create a new release branch
go run ./pkg/release create-release-branch --branch-name release/v1.2.3
```

## Internal Architecture

The tool follows a clean architecture pattern, separating concerns into distinct layers:

- **`cmd/`**: Contains the Cobra command definitions, responsible for parsing flags and arguments and invoking the appropriate use cases.
- **`internal/usecase/`**: Holds the business logic for each command. Use cases orchestrate operations by interacting with repositories and services.
- **`internal/domain/`**: Defines the core data structures of the application, such as `Version`, `Release`, and `Package`.
- **`internal/repository/`**: Provides interfaces and implementations for data persistence and retrieval (e.g., Git, GitHub API, filesystem).
- **`internal/service/`**: Contains interfaces and implementations for external services, such as `git-cliff` and `npm`.
- **`internal/config/`**: Handles loading and validation of the application configuration.

# Environment Variables for pkg/release

This document lists all environment variables used by the pkg/release tool and the release workflow.

## Core Configuration Variables

These environment variables are handled by pkg/release configuration:

### GitHub Configuration

- **GITHUB_TOKEN** or **COMPOZY_RELEASE_GITHUB_TOKEN**
  - Purpose: GitHub API authentication (optional for most commands)
  - Used by: Future GitHub API operations (currently unused)
  - Required: No (only for GitHub API operations)
  - In CI: Available as `${{ secrets.GITHUB_TOKEN }}`

- **GITHUB_OWNER** or **COMPOZY_RELEASE_GITHUB_OWNER**
  - Purpose: GitHub repository owner
  - Default: `compozy`
  - Required: No

- **GITHUB_REPO** or **COMPOZY_RELEASE_GITHUB_REPO**
  - Purpose: GitHub repository name
  - Default: `compozy`
  - Required: No

### NPM Configuration

- **NPM_TOKEN** or **COMPOZY_RELEASE_NPM_TOKEN**
  - Purpose: NPM registry authentication for publishing packages
  - Used by: `publish-npm-packages` command
  - Required: Yes (for NPM publishing)
  - In CI: Available as `${{ secrets.NPM_TOKEN }}`
  - Note: The npm CLI automatically uses NPM_TOKEN from environment

### Tool Configuration

- **TOOLS_DIR** or **COMPOZY_RELEASE_TOOLS_DIR**
  - Purpose: Directory containing NPM packages to publish
  - Default: `tools`
  - Required: No

## External Tool Environment Variables

These are used directly by external tools, not by pkg/release:

### GoReleaser

- **GORELEASER_KEY**
  - Purpose: License key for GoReleaser Pro
  - Used by: goreleaser CLI
  - In CI: `${{ secrets.GORELEASER_KEY }}`

- **GORELEASER_CURRENT_TAG**
  - Purpose: Current tag for GoReleaser
  - Used by: goreleaser CLI
  - In CI: Set to the version being released

### Other CI Secrets

- **RELEASE_TOKEN**
  - Purpose: Additional GitHub token with extended permissions
  - Used by: GoReleaser for releases
  - In CI: `${{ secrets.RELEASE_TOKEN }}`

- **AUR_KEY**
  - Purpose: AUR (Arch User Repository) authentication
  - Used by: GoReleaser for AUR publishing
  - In CI: `${{ secrets.AUR_KEY }}`

- **COSIGN_EXPERIMENTAL**
  - Purpose: Enable experimental cosign features
  - Used by: cosign for artifact signing
  - In CI: Set to `1`

## Configuration Precedence

1. Environment variables take precedence over config file
2. For each config field, multiple environment variable names are checked in order:
   - Standard name (e.g., `GITHUB_TOKEN`)
   - Prefixed name (e.g., `COMPOZY_RELEASE_GITHUB_TOKEN`)
3. If no environment variable is set, defaults are used

## Local Development

For local development, you can:

1. Set environment variables in your shell
2. Create a `.compozy-release.yaml` config file
3. Use defaults (works for most commands except NPM publishing)

Example `.compozy-release.yaml`:

```yaml
github_token: "your-github-token" # Optional
github_owner: "compozy"
github_repo: "compozy"
tools_dir: "tools"
npm_token: "your-npm-token" # Required for publishing
```

## CI/CD Setup

The GitHub Actions workflow sets these environment variables:

- Commands that don't need tokens: Run without special configuration
- NPM publishing: Requires `NPM_TOKEN` secret to be set
- GoReleaser: Requires `GORELEASER_KEY` secret
- Full release: Requires all secrets mentioned above
