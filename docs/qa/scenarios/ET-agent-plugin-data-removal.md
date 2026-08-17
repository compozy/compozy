---
id: ET-agent-plugin-data-removal
area: ET
title: Preserve Agent Plugins data on update and remove it safely
persona: Bruno
journey: J-extension-distribution
expected: `PLUGIN_DATA` is absent until the first stdio launch, survives an update byte-for-byte, is deleted on remove, is renamed out of the reusable instance key when direct deletion fails, and leaves the extension installed when both deletion and quarantine fail.
entry_points: compozy extension enable <name>; compozy extension update <name>; compozy extension remove <name> --global; POST /api/extensions/:name/enable, POST /api/extensions/update, and DELETE /api/extensions/:name over HTTP and UDS; compozy__extensions_enable; compozy__extensions_update; compozy__extensions_remove; extension stdio launch
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/reports/2026-08-16-agent-plugins.md#session-debriefs
last_report: docs/qa/reports/2026-08-16-agent-plugins.md
overlaps: ET-020; ET-023; ET-agent-plugin-source-install
---

QA impact 2026-08-16: portable instances gained a dedicated data root and remove-time quarantine
contract. Task 08 must exercise clean removal, injected direct-delete failure, and injected
delete-plus-rename failure while proving a completed remove leaves the deterministic name reusable.

QA 2026-08-16: first launch created the deterministic data root, update retained its sentinel bytes,
and the shipped clean-delete/quarantine/fail-closed suite passed. Removing and reinstalling the same
name did not reuse deleted data.
