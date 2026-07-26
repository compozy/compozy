# CodeRabbit review-watch loop (`compozy reviews watch`)

## Contents

1. When this step applies
2. Resolve the task slug
3. The recipe command
4. Flag reference
5. Alternative loop modes
6. Failure paths

## 1. When this step applies

Run this step only when the pre-flight detector reports `compozy: true` AND a CodeRabbit-style external reviewer is configured for the PR. If `compozy` is missing from `PATH`, skip the step entirely — do not install it, and do not block the rest of the ship-pr flow. The reviewer integration is opt-in.

The watch loop polls the configured review provider (here, CodeRabbit) for new comment rounds on the PR, materialises each round as `.compozy/tasks/<slug>/reviews-NNN/`, spawns a child fix run for each, and (when `--auto-commit --auto-push` are set) pushes the resulting fixes back to the PR branch automatically.

## 2. Resolve the task slug

The slug is the workflow directory under `.compozy/tasks/`. The watch daemon stores state there; it must exist before the command runs.

Resolution order:

1. **Match an existing slug.** The detector returns `task_slug_candidates` listing every immediate subdirectory of `.compozy/tasks/`. If the current git branch name (or a substring of it) matches one of those slugs, use that.
2. **Existing slug from the user.** If multiple candidates match, ask the user which slug owns this PR before guessing.
3. **Create a new slug.** Otherwise, derive a kebab-case slug from the branch name (e.g., `feat/multi-account` → `multi-account`) and create the directory:

   ```bash
   mkdir -p ".compozy/tasks/${SLUG}"
   ```

   No further bootstrapping is required — the watch daemon populates the rest.

Slug conventions observed in the wild (from `compozy/agh/.compozy/tasks/`): `e2e-final`, `ui-final`, `final-qa`, `mem-v2`, `redesign-v2`, `site-copy`, `tailwind-refac`. Short, kebab-case, no version numbers unless they disambiguate.

## 3. The recipe command

```bash
compozy reviews watch \
  --auto-commit --auto-push \
  --batch-size 20 \
  --ide codex --model gpt-5.4 --reasoning-effort high \
  --max-rounds 3 \
  --name "${SLUG}" \
  --pr "${PR_NUMBER}" \
  --provider coderabbit
```

`${PR_NUMBER}` is the integer captured from `gh pr create`'s output in operating-loop step 6 (e.g., the `123` in `https://github.com/owner/repo/pull/123`). The slug is the value resolved in section 2.

Run the command in a foreground terminal where you can watch the cockpit UI, OR pass `--attach detach` to let it run in the background; both are valid for the end-of-feature flow.

## 4. Flag reference

Verified against `looper/internal/cli/reviews_exec_daemon.go:151–192`. Only the flags used in the recipe are listed; run `compozy reviews watch --help` for the full set.

| Flag                                       | Purpose                                                                                              |
| ------------------------------------------ | ---------------------------------------------------------------------------------------------------- |
| `--provider <name>`                        | Review provider name. Use `coderabbit` for CodeRabbit-style reviews.                                 |
| `--pr <number>`                            | Pull request number on the configured GitHub remote.                                                 |
| `--name <slug>`                            | Workflow name — the subdirectory under `.compozy/tasks/<slug>/` where state is written.              |
| `--auto-commit`                            | Commit the fixes produced by each child fix run automatically.                                       |
| `--auto-push`                              | Push committed fixes after each successful round (requires `--auto-commit`).                         |
| `--batch-size <N>`                         | Group up to N file-level issues into a single fix run. Default 1; recipe uses 20 for throughput.     |
| `--max-rounds <N>`                         | Stop after N watch rounds even if reviews are still arriving. Default: daemon config. 3 is the recipe. |
| `--until-clean`                            | Alternative to `--max-rounds` — keep looping until the PR head is reviewed clean.                    |
| `--ide <codex\|...>`                       | IDE/runtime backing the fix runs.                                                                    |
| `--model <id>`                             | Model id passed to the fix runs (e.g., `gpt-5.4`).                                                   |
| `--reasoning-effort <low\|medium\|high>`   | Forwarded to models that accept the parameter.                                                       |
| `--poll-interval <duration>`               | Provider polling interval, e.g. `30s`. Overrides daemon default.                                     |
| `--review-timeout <duration>`              | Maximum wait per round for provider review to arrive.                                                |
| `--quiet-period <duration>`                | Delay after pushing fixes before re-checking provider status.                                        |
| `--attach <auto\|ui\|stream\|detach>`      | How the CLI client attaches to the daemon. `auto` (default) opens the UI when TTY-attached.          |
| `--push-remote <name>` / `--push-branch <name>` | Override git remote / branch for `--auto-push`. Default: the PR's head ref on `origin`.        |

## 5. Alternative loop modes

The recipe uses `--max-rounds 3`. Two other reasonable shapes:

- `--until-clean` — let the watcher loop until the PR is reviewed clean. Best for small / mid PRs where review rounds converge quickly. Skip `--max-rounds`.
- `--max-rounds 1` with manual re-runs — useful when the orchestrator wants to inspect each round before continuing. Re-invoke the same command to start the next round.

## 6. Failure paths

- **`compozy: command not found`.** The detector should have caught this and the step should already be skipped. If the user wants to install `compozy`, point them at the `looper` repo's README.
- **Provider auth missing.** `compozy reviews watch` returns an authentication error from CodeRabbit. The user must configure the provider's GitHub App / token via `compozy` config before the watch can read reviews.
- **`.compozy/tasks/<slug>/` missing.** The daemon refuses to start with a clear error. Re-run after `mkdir -p ".compozy/tasks/<slug>/"` per section 2.
- **PR number mismatch.** If `--pr` points to a PR on a different remote/repo than `origin`, the watcher cannot pair reviews to commits. Verify with `gh pr view <number> --json url` before re-running.
- **`--auto-push` race.** When the developer pushes manually mid-round, the daemon may try to push on top of a stale tip. Re-run; the daemon recovers idempotently. Do not `git reset` to "fix" it.
