---
id: ET-agent-plugin-validation
area: ET
title: Validate Agent Plugins packages without installing them
persona: Bruno
journey: J-extension-agent-authoring
expected: Validation detects portable, native, dual-manifest, client-specific, warning-only, and fatal packages without executing code or writing install state; portable output reports `format`, `would_ingest`, ordered issues, and zero exit for skips while fatal violations return the matching deterministic code and nonzero exit.
entry_points: compozy extension validate <path> -o json|jsonl|toon; extension validation HTTP/UDS service; compozy__extensions_validate; /docs/extensions/agent-plugins
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-extension-code-first-authoring; ET-extension-agent-guided-authoring; ET-agent-plugin-native-precedence
---

QA impact 2026-08-16: validation now owns a portable conformance branch. Task 08 must reproduce the
shipped human and structured output shapes from a stamped binary and prove validation leaves the
registry, data root, subprocess table, and live resources untouched.

