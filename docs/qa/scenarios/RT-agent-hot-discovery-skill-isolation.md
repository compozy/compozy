---
id: RT-agent-hot-discovery-skill-isolation
area: RT
title: A newly created agent launches even when one local skill is invalid
persona: Dora
journey: J-17
expected: A default-profile agent created while the daemon is running appears in the live agent catalog without reloading the app and can create a durable session in the selected workspace. If one agent-local skill is malformed or fails verification, only that skill is omitted, valid sibling skills remain available, a scoped diagnostic is recorded, and session creation still succeeds without partial or duplicate sessions.
entry_points: compozy agent create; web Agents; web Start session; POST /api/sessions; compozy session list; compozy skill list
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-09-14-marketplace-review-public.md
last_report: docs/qa/reports/2026-09-14-marketplace-review-public.md
overlaps: MS-web-session-simple-advanced-launch; ET-006
---

QA impact 2026-08-27: default-profile agent discovery now watches and resolves the same profile-aware
roots used by session creation. Agent-local skill loading isolates each invalid declaration instead
of rejecting the agent's full effective catalog or blocking session creation.

PR636 review retest: start with an installed ClawHub skill and a persisted MCP resource formerly
published from that skill. On restart and hot discovery, the skill body/provenance remain available,
but its embedded MCP must be absent from callable discovery. Local skill MCPs and manual MCPs
must remain available. A reconciliation failure must prevent startup from serving stale resources.
The owning daemon reconstruction test passed with real SQLite; the targeted public discovery walk is recorded below.

PR636 targeted public result 2026-09-14: retained ClawHub body/provenance, restart and hot local-MCP publication passed in steps 169–181 and 313–322. Manual/local resource canaries remained; callable/stale-row behavior retains owning daemon integration evidence. See docs/qa/reports/2026-09-14-marketplace-review-public.md.
