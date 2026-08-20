---
id: ET-web-session-permission-dock
area: ET
title: Permission and clarification decisions dock on the composer
persona: Théo
journey: J-answer-agent-requests
expected: A pending permission docks above the composer (fused corners) with buttons only for the decisions the runtime offers; keys 1–4 map to allow-once / allow-always / reject-once / reject-always, key 4 firing even while the reject split menu is closed; digit shortcuts ignore focused inputs; resolving leaves a one-line receipt in the transcript for BOTH outcomes (allowed and rejected). A pending clarification docks with 30px choice rows on keys 1–9, or the free-text form when no choices ship (Enter submits, Shift+Enter breaks); the deadline hint is static, sans with tabular figures, and never ticks; submitting/retryable errors render as quiet dock status lines; multiple pending decisions queue with a sans tabular "1/N" counter, permissions first. No text in the dock family renders below 11px or in mono except the choice keycaps.
entry_points: web session window composer zone; POST session approve; session clarifications REST
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa; docs/qa/reports/2026-08-20-ui-normies-retry.md
last_report: docs/qa/reports/2026-08-20-ui-normies-retry.md
overlaps: RT-session-clarification-roundtrip
---

2026-08-20 retry: skipped by explicit user instruction. No permission or clarification decision was submitted.

story: As a person steering an agent I decide permissions and answer questions right at the composer without chasing a sticky card through the transcript, and the transcript keeps a truthful one-line record of every decision.

errors:

inventory: Needs QA — introduced by the session transcript redesign (composer decision dock, 2026-07-29).

2026-08-20 qa-impact: reset by the normie-friendly UI foundation pass, which de-mono'd the dock
family — this is the surface where a non-technical person allows or refuses what an agent does, so
it was the pass's highest-value micro-type target. `dock-deadline` and `dock-count` moved from
`font-mono text-[10px]` to sans `text-badge` with `tabular-nums`; `dock-meta` and `dock-status` moved
from `text-[11px]` to `text-form-label`; `dock-key` and `choice.tsx`'s keycap moved from `text-[9px]`
and `text-[10px]` to `text-mono-id` (keycaps stay mono on purpose — they represent physical keys);
`ChoiceHint` moved to `text-form-label`. The prior `expected:` asserted "static mono" and a "mono
1/N", both now false, so the contract text was rewritten alongside the reset.

The decision semantics — which buttons render, the 1–4 and 1–9 key maps, key 4 with the split menu
closed, digit shortcuts yielding to focused inputs, one receipt per outcome, queue order — are
untouched by the pass and are the parts a re-walk should confirm still hold.
