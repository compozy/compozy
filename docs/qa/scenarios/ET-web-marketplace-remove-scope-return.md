---
id: ET-web-marketplace-remove-scope-return
area: ET
title: Return removed items to Marketplace scope
persona: Bruno
journey: J-marketplace-acquisition
expected: Removing an installed extension requires confirmation by its local name, updates the Installed list and catalog marker, and leaves the catalog entry installable when its source still publishes it. A same-name entry from another origin is unaffected.
entry_points: /marketplace/installed row overflow; installed extension detail
qa_status: untested
bug_ids:
fix_status:
retest_status: untested
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: ET-web-marketplace-installed-management; ET-web-extensions-manage; ET-web-marketplace-skill-install
---

Marketplace catalog task 01 (2026-09-12): Cancel once, submit an incorrect name, then confirm the exact installed name. Check the inventory and catalog after success and after a failed removal. Return navigation must stay within the single catalog and Installed routes.

Execution is deferred to tasks 09/10 by the loop delivery contract. Earlier evidence and notes below describe the previous surface and do not verify this contract.


Added by the unified Marketplace hard cut. Verify cancel, failed mutation, successful mutation,
fresh reload, and removal of a non-catalog installed item without inventing a Marketplace card.

QA impact 2026-07-18: successful MCP removal feedback now reports whether the config applied now
or requires a daemon restart, using the lifecycle returned by the exact-owner delete operation.
