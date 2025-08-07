# PRD: Compozy Release CLI

**Author:** Compozy AI Agent
**Status:** Draft
**Date:** 2024-07-29

## 1. Introduction

This document outlines the plan to transition the Compozy project's release process from a collection of shell scripts to a dedicated, standalone command-line interface (CLI) tool written in Go. This new tool, `compozy-release`, will be developed within the `pkg/release` directory but built as a standalone binary. It will encapsulate all release-related logic, improving maintainability, testability, and reliability for execution within CI/CD pipelines, specifically GitHub Actions.

## 2. Problem Statement

The current release process relies on a set of shell scripts located in `scripts/release`. While functional, this approach has several drawbacks:

- **Maintainability:** Shell scripts are difficult to test, debug, and scale.
- **Portability:** They can have subtle inconsistencies when run on different developer machines or CI/CD environments.
- **Error Handling:** Robust error handling and recovery are complex to implement in shell scripts.
- **Extensibility:** Adding new features or modifying existing ones is cumbersome and error-prone.

Migrating to a Go-based CLI will address these issues by providing a structured, testable, and more robust foundation for our release automation.

## 3. Goals and Objectives

- Replace all shell scripts in `scripts/release` with equivalent commands in the `compozy-release` CLI.
- Create a single, statically-linked binary that can be easily executed in any environment, especially GitHub Actions.
- Improve the reliability and error-handling of the release process.
- Improve the maintainability and testability of the release logic.
- Maintain the same external dependencies: `git-cliff` and `goreleaser` are required.
- Introduce a dedicated configuration file, `.compozy-release.yaml`, for release-specific settings.

## 4. Non-Goals

- Introducing new features or changing the existing release workflow logic. The initial goal is a 1:1 migration of functionality.
- Removing the dependency on `git-cliff` or `goreleaser`.
- Integrating the release CLI directly into the main `compozy` application. It will remain a standalone tool.

## 5. User Stories

- As a developer, I want to run a single command to check if there are pending changes that warrant a new release, so I can quickly assess the state of the repository.
- As a developer, I want to run a command to automatically calculate the next semantic version based on conventional commit messages, so I don't have to determine it manually.
- As a release manager, I want to generate a changelog for a new release, so I can include it in the release notes.
- As a release manager, I want to create a release branch and Git tag with a single command, so I can streamline the release preparation process.
- As a release manager, I want the tool to automatically update `package.json` files for all NPM packages under the `tools/` directory, so they are all in sync with the new release version.
- As a release manager, I want to publish all updated NPM packages to the registry, so they are available for users.

## 6. Requirements

### Functional Requirements

- The CLI must be invokable as a standalone binary (e.g., `./compozy-release <command>`).
- It must provide subcommands that map directly to the existing scripts:
  - `check-changes`
  - `calculate-version`
  - `extract-version`
  - `generate-changelog`
  - `create-release-branch`
  - `create-git-tag`
  - `update-package-versions`
  - `prepare-pr-body`
  - `update-main-changelog`
  - `publish-npm-packages`
- The CLI must read its configuration from a `.compozy-release.yaml` file if it exists.
- The CLI must be able to run in a GitHub Actions environment and produce outputs compatible with it (e.g., `echo "key=value" >> $GITHUB_OUTPUT`).
- It must successfully wrap and execute `git-cliff` and `goreleaser` commands, capturing their output.

### Non-Functional Requirements

- **Testability:** The architecture must be highly testable, with clear separation of concerns allowing for unit and integration tests.
- **Performance:** The tool should be performant and not add significant overhead to the release process.
- **Reliability:** The tool must have robust error handling and provide clear, actionable error messages.
- **Maintainability:** The codebase must be well-structured, following Go best practices and Clean Architecture principles.

## 7. Technical Proposal

The `compozy-release` tool will be built in Go and reside within the `pkg/release` directory. It will be architected following Clean Architecture principles to ensure separation of concerns and testability.

### 7.1. Package Structure

The proposed structure within `pkg/release` will be:

```
release/
├── cmd/                // Cobra command definitions
│   ├── root.go
│   ├── check_changes.go
│   └── ... (one file per command)
├── internal/
│   ├── config/         // Viper configuration loading
│   ├── domain/         // Core entities (Version, Release, etc.)
│   ├── usecase/        // Business logic for each command
│   ├── repository/     // Interfaces for external systems (Git, FS, GitHub)
│   └── service/        // Wrappers for external tools (git-cliff, goreleaser)
├── main.go             // Main entry point for the CLI binary
└── .compozy-release.yaml.example // Example configuration file
```

### 7.2. Core Components

- **CLI (`cmd/`):** Built using `spf13/cobra`. Each subcommand will instantiate and execute its corresponding use case.
- **Configuration (`internal/config`):** Managed by `spf13/viper`, loading from `.compozy-release.yaml` and environment variables.
- **Domain (`internal/domain`):** Contains core data structures like `Version` (using `Masterminds/semver`), `Release`, and `Package`. These are plain Go structs with no business logic.
- **Use Cases (`internal/usecase`):** Each use case contains the logic for one command (e.g., `CheckChangesUseCase`). It will orchestrate calls to repositories and services.
- **Repositories (`internal/repository`):** Defines interfaces for data access and external systems. Implementations will be provided for:
  - `GitRepository`: Using `go-git/go-git` for native Git operations.
  - `GithubRepository`: Using `google/go-github` for interacting with the GitHub API.
  - `FileSystemRepository`: Using `spf13/afero` for testable file operations.
- **Services (`internal/service`):** Wrappers around external command-line tools.
  - `CliffService`: Executes `git-cliff` commands via `os/exec`.
  - `GoReleaserService`: Executes `goreleaser` commands via `os/exec`.
  - `NpmService`: Executes `npm` commands via `os/exec`.

### 7.3. Dependencies

- **CLI:** `spf13/cobra`, `spf13/viper`
- **Git:** `go-git/go-git`
- **GitHub:** `google/go-github`
- **Versioning:** `Masterminds/semver`
- **Filesystem:** `spf13/afero`
- **Logging:** `go.uber.org/zap`
- **HTTP:** `go-resty/resty` (for NPM registry checks)

### 7.4. Build and Distribution

A `main.go` file will be created in the root of `pkg/release` to build the standalone binary. A GoReleaser configuration can be added later to automate the build process for different platforms if needed.

## 8. Risks and Mitigation

- **Risk:** External tools (`git-cliff`, `goreleaser`) are not installed in the execution environment.
  - **Mitigation:** The CLI will perform a check for the existence of these binaries at startup and return a clear error message if they are not found.
- **Risk:** Subtle differences in behavior between the Go implementation and the original shell scripts.
  - **Mitigation:** The tool will be tested against the existing GitHub Actions workflow to ensure outputs are identical and the overall process remains unchanged.
