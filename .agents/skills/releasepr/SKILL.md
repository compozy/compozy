---
name: releasepr
description: "Guides adoption and day-to-day use of the pr-release CLI in a consuming GitHub repository: installing the binary, wiring the release GitHub Actions workflow, authoring .pr-release.yaml, following conventional commits for semantic versioning, adding custom release notes with add-note, validating with dry-run, and understanding the release-PR then merge then production-release lifecycle. Use when a project depends on compozy/releasepr (pr-release) and needs to configure, run, or troubleshoot its release automation. Not for editing the pr-release Go source itself, and not a general changelog or semantic-versioning tutorial."
license: MIT
metadata:
  domain: release-automation
  triggers: pr-release, releasepr, release PR, changelog, conventional commits, git-cliff, GoReleaser, .pr-release.yaml, add-note, dry-run
  role: specialist
  scope: integration
---

`pr-release` (Go module `github.com/compozy/releasepr`) automates the GitHub
release workflow: it inspects conventional commits, computes the next semantic
version, generates a `git-cliff` changelog, opens or updates a release pull
request, and — once that PR merges — drives a GoReleaser production release.
This skill is for agents working in a **consuming project** that depends on
pr-release. It is not for hacking on the pr-release Go source.

The detail that makes setup and usage correct lives in `references/`. The
inline content in this file is a **dispatcher and a tripwire, not the
contract**. Match the task in the router, then read the listed file in full
before acting.

## Required Reading Router

Match the task to the row. Read the listed file(s) **in full before** producing
output or running commands. They are load-bearing, not appendices.

| Task                                                          | MUST read                                                          |
| ------------------------------------------------------------- | ------------------------------------------------------------------ |
| Set up pr-release in a repo for the first time                | `references/setup.md` + `references/configuration.md`              |
| Create or change `.pr-release.yaml` / env vars / artifacts    | `references/configuration.md`                                      |
| Run or script a CLI command, choose flags                     | `references/commands.md`                                           |
| Understand or debug the release lifecycle / CI behavior       | `references/release-workflow.md`                                   |
| Write commit messages or add custom release notes             | `references/release-notes.md`                                      |
| A release step failed or behaved unexpectedly                 | `references/troubleshooting.md`                                    |

## Reference Index

- `references/setup.md` — install options (prebuilt tarball, `go install`), the
  GitHub token requirement, owner/repo auto-detection order, copying the
  workflow template, and the first dry-run.
- `references/configuration.md` — every `.pr-release.yaml` field, defaults,
  validation rules, the full environment-variable alias matrix, and
  `release_artifacts` constraints.
- `references/commands.md` — `pr-release`, `dry-run`, `add-note`, `version`:
  every flag and its exact behavior.
- `references/release-workflow.md` — the end-to-end lifecycle: which commits
  trigger what, release branch/PR naming, dry-run checks, and how a merge
  becomes a production release.
- `references/release-notes.md` — conventional-commit prefixes that drive the
  version bump, the `add-note` workflow, and `.release-notes/` archival.
- `references/troubleshooting.md` — symptom → cause → fix for the common
  failures (token format, owner/repo detection, "no release PR produced", etc.).

## Operating loop

1. **Confirm the dependency.** Verify the project actually uses pr-release
   (a release workflow invoking `pr-release`, a `.pr-release.yaml`, or an
   explicit request). If it does not, this skill does not apply.

2. **Verify wiring before changing anything.** Run the read-only helper from
   the consuming repo root:
   `bash skills/releasepr/scripts/check-setup.sh`
   It reports PASS/WARN/FAIL for the binary, token, config file, owner/repo
   resolution, conventional commits, and the workflow file. It only inspects —
   it never writes, pushes, or calls mutating APIs.

3. **Set up (if check-setup reports missing pieces).** Copy
   `skills/releasepr/assets/release.yml.template` into
   `.github/workflows/` and `skills/releasepr/assets/pr-release.yaml.template`
   to `.pr-release.yaml`, then adjust.
   **STOP. Read `references/setup.md` and `references/configuration.md` in full
   before installing pr-release or editing any config.** The asset templates and
   the bullets here are starting points, not the contract — defaults, validation
   ranges, and the env-var matrix are only complete in those files.

4. **Run or script a command.** The de-facto standard CI invocation across
   every observed consumer is
   `pr-release pr-release --force --enable-rollback --ci-output` — install via
   `go run "<module>@<pinned tag>"`. `--force` makes it idempotent, not "release
   with no changes".
   **STOP. Read `references/commands.md` in full before running `pr-release`,
   `dry-run`, or `add-note`, or adding any flag to a script.** The flag list
   here is a tripwire; semantics (e.g. `--rollback`, `--session-id`,
   `--skip-pr`) are only correct in that file.

5. **Reason about lifecycle / CI.** Before explaining why a release did or did
   not happen, or editing the workflow.
   **STOP. Read `references/release-workflow.md` in full before diagnosing CI
   behavior or modifying the release workflow.** Trigger rules and the
   merge → production-release path are only accurate there.

6. **Diagnose a failure.**
   **STOP. Read `references/troubleshooting.md` in full before proposing a fix
   for any pr-release error.** Match the exact error string to its row.

## Gist tripwires (incomplete on purpose)

Conventional-commit prefixes drive the version bump — wrong prefixes mean wrong
versions or "no changes":

- `fix:` → patch · `feat:` → minor · `feat!:` / `BREAKING CHANGE:` → major.
- A commit subject starting with `release:` or `ci(release):` merged to the
  default branch is what triggers the **production release** — do not hand-write
  such commits.
- Bot commits and `Merge pull request` commits are skipped by the release-PR job.

**STOP. Read `references/release-notes.md` and `references/release-workflow.md`
in full before advising on commit messages or release timing.** The three
bullets above are tripwires, not the rule set.

Setup prerequisites that cause the most real-world failures:

- Checkout MUST be unshallow with tags (`fetch-depth: 0`, `fetch-tags: true`)
  or the version is wrong.
- `git-cliff` MUST be installed in the runner; pr-release shells out to it.
- Use a dedicated `RELEASE_TOKEN` (PAT/App), not the default `GITHUB_TOKEN`,
  or downstream checks won't dispatch.

**STOP. Read `references/setup.md` in full before installing or wiring
pr-release in any repository.** These three are tripwires, not the setup
contract.

## Bundled resources

- `scripts/check-setup.sh` — **read-only** consumer-repo wiring verifier. Safe
  to run anytime; performs no writes, pushes, or mutating API calls.
- `assets/pr-release.yaml.template` — copy-in starter config (every field
  commented).
- `assets/release.yml.template` — copy-in starter GitHub Actions release
  workflow for a consuming repo.
