# Release Scripts

This directory contains shell scripts extracted from the GitHub Actions release workflow to reduce YAML complexity and improve maintainability.

## Scripts Overview

### `check-changes.sh`

Checks if there are any changes since the last tag that warrant a new release.

- **Output**: `has_changes=true/false`, `latest_tag=<tag>`
- **Usage**: `./check-changes.sh`

### `calculate-version.sh`

Calculates the next version based on conventional commits using git-cliff.

- **Arguments**: `$1` - Initial version (fallback if no tags exist)
- **Output**: `version=<version>`, `version_number=<version without v>`
- **Usage**: `./calculate-version.sh 0.0.4`

### `generate-changelog.sh`

Generates changelog using git-cliff for different stages of the release process.

- **Arguments**:
  - `$1` - Version tag
  - `$2` - Mode ("update" for PR, "release" for final)
- **Output**: Creates/updates CHANGELOG.md and related files
- **Usage**: `./generate-changelog.sh v1.2.3 update`

### `create-release-branch.sh`

Creates or updates a release branch.

- **Arguments**:
  - `$1` - Version
  - `$2` - Branch prefix (default: "release/")
  - `$3` - GitHub repository (owner/repo)
- **Output**: `branch_name=<branch>`
- **Usage**: `./create-release-branch.sh v1.2.3 release/ owner/repo`

### `prepare-pr-body.sh`

Prepares the pull request body from a template.

- **Arguments**:
  - `$1` - Version
  - `$2` - Template file path (optional)
  - `$3` - Changelog file (optional)
  - `$4` - Output file (optional)
- **Output**: Creates PR_BODY.md
- **Usage**: `./prepare-pr-body.sh v1.2.3`

### `extract-version.sh`

Extracts version from a commit message.

- **Arguments**: `$1` - Commit message
- **Output**: `version=<version>`
- **Usage**: `./extract-version.sh "ci(release): Release v1.2.3"`

### `check-tag-exists.sh`

Checks if a git tag already exists (for idempotency).

- **Arguments**: `$1` - Version tag
- **Output**: `tag_exists=true/false`
- **Usage**: `./check-tag-exists.sh v1.2.3`

### `create-git-tag.sh`

Creates and pushes a git tag.

- **Arguments**: `$1` - Version tag
- **Usage**: `./create-git-tag.sh v1.2.3`

### `update-main-changelog.sh`

Updates the changelog in the main branch after release.

- **Arguments**: `$1` - Version tag
- **Usage**: `./update-main-changelog.sh v1.2.3`

### `update-package-versions.sh`

Updates version in all package.json files (root and tools packages).

- **Arguments**: `$1` - Version (with or without 'v' prefix)
- **Output**: `version=<version>`
- **Usage**: `./update-package-versions.sh v1.2.3` or `./update-package-versions.sh 1.2.3`
- **Note**: Automatically strips 'v' prefix if present for package.json compatibility

### `publish-npm-packages.sh`

Publishes all tools packages to NPM registry.

- **Arguments**: `--dry-run` (optional) - Validate without publishing
- **Output**: `published_count=<number>`, `failed_count=<number>`
- **Usage**:
  - Production: `./publish-npm-packages.sh`
  - Testing: `./publish-npm-packages.sh --dry-run`
- **Features**:
  - Skips private packages automatically
  - Checks if version already exists (idempotent)
  - Verifies NPM authentication
  - Provides detailed summary

## Integration with GitHub Actions

These scripts are called from `.github/workflows/release.yml` to perform various release tasks. The workflow captures the script output and parses relevant variables for GitHub Actions using:

```yaml
run: |
  OUTPUT=$(./scripts/release/script-name.sh args)
  echo "$OUTPUT"
  # Parse output for GitHub Actions
  echo "$OUTPUT" | grep "variable=" | tail -1 >> $GITHUB_OUTPUT
```

## Requirements

- **git-cliff**: For changelog generation and version calculation
- **gh CLI**: For GitHub API operations (optional but recommended)
- **git**: Standard git commands
- **bash**: All scripts are written in bash
- **bun**: For JSON manipulation in package version updates (with fallbacks to jq/sed)
- **npm**: For publishing packages to NPM registry
- **NPM_TOKEN**: Environment variable required for NPM authentication

## Error Handling

All scripts use `set -e` to exit on errors, ensuring the workflow fails fast if any step encounters an issue.

## Custom Version Bump Patterns

The release process uses git-cliff for version calculation with custom regex patterns defined in `cliff.toml`:

### Major Version Bumps

Major version bumps (x.0.0) are triggered by:

- Commits containing `MAJOR` anywhere in the message
- Commits containing `BREAKING CHANGE` or `BREAKING-CHANGE` in the message
- Conventional commits with `!` suffix (e.g., `feat!:`, `fix!:`)

Examples:

```bash
git commit -m "feat!: complete API redesign"
git commit -m "refactor: BREAKING CHANGE - remove deprecated endpoints"
git commit -m "chore: MAJOR version bump for new architecture"
```

### Minor Version Bumps

Minor version bumps (0.x.0) are triggered by:

- Commits with `feat:` or `feature:` prefix (conventional)
- Commits containing `MINOR` anywhere in the message
- Standard feature additions following conventional commits

Examples:

```bash
git commit -m "feat: add new workflow execution API"
git commit -m "feature: implement MCP server integration"
git commit -m "chore: MINOR version bump for new features"
```

### Patch Version Bumps

Patch version bumps (0.0.x) are triggered by:

- Commits with `fix:` prefix (conventional)
- Commits with `perf:` prefix (performance improvements)
- Any other conventional commit that affects functionality

Examples:

```bash
git commit -m "fix: resolve memory leak in task executor"
git commit -m "perf: optimize database query performance"
```

### Non-Bumping Commits

These commits do NOT trigger version bumps:

- `chore:` - maintenance tasks
- `docs:` - documentation only
- `style:` - formatting changes
- `test:` - test additions/changes
- `ci:` - CI/CD changes
- `build:` - build system changes (unless breaking)

## Contributing

When modifying these scripts:

1. Test locally first
2. Ensure output format remains compatible with the workflow parser
3. Update this README if adding new scripts or changing interfaces
4. Keep scripts focused on single responsibilities
5. Follow conventional commits for proper version bumping
