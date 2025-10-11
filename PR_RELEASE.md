# Release PR Orchestration

## What Changed

- Internal `pkg/release` CLI has been removed in favor of the external module `github.com/compozy/releasepr@v0.0.10`.
- `.github/workflows/release.yml` now shells out to `go run $PR_RELEASE_MODULE ...` for both the PR workflow and dry-run validation.
- The Makefile no longer builds the `compozy-release` binary; the only supported entrypoint is `go run github.com/compozy/releasepr@v0.0.10`.

These changes keep the existing release UX (changelog generation, branch creation, PR updates, GoReleaser, and npm publish) but remove the duplicated orchestration code from this repository.

## Quickstart

- **Create/refresh a Release PR locally**
  ```bash
  go run github.com/compozy/releasepr@v0.0.10 pr-release --enable-rollback --ci-output
  ```
  Add `--force` to bypass the "no changes" guard when testing.
- **Dry-run validation**
  ```bash
  go run github.com/compozy/releasepr@v0.0.10 dry-run --ci-output
  ```
- The module reads `.pr-release.yaml` or `.compozy-release.yaml` from the repo root. It also honors the same environment variables the old tool expected (`RELEASE_TOKEN`, `GITHUB_TOKEN`, `GITHUB_REPOSITORY`, `GITHUB_REPOSITORY_OWNER`, `NPM_TOKEN`, etc.).

## CI wiring

- `.github/workflows/release.yml` defines `PR_RELEASE_MODULE=github.com/compozy/releasepr@v0.0.10` and invokes:
  - `go run "$PR_RELEASE_MODULE" pr-release --enable-rollback --ci-output` (optionally `--force`).
  - `go run "$PR_RELEASE_MODULE" dry-run --ci-output` for PR validation.
- Existing setup steps (`setup-go`, `setup-node`, `setup-git-cliff`, `setup-release`) remain unchanged. The workflow still triggers GoReleaser and `scripts/publish-npm.sh` after a merge containing `ci(release):`.

## Known gaps

- The upstream module only bumps the root `package.json`. If we reintroduce workspace publishing under `tools/`, we must extend `github.com/compozy/releasepr` (preferred) or add a temporary CI step to align workspace versions before publishing.
- Ensure `git-cliff`, Node 22+, and the Go toolchain are available locally when running the commands by hand.

## Follow-up tasks

- Contribute workspace version bump support to `github.com/compozy/releasepr` and update the pinned version once released.
- Periodically review release logs to verify parity with the previous internal implementation and update this document with any new flags or workflow changes.
