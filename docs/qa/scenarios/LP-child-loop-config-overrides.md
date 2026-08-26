---
id: LP-child-loop-config-overrides
area: LP
title: Apply ephemeral configuration to one child Loop
persona: Ada
journey: J-await-child-loop
expected: A workspace-authored parent Loop passes typed runtime rules and finite budgets through run-loop.params.config_overrides; the child reports those values as per-run config, the parent and stored child config remain unchanged, and omitting the field preserves existing child execution.
entry_points: compozy loop validate; compozy loop run; compozy loop status; HTTP and UDS Loop run detail
qa_status: pass
bug_ids:
fix_status: fixed
retest_status: pass
fix_commits:
evidence: isolated lab child-loop-config-overrides-20260826-220440-305142; await parent looprun-8c127e3412b62072 / child looprun-de6aba4f607ad492; default parent looprun-9d3971551b3da5a6 / child looprun-8d1bdc727ed0f52d; detach parent looprun-27cc6dbfcb141eb3 / child looprun-e229b1452ec86c8d
last_report: docs/qa/reports/2026-08-26-child-loop-config-overrides.md
overlaps: LP-effective-config-provenance; LP-run-loop-await-child-ordering
---

Use provider-free parent and child Loops so the walk tests Loop composition rather than an agent
driver. The parent materializes a numeric token budget and a runtime-rule array, then starts the
child in `await` mode with finite iteration and wall limits. Compare the child run's effective
config and source map with the parent and the child's stored config. Repeat with an otherwise
identical parent that omits `config_overrides`, then exercise `detach` and an invalid unknown key.

QA 2026-08-26: The fresh isolated daemon accepted typed output references for token budget and
runtime rules. Await and detach child runs reported the requested values with `per_run` provenance,
the parent and stored child configuration stayed unchanged, the omitted field preserved defaults,
and the misspelled `iteration_caps` key was rejected during validation. Teardown was clean.
