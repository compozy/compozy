---
id: ET-web-marketplace-remove-scope-return
area: ET
title: Return removed items to Marketplace scope
persona: Bruno
journey: J-marketplace-acquisition
expected: Removing an installed skill, extension, or MCP server succeeds only after typed confirmation, reconciles discovery and inventory, switches to Marketplace scope, and leaves the removed entry available for a fresh install when it remains catalog-backed.
entry_points: Marketplace Installed card overflow; Marketplace installed detail management
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-marketplace-installed-management; ET-web-extensions-manage; ET-web-marketplace-skill-install
---

Added by the unified Marketplace hard cut. Verify cancel, failed mutation, successful mutation,
fresh reload, and removal of a non-catalog installed item without inventing a Marketplace card.

QA impact 2026-07-18: successful MCP removal feedback now reports whether the config applied now
or requires a daemon restart, using the lifecycle returned by the exact-owner delete operation.
