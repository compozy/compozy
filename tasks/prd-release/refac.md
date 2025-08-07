Complete Release Consolidation Plan

Executive Summary

Consolidate the current 11+ individual release commands into 2 comprehensive commands (pr-release and release) to simplify the release workflow and improve maintainability.

---

PHASE 1: Command Architecture Design

Command 1: pr-release

Purpose: Handle entire PR creation/update workflow
Usage: go run ./pkg/release pr-release [flags]

Consolidated Operations:
├── check-changes
├── calculate-version
├── create-release-branch
├── update-package-versions
├── generate-changelog
└── prepare-pr-body

Command 2: release

Purpose: Handle production release after PR merge
Usage: go run ./pkg/release release [flags]

Consolidated Operations:
├── extract-version (from commit)
├── create-git-tag
├── generate-changelog (final)
├── publish-npm-packages
└── update-main-changelog

---

PHASE 2: Implementation Structure

File Organization

pkg/release/
├── cmd/
│ ├── pr_release.go # NEW: PR workflow command
│ ├── release.go # NEW: Production release command
│ └── root.go # Update with new commands
├── internal/
│ ├── orchestrator/ # NEW: Orchestration layer
│ │ ├── pr_workflow.go
│ │ └── release_workflow.go
│ └── [existing structure remains]

Code Architecture Pattern

┌─────────────────┐
│ CLI Command │
└────────┬────────┘
│
┌────────▼────────┐
│ Orchestrator │ (NEW: Coordinates multiple use cases)
└────────┬────────┘
│
┌────────▼────────┐
│ Use Cases │ (EXISTING: Reused without modification)
└────────┬────────┘
│
┌────────▼────────┐
│ Services/Repos │ (EXISTING: Unchanged)
└─────────────────┘

---

PHASE 3: Command Implementation Details

pr-release Command Flow

type PRReleaseConfig struct {
ForceRelease bool
DryRun bool
CIOutput bool
SkipPR bool // For testing without PR creation
}

func (o \*PROrchestrator) Execute(ctx context.Context, cfg PRReleaseConfig) error {
// 1. Check for changes
if !hasChanges && !cfg.ForceRelease {
return nil
}

      // 2. Calculate version
      version := o.calculateVersion(ctx)

      // 3. Create release branch
      branch := o.createBranch(ctx, version)

      // 4. Update all versions
      o.updateVersions(ctx, version)

      // 5. Generate changelog
      changelog := o.generateChangelog(ctx, version, "unreleased")

      // 6. Commit and push
      o.commitChanges(ctx, version, branch)

      // 7. Create/Update PR (unless skipped)
      if !cfg.SkipPR {
          o.createPullRequest(ctx, version, changelog)
      }

      return nil

}

release Command Flow

type ReleaseConfig struct {
Version string // Can be extracted or provided
SkipTag bool
SkipNPM bool
CIOutput bool
}

func (o \*ReleaseOrchestrator) Execute(ctx context.Context, cfg ReleaseConfig) error {
// 1. Extract/validate version
version := o.extractVersion(ctx, cfg.Version)

      // 2. Check idempotency
      if o.tagExists(ctx, version) && !cfg.SkipTag {
          return nil // Already released
      }

      // 3. Create git tag
      if !cfg.SkipTag {
          o.createTag(ctx, version)
      }

      // 4. Generate final changelog
      changelog := o.generateChangelog(ctx, version, "release")

      // 5. Run GoReleaser (external)
      o.runGoReleaser(ctx, version, changelog)

      // 6. Publish NPM packages
      if !cfg.SkipNPM {
          o.publishNPM(ctx)
      }

      // 7. Update main changelog
      o.updateMainChangelog(ctx, changelog)

      return nil

}

---

PHASE 4: Simplified Workflow YAML

Before: 420+ lines → After: ~150 lines

name: Release

on:
push:
branches: [main]
pull_request:
types: [opened, synchronize, labeled]
workflow_dispatch:
inputs:
force_release:
type: boolean
default: false

jobs: # Job 1: Create/Update Release PR (50% reduction)
release-pr:
if: github.event_name == 'push' || github.event_name == 'workflow_dispatch'
runs-on: ubuntu-latest
steps: - uses: actions/checkout@v4
with:
fetch-depth: 0
token: ${{ secrets.RELEASE_TOKEN }}

        - uses: actions/setup-go@v5
          with:
            go-version: "1.24.x"

        - name: Create Release PR
          run: |
            go run ./pkg/release pr-release \
              --force=${{ github.event.inputs.force_release }} \
              --ci-output
          env:
            GITHUB_TOKEN: ${{ secrets.RELEASE_TOKEN }}

    # Job 2: Production Release (60% reduction)
    release:
      if: contains(github.event.head_commit.message, 'release:')
      runs-on: ubuntu-latest
      steps:
        - uses: actions/checkout@v4
          with:
            fetch-depth: 0
            token: ${{ secrets.RELEASE_TOKEN }}

        - uses: actions/setup-go@v5
          with:
            go-version: "1.24.x"

        - uses: goreleaser/goreleaser-action@v6
          with:
            install-only: true

        - name: Execute Release
          run: |
            go run ./pkg/release release --ci-output
          env:
            GITHUB_TOKEN: ${{ secrets.RELEASE_TOKEN }}
            NPM_TOKEN: ${{ secrets.NPM_TOKEN }}
            GORELEASER_KEY: ${{ secrets.GORELEASER_KEY }}

---

PHASE 5: Testing & Migration Strategy

Testing Approach

1. Unit Tests
   - Test each orchestrator method independently
   - Mock all external dependencies (git, npm, etc.)
   - Validate error handling paths

2. Integration Tests
   - Test full command flows with test repositories
   - Verify idempotency and rollback scenarios
   - Test CI output format compatibility

3. Dry-Run Testing

# Test PR workflow without creating PR

go run ./pkg/release pr-release --dry-run --skip-pr

# Test release without publishing

go run ./pkg/release release --dry-run --skip-npm

Migration Steps

1. Parallel Run Phase (1-2 releases)
   - Keep old workflow active
   - Add new workflow as release-v2.yml
   - Compare outputs and behavior

2. Switchover
   - Replace main workflow with simplified version
   - Keep old commands available but deprecated
   - Monitor first 2-3 releases closely

3. Cleanup (after validation)
   - Remove individual command files
   - Archive old workflow for reference

Rollback Procedure

# Quick rollback if issues arise

git revert <commit-with-changes>

# Or switch workflows

mv .github/workflows/release.yml .github/workflows/release-new.yml
mv .github/workflows/release-old.yml .github/workflows/release.yml

---

Benefits Summary

1. Code Reduction: ~70% fewer lines in workflow YAML
2. Maintainability: Centralized orchestration logic
3. Testability: Comprehensive unit/integration test coverage
4. Flexibility: Easy to add new steps or modify flow
5. Error Handling: Consistent error handling across all operations
6. Reusability: Commands work in CI and local development
