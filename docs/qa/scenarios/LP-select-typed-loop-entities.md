---
id: LP-select-typed-loop-entities
area: LP
title: Select exact typed entities and runtime before starting a Loop
persona: Lea
journey: J-01
expected: Shared Web controls, CLI prompts, and structured start surfaces preserve exact entity and runtime values, reject stale references with input_validation, and create no run on failure
entry_points: web /loops/:name run form; CLI compozy loop run --input; HTTP/UDS Loop run route; compozy__loop_run
qa_status: untested
bug_ids: BUG-20260729-tool-invoke-structural-redaction; BUG-20260818-runtime-input-split-controls
fix_status: fixed
retest_status: pass
fix_commits: f3b8837; 46dd8ae
evidence: /Users/pedronauck/dev/qa-labs/compozy-typed-loop-inputs-remediation-20260819-062429-798135-lab/qa-artifacts/qa/runtime-selector.png; /Users/pedronauck/dev/qa-labs/compozy-typed-loop-inputs-remediation-20260819-062429-798135-lab/qa-artifacts/qa/journey-log.jsonl
last_report: docs/qa/reports/2026-08-19-typed-loop-inputs-remediation.md
overlaps: LP-002; LP-loop-input-defaults; LP-runtime-validation-preflight
---

Exercise `agent`, every closed `ref.kind`, authored `enum`, partial `runtime`, and the plain-text
`file` control. Confirm redacted Vault metadata, exact custom model IDs, manual stale-value fallback,
TTY-only prompting, and `--no-prompt` behavior.

QA impact 2026-08-19: a runtime selected through Web, CLI, HTTP/UDS, or the native tool now drives
nodes that directly reference that input. Reset to verify the selected value reaches the bound
runtime and its status provenance is `input`.

QA impact 2026-08-19: the shared runtime selector now preserves `normal|fast` speed, and compact CLI
input accepts the documented `:speed=` suffix including speed-only intent.
