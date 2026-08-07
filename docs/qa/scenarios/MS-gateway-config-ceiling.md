---
id: MS-gateway-config-ceiling
area: MS
title: Configure the gateway ceiling without enabling a surface
persona: Dora
journey: J-expose-and-pair-gateway
expected: Supported gateway keys persist with truthful live or restart-required lifecycle, invalid values preserve the last good configuration, and gateway.public_ui.enabled is rejected because only the database may enable a surface.
entry_points: compozy config set gateway.*; compozy config get gateway; global config.toml
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-gateway-local-only-boot
---

This scenario owns the operator-global configuration ceiling and tunables. Provider and surface
activation remain deliberately outside configuration and arrive through explicit gateway commands
in later slices.
