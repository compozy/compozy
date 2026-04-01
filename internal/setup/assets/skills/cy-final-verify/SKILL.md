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

## Red Flags

- Using "should", "probably", or "seems to"
- Expressing satisfaction before verification
- About to commit, push, or open a PR without verification
- Trusting another agent's success report
- Relying on partial verification
- Thinking "just this once"
- Any wording that implies success without current evidence

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

## Error Handling

- If the correct verification command is unclear, identify it before making any completion claim.
- If verification fails, report the actual failure and keep the task open.
- If only partial verification is available, state that limitation explicitly and avoid completion language.
