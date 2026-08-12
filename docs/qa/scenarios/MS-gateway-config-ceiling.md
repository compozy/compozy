---
id: MS-gateway-config-ceiling
area: MS
title: Configure the gateway ceiling without enabling a surface
persona: Dora
journey: J-expose-and-pair-gateway
expected: Supported gateway keys persist with truthful live or restart-required lifecycle, invalid values preserve the last good configuration, and gateway.public_ui.enabled is rejected because only the database may enable a surface.
entry_points: compozy config set gateway.*; compozy config get gateway; global config.toml
qa_status: pass
bug_ids: BUG-20260807-gateway-live-config-copy;BUG-20260812-global-workspace-gateway-config
fix_status: fixed
retest_status: pass
fix_commits:
evidence: docs/qa/reports/2026-08-12-pr-webhook-release-notes.md
last_report: docs/qa/reports/2026-08-12-pr-webhook-release-notes.md
overlaps: RT-gateway-local-only-boot
---

This scenario owns the operator-global configuration ceiling and tunables. Provider and surface
activation remain deliberately outside configuration and arrive through explicit gateway commands
in later slices.

QA walk 2026-08-07: `gateway.enabled` applied live without enabling a surface, the Settings copy
matched runtime truth after its fix, and the daemon remained local-only with no advertised address.

QA impact 2026-08-12: reset after fixing operator-home workspace loading so the global Gateway
configuration is not reinterpreted as a workspace overlay.

QA walk 2026-08-12: the primary daemon started with the operator-home workspace still registered,
and the repeated global Gateway write applied live without a workspace-overlay error.
