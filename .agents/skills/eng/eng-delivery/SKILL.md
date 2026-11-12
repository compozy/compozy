---
name: eng-delivery
description: Deliver work in the compozy/compozy repository. Use when dispatching or supervising sessions, opening or updating a PR, filing an issue, handling review feedback or CI, or declaring work done. Excludes product behavior of the shipped spec-cycle agents.
---

# Delivery

Repository rules for getting work from a dispatch to a merged PR. `CLAUDE.md` owns the CI-only gate and worktree rules; repeat them in every dispatch brief, because a child starts with only its brief and repository instructions.

Done means an open, ready PR on its final head, the merge-gate review criterion met for that head, every actionable review finding addressed, and `merge-gate` told. Merging belongs to `merge-gate`; a release needs a direct operator message to it.

## Dispatch

| Work | Provider | Model | Effort |
| --- | --- | --- | --- |
| Backend | `codex` | `gpt-6-astra` | `medium` |
| Frontend | `claude` | `claude-opus-5-5` | `medium` |
| Issue fix | `claude` | `claude-opus-5-5` | `medium` |
| Final review | `codex` and `claude` | `gpt-6-astra`, `claude-opus-5-5` | `xhigh`, `medium` |

Operator overrides win. Never substitute a model silently.

- The orchestrator is the quality guarantor. Send back unfinished work, and verify the result yourself before reporting it.
- Use child sessions from `compozy__session_create`, not subagents. A CLI fallback must pass `--parent <your session>`.
- Give each child a new worktree. Name it from the slice or issue so a retry cannot duplicate it, and keep the root checkout clean.
- Set provider, model and effort on the first prompt, confirm the effective runtime with `compozy__session_status`, and register a monitor. Do not answer a child's report in the same message chain; read it on the monitor wake.
- Keep about 4 workers active. Cut slices that can run in parallel and state which slice depends on which.
- Workers never run repository-wide gates. Say so explicitly in the brief, and name repository-wide lint, full test suites and E2E too.
- Prototyped screens and flows follow their prototype.
- Workers do not co-edit shared files such as `docs/_memory/change-impact.md`. Their impact notes go in the PR body, and the orchestrator consolidates them.

## Integration

Squash each finished slice into the integration worktree and resolve conflicts there; regenerate generated files through their owner. After the last slice, run the final review tier read-only and fix what it finds before opening the PR.

Stop finished children once their work is integrated. After a merge, remove only the worktrees and branches you created.

## Issues

Search for a duplicate before creating one, and reuse an existing issue. A closed issue or an existing PR means checking whether work remains first.

File issues in English on GitHub with reproduction, actual and expected behavior, evidence with code pointers, acceptance criteria, and QA or compatibility impact. Remove private paths, internal runtime identifiers, credentials and unrelated log content. Verify publication.

Every issue gets a type when it is created, including follow-ups filed by executors: `gh issue create --type <Bug|Feature|Task>`, or `gh issue edit <n> --type <T>` immediately after. Use Bug for defects and regressions, Feature for new capabilities, and Task for tests, chores, build and dependency work.

Independent issues get separate worktrees; several symptoms of one defect stay together.

## Pull request

Write everything in English, from `.github/PULL_REQUEST_TEMPLATE.md`. Keep its exact three headings and checklist, fill every section, and leave no HTML comments.

- Title: `<type>: <description>` or `<type>!: <description>`, describing the result. Squash subjects become release notes, so if the title mentions the product, it says `CompozyOS`, never bare `Compozy`.
- Link the issue with `Closes #<n>`, or `Refs #<n>` when closing needs a check you cannot make. For work without an issue, include an explicit `No issue: <reason>` line.
- Cover the trigger, root cause, observable before and after, tradeoffs, verification, and cross-surface, compatibility and QA impact, for a reader without this conversation. Keep it current as remediation changes the work.
- Omit private identifiers, host paths and credentials.
- A draft PR or first push is a checkpoint, not delivery.

### Screenshots

Host UI evidence in the R2 bucket `compozy-pr-evidence`, never through `gh --attach`, whose URLs expire.

```bash
wrangler r2 object put compozy-pr-evidence/<slug>/<short-sha>/<NN>-<name>.png --file <abs-path> --content-type image/png --remote
curl -sI https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/<key>
```

`--remote` is required. Expect `200` and `image/png` before using the URL in the PR body.

## CI

The latest `PR body` check must be `completed/success` on the current head before `ready:` or merge; body edits rerun this lightweight workflow. It enforces nonempty template sections, no HTML comments, an issue link or `No issue: <reason>`, no host paths or GitHub-hosted images, and CompozyOS title naming. Release branches `release/v*` and dependency bots pass explicitly. pr-watcher also reviews concrete verification, the cross-surface impact audit and R2 screenshot responses for UI changes.

There is no merge queue. Merge-train merges directly once its CodeRabbit rounds or fallback criterion and zero unresolved threads hold, with the target, title, head and mergeability verified. Other CI may be pending or red; `main` may go red briefly and is fixed right after. List every non-green check on the merged head for immediate repair. Report any remaining branch rule that blocks a direct merge; never bypass it with `--admin`.

Inspect these checks on the current head:

- `Verify (fmt, lint, unit, build)`, `E2E (combined lane)`, `PR title`, `PR body`, `React Doctor`
- Every other reported check, recording any result that is not successful or legitimately skipped; [Reviewers](#reviewers) owns the CodeRabbit criterion

Both gates always run and fail when `Detect changes` fails. The fast lane selects affected areas. Heavy lanes (integration, Windows and E2E) run on main pushes and nightly, plus manual runs; pull requests opt in with the `ci:full` label and a CI re-run, or by changing the CI pipeline itself. `E2E (combined lane)` succeeds when heavy lanes are not selected. Pending, cancelled or missing is not green, and success on an earlier head does not count. Other check results inform repair; `PR body` blocks the merge-gate criterion until successful.

Diagnose finished failures while other jobs run. Wait through a monitor, never a shell loop. A watch command can exit non-zero on a network error, so read the real conclusion with `gh run view`.

## Reviewers

CodeRabbit must meet merge-gate HEARTBEAT.md step 2: two distinct reviewed commits, a third after a heavy second round, or the fallback below. Every review thread must be resolved. Inspect Greptile and React Doctor results and address actionable findings as they arrive; other pending or red checks do not block merging. Elapsed time or silence proves no CodeRabbit coverage. Do not install or configure a reviewer.

- Every push restarts CI and the reviewers, so remediation works in rounds: one push per round, never one per finding.
- CodeRabbit allows 8 reviews per hour across the repository, so parallel PRs stall on it. Do not request a re-review of an unchanged head.
- CodeRabbit pauses after several pushed commits and its check shows `skipping`. Comment `@coderabbitai review` before claiming coverage.
- **CodeRabbit fallback.** When CodeRabbit blocks the PR because it is rate-limited, refuses to re-review an already-reviewed head, or skips a PR over its 300-file limit, replace it:
  1. Run a Claude Opus 5.5 reviewer subagent on the full PR diff. Use a plain reviewer brief, not `deep-review`. For the file limit only, an earlier external agent review of the same change, such as spec peer-review rounds, replaces this pass.
  2. Post one PR comment that starts with `## CodeRabbit fallback review` and names the reviewed head, the trigger, and each finding, or links the earlier review. `merge-gate` accepts only this heading from `pedronauck`.
  3. Fix or answer every finding like any other reviewer's.
  4. Dismiss a stale CodeRabbit `CHANGES_REQUESTED` review with a message linking that comment.

  Inspect Greptile, React Doctor and CI results and report non-green checks; only `PR body` blocks the merge-gate criterion.

A round starts as soon as a reviewer posts findings or a check fails on the current head; do not wait for the rest of CI to finish first, because the fix push restarts CI anyway. Fold in every finding already posted, push once, then inspect CI and satisfy the review criterion on that new head. Collect every page of reviews, inline threads and PR comments, including collapsed sections and the CodeRabbit "Follow-up" and "outside diff" notes that exist only in review bodies, and address every actionable finding at any severity, including human feedback and findings marked outdated or outside changed lines. Green CI on a head without the required CodeRabbit rounds or fallback is not delivered.

1. Validate each finding against the current code and the contract.
2. Fix every valid, in-scope finding and every failed check locally, with focused tests and lint for what changed, then push once. The `pre-push` hook rejects format and lint failures before they reach CI.
3. Reply to each thread with that pushed commit or other evidence, then resolve it. For a false positive, give a concrete rationale.
4. For a disputed or materially out-of-scope finding, reply with the rationale and escalate it. Resolve the thread once the operator or reviewer settles it; until then delivery stays open, because `merge-gate` and the `main` ruleset both require every thread resolved.

Start the next round on the new head until nothing actionable remains.

## Handoff

Find the `merge-gate` main session with `compozy agent instance list --agent merge-gate`. If the operator has not created the instance, report that as a blocker and leave delivery open. Capture the head, confirm it did not move during verification, then send that session one line with `compozy__session_prompt`:

```text
ready: #<number> <head-sha>
```

A refusal names the failed condition. Fix it, satisfy the review criterion on the new head, and hand off again with the new head.
