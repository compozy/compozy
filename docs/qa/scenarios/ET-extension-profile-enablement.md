---
id: ET-extension-profile-enablement
area: ET
title: Enable an extension independently per profile
persona: Bruno
journey: J-extension-kit-lifecycle
expected: A new install is enabled by default in every active profile; disabling it in marketing hides only marketing resources, leaves finance live, and persists as one exception row; CLI, HTTP, UDS, native tools, web detail, inventory, and command-palette projection show the same effective state.
entry_points: /marketplace/extension/{entry_id}; compozy --profile <name> extension enable|disable; GET|PUT /api/extensions/{name}/enablement; compozy__extensions_enable|disable
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-ext-kit-enable; ET-web-extensions-manage; ET-extension-palette-contributions
---

Flagged by profiles Task 09. The final profiles QA cycle owns the first walk.
