# Forbidden prompt phrases

The auditor scans every prompt sent to an agent under test (journey-log entries with `surface=provider` or `surface=runtime`, the provider-attempt file, every `*.jsonl` under `qa-artifacts/qa/`, and any captured `agh session prompt` payload) for the phrases below. A match is a hard blocker (auditor exit code 2).

Exception: the operator kickoff message is exempt **only** when the journey-log entry is marked `kickoff: true` AND `surface: runtime` AND `actor` matches the playbook's `operator_persona.name` (or `operator_persona.role` if name is absent). Anything else carrying these phrases is rejected.

## Hard-blocking literals (case-insensitive substring match)

- `you are the qa`
- `you are the test`
- `you are a tester`
- `you are an auditor`
- `you are the auditor`
- `qa lead`
- `qa scenario`
- `qa pass`
- `qa fail`
- `qa-output-path`
- `release readiness`
- `release-readiness`
- `go/no-go`
- `go / no-go`
- `go-no-go`
- `pass/fail criteria`
- `pass/fail outcome`
- `inspect the workspace`
- `inspect the current workspace`
- `create a markdown artifact`
- `create a concise but concrete markdown`
- `final report template`
- `verification report`
- `audit coverage`
- `audit checklist`
- `behavioral charter`
- `scenario contract`
- `disruption probe`
- `provider probe`
- `provider-launch-review`

## Hard-blocking regex patterns

- `\bTC-(SCEN|INT|FUNC|UI|REG|SEC|PERF)-\d+` — explicit test-case ids.
- `\bSMOKE-\d+` — smoke test ids.
- `\bBUG-\d+` — agent must not be told to file or read bug reports as the work.
- `(?i)required\s+(initial\s+)?defect` — staged-defect phrasing.
- `(?i)attempt\s+to\s+(break|reject|reproduce)` — explicit probe phrasing.
- `(?i)expected\s+(pass|fail|blocked)\s+(criteria|verdict|state)` — pass/fail-driven prompts.

## Why these are blocked

Each phrase teaches the agent it is being **observed and graded**, not building. That changes the agent's behavior: it produces evidence shaped for an evaluator (markdown reports, checklists, defect-then-fix theater) instead of producing real product work. Real-scenario QA validates the AGH runtime by watching agents behave as if the work were real — which is only possible if the prompts they receive **are** real product work.

## How to add a new phrase

1. Append the literal under the right section (case-insensitive substring) or a regex under "Hard-blocking regex patterns".
2. Document **why** it is blocked in one short sentence (what staged behavior it would invite).
3. Add a fixture line to the negative-test corpus referenced from the auditor smoke test so the rule is exercised.
4. Never relax an existing rule without a written incident citing the false positive.

## Allowlist (specific, narrow)

The following are NOT forbidden even though they sound similar:
- "review" (used freely — peer review is a real product activity).
- "verdict" (compliance verdicts, design verdicts are real product output).
- "test" only when paired with code (`unit test`, `integration test`, `vitest`, `go test`) — agents writing tests is real product work.
- "decision" (decision memos are real product artifacts).

If a real product activity casually uses one of the forbidden literals (e.g., a lifecycle email subject literally says "Did our changes pass your test?"), wrap that artifact in code/markdown fences inside the agent's output rather than in the prompt — the auditor scans **prompts**, not deliverables.
