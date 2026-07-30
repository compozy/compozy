---
id: ET-web-session-permission-dock
area: ET
title: Permission and clarification decisions dock on the composer
persona: Théo
journey: J-answer-agent-requests
expected: A pending permission docks above the composer (fused corners) with buttons only for the decisions the runtime offers; keys 1–4 map to allow-once / allow-always / reject-once / reject-always, key 4 firing even while the reject split menu is closed; digit shortcuts ignore focused inputs; resolving leaves a one-line receipt in the transcript for BOTH outcomes (allowed and rejected). A pending clarification docks with 30px choice rows on keys 1–9, or the free-text form when no choices ship (Enter submits, Shift+Enter breaks); the deadline hint is static mono and never ticks; submitting/retryable errors render as quiet dock status lines; multiple pending decisions queue with a mono "1/N" counter, permissions first.
entry_points: web session window composer zone; POST session approve; session clarifications REST
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: RT-session-clarification-roundtrip
---

story: As a person steering an agent I decide permissions and answer questions right at the composer without chasing a sticky card through the transcript, and the transcript keeps a truthful one-line record of every decision.

errors:

inventory: Needs QA — introduced by the session transcript redesign (composer decision dock, 2026-07-29).
