# Peer Review

Independent reviewers pressure-test a saved, approved spec. Review is optional and runs only when the operator chooses it.

## Ask once

Ask these together, once, through the runtime's question tool. Without one, ask in a single message and stop.

```text
Q1 Review this spec?          a) Yes  b) No
Q2 Reviewers?                 a) One  b) Two, different providers  c) Choose runtime per reviewer
Q3 Maximum rounds?            a) 1 (Recommended)  b) 2  c) 3
```

If the operator declines, end authoring and write no review artifacts. For each reviewer, settle provider, model and reasoning effort, using project instructions when they name them and this session's runtime otherwise. Never substitute a different model than the operator chose.

The answers authorize every round up to the maximum. Do not ask again between rounds.

## Run a round

Files live in `<spec-dir>/qa/`, addressed by absolute path, and each round has its own `<N>`. Never overwrite an earlier round.

1. Create one child session per reviewer with agent `reviewer`.
2. Send the prompt below as the first prompt, with runtime set explicitly, and confirm the effective runtime.
3. Register a session monitor where one is available. Otherwise use a timer monitor and re-arm it until the child settles.
4. Read the findings files when the monitor wakes you. The file is the result; session output is not.

If native sessions or a `reviewer` agent are unavailable, report that limitation and stop. The authoring self-check needs no reviewer.

A reviewer that stops early keeps what it wrote. Summarize that partial file as incomplete, and do not treat it as a clean round.

### Round 1 prompt

```text
You are reviewing a spec written by another agent. Read every file below before reasoning.

Spec: <spec path>
Surface contracts: <_dx.md and _uiux.md paths, or none>
Stories and tests: <_user_stories.md and _tests.md paths>
ADRs: <paths, or none>
Research: <paths, or none>
Project rules: <instruction files and compatibility policy, or none>

Write exactly one file: <findings path>. Write it incrementally, and add each finding as you confirm it, so an interrupted session keeps its work. Edit nothing else. If you cannot write that file, stop and report it; never print findings as a fallback.

Blockers prevent approval: unsound concurrency, missing migration paths, under-specified safety invariants, contradictions between the technical design and the surface contracts, a motivating problem no slice solves end to end, or a violation of the project's rules. Nits are non-blocking improvements. Every finding names a real section or path. Do not invent evidence.

Finish with one line: Wrote <findings path>
```

### Findings file

`qa/peer-review-findings-round<N>-<reviewer>.md`, where `<reviewer>` is a short label such as the provider name:

```markdown
---
round: N
readiness: READY|BLOCKED|NEEDS_REWORK
reviewer_runtime: <provider>
reviewer_model: <effective model>
generated_at: <ISO-8601 timestamp>
---

# Summary

# Blockers

## B-001: <title>

- Section: <section or path>
- Issue: <one paragraph>
- Rationale: <why this blocks approval>
- Suggested fix: <concrete change>

# Nits

## N-001: <title>

- Section: <section or path>
- Issue: <one line>
- Suggested fix: <one line>

# Evidence

# Deferred Or Follow-Up
```

Empty sections say `None.`

## After a round

1. Check each finding against the spec. Drop false positives with a stated reason.
2. Fix verified blockers and worthwhile nits in the spec and its companions.
3. Escalate a blocker that needs an operator decision: product scope, a compatibility commitment, or a choice between real alternatives. Never resolve it yourself. Keep it open until the operator answers.
4. Write `qa/peer-review-summary-round<N>.md` with each reviewer's verdict, what was incorporated with the files changed, what was deferred, and what remains open.

## Stop or continue

- A round with no blockers from any reviewer ends the review. No later round runs.
- Otherwise, run the next round if the maximum allows it. Each later round covers changed blockers only: send the same session the changed files, the summary path, the blockers to verify, and the new findings path. Do not resend the full spec. Ask it to confirm each fix and check the changed text for regressions, continuing finding IDs. If that session is gone, create a new one with the same narrow scope.
- At the maximum with blockers open, stop. List them as unresolved in the final summary and in your report to the operator.

Stop reviewer sessions when the review ends. Review never commits, starts implementation, or approves the spec.
