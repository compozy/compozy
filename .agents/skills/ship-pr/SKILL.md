---
name: ship-pr
description: Ships a finished feature, bug fix, or refactor end-to-end — explores the change's impact on docs/site/README, generates release notes (via pr-release if present, otherwise inline from git log), writes a complete PR description (including QA screenshots and summary when qa-report/qa-execution artifacts exist), commits staged changes following the repo's commitlint config, opens the PR via gh CLI, and optionally launches a CodeRabbit review watch loop. Use when a feature/bug/refactor is implementation-complete and ready to ship. Do not use for draft PRs without finished work, for amending an already-merged PR, for repo-level release publishing (use pr-release directly), or for in-progress development checkpoints.
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---
# ship-pr — Finish-a-Feature Release & PR Workflow

This skill drives the end-of-feature ritual: detect what changed, document its impact, generate release notes, write a complete PR description, commit cleanly, open the PR via `gh`, and (optionally) start an automated review-watch loop.

Two layers exist:

- **Core path (always runs):** explore impact → build PR description → commit → `gh pr create`.
- **Optional integrations (auto-detected):** `pr-release` for release notes, `skeeper` for spec sync, `qa-report`/`qa-execution` artifacts under `docs/qa/`, `compozy reviews watch` for CodeRabbit auto-fix.

Optional steps are **skipped silently** when the supporting tool or artifact is not present — the skill never blocks on missing tooling.

## Required Reading Router

Match the phase to the row. Read the listed file **in full before** acting on that phase. Inline content in this SKILL.md is a tripwire, not the source of truth.

| Phase                                                | MUST read                                                |
| ---------------------------------------------------- | -------------------------------------------------------- |
| Explore PR impact (docs/site/README/changelog)       | `references/explore-impact.md`                           |
| Generate release notes                               | `references/release-notes.md`                            |
| Build PR description                                 | `references/pr-description.md`                           |
| Build PR description **with QA artifacts**           | `references/pr-description.md` + `references/qa-artifacts.md` |
| Commit staged changes                                | `references/commit-conventions.md`                       |
| Launch CodeRabbit review-watch loop                  | `references/coderabbit-watch.md`                         |

## Reference Index

- `references/explore-impact.md` — how to fan out parallel Explore subagents to surface every doc/site/README/CHANGELOG that the change touches, and how to aggregate their output.
- `references/release-notes.md` — two flows: invoke `pr-release add-note` when the CLI is on PATH, otherwise build an inline conventional-commit changelog for the PR body using the same emoji legend pr-release uses.
- `references/pr-description.md` — PR body template (Summary / Changes / Release Notes / QA / Test plan) and the rules for filling each section honestly.
- `references/qa-artifacts.md` — how to detect living `docs/qa/` artifacts, extract bug counts and screenshot paths, and inject them into the PR body.
- `references/commit-conventions.md` — commitlint-config detection priority, how to read `type-enum`/`scope-enum`, and the staging/HEREDOC commit rules.
- `references/coderabbit-watch.md` — exact `compozy reviews watch` command, task-slug resolution, and the skip-if-missing guard.

## Pre-flight: detect tooling

Run the bundled detector (**read-only** — probes the filesystem and `PATH` only):

```bash
bash skills/mine/ship-pr/scripts/detect-tooling.sh
```

It prints a JSON object describing what is available:

```json
{
  "gh": true,
  "pr_release": false,
  "compozy": true,
  "skeeper_installed": true,
  "skeeper_configured": false,
  "commitlint_config": "commitlint.config.cjs",
  "qa_output_paths": ["docs/qa"],
  "task_slug_candidates": ["my-feature"]
}
```

Branch decisions off this output. Notes:

- `skeeper_installed` reports whether the binary is on PATH; `skeeper_configured` is `true` only when both `.skeeper/` and `skeeper.lock` exist (the gating signal for step 3).
- `commitlint_config` is an empty string when no config is found — that means "use `@commitlint/config-conventional` defaults" per `references/commit-conventions.md` §7.

Hard requirements: `gh` on PATH and a git working tree with changes ready to ship (staged or unstaged in tracked files). Abort with a clear message if either is missing — do not invent workarounds.

## Operating Loop

Run the steps in order. Skip optional steps cleanly when the detector reports the supporting tool is absent.

### 1. Explore the change's impact

Fan out up to **3 parallel** Explore subagents over the diff against the base branch. Each agent gets a distinct slice: docs site / README & top-level docs / `.github/`, CHANGELOG, examples & e2e. Aggregate results into a single bullet list grouped by impact area — this list feeds steps 2 and 4.

Gist tripwires:

- Any new public API, CLI flag, config key, or env var → must be reflected in docs or README before shipping.
- Any new user-visible UI element → must have an entry in release notes.
- Any change to `.github/workflows/` → call it out separately; reviewers always want to see CI diffs.

**STOP. Read `references/explore-impact.md` in full before launching the Explore agents.** The inline tripwires above are not the contract — the reference defines the exact search prompts, the parallelism budget, and the aggregation format.

### 2. Generate release notes

Two flows, selected by the detector:

- **Flow A — `pr-release` on PATH.** For each user-visible bullet from step 1, run `pr-release add-note --title "<title>" --type {feature|fix|breaking|highlight} --body "<markdown>"`. The notes land in `.release-notes/` and are picked up automatically by the next release PR.
- **Flow B — fallback.** Build an inline changelog block to embed in the PR description: group `git log <base>..HEAD` by conventional-commit type using the same legend the project's `cliff.toml` uses (🎉 Features, 🐛 Bug Fixes, ⚡ Performance Improvements, 🔒 Security, 📚 Documentation, 📦 Build System, 🔧 CI/CD, ♻️ Refactoring, 🧪 Testing, 💅 Style, 🔧 Miscellaneous Tasks, ⏪ Reverts, Other Changes).

**STOP. Read `references/release-notes.md` in full before running either flow.** It contains the exact CLI invocations, the conventional-commit grouping rules, and the worked examples for both flows.

### 3. (Optional) Skeeper sync

Only if the detector reports `skeeper_configured: true` (i.e., `.skeeper/` directory and `skeeper.lock` both exist):

```bash
skeeper sync --dry-run         # preview
skeeper sync --commit -m "..."  # apply + stage skeeper.lock
```

Review the dry-run plan before applying. Skip this step entirely otherwise — do not install or initialise skeeper as a side effect.

### 4. Build the PR description

Assemble these sections in order: **Summary** (≤ 3 bullets, focus on user-visible outcome), **Changes** (grouped by impact area from step 1), **Release Notes** (excerpt from step 2; for Flow B paste the full inline changelog here), **QA** (only when the detector reports `qa_output_paths`), **Test plan** (checklist of checks actually performed — never invent results).

Gist tripwires:

- Title ≤ 70 characters; everything else goes in the body.
- "Test plan" must reflect what actually ran. Unrun checks belong in a follow-up issue, not in the checklist as if they passed.
- If the repo ships a `.github/PULL_REQUEST_TEMPLATE.md`, align the section headings to it before merging in the content above.

**STOP. Read `references/pr-description.md` in full before writing the body.** When the detector reports QA artifacts, **also STOP and read `references/qa-artifacts.md`** before drafting the QA section — that reference defines the path conventions, severity extraction, and screenshot linking rules.

### 5. Commit staged changes

Detect the repo's commit convention via the detector's `commitlint_config` value (fallback to `@commitlint/config-conventional` defaults). Stage files **by name** (never `git add -A`), write the commit message via HEREDOC, and never use `--no-verify` or `--no-gpg-sign`.

Gist tripwires:

- Do not stage files outside the scope of this feature, even if `git status` lists them as modified.
- If a pre-commit hook fails, fix the underlying issue and create a **new** commit. Do not `--amend` after a failed hook — the prior commit may belong to unrelated work.

**STOP. Read `references/commit-conventions.md` in full before staging or committing.** It contains the config-detection priority, the rules for reading `type-enum`/`scope-enum`, and the exact HEREDOC commit pattern.

### 6. Open the PR via `gh`

Refuse to push from the repo's main branch — opening a PR requires a feature branch with a distinct head:

```bash
BRANCH="$(git rev-parse --abbrev-ref HEAD)"
case "$BRANCH" in
  main|master|develop|trunk) echo "Refusing to ship from $BRANCH. Create a feature branch first." >&2; exit 1 ;;
esac

git push -u origin "$BRANCH"
gh pr create --title "<title>" --body-file "$BODY_FILE"
```

`$BODY_FILE` is the temp file written in step 4 (per `references/pr-description.md` §5). Capture the returned PR URL and number from `gh`'s stdout — the PR number is the input for step 7.

### 7. (Optional) CodeRabbit review-watch loop

Only if the detector reports `compozy: true`. Resolve the task slug:

1. If the detector reports any `task_slug_candidates`, pick the one whose name best matches the current git branch (or ask the user when ambiguous).
2. Otherwise, derive a kebab-case slug from the branch name and create `.compozy/tasks/<slug>/` (just `mkdir -p` — the `compozy reviews watch` daemon populates the rest).

Then run, substituting `<slug>` and `<number>`:

```bash
compozy reviews watch \
  --auto-commit --auto-push \
  --batch-size 20 \
  --ide codex --model gpt-5.4 --reasoning-effort high \
  --max-rounds 3 \
  --name <slug> \
  --pr <number> \
  --provider coderabbit
```

**STOP. Read `references/coderabbit-watch.md` in full before invoking the command.** It documents every flag, the alternative `--until-clean` mode, the attach-mode options, and the failure paths (missing `compozy`, missing `.compozy/tasks/<slug>/`, provider auth missing).

## When NOT to use

- Draft PRs of unfinished work — finish the implementation first; this skill assumes the change is shippable.
- Amending an already-merged PR — the `gh pr create` step would fail, and re-running `compozy reviews watch` against a merged PR has no effect.
- Repo-level release publishing (cutting a tagged release, opening a release PR off `RELEASE_BODY.md`) — that is the `pr-release` CLI's own `pr-release` subcommand. This skill only adds release-note entries; it does not orchestrate the release branch.
- In-progress development checkpoints / WIP commits — use a plain `git commit` instead. This skill is the end-of-feature ritual.
