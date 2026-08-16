---
id: ET-agent-plugin-marketplace-install
area: ET
title: Install an Agent Plugins catalog entry from Marketplace
persona: Ada
journey: J-marketplace-acquisition
expected: A catalog entry marked `format: agent-plugin` shows a neutral Agent Plugin badge on the card and detail view, follows the normal trust and install flow, lands on extension management with format and skipped diagnostics visible, and still relies on acquired-package detection when catalog metadata is absent or stale.
entry_points: Web /marketplace/extensions?tab=market and /marketplace/extension/:entryId; Web extension trust/install dialog, /marketplace/extensions, and /settings/extensions; compozy marketplace search --kind extension; GET /api/extensions/marketplace over HTTP and UDS; curated catalog feed
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-catalog-navigation; ET-web-marketplace-detail-redesign; ET-web-marketplace-installed-management
---

QA impact 2026-08-16: Marketplace can display the portable format and the extension detail surface can
show ingest skips. Task 08 must drive the browser flow and compare it with structured catalog and
installed payloads; the badge must never override runtime detection.
