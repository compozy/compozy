---
id: LP-loop-config-file-snake-case
area: LP
title: Apply documented Loop overrides from JSON and YAML files
persona: Bruno
journey: J-configure-and-run-loop
expected: The CLI accepts documented snake_case Loop override fields from JSON and YAML files for both loop run --config-file and loop configure --file, while rejecting unknown fields before daemon mutation.
entry_points: compozy loop run --config-file; compozy loop configure --file
qa_status: pass
bug_ids: BUG-20260801-loop-config-file-snake-case
fix_status: fixed
retest_status: pass
fix_commits: 38b2d40
evidence: /Users/pedronauck/dev/qa-labs/compozy-loops-coderabbit-remediation-20260802-013626-435207-lab/qa-artifacts/qa/observed-results.md; /Users/pedronauck/dev/qa-labs/compozy-loops-coderabbit-remediation-20260802-013626-435207-lab/qa-artifacts/qa/loop-config-smoke-run.png
last_report: docs/qa/reports/2026-08-01-loops-coderabbit-remediation.md
overlaps:
---

Exercise iteration_cap, no_progress_window, and gate_max_revisions through both supported file
formats and both CLI verbs. Confirm the stored or per-run override matches the file and that strict
unknown-field validation remains intact.

2026-08-01 CodeRabbit remediation: reset to `untested` because nested YAML
`enabled_checks_json` decoding changed after the prior walk.

2026-08-02 retest: JSON preview and YAML persistence preserved nested command and group values;
an unknown YAML field failed before mutation, and a fresh dry-run retained the valid configuration.
A live Terra/high session independently read and summarized the reused YAML artifact.
