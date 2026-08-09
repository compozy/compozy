---
id: MS-gateway-config-ceiling
area: MS
title: Configure the gateway ceiling without enabling a surface
persona: Dora
journey: J-expose-and-pair-gateway
expected: Supported gateway keys persist with truthful live or restart-required lifecycle, invalid values preserve the last good configuration, and gateway.public_ui.enabled is rejected because only the database may enable a surface.
entry_points: compozy config set gateway.*; compozy config get gateway; global config.toml
qa_status: pass
bug_ids: BUG-20260807-gateway-live-config-copy
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/07-live-enable-config-set.json;/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/screenshots/04-gateway-ceiling-live-on.png
last_report: docs/qa/reports/2026-08-07-remote-gateway.md
overlaps: RT-gateway-local-only-boot
---

This scenario owns the operator-global configuration ceiling and tunables. Provider and surface
activation remain deliberately outside configuration and arrive through explicit gateway commands
in later slices.

QA walk 2026-08-07: `gateway.enabled` applied live without enabling a surface, the Settings copy
matched runtime truth after its fix, and the daemon remained local-only with no advertised address.
