---
id: ET-agent-plugin-data-removal
area: ET
title: Preserve Agent Plugins data on update and remove it safely
persona: Bruno
journey: J-extension-distribution
expected: `PLUGIN_DATA` is absent until the first stdio launch, survives an update byte-for-byte, is deleted on remove, is renamed out of the reusable instance key when direct deletion fails, and leaves the extension installed when both deletion and quarantine fail.
entry_points: compozy extension install|enable|update|remove --global; POST and DELETE /api/extensions lifecycle routes over HTTP and UDS; compozy__extensions_install|enable|update|remove; extension stdio launch
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-020; ET-023; ET-agent-plugin-source-install
---

QA impact 2026-08-16: portable instances gained a dedicated data root and remove-time quarantine
contract. Task 08 must exercise clean removal, injected direct-delete failure, and injected
delete-plus-rename failure while proving a completed remove leaves the deterministic name reusable.

