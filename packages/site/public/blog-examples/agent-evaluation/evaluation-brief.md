# Repository coding-agent evaluation

Fill this out before either run. Use the same completed brief for every candidate.

## Frozen conditions

- Task ID and attempt:
- Repository and full baseline commit:
- Candidate, version, model, and effective settings:
- Candidate working directory:
- Runtime home, database, ports, and external resources:
- Allowed tools and approval policy:
- Time limit and follow-up budget:
- Dependency installation/cache conditions:
- Checks already failing at baseline:
- Evaluation date:

## Task given to the agent

Implement [one observable behavior] in [owning module].

The current behavior is [reproduction and actual result].
The required behavior is [specific result, including an edge case].
Compatibility that must be preserved: [existing input/output or user state].

Read repository instructions and nearby implementation before editing.
Allowed files or areas: [explicit scope].
Forbidden changes: [unrelated cleanup, dependency changes, generated files, etc.].
Do not weaken or remove tests. Do not publish, deploy, or contact external services.

Run [canonical checks with exact commands]. Report which checks ran, their exit
status, and any remaining failures. Cite relevant files in your explanation.
If the task cannot be completed within the budget, report the blocker and retain
the partial changes for review. Do not claim completion from a plausible patch.

## Acceptance criteria, set by the evaluator

1. [Observable behavior and evidence required.]
2. [Edge case and evidence required.]
3. [Compatibility and scope requirement.]

## Reviewer record

- Agent wall-clock minutes:
- Active human minutes during the attempt:
- Human review minutes after the attempt:
- Human repair minutes after review:
- Follow-up prompts and interventions, verbatim:
- Original patch and terminal evidence locations:
- Correctness: pass / partial / fail / blocked
- Baseline check results and final check results:
- Acceptance check outcome at the original candidate output:
- Out-of-scope changes and defects:
- Cost amount, currency, and source; or unknown:
- Decision for this task class and reason:

Blank or unknown values are not zero. Preserve failed, interrupted, and blocked
attempts. Repaired output is separate from the original candidate result.
