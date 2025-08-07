# Setup Release Tools Action

A comprehensive GitHub Action that sets up all necessary tools for release processes, ensuring consistency between dry-run and production release workflows.

## Features

- **GoReleaser**: Configurable version and distribution (standard/pro)
- **git-cliff**: Changelog generation with caching
- **Syft**: Software Bill of Materials (SBOM) generation
- **Cosign**: Artifact signing for security
- **Docker**: Multi-platform container building with buildx and QEMU
- **Verification**: Automatic validation of all tool installations

## Usage

### Basic Usage (Dry-run)

```yaml
- name: Setup Release Tools
  uses: ./.github/actions/setup-release-tools
  with:
    goreleaser-distribution: goreleaser-pro
```

### Production Release

```yaml
- name: Setup Release Tools
  uses: ./.github/actions/setup-release-tools
  with:
    goreleaser-distribution: goreleaser-pro
    setup-docker: true
    setup-docker-login: true
    setup-qemu: true
    docker-registry: ghcr.io
    docker-username: ${{ github.actor }}
    docker-token: ${{ secrets.RELEASE_TOKEN }}
```

## Inputs

| Input | Description | Default |
|-------|-------------|---------|
| `goreleaser-version` | GoReleaser version to install | `latest` |
| `goreleaser-distribution` | GoReleaser distribution (goreleaser or goreleaser-pro) | `goreleaser-pro` |
| `git-cliff-version` | git-cliff version to install | `2.7.0` |
| `cosign-version` | Cosign version to install | `v2.2.4` |
| `setup-docker` | Whether to setup Docker buildx | `false` |
| `setup-docker-login` | Whether to setup Docker registry login | `false` |
| `setup-qemu` | Whether to setup QEMU for cross-platform building | `true` |
| `docker-registry` | Docker registry to login to | `ghcr.io` |
| `docker-username` | Docker registry username | - |
| `docker-token` | Docker registry token/password | - |

## Outputs

| Output | Description |
|--------|-------------|
| `goreleaser-version` | The GoReleaser version that was installed |
| `git-cliff-version` | The git-cliff version that was installed |
| `cosign-version` | The Cosign version that was installed |
| `syft-version` | The Syft version that was installed |

## Tools Installed

1. **git-cliff**: Fast and conventional changelog generator
   - Installed via Bun with caching for faster CI runs
   - Used for generating release notes and changelogs

2. **GoReleaser**: Build, package, and publish Go projects
   - Supports both standard and Pro versions
   - Handles cross-compilation, packaging, and publishing

3. **Syft**: SBOM generation tool by Anchore
   - Required for GoReleaser's SBOM generation feature
   - Creates software bill of materials for security compliance

4. **Cosign**: Container image and artifact signing
   - Implements keyless signing with Sigstore
   - Signs release artifacts and container images

5. **Docker Tools** (conditional):
   - **QEMU**: Cross-platform emulation for multi-arch builds
   - **Docker Buildx**: Advanced Docker build features
   - **Registry Login**: Authentication for pushing images

## Verification

The action includes built-in verification that ensures all tools are properly installed and functional. If any tool fails verification, the action will exit with an error.

## Architecture

This action centralizes all release dependencies to prevent the kind of missing dependency issues that occurred with the original dry-run workflow. By using the same action for both dry-run and production releases, we ensure consistency and reduce maintenance overhead.

## Dependencies

- Requires Go to be installed (typically via `actions/setup-go`)
- Docker tools require a Linux runner
- Some tools may require specific permissions (contents: write, packages: write)