---
id: ET-agent-plugin-marketplace-install
area: ET
title: Install an Agent Plugins catalog entry from Marketplace
persona: Bruno
journey: J-marketplace-acquisition
expected: A catalog entry marked `format: agent-plugin` shows a neutral Agent Plugin badge on the card and detail view, follows the normal trust and install flow, lands on extension management with format and skipped diagnostics visible, and still relies on acquired-package detection when catalog metadata is absent or stale.
entry_points: Web /marketplace/extensions?tab=market and /marketplace/extension/:entryId; Web extension trust/install dialog, /marketplace/extensions, and /settings/extensions; compozy marketplace search --kind extension; GET /api/extensions/marketplace over HTTP and UDS; curated catalog feed
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status: pending
fix_commits:
evidence: docs/qa/reports/2026-08-16-agent-plugins.md#marketplace-and-browser; /Users/pedronauck/.config/browser-harness/agent-workspace/recordings/agent-plugin-marketplace; /Users/pedronauck/dev/qa-labs/compozy-agent-plugins-20260816-20260816-061032-351590-lab/qa-artifacts/browser-screenshots/installed-agent-plugin.png; /Users/pedronauck/dev/qa-labs/compozy-agent-plugins-20260816-20260816-061032-351590-lab/qa-artifacts/browser-screenshots/agent-plugin-inventory.png
last_report: docs/qa/reports/2026-08-16-agent-plugins.md
overlaps: ET-web-catalog-navigation; ET-web-marketplace-detail-redesign; ET-web-marketplace-installed-management
---

QA impact 2026-08-16: Marketplace can display the portable format and the extension detail surface can
show ingest skips. Task 08 must drive the browser flow and compare it with structured catalog and
installed payloads; the badge must never override runtime detection.

QA 2026-08-16: card, detail, neutral badge, trust dialog, installed management, and skipped inventory
were walked in the real browser. The fixture catalog's synthetic GitHub release URL returned 404 at
the final install mutation, so acquisition itself remains `blocked-verify`; the same bytes were then
installed through the public CLI to verify the installed Web state without weakening HTTPS/SSRF rules.
