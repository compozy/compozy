---
title: Watch mode for PR review remediation
type: feature
---

`compozy reviews watch` runs a long-lived loop that polls your review provider, fetches each new actionable round, runs `reviews fix`, optionally auto-pushes the resulting commits, and repeats until the PR is clean or a max-rounds cap is hit. The watch run shows up in the dashboard as a parent run with each round's `reviews fix` linked underneath, so you can step away from a noisy PR and come back to a finished branch.

### CLI

```bash
# Auto-push each round until clean (or max 6 rounds)
compozy reviews watch tools-registry --provider coderabbit --pr 85 \
  --auto-push --until-clean --max-rounds 6

# Follow events live instead of backgrounding
compozy reviews watch tools-registry --provider coderabbit --pr 85 --stream

# Tune timing
compozy reviews watch my-feature --provider coderabbit --pr 85 \
  --poll-interval 30s --review-timeout 30m --quiet-period 20s
```

`reviews watch` does not support cockpit UI attach — `--ui`, `--attach ui`, and `--tui` are rejected. Use `--stream` to follow events or `--detach` for fire-and-forget.

### TOML

```toml
[defaults]
auto_commit = true   # required when watch_reviews.auto_push = true

[fetch_reviews]
provider = "coderabbit"

[watch_reviews]
max_rounds     = 6
poll_interval  = "30s"
review_timeout = "30m"
quiet_period   = "20s"
auto_push      = true
until_clean    = true
push_remote    = "origin"
push_branch    = "feature/reviews"   # must be set together with push_remote
```

### How it works

1. Take a snapshot of git state and reconcile any already-committed unpushed commits (emitted as `round = 0` push events).
2. Poll the provider every `poll_interval` until the PR head is **settled** — for CodeRabbit, that means the latest commit status is `success`, not just any submitted review.
3. Wait `quiet_period` for in-flight review activity to drain, then re-check status.
4. If the next round has actionable issues, spawn a child `reviews fix` run, await its terminal state, and (with `--auto-push`) `git push <remote> HEAD:<branch>`.
5. Loop until `clean` (provider returns no actionable issues) or `max_rounds`.

Defaults: 6 rounds, 30 s poll, 30 m review timeout, 20 s quiet period.

### Auto-push safety rails

- Forces `auto_commit=true` on child runs; rejects `--auto-commit=false`.
- Only ever runs `git push <remote> HEAD:<branch>` — never `restore`, `reset`, `clean`, or branch switching.
- Reconciles existing unpushed commits at startup so a watch run never re-pushes work it didn't produce.
- Config-driven `auto_push=true` requires `defaults.auto_commit=true`.
- `push_remote` and `push_branch` must be set together (or both omitted to resolve upstream).

### Extension hooks

Four new hooks let extensions observe and gate the loop. Hooks can veto a round / push (`continue=false`/`push=false` + `stop_reason`) but cannot fake a clean state:

| Hook                      | Fires                                                  |
| ------------------------- | ------------------------------------------------------ |
| `review.watch_pre_round`  | Before each provider poll / fix round                  |
| `review.watch_post_round` | After a child `reviews fix` run reaches terminal state |
| `review.watch_pre_push`   | Before auto-push, with the resolved remote/branch      |
| `review.watch_finished`   | When the watch loop ends (clean, max-rounds, or error) |

### Caveats

- Provider support: **CodeRabbit only** for the settle-gating logic in this release; other providers are wired via the registry but settle behavior is CodeRabbit-specific.
- Provider auth still uses the existing fetch path (CodeRabbit token, GitHub PR access). Shorter `poll_interval` values increase pressure on those rate limits.
- Each watch run shows up in the dashboard with a persisted `parent_run_id`; the parent and the active child collapse into a single active row, full history retained.
