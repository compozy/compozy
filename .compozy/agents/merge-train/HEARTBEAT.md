---
version: 1
enabled: true
summary: "Scan Pedro's green pull requests to main, verify each one, squash-merge the passing ones, then sync the root checkout."
preferences:
  min_interval: "1h"
  active_hours:
    - timezone: "America/Sao_Paulo"
      start: "08:00"
      end: "22:00"
context:
  include:
    - self
---

# Wake Checklist

Operator authorization (Pedro, direct message, 2026-11-03): on each wake, scan and merge without a `ready:` handoff, limited to the scope below. Everything else in AGENT.md still applies.

1. List candidates: `gh pr list --repo compozy/compozy --base main --author pedronauck --state open --json number,isDraft,headRefOid,headRefName --limit 100`. Drop drafts. Drop release pull requests (head branch `release/*`); a release runs only on a direct operator message. Only PRs authored by `pedronauck` are in scope; ignore every other author.
2. In ascending PR number, check the current `headRefOid` with fresh reads. Operator criterion (Pedro, 2026-11-03), which replaces AGENT.md conditions 4–5 for this scan: the PR has at least 2 CodeRabbit review rounds (CodeRabbit reviews on 2 or more distinct commits) and zero unresolved review threads; CI may still be pending. Release-push exception (Pedro, 2026-11-03, until the next release is published): open threads don't block. Reply to each thread, resolve it, merge, and delegate a code_implementer subagent to fix the findings on main under the host-load rule. Dismiss stale CodeRabbit `CHANGES_REQUESTED` reviews that block it; never dismiss a human review. Target, title and mergeability conditions still apply. A `CONFLICTING` PR that meets the criterion is rebased onto `main` with the `git-rebase` skill in its worktree, pushed with `--force-with-lease`, then merged. A PR that fails the criterion is skipped silently this wake.
3. For each PR that passes, merge with the Merge section command bound to that full head, confirm `MERGED`, then run the post-merge root sync (agent memory note "post-merge root sync") before the next PR. One PR at a time.
4. A `CONFLICTING` PR found by the scan is listed in the wake summary, not repaired; repair runs only through the Conflicts section after a `ready:` handoff.
5. Do not merge while a release is publishing: a release PR I am driving, or a running `Release` workflow on `main` whose head commit subject starts with `build: release ` or `build: prepare release `. The `Release` run that every ordinary push to `main` starts only refreshes the release PR and does not block merges.
6. End with a one-line summary: merged PRs with heads, skipped PRs with the first failing condition. If nothing changed, say so in one line.
