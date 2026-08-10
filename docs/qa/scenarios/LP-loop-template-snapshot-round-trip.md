---
id: LP-loop-template-snapshot-round-trip
area: LP
title: Start a templated parent Loop after dry-run
persona: Ada
journey: J-01
expected: A templated parent Loop validates and dry-runs with resolved inputs, then its persisted Run exposes the raw authored executed definition beside a one-pass materialized contract while every Goal agent receives resolved text and literal braces inside input values remain literal.
entry_points: compozy loop validate; compozy loop run --dry-run; compozy loop run; compozy loop status over UDS; POST /api/workspaces/:workspace_id/loops/:name/run over HTTP
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-08-10-loop-convergence/raw-loop-definition.png; /Users/pedronauck/dev/qa-labs/compozy-loop-convergence-20260810-034845-371840-lab/qa-artifacts/qa/evidence/run-proof.json; /Users/pedronauck/dev/qa-labs/compozy-loop-convergence-20260810-034845-371840-lab/qa-artifacts/qa/qa-audit-report.md
last_report: docs/qa/reports/2026-08-10-loop-convergence.md
overlaps: LP-006; TA-068
---

QA impact 2026-08-05: added for issue #313. The walk must prove the dry-run leaves the Runs list
unchanged, the real submission returns a Run id, and a fresh public read finds that same persisted
Run. It must keep the default `await` child modes and template references intact; a successful
preview followed by a manifest-integrity failure is a regression.

Taxonomy: the CLI/HTTP/UDS walk owns journey, functional round-trip, error recovery, and structured
surface consistency. Web responsiveness, mobile, and visual design are deliberate skips because this
change adds no UI or rendered copy.

QA result 2026-08-05: Ada validated the workspace parent and confirmed that `review-and-fix` came
from the marketplace while `workspace-child` came from the workspace. Two identical dry-runs left
the Runs list empty. The real UDS submission created `looprun-4b66e31d8f935186`; fresh CLI/UDS and
HTTP reads agreed on digest `sha256:73a51e3b67426d74ce8a9749973e038819463c549e62f77a83353f42aa1eaf85`
and exposed both child nodes with `mode: await` plus the authored input/output templates.

QA impact 2026-08-10: reset because run and dry-run details now separate the raw
`executed_definition` from `materialized_contract`, and Goal parameters materialize recursively at
one execution boundary. The walk must prove resolved input text reaches the Goal without a second
template pass.

QA result 2026-08-10: Ada dry-ran `qa-goal-convergence` and confirmed that no Run was created,
`slug` resolved to `weather-app`, and the literal input `{{ .inputs.shadow }}` remained literal. The
real Run preserved `{{ .inputs.slug }}` in `executed_definition`, exposed the resolved Goal in
`materialized_contract`, and reached `done` with the same one-pass result.
