# Troubleshooting

Match the exact error or symptom to a row before proposing a fix.

| Symptom / error | Cause | Fix |
| --------------- | ----- | --- |
| `config validation failed: invalid github_token: invalid token format` | Token does not match any accepted pattern. | Use a classic PAT (40 hex), fine-grained (`github_pat_`+82), app token (`ghs_`+36), or OAuth (`gho_`+36). Check for whitespace/quotes in the secret. See `configuration.md`. |
| `github_token is required for GitHub operations` | No token resolved for a GitHub step. | Set `GITHUB_TOKEN`/`RELEASE_TOKEN`/`PR_RELEASE_GITHUB_TOKEN`/`COMPOZY_RELEASE_GITHUB_TOKEN`. In CI, ensure the secret is exposed to that job's env. |
| `unable to determine GitHub owner/repo; set via config or environment` | No `github_owner/repo`, no `GITHUB_REPOSITORY*`, and `origin` not parseable. | Set `GITHUB_REPOSITORY=owner/repo` (auto in Actions) or `github_owner`/`github_repo` in `.pr-release.yaml`, or add a parseable `origin` remote. |
| `config validation failed: invalid owner format` / `owner too long` / `invalid repository format` / `repository too long` | Owner/repo fail the name regex or length (owner ≤ 39, repo ≤ 100). | Correct the configured `github_owner`/`github_repo`. |
| "No release PR branch produced; skipping release PR checks." | No conventional commits since the last tag → no version bump. | Expected. Land `feat:`/`fix:` commits, or force with `pr-release pr-release --force` (or the `force_release` dispatch input). See `release-notes.md`. |
| `config validation failed: git_push_timeout_minutes must be between 1 and 30` | Out-of-range value. | Set `git_push_timeout_minutes` (or `GIT_PUSH_TIMEOUT_MINUTES`) to 1–30. |
| `config validation failed: tools_dir cannot be empty` / `tools_dir contains invalid path traversal` | `tools_dir` empty or contains `..`. | Set a non-empty repo-relative directory (default `tools`). |
| `release_artifacts[i].command must be one of: bun, go, make, node, npm, npx, pnpm, yarn` | Unsupported command in a `release_artifacts` entry. | Use one of the allowed commands; wrap others via `make`/`npm` scripts. |
| `release_artifacts[i].add must include at least one path or glob` / `path must be repository-relative` / `path cannot contain traversal` | Missing/invalid `add` entry. | Provide ≥ 1 repo-relative path/glob, no absolute paths, no `..`. |
| `release_artifacts[i].timeout_seconds must be between 1 and 3600` | Out-of-range timeout. | Use 1–3600, or omit (`0` = unset). |
| `config validation failed: invalid log_level` / `invalid log_format` | Value outside the allowed set. | `log_level` ∈ debug/info/warn/error; `log_format` ∈ json/console. |
| Dry-run CI job never runs on the release PR | PR title prefix not matched. | Keep the title `release: Release vX.Y.Z` or `ci(release): Release vX.Y.Z`; do not rename the release branch pattern. See `release-workflow.md`. |
| Production release never fires after merge | Merge commit subject not `release:`/`ci(release):`, or pushed to a non-default branch. | Merge the release PR so the release commit lands on the default branch with the expected subject prefix. |
| Release-PR job did not run on a normal push | Head commit was a skipped kind (bot, `release:`, `ci(release):`, `Merge pull request`). | Expected by design. Push a regular conventional commit, or dispatch the workflow with mode `release-pr`. |
| Follow-up workflows not dispatched from the release-PR job | Default `GITHUB_TOKEN` cannot trigger other workflows. | Use a dedicated `RELEASE_TOKEN` PAT/app token for that job. |
| Wrong / unexpectedly low version, or always the initial version | Shallow checkout — no history/tags for `git-cliff`. | `actions/checkout@v4` with `fetch-depth: 0` and `fetch-tags: true`. See `setup.md`. |
| First-ever release picks `v0.0.x` off `v0.0.0` instead of intended baseline | Repo has no tags and `INITIAL_VERSION` is unset. | Set `INITIAL_VERSION` (e.g. `v0.0.1`) in the workflow env. See `configuration.md`. |
| `git cliff: command not found` / changelog or version step fails | `git-cliff` not installed in the runner. | Install git-cliff before invoking pr-release (binary, `taiki-e/install-action`, pipx, or bun/npm). See `setup.md`. |
| `release_artifacts` command cannot see version/branch values | Script reads the wrong variable names. | Use the injected `PR_RELEASE_*` vars (`PR_RELEASE_VERSION`, `PR_RELEASE_BRANCH`, etc.). See `configuration.md`. |
| `go run` in CI fails with temp-dir/permission errors | Restricted default `TMPDIR` on the runner. | Point a writable temp dir, e.g. set `GOTMPDIR` to `${{ runner.temp }}/go-tmp` (created beforehand). |

If the error is not listed, re-read `configuration.md` (validation) and
`release-workflow.md` (trigger rules) before guessing — most failures are a
config validation rule or a CI trigger condition.
