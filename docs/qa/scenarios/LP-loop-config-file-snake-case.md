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
fix_commits: pending Task 07 commit
evidence: /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/loop/config-file-json-run-status-v6.json; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/loop/config-file-yaml-configure-v6.json
last_report: docs/qa/reports/2026-08-01-loops-paper-adoption.md
overlaps:
---

Exercise iteration_cap, no_progress_window, and gate_max_revisions through both supported file
formats and both CLI verbs. Confirm the stored or per-run override matches the file and that strict
unknown-field validation remains intact.
