# Configuration reference

## Contents

- Resolution order
- Config file names
- `.pr-release.yaml` fields and defaults
- Validation rules
- `release_artifacts` schema
- Environment variables injected into `release_artifacts` commands
- Environment variable matrix
- Repository detection variables
- `INITIAL_VERSION` (first release baseline)

## Resolution order

Settings are resolved by Viper in this order (later sources do not override an
explicit earlier value; defaults fill the rest):

1. Environment variables (bound aliases below; also `AutomaticEnv`).
2. YAML config file in the working directory.
3. Built-in defaults.
4. Repository owner/repo: config/env, then `GITHUB_REPOSITORY` /
   `GITHUB_REPOSITORY_OWNER` / `GITHUB_REPOSITORY_NAME`, then the `origin`
   remote URL.

## Config file names

Searched in the current directory, first found wins:
`.pr-release.yaml` (preferred) or `.compozy-release.yaml` (legacy). YAML only.

## `.pr-release.yaml` fields and defaults

| Key                        | Type     | Default                              | Notes |
| -------------------------- | -------- | ------------------------------------ | ----- |
| `github_token`             | string   | (none)                               | Required for GitHub operations. Format-validated (see below). |
| `github_owner`             | string   | auto-detected                        | Override repository owner. |
| `github_repo`              | string   | auto-detected                        | Override repository name. |
| `tools_dir`                | string   | `tools`                              | NPM workspace directory; cannot be empty; no `..`. |
| `npm_token`                | string   | (none)                               | Only needed when publishing npm packages. |
| `log_level`                | string   | `info`                               | One of `debug`, `info`, `warn`, `error`. |
| `log_format`               | string   | `json` (or `console` when in CI)     | One of `json`, `console`. CI auto-detected. |
| `git_push_timeout_minutes` | int      | `2`                                  | Must be 1–30 inclusive. |
| `release_artifacts`        | list     | (empty)                              | Optional extra build commands; schema below. |

CI auto-detection (drives the `log_format` default) checks any of: `CI`,
`CONTINUOUS_INTEGRATION`, `GITHUB_ACTIONS`, `GITLAB_CI`, `CIRCLECI`, `TRAVIS`,
`JENKINS_URL`, `BUILDKITE`, `DRONE`, `TEAMCITY_VERSION`.

## Validation rules

Validation runs at load time; failure aborts the command with
`config validation failed: ...`.

- `github_token` (only if set): must match exactly one of — classic PAT
  `^[a-fA-F0-9]{40}$`; fine-grained `^github_pat_[a-zA-Z0-9_]{82}$`; app token
  `^ghs_[a-zA-Z0-9]{36}$`; OAuth `^gho_[a-zA-Z0-9]{36}$`. Otherwise
  `invalid token format`.
- `github_owner`: non-empty, matches
  `^[a-zA-Z0-9][a-zA-Z0-9\-_.]*[a-zA-Z0-9]$|^[a-zA-Z0-9]$`, ≤ 39 chars.
- `github_repo`: same name regex, ≤ 100 chars.
- `tools_dir`: non-empty and must not contain `..`.
- `log_level` / `log_format`: must be in the allowed sets above.
- `git_push_timeout_minutes`: integer 1–30.
- For GitHub operations specifically, `github_token` must be present
  (`github_token is required for GitHub operations`).

## `release_artifacts` schema

Each entry runs an additional build step whose outputs are staged into the
release. All fields validated at load time.

| Field             | Required | Rule |
| ----------------- | -------- | ---- |
| `name`            | yes      | Non-empty label. |
| `command`         | yes      | Exactly one of: `bun`, `go`, `make`, `node`, `npm`, `npx`, `pnpm`, `yarn`. |
| `args`            | no       | String list passed to the command. |
| `add`             | yes      | ≥ 1 path/glob; each repo-relative, not absolute, no `..` segment. |
| `timeout_seconds` | no       | `0` = unset; otherwise 1–3600. |

Example:

```yaml
release_artifacts:
  - name: Frontend bundle
    command: npm
    args: ["run", "build"]
    add: ["dist/**", "*.min.js"]
    timeout_seconds: 300
```

## Environment variables injected into `release_artifacts` commands

When a `release_artifacts` command runs, pr-release injects these env vars (in
addition to the inherited process environment) so the command can generate
version-aware output and stage it via `add`:

| Variable | Value |
| -------- | ----- |
| `PR_RELEASE_VERSION`        | Next version, e.g. `v1.4.0` |
| `PR_RELEASE_VERSION_NUMBER` | Version without the leading `v`, e.g. `1.4.0` |
| `PR_RELEASE_BRANCH`         | The `release/vX.Y.Z` branch name |
| `PR_RELEASE_PREVIOUS_TAG`   | The previous release tag |
| `PR_RELEASE_CHANGELOG_PATH` | `CHANGELOG.md` |
| `PR_RELEASE_BODY_PATH`      | `RELEASE_BODY.md` |
| `PR_RELEASE_NOTES_PATH`     | `RELEASE_NOTES.md` |
| `PR_RELEASE_DATE`           | UTC RFC3339 timestamp |
| `PR_RELEASE_GITHUB_OWNER`   | Resolved owner |
| `PR_RELEASE_GITHUB_REPO`    | Resolved repo |

Real-world example (a consumer generating site changelog MDX from a bun script
and committing it into the release PR):

```yaml
release_artifacts:
  - name: site-changelog
    command: bun
    args: ["run", "release:site-changelog"]
    add: ["packages/site/content/blog/changelog/*.mdx"]
    timeout_seconds: 120
```

## Environment variable matrix

Each config key binds the listed env vars (first non-empty wins):

| Config key                 | Env var aliases |
| -------------------------- | --------------- |
| `github_token`             | `GITHUB_TOKEN`, `PR_RELEASE_GITHUB_TOKEN`, `COMPOZY_RELEASE_GITHUB_TOKEN`, `RELEASE_TOKEN` |
| `github_owner`             | `GITHUB_OWNER`, `PR_RELEASE_GITHUB_OWNER`, `COMPOZY_RELEASE_GITHUB_OWNER` |
| `github_repo`              | `GITHUB_REPO`, `PR_RELEASE_GITHUB_REPO`, `COMPOZY_RELEASE_GITHUB_REPO` |
| `tools_dir`                | `TOOLS_DIR`, `PR_RELEASE_TOOLS_DIR`, `COMPOZY_RELEASE_TOOLS_DIR` |
| `log_level`                | `LOG_LEVEL`, `PR_RELEASE_LOG_LEVEL`, `COMPOZY_RELEASE_LOG_LEVEL` |
| `log_format`               | `LOG_FORMAT`, `PR_RELEASE_LOG_FORMAT`, `COMPOZY_RELEASE_LOG_FORMAT` |
| `npm_token`                | `NPM_TOKEN`, `PR_RELEASE_NPM_TOKEN`, `COMPOZY_RELEASE_NPM_TOKEN` |
| `git_push_timeout_minutes` | `GIT_PUSH_TIMEOUT_MINUTES`, `PR_RELEASE_GIT_PUSH_TIMEOUT_MINUTES`, `COMPOZY_RELEASE_GIT_PUSH_TIMEOUT_MINUTES` |

## Repository detection variables

Used only to resolve owner/repo when `github_owner`/`github_repo` are unset:

- `GITHUB_REPOSITORY` — `owner/repo` slug; highest priority. Set automatically
  by GitHub Actions.
- `GITHUB_REPOSITORY_OWNER` — owner fallback.
- `GITHUB_REPOSITORY_NAME` — repo-name fallback.
- Otherwise the `origin` remote URL (SSH `git@host:owner/repo.git` or HTTPS
  `https://host/owner/repo.git`) is parsed.

## `INITIAL_VERSION` (first release baseline)

When the repository has **no git tags**, version calculation uses the
`INITIAL_VERSION` environment variable as the baseline tag (e.g.
`INITIAL_VERSION=v0.0.1`). If it is unset, the baseline defaults to `v0.0.0`.
This only matters for a project's very first release; once a tag exists it is
ignored. Set it in the workflow env so the first published version is
intentional, not an accidental `v0.0.x` off `v0.0.0`.

## Orchestrator timeout/retry env (advanced, optional)

Read at runtime by the orchestrator; durations use Go duration strings
(e.g. `90m`):

- `WORKFLOW_TIMEOUT` (default 60m), `RELEASE_WORKFLOW_TIMEOUT` (default 120m),
  `ROLLBACK_TIMEOUT` (default 10m), `RETRY_DELAY` (default 1s).
- `RETRY_COUNT` (integer, default 3).
