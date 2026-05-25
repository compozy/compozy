# Setting up pr-release in a consuming repository

This file is the contract for first-time setup. Read it fully before installing
or wiring pr-release. For every configuration field and validation rule, also
read `configuration.md` (the router pairs them).

## Contents

- Prerequisites (hard requirements)
- Install the CLI
- GitHub token and workflow permissions
- Owner/repo resolution and INITIAL_VERSION
- Add the release workflow
- Optional config file
- Verify wiring
- First safe run

## Prerequisites (hard requirements)

pr-release will compute the wrong version or fail without these:

- **Full git history + tags in CI.** The version is derived from commits since
  the last tag, so the checkout MUST be unshallow with tags:
  `actions/checkout@v4` with `fetch-depth: 0` and `fetch-tags: true`. A shallow
  clone is the most common cause of a wrong/initial version.
- **`git-cliff` available at runtime.** pr-release shells out to `git cliff`
  for the changelog and version bump. Install it in CI before invoking
  pr-release (a binary download, the `taiki-e/install-action` for `git-cliff`,
  `pipx install git-cliff`, or a bun/npm package — all observed in practice).
- **Go toolchain** if using the recommended `go run` install (below).

## Install the CLI

**Recommended (matches every observed consumer): `go run` with a pinned
version.** Pin the module+version once in a workflow env var and invoke it
directly — Go module caching makes this fast and reproducible, with no separate
install/build step:

```yaml
env:
  PR_RELEASE_MODULE: github.com/compozy/releasepr@v0.0.21  # pin a real tag
# ...
  - run: go run "${{ env.PR_RELEASE_MODULE }}" pr-release --force --enable-rollback --ci-output
```

Always pin to an explicit tag (not `@latest`) so releases are reproducible.

Alternative — prebuilt archive (for environments without a Go toolchain):

```bash
VERSION=$(curl -sSf https://api.github.com/repos/compozy/releasepr/releases/latest | jq -r .tag_name)
OS=$(uname | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m); case "$ARCH" in x86_64|amd64) ARCH=x86_64;; arm64|aarch64) ARCH=arm64;; *) exit 1;; esac
curl -L "https://github.com/compozy/releasepr/releases/download/${VERSION}/pr-release_${VERSION}_${OS}_${ARCH}.tar.gz" -o pr-release.tgz
tar -xzf pr-release.tgz && ./pr-release/pr-release version
```

Alternative — `go install github.com/compozy/releasepr@<tag>` (binary named
`releasepr`). `version` prints `Version`/`Commit`/`Built` (`dev`/`unknown` for
non-release builds); use it as the install smoke test.

## GitHub token and workflow permissions

Use a **dedicated `RELEASE_TOKEN` secret** (a PAT or GitHub App token), not the
default `GITHUB_TOKEN`. Reason: the default `GITHUB_TOKEN` cannot trigger
downstream workflows, so the dry-run/CI checks dispatched after the release PR
is opened would not run. Every observed consumer passes `RELEASE_TOKEN` to
pr-release as `GITHUB_TOKEN`, and reuses it for the downstream publish
(GoReleaser, Homebrew, registry pushes):

```yaml
permissions:
  contents: write
  pull-requests: write
  # add for the downstream production release job:
  packages: write
  id-token: write
  attestations: write
# ...
    env:
      GITHUB_TOKEN: ${{ secrets.RELEASE_TOKEN }}
      RELEASE_TOKEN: ${{ secrets.RELEASE_TOKEN }}
```

Token must be a recognized format (classic PAT 40 hex; fine-grained
`github_pat_`+82; app `ghs_`+36; OAuth `gho_`+36) or load fails before any API
call. See `configuration.md` for the full env-var alias matrix.

## Owner/repo resolution and INITIAL_VERSION

Owner/repo is resolved automatically: config `github_owner`/`github_repo` →
`GITHUB_REPOSITORY`/`GITHUB_REPOSITORY_OWNER`/`GITHUB_REPOSITORY_NAME` (auto-set
by GitHub Actions) → the `origin` remote URL. If none resolve the run fails with
`unable to determine GitHub owner/repo`; see `troubleshooting.md`.

**`INITIAL_VERSION`** — when the repo has **no tags yet**, pr-release reads the
`INITIAL_VERSION` env var as the baseline (e.g. `v0.0.1`); without it the
baseline defaults to `v0.0.0`. Set it in the workflow env for a project's very
first release so the first version is what you intend.

## Add the release workflow

Copy `skills/releasepr/assets/release.yml.template` to
`.github/workflows/release.yml` and adjust the pinned `PR_RELEASE_MODULE`
version, Go version, default branch, and secret names. The template wires the
release-PR job on pushes to the default branch and a dry-run on release PRs; the
production publish is project-specific. The full lifecycle and trigger rules are
in `release-workflow.md`.

## Optional config file

A config file is optional — several consumers run purely on workflow env vars
and CLI flags with no file. To pin behavior or declare `release_artifacts`, copy
`skills/releasepr/assets/pr-release.yaml.template` to `.pr-release.yaml` (legacy
name `.compozy-release.yaml`). Every field, default, and the injected
`release_artifacts` env vars are in `configuration.md` — read it before editing.

## Verify wiring

From the consuming repo root, run the read-only verifier (no writes, pushes, or
mutating API calls):

```bash
bash skills/releasepr/scripts/check-setup.sh
```

It reports PASS/WARN/FAIL for the binary, token presence, config file,
owner/repo resolution, conventional commits, and the workflow file.

## First safe run

```bash
pr-release dry-run --ci-output
# or exercise the release-PR orchestrator without writing:
pr-release pr-release --dry-run --ci-output
```

For exact flag semantics see `commands.md`.
