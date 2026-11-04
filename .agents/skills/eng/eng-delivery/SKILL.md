---
name: eng-delivery
description: Deliver work in the compozy/compozy repository. Use when dispatching or supervising sessions, opening or updating a PR, filing an issue, handling review feedback or CI, or declaring work done. Excludes product behavior of the shipped spec-cycle agents.
---

# Delivery

Repository rules for getting work from a dispatch to a merged PR. `CLAUDE.md` owns the CI-only gate and worktree rules; repeat them in every dispatch brief, because a child starts with only its brief and repository instructions.

Done means an open, ready PR on its final head, every check in [CI](#ci) green for that head, every reviewer closed, and `merge-train` told. Merging belongs to `merge-train`; a release needs a direct operator message to it.

## Dispatch

| Work | Provider | Model | Effort |
| --- | --- | --- | --- |
| Backend | `codex` | `gpt-6-astra` | `medium` |
| Frontend | `claude` | `claude-opus-5-5` | `medium` |
| Issue fix | `claude` | `claude-opus-5-5` | `medium` |
| Final review | `codex` and `claude` | `gpt-6-astra`, `claude-opus-5-5` | `xhigh`, `high` |

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

Independent issues get separate worktrees; several symptoms of one defect stay together.

## Pull request

Write everything in English, from `.github/PULL_REQUEST_TEMPLATE.md`. Do not keep its historical comment.

- Title: `<type>: <description>` or `<type>!: <description>`, describing the result. Squash subjects become release notes, so if the title mentions the product, it says `CompozyOS`, never bare `Compozy`.
- Link the issue with `Closes #<n>`, or `Refs #<n>` when closing needs a check you cannot make.
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

The `main` ruleset requires no status checks, so `gh pr checks --required` reports nothing. This list is the gate, for the current head only:

- `Verify (fmt, lint, unit, build)`, `E2E (combined lane)`, `PR title`, `React Doctor`
- Every other reported check except CodeRabbit, which [Reviewers](#reviewers) owns, is successful or legitimately skipped

Both gates always run and fail when `Detect changes` fails. A lane inside them may skip only because the change does not touch its area. When the repository variable `CI_MERGE_QUEUE` is `true`, pull requests defer the heavy lane (integration, Windows and E2E) to the merge queue, and `E2E (combined lane)` passes with a "Heavy lane deferred" notice. Add the `ci:full` label and re-run CI to run the heavy lane on the pull request, for example after an E2E fix. Pending, cancelled or missing is not green, and success on an earlier head does not count.

Diagnose finished failures while other jobs run. Wait through a monitor, never a shell loop. A watch command can exit non-zero on a network error, so read the real conclusion with `gh run view`.

## Reviewers

CodeRabbit, Greptile and React Doctor must all finish on the final changes. Confirm each from its review or check on that head; elapsed time, a score or silence proves nothing. Do not install or configure a reviewer. If one cannot run, report the blocker and leave delivery open, except for the CodeRabbit fallback below.

- Every push restarts CI and the reviewers, so remediation works in rounds: one push per round, never one per finding.
- CodeRabbit allows 8 reviews per hour across the repository, so parallel PRs stall on it. Do not request a re-review of an unchanged head.
- CodeRabbit pauses after several pushed commits and its check shows `skipping`. Comment `@coderabbitai review` before claiming coverage.
- **CodeRabbit fallback.** When CodeRabbit blocks the PR because it is rate-limited, refuses to re-review an already-reviewed head, or skips a PR over its 300-file limit, replace it:
  1. Run a Claude Opus 5.5 reviewer subagent on the full PR diff. Use a plain reviewer brief, not `deep-review`. For the file limit only, an earlier external agent review of the same change, such as spec peer-review rounds, replaces this pass.
  2. Post one PR comment that starts with `## CodeRabbit fallback review` and names the reviewed head, the trigger, and each finding, or links the earlier review. `merge-train` accepts only this heading from `pedronauck`.
  3. Fix or answer every finding like any other reviewer's.
  4. Dismiss a stale CodeRabbit `CHANGES_REQUESTED` review with a message linking that comment.

  Greptile, React Doctor and CI still apply.

A round starts when CI and every reviewer have finished on the current head; a failed check may start it earlier, and the round still folds in every finding already posted. Collect every page of reviews, inline threads and PR comments, including collapsed sections, and address every actionable finding at any severity, including human feedback and findings marked outdated or outside changed lines.

1. Validate each finding against the current code and the contract.
2. Fix every valid, in-scope finding and every failed check locally, with focused tests and lint for what changed, then push once. The `pre-push` hook rejects format and lint failures before they reach CI.
3. Reply to each thread with that pushed commit or other evidence, then resolve it. For a false positive, give a concrete rationale.
4. For a disputed or materially out-of-scope finding, reply with the rationale and escalate it. Resolve the thread once the operator or reviewer settles it; until then delivery stays open, because `merge-train` and the `main` ruleset both require every thread resolved.

Start the next round on the new head until nothing actionable remains.

## Handoff

Find the `merge-train` main session with `compozy agent instance list --agent merge-train`. If the operator has not created the instance, report that as a blocker and leave delivery open. Capture the head, confirm it did not move during verification, then send that session one line with `compozy__session_prompt`:

```text
ready: #<number> <head-sha>
```

A refusal names the failed condition. Fix it, wait for CI and reviewers on the new head, and hand off again with the new head.
