---
id: MS-workspace-resolution-provenance
area: MS
title: Explain the workspace resolution source in structured output
persona: Ada
journey: J-operate-workspace-context
expected: JSON, JSONL, and TOON output identify the winning workspace resolution tier without changing human output or the command payload shape.
entry_points: workspace-scoped CLI commands with -o json; -o jsonl; -o toon
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-ms-wave2-current-20260730-061842-796290-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: MS-workspace-resolution-chain
---

Exercise positional, `--workspace`, `COMPOZY_WORKSPACE`, validated session identity, and cwd
resolution across representative workspace-scoped command families. Capture each structured output
and confirm `resolution_source` is exactly `positional`, `flag`, `env`, `session_identity`, or `cwd`.
Confirm JSON arrays remain arrays, JSONL keeps item/page records parseable, TOON exposes the field,
and human output does not gain machine metadata.

QA impact 2026-07-28: resolution provenance is new public CLI output. Planning flag only; no QA replay
ran in this implementation slice.
