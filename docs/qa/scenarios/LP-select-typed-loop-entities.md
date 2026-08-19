---
id: LP-select-typed-loop-entities
area: LP
title: Select exact typed entities and runtime before starting a Loop
persona: Lea
journey: J-01
expected: Shared Web controls, CLI prompts, and structured start surfaces preserve exact entity and runtime values, reject stale references with input_validation, and create no run on failure
entry_points: web /loops/:name run form; CLI compozy loop run --input; HTTP/UDS Loop run route; compozy__loop_run
qa_status: pass
bug_ids: BUG-20260729-tool-invoke-structural-redaction; BUG-20260818-runtime-input-split-controls
fix_status: fixed
retest_status: pass
fix_commits: f3b8837
evidence: /Users/pedronauck/dev/qa-labs/compozy-typed-loop-inputs-20260819-015537-040869-lab/qa-artifacts/qa/typed-loop-dry-run.png; /Users/pedronauck/dev/qa-labs/compozy-typed-loop-inputs-20260819-015537-040869-lab/qa-artifacts/qa/typed-runtime-selector-story.png
last_report: docs/qa/reports/2026-08-18-typed-loop-inputs.md
overlaps: LP-002; LP-loop-input-defaults; LP-runtime-validation-preflight
---

Exercise `agent`, every closed `ref.kind`, authored `enum`, partial `runtime`, and the plain-text
`file` control. Confirm redacted Vault metadata, exact custom model IDs, manual stale-value fallback,
TTY-only prompting, and `--no-prompt` behavior.
