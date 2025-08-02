# Homebrew Tap Setup Guide

This guide walks through setting up automatic Homebrew formula updates when releasing Compozy.

## Overview

The Homebrew tap integration allows users to install Compozy using:

```bash
brew tap compozy/compozy
brew install compozy
```

When you create a new release, GoReleaser will automatically:

1. Build the binaries for macOS and Linux
2. Calculate checksums
3. Update the Homebrew formula in your tap repository
4. Push the changes to the tap repository

## Prerequisites

### 1. Create the Homebrew Tap Repository

1. Create a new GitHub repository named `homebrew-compozy` under the `compozy` organization
   - The repository name MUST start with `homebrew-`
   - This will be accessible as `compozy/compozy` tap

2. Initialize the repository with a basic README:

````markdown
# Compozy Homebrew Tap

Homebrew formulae for installing Compozy.

## Installation

```bash
brew tap compozy/compozy
brew install compozy
```
````

## Available Formulae

- `compozy` - Next-level Agentic Orchestration Platform

````

3. Create a `Formula` directory in the repository
   - This is where GoReleaser will place the formula file

### 2. Create a GitHub Personal Access Token

1. Go to GitHub Settings → Developer settings → Personal access tokens → Tokens (classic)

2. Click "Generate new token (classic)"

3. Give it a descriptive name like "Homebrew Tap Release"

4. Select the following permissions:
   - **repo** (Full control of private repositories)
     - This gives access to both your main repo and the tap repo
   - **workflow** (Update GitHub Action workflows)
     - Required if your tap repo has GitHub Actions

5. Set an expiration (recommended: 1 year, then rotate)

6. Click "Generate token" and copy the token immediately

### 3. Add the Token to Repository Secrets

1. Go to your main Compozy repository settings
2. Navigate to Settings → Secrets and variables → Actions
3. Click "New repository secret"
4. Name: `HOMEBREW_TAP_GITHUB_TOKEN`
5. Value: Paste the personal access token you created
6. Click "Add secret"

## Configuration Details

### GoReleaser Configuration

The `.goreleaser.yml` has been updated with:

```yaml
brews:
  - name: compozy
    repository:
      owner: compozy
      name: homebrew-compozy
      branch: main
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"

    commit_author:
      name: goreleaserbot
      email: bot@goreleaser.com

    commit_msg_template: "Brew formula update for {{ .ProjectName }} version {{ .Tag }}"

    directory: Formula

    # ... other configuration
````

### GitHub Actions Workflow

The `.github/workflows/release.yml` has been updated to include:

```yaml
env:
  GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
  HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}
  COSIGN_EXPERIMENTAL: 1
```

## Release Process

When you create a new release:

1. **Tag your release**:

   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

2. **GoReleaser will automatically**:
   - Build binaries for all platforms
   - Create a GitHub release with artifacts
   - Generate and push a Homebrew formula to your tap repository

3. **The formula will include**:
   - Binary download URLs for macOS (Intel and Apple Silicon)
   - SHA256 checksums for verification
   - Installation instructions
   - Dependencies (git and docker as optional)
   - Shell completion setup

4. **Users can then install**:

   ```bash
   brew tap compozy/compozy
   brew install compozy
   ```

   Or upgrade to the latest version:

   ```bash
   brew update
   brew upgrade compozy
   ```

## Troubleshooting

### Token Permission Issues

If the release fails with permission errors:

- Ensure the token has access to BOTH repositories (main and tap)
- Verify the token hasn't expired
- Check that the tap repository exists and has a `Formula` directory

### Formula Updates Not Appearing

If the formula doesn't update:

- Check the GoReleaser logs in the GitHub Actions run
- Verify the tap repository name starts with `homebrew-`
- Ensure the branch name in the config matches your tap repo's default branch

### Installation Issues

If users report installation problems:

- Check that the binary URLs in the formula are accessible
- Verify the SHA256 checksums match the released binaries
- Ensure the formula syntax is valid using `brew audit --strict compozy`

## Testing Locally

Before releasing, you can test the Homebrew formula locally:

1. Create a test formula file locally
2. Install using the local formula:
   ```bash
   brew install --build-from-source ./compozy.rb
   ```
3. Test the installation:
   ```bash
   compozy version
   ```

## Maintenance

- **Token Rotation**: Rotate the personal access token annually for security
- **Formula Updates**: The formula is automatically updated with each release
- **Manual Updates**: If needed, you can manually edit the formula in the tap repository
