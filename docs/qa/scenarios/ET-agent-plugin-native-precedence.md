---
id: ET-agent-plugin-native-precedence
area: ET
title: Keep native manifest precedence in a dual-manifest package
persona: Bruno
journey: J-extension-distribution
expected: A package containing both a native extension manifest and `plugin.json` installs as `format: compozy`, uses only the native identity and resources, and reports one deterministic informational note without portable resources leaking into the instance.
entry_points: compozy extension validate <dual-manifest-path>; compozy extension install <dual-manifest-path> --allow-unverified --yes; POST /api/extensions over HTTP and UDS; compozy__extensions_install
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-16-agent-plugins/conformance-checklist.json; docs/qa/reports/2026-08-16-agent-plugins.md#session-debriefs
last_report: docs/qa/reports/2026-08-16-agent-plugins.md
overlaps: ET-agent-plugin-source-install; ET-extension-manifest-v2-surfaces
---

QA impact 2026-08-16: root detection now resolves the dual-manifest case explicitly. Task 08 owns the
native-precedence proof and must compare human output with structured payloads rather than treating the
note as a second installed format.

QA 2026-08-16: the dual-manifest fixture resolved to `format: compozy` across human and structured
validation/install reads, emitted one deterministic precedence note, and published no portable sibling.
