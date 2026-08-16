---
id: ET-agent-plugin-validation
area: ET
title: Validate Agent Plugins packages without installing them
persona: Bruno
journey: J-extension-agent-authoring
expected: Validation detects portable, native, dual-manifest, client-specific, warning-only, and fatal packages without executing code or writing install state; portable output reports `format`, `would_ingest`, ordered issues, and zero exit for skips while fatal violations return the matching deterministic code and nonzero exit.
entry_points: compozy extension validate <path> -o human|json|jsonl|toon; compozy__extensions_validate; https://compozy.com/docs/extensions/agent-plugins
qa_status: pass
bug_ids: BUG-20260816-agent-plugin-path-projection; BUG-20260816-agent-plugin-validation-exit
fix_status: fixed
retest_status: pass
fix_commits: 35100d40b55c
evidence: docs/qa/evidence/2026-08-16-agent-plugins/conformance-checklist.json; docs/qa/reports/2026-08-16-agent-plugins.md#session-debriefs
last_report: docs/qa/reports/2026-08-16-agent-plugins.md
overlaps: ET-extension-code-first-authoring; ET-extension-agent-guided-authoring; ET-agent-plugin-native-precedence
---

QA impact 2026-08-16: validation now owns a portable conformance branch. Task 08 must reproduce the
shipped human and structured output shapes from a stamped binary and prove validation leaves the
registry, data root, subprocess table, and live resources untouched.

The standard's complete eight-item evidence gate is owned by `ET-agent-plugin-conformance-walk`;
this row owns the public validation command and its deterministic output/side-effect contract.

QA 2026-08-16: portable, native, dual-manifest, client-specific, warning-only, and fatal fixtures
produced matching human/structured semantics without registry or runtime mutation. The fix loop made
fatal results exit 1 while valid packages with component skips continue to exit 0.
