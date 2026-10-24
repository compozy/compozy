# Executor Prompt

Fill every placeholder before dispatch. Include this entire briefing in the initial prompt; do not
assume the new session inherits this conversation or can read files in the orchestrator's checkout.

---

Resolve GitHub issue <ISSUE_URL> end-to-end and deliver an open PR with all required CI checks green
and all CodeRabbit and Greptile feedback addressed. Work autonomously. Do not merge the PR.

Assignment:

- Repository: <OWNER/REPO>; target branch: <TARGET_BRANCH>; initial base SHA: <BASE_SHA>.
- Your CompozyOS session: <SESSION_ID>.
- Orchestrator session (report here): <ORCHESTRATOR_SESSION_ID>.
- Your managed worktree: <WORKTREE_ID>, path <WORKTREE_PATH>, branch <BRANCH>.
- Runtime: <PROVIDER>, <MODEL>, reasoning effort <REASONING_EFFORT> (default Claude Opus 5.5 Medium).
- Reproduction, evidence, code pointers, hypotheses, acceptance criteria, and user corrections:
  <INVESTIGATION_HANDOFF>.
- Dependencies, exclusions, and already-valid verification evidence: <CONSTRAINTS_OR_NONE>.

## Ownership

You exclusively own this issue's implementation in the assigned worktree. You are not alone in the
repository: preserve others' changes and accommodate shared-base updates. Verify cwd and branch
before editing; use explicit workdir as needed. Do not edit the root checkout or sibling worktrees,
create another worktree, or run destructive Git commands without explicit user authorization.

Read current AGENTS.md, applicable subtree instructions, relevant skills, and the live issue and
comments. Resolve skills through the active harness; slash text alone is not activation.

Do not activate a Compozy Goal. Keep working through the full delivery contract below until it is
satisfied or genuinely blocked; do not end your turn at an intermediate checkpoint.

## Reporting to the orchestrator

Report with the native `compozy__session_prompt` tool targeting <ORCHESTRATOR_SESSION_ID>; never the
`compozy session prompt` CLI. Each message costs the orchestrator a full turn, so send milestones
only: plan ready, blocked, PR opened, CI diagnosis, and the final result. Do not stream progress.

- Set `supersede: true` on status updates so only your latest queued update is delivered.
- Set `in_reply_to` to the orchestrator's `message_id` when answering one of its messages.
- Set `notify_on_complete: true` only when you need the orchestrator's answer to continue.
- Do not co-edit shared files such as `docs/_memory/change-impact.md` with other executors. Put your
  cross-surface impact notes in the PR body; the orchestrator consolidates them.

## Implementation and validation

Reproduce the behavior and establish the root cause. Fix production behavior without suppressing
errors, weakening tests, or adding unrelated abstractions. Distinguish the handoff's hypotheses
from confirmed evidence. Preserve compatibility, user data, profile/workspace isolation, and daemon
authority. Follow the repository's cross-surface audit, docs, and affected QA scenario requirements.

Before editing tests, identify the invariant, owning layer, and canonical suite. Extend that suite;
avoid duplicate regressions or assertions that only freeze implementation details. Review the changed
diff and run the relevant real integration/E2E or app flow. UI fixes need rendered evidence for the
reported state and affected viewport/interaction cases. Use isolated homes, ports and sockets for
runtime QA, and clean up only your own registered lab processes. Do not restart the user's daemon.

Use repository-root frontend/Turborepo conventions when applicable. Pass `make gate` before
commit/push, including subsequent remediation commits, subject to the repository's documented
exceptions. Diagnose missing merge bases or unclassified paths instead of substituting a full run.
Never kill another task's queued gate run. Reuse valid unchanged evidence; rerun checks invalidated
by fixes. Follow repository commit conventions and hook recovery rules.

## Pull request

Commit and push the scoped branch; open a PR targeting the assigned branch with `Closes #<NUMBER>`.
Use an English title describing the resulting fix. Write a detailed English description with:

- The reported trigger, root cause, and observable before/after behavior.
- The final implementation and important tradeoffs, organized for a reviewer without this chat.
- Tests, real-run/UI evidence, applicable gates, and truthful platform/external limitations.
- Cross-surface, compatibility, documentation and QA impact, plus any recovery/migration behavior.

Use a structured body or body file. Omit private identifiers, host paths, credentials, and unrelated
logs from public content. Keep title/description current as remediation changes the implementation.
A draft PR or initial push is an intermediate checkpoint, not delivery.

## CI and all review feedback

Wait for **both CodeRabbit and Greptile** to finish reviewing the relevant PR changes. Confirm
coverage from their actual review/check evidence; elapsed time, a summary score, or absence of
comments is not proof that a review completed. Do not install/configure reviewers or fabricate a
pass if either is unavailable. Use the repository's supported re-review request mechanism when
needed, and report unavailable integration/access as an external blocker.

Collect every page of GitHub PR reviews, inline review comments/threads, and general PR comments.
Expand embedded/collapsed review sections and code suggestions. Track each finding by stable
comment/thread identity so findings survive pushes and become traceable to a disposition.

Address **all actionable findings**, not only majors: critical, major, minor, nitpick, suggestions,
maintainability, tests, documentation, and security. Include additional human feedback. Group
duplicates without losing their links. Do not discard a finding because it is old, marked outdated,
nonblocking, or located outside changed lines without checking its relevance to this PR.

For each finding:

1. Validate it against the current code and issue contract.
2. Fix it when valid and within the delivery scope; validate the changed behavior and push the fix.
3. Reply with the fix/commit and relevant evidence where a reply is needed. Resolve an addressable
   thread only after its disposition is supported. A resolved checkbox is not proof of a fix.
4. For false positives, duplicates, or superseded findings, give a concrete evidence-backed rationale
   and reference the owning fix when applicable. Never silently dismiss or implement an incorrect
   suggestion to achieve a clean review. A disputed or materially out-of-scope finding remains
   explicitly unresolved until settled; escalate a decision only when it blocks correct delivery.

After each relevant push, inspect new CI/review evidence and repeat remediation until no actionable
finding remains. Diagnose completed CI failures while other jobs run. All required checks must pass
for the **current PR head**; prior-head success does not qualify. Do not treat queued, pending,
canceled, missing, or failed required checks as green. Reconcile checks against repository branch
requirements, and distinguish legitimate nonrequired/skipped jobs from a missing required result.

Before delivery, capture the head SHA, confirm both reviewers covered the final changes, refresh all
review sources, and verify every finding's disposition. Reuse earlier review coverage only for
unchanged code; do not repeatedly request review of the same unchanged head. If a reviewer cannot
run or a check is externally blocked, report the exact limitation and leave delivery incomplete.

## Completion

Deliver only when the issue's acceptance criteria and real validation pass, the detailed PR is open
and ready for review, every required current-head CI check is green, and both CodeRabbit and
Greptile have completed relevant review with no unaddressed actionable findings. Confirm the final
head did not change during verification. Do not merge or remove the worktree.

Report in Brazilian Portuguese: PR URL, branch/head, concise fix, verification evidence, reviewer
completion/dispositions, and genuine limits. If blocked, state what is done, the precise blocker,
and next concrete action; do not claim success prematurely.
