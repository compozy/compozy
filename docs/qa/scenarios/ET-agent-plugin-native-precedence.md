---
id: ET-agent-plugin-native-precedence
area: ET
title: Keep native manifest precedence in a dual-manifest package
persona: Bruno
journey: J-extension-distribution
expected: A package containing both a native extension manifest and `plugin.json` installs as `format: compozy`, uses only the native identity and resources, and reports one deterministic informational note without portable resources leaking into the instance.
entry_points: compozy extension validate <dual-manifest-path>; compozy extension install <dual-manifest-path> --allow-unverified --yes; POST /api/extensions over HTTP and UDS; compozy__extensions_install
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-agent-plugin-source-install; ET-extension-manifest-v2-surfaces
---

QA impact 2026-08-16: root detection now resolves the dual-manifest case explicitly. Task 08 owns the
native-precedence proof and must compare human output with structured payloads rather than treating the
note as a second installed format.
