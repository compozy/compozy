---
name: cy-final-verify
description: Enforces fresh verification evidence before any completion, fix, or passing claim, and before commits or PR creation. Use when an agent is about to report success, hand off work, or commit code. Do not use for early planning, brainstorming, or tasks that have not yet reached a concrete verification step.
---

# Verification Before Completion

## Overview

Claiming work is complete without verification is dishonesty, not efficiency.

**Core principle:** Evidence before claims, always.

**Violating the letter of this rule is violating the spirit of this rule.**

## The Iron Law

```
NO COMPLETION CLAIMS WITHOUT FRESH VERIFICATION EVIDENCE
```

If the verification command has not been run in the current message, the result cannot be claimed.

## The Gate Function

```
BEFORE claiming any status or expressing satisfaction:

1. IDENTIFY: What command proves this claim?
2. RUN: Execute the FULL command (fresh, complete)
3. READ: Full output, check exit code, count failures
4. VERIFY: Does output confirm the claim?
   - If NO: State actual status with evidence
   - If YES: State claim WITH evidence
5. ONLY THEN: Make the claim

Skip any step = lying, not verifying
```

## Scope of Verification

Match the verification scope to the claim scope:

- **Narrow claim** (e.g., "this test passes"): Run the specific test.
- **Broad claim** (e.g., "task complete", "ready to commit"): Run the **full verification pipeline** — formatting, linting, all tests, and build. If the project defines a single gate command (e.g., `make verify`), run that.

A narrow verification does not support a broad claim. Running `make test` alone does not justify "task complete." Running the linter alone does not justify "ready to commit." The verification scope must be equal to or broader than the claim scope.

**If in doubt, run the full pipeline.** Over-verification wastes minutes. Under-verification wastes hours.

**Passing pipeline != meeting requirements.** A green build proves the code compiles, lints, and passes existing tests. It does not prove the implementation matches the requirements. For "task complete" or "requirements met" claims, also verify the deliverables against the original specification — line by line, not by assumption. In a spec/PRD workflow, "the original specification" means the canonical artifacts in the spec directory (example documents, input tables, parity maps, QA seeds) — never just the task file's paraphrase of them (see "Spec Contract Parity").

## Common Failures

| Claim                 | Requires                        | Not Sufficient                 |
| --------------------- | ------------------------------- | ------------------------------ |
| Tests pass            | Test command output: 0 failures | Previous run, "should pass"    |
| Linter clean          | Linter output: 0 errors         | Partial check, extrapolation   |
| Build succeeds        | Build command: exit 0           | Linter passing, logs look good |
| Bug fixed             | Test original symptom: passes   | Code changed, assumed fixed    |
| Regression test works | Red-green cycle verified        | Test passes once               |
| Agent completed       | VCS diff shows changes          | Agent reports "success"        |
| Requirements met      | Line-by-line checklist          | Tests passing                  |
| Matches spec contract | Field-by-field diff vs canonical spec artifacts | Task-file paraphrase satisfied, checkboxes ticked |

## Red Flags

- Using "should", "probably", or "seems to"
- Expressing satisfaction before verification
- About to commit, push, or open a PR without verification
- Trusting another agent's success report
- Relying on partial verification
- Thinking "just this once"
- Any wording that implies success without current evidence
- Verifying against a task file's paraphrase when a canonical contract artifact exists in the spec directory

## Rationalization Prevention

| Excuse                                  | Reality                |
| --------------------------------------- | ---------------------- |
| "Should work now"                       | Run the verification   |
| "I'm confident"                         | Confidence ≠ evidence  |
| "Just this once"                        | No exceptions          |
| "Linter passed"                         | Linter ≠ compiler      |
| "Agent said success"                    | Verify independently   |
| "I'm tired"                             | Exhaustion ≠ excuse    |
| "Partial check is enough"               | Partial proves nothing |
| "Different words so rule doesn't apply" | Spirit over letter     |

## When To Apply

Apply this skill before:

- any success or completion claim
- any expression of satisfaction with the implementation state
- any commit or PR creation
- any handoff that implies correctness
- moving to the next task based on completion

## Optional Companion Skills

Two companion gates below are **conditional**. Probe the agent skill roots (`.agents/skills/`, `.claude/skills/`, or the runtime's skill catalog) for a `SKILL.md` under the named skill. If the skill is absent, skip that gate entirely — do not invent a substitute, and do not fail verification for the missing companion.

| Gate | Requires | If missing |
| ---- | -------- | ---------- |
| Deslop Gate | `deslop` | Skip deslop; proceed to verify |
| QA Tracker Impact | both `qa-report` and `qa-execution` | Skip the QA impact flag |

## Deslop Gate (code-change tasks)

**Only when the `deslop` skill exists.** Before collecting completion evidence, run `deslop` on the branch diff and apply its cleanup. Order matters: deslop mutates code, so verification evidence gathered before it is stale — deslop first, then verify. Skipping it when the skill is present ships AI slop (noise comments, defensive clutter, style drift) into permanent artifacts.

## Pre-Commit and Pre-PR Gate

Commits and PRs are permanent artifacts. They require the highest verification standard.

**Before `git commit`:**
1. If the `deslop` skill exists, run the deslop pass (see "Deslop Gate" above).
2. Run the full verification pipeline (e.g., `make verify`). Not a subset. The full pipeline.
3. Confirm zero errors, zero warnings, zero test failures in the output.
4. If both `qa-report` and `qa-execution` skills exist and the project keeps a living QA tracker (e.g. `docs/qa/state.csv`), apply the QA impact flag (see "QA Tracker Impact" below).
5. Produce a Verification Report (see template below) with verdict PASS.
6. Only then run `git commit`.

**Before creating a PR:**
1. All of the above, plus:
2. Verify the diff matches the intended changes (`git diff` review).
3. Confirm no unrelated files are staged.

If the full pipeline has not passed in this session after the last code change, the commit or PR must not proceed.

## QA Tracker Impact (living QA docs)

**Only when both `qa-report` and `qa-execution` skills exist.** A green pipeline proves the code works; it does not keep QA verdicts honest. When the project also keeps a living QA tracker (e.g. `docs/qa/state.csv`), a completion claim also requires the impact flag — one question, ~1 minute:

> Does this diff change user-visible behavior (UI, CLI verb, API route, config key, user-facing copy)?

- **No** (pure refactor, internal-only): state "no user-visible change" in the completion notes. Done.
- **New behavior:** add the scenario row(s) to the tracker with status `untested`.
- **Changed behavior:** reset the affected rows' `qa_status` to `untested` (a stale `pass` is worse than no verdict).

**Flag, don't retest.** Running QA is the QA cycle's job — `untested` rows are exactly its scope. Skipping the flag silently (when the companion skills and tracker are present) is a stale-verdict claim: the same dishonesty as claiming tests pass without running them.

## Spec Contract Parity (PRD/spec workflows)

A green pipeline and ticked task checkboxes do not prove the deliverable matches the spec. When the work executes a task from a spec directory (PRD/TechSpec plus sibling artifacts), a "task complete" claim additionally requires:

1. List the canonical contract artifacts for this task found during grounding (e.g. `_examples.md`, `_qa.md` input tables, `_tests.md` test contracts, `_user_stories.md` acceptance criteria, parity maps). If none are known, survey the spec directory now — absence must be proven, not assumed.
2. Compare the deliverable to each artifact field by field: names, types, defaults, required flags, shapes, topologies, behaviors. Paraphrase-level similarity is not parity.
3. Any mismatch fails the completion claim — fix the deliverable to match the resolved contract and re-verify; never reinterpret the canonical artifact to match what was built, and never pause to ask which side wins.
4. Cite the compared artifacts in the Verification Report (`Contract parity:` line).

Failure mode this section exists to prevent (real incident): a task shipped "green" through seven peer-review rounds while contradicting the spec's canonical example document — every check measured engineering quality against the task file's paraphrase, and nothing ever compared the deliverable to the canonical contract.

## Verification Report Template

Verification is not complete until the agent **cites actual command output** in their response. "I ran it and it passed" is not evidence. If the verification output is not shown, the verification did not happen.

Every verification must be reported using this structure. Do not deviate.

```
VERIFICATION REPORT
-------------------
Claim: [What is being claimed — e.g., "tests pass", "build succeeds", "task complete"]
Command: [Exact command run — e.g., `make verify`]
Executed: [Timestamp or "just now, after all changes"]
Exit code: [0 or non-zero]
Output summary: [Key lines from output — pass count, error count, build result]
Warnings: [Any warnings, or "none"]
Errors: [Any errors, or "none"]
Contract parity: [spec-workflow tasks: artifacts compared + PASS/mismatch; otherwise "n/a"]
Verdict: PASS or FAIL
```

If the verdict is FAIL, do not use completion language. State what failed and what remains.

If the verdict is PASS, the claim may proceed — but only the specific claim supported by the evidence. "Tests pass" does not mean "build succeeds."

## When Verification Fails

Verification failure is not a dead end. It is information. Follow this protocol:

1. **Read the failure.** Identify the exact error: which command failed, which test, which lint rule, which build error. Quote the relevant output lines.
2. **Diagnose the root cause.** Do not guess. Read the error message. Trace it to the source. If multiple things failed, address them one at a time starting with the first failure.
3. **Fix the root cause.** Apply the minimal change that addresses the actual error. Do not apply workarounds, suppress warnings, or skip checks.
4. **Re-verify from scratch.** Run the full verification command again. Do not assume the fix worked. Do not run only the previously-failing subset.
5. **Report with evidence.** Use the Verification Report Template. If it passes now, the claim may proceed. If it fails again, return to step 1.

**Never:**
- Claim partial success ("3 of 4 checks pass, close enough")
- Skip re-verification after a fix ("I fixed the error, so it should pass now")
- Blame the tooling ("the linter is wrong") without evidence of a false positive
- Move on to the next task while verification is still failing

If the correct verification command is unclear, identify it before making any completion claim. If only partial verification is available, state that limitation explicitly and avoid completion language.
