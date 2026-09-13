---
id: ET-agent-plugin-marketplace-install
area: ET
title: Install an Agent Plugins catalog entry from Marketplace
persona: Bruno
journey: J-marketplace-acquisition
expected: A catalog entry marked `format: agent-plugin` shows a neutral Agent Plugin badge on the card and detail view, follows the normal trust and install flow, lands on extension management with format and skipped diagnostics visible, and still relies on acquired-package detection when catalog metadata is absent or stale.
entry_points: Web /marketplace and entry detail; Web extension trust/install dialog and /settings/extensions; compozy marketplace search; GET /api/marketplace over HTTP and UDS; POST /api/extensions; curated catalog feed
qa_status: untested
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

QA impact 2026-09-13 (task_06; final live/visual owner task_10 VC-05): install client
layouts from local paths and sources through the current extension surfaces. Verify root
manifest precedence, recorded layout, unchanged source/package bytes, and only authored
resources. Open Design contributes one MCP and zero packaged skills; loop-engineering
contributes seven skills and no MCP. An explicit package with commands/agents/hooks must
report `client_component_ignored` with zero loaded hooks. Verify update, removal, dev reload,
trust and scoped resource delivery. Focused Go lifecycle evidence does not close this live row;
previous browser evidence above predates the client adapter and current Marketplace routes.

Task03 input step (final tasks09/10): for a plugin that declares inputs, confirm unverified trust
explicitly, review the acquired manifest and complete the same typed fields used by curated
extensions. The request retains the approved digest and allow_unverified; input edits do not
reacquire a preview. A source change invalidates confirmation. An update requiring new values
uses the candidate input_definitions response, keeping the selected instance and prior inputs.
After restarting the daemon, confirm that configured MCPs retain their URL and boolean inputs.
A second profile without those inputs must still report missing configuration and publish no MCPs
from that package; it must not prevent installation or publication in the configured profile.
Do not invent input fields for a package that declares none.
