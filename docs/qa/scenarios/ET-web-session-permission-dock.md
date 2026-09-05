---
id: ET-web-session-permission-dock
area: ET
title: Permission and clarification decisions dock on the composer
persona: Théo
journey: J-answer-agent-requests
expected: A pending permission docks above the composer (fused corners) with buttons only for the decisions the runtime offers; keys 1–4 map to allow-once / allow-always / reject-once / reject-always, key 4 firing even while the reject split menu is closed; digit shortcuts ignore focused inputs; resolving leaves a one-line receipt in the transcript for BOTH outcomes (allowed and rejected). A pending clarification docks with 30px choice rows on keys 1–9, or the free-text form when no choices ship (Enter submits, Shift+Enter breaks); the deadline hint is static, sans with tabular figures, and never ticks; submitting/retryable errors render as quiet dock status lines; multiple pending decisions queue with a sans tabular "1/N" counter, permissions first. No text in the dock family renders below 11px or in mono except the choice keycaps.
entry_points: web session window composer zone; POST session approve; session clarifications REST
qa_status: untested
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

2026-09-05 qa-impact (sessions-stability task_02, Web slice, receipt attribution): the transcript's
decided-permission receipt now names who decided only from the daemon's resolved interaction row
(`GET …/sessions/{id}/interactions?status=resolved`, field `resolved_by`); the transcript part carries
the decision but no actor, so the previous hardcoded "Not allowed by you" is gone. Leads by actor:
`operator`/`operator:*` → `Not allowed by you · <subject>` (allows keep the bare `Allowed <subject>
once`); `timeout` → `Timed out before anyone answered · <command> did not run` (generic tools:
`Timed out before anyone answered · <tool> — <subject>`); `provider`/`system` → `Allowed by the
runtime · …` / `Not allowed by the runtime · …` (never asserts whether a question was shown:
`provider` is also the daemon's fallback actor); `agent_session:<id>` → `Allowed by another
agent · …` / `Not allowed by another agent · …`; no row or an unknown actor → neutral `Not allowed ·
<subject>` and the bare `Allowed …`. The receipt exposes `data-actor` (`you` | `agent` | `timeout` |
`runtime` | `unknown`) next to `data-decision`. The Web reads `?status=resolved` while a decided
receipt is on screen and once more per newly decided ask (no polling), so a decision made in the dock
attributes itself on the next render after the daemon's transcript event. Re-walk: (1) decide an
ask in the dock → receipt reads "Not allowed by you" / "Allowed … once", `data-actor="you"`, and
`GET …/interactions?status=resolved` shows `resolved_by: "operator"`; (2) let an ask sit past
`permission_timeout` → receipt reads "Timed out before anyone answered · <command> did not run",
`data-actor="timeout"`, the row says `resolved_by: "timeout"`, and neither "by you" nor "provider"
appears; (3) an agent with `permissions.mode = approve-all` → allowed receipts read "Allowed by the
runtime", `data-actor="runtime"`; (4) reload the window → the same attributions render from the
resolved rows on first load; (5) open a second session window in another workspace whose transcript
reuses a request id → its undecided/unattributed receipt never shows the first session's actor.
Only `kind: permission` rows attribute a receipt (a resolved clarification may share the request
id). Restart-expired receipts (below) are unchanged. Not walked here.

2026-09-05 qa-impact (sessions-stability task_02, Web slice): reset for the restart-expired decision
truth. Pre-crash permission and clarification asks the daemon expires at boot (interaction status
`canceled`, resolution `failed-by-restart`, resolved by `system`) are no longer actionable in the Web:
the composer dock never docks them and the transcript renders a neutral receipt from the durable
interaction row — `Not decided · CompozyOS restarted before you answered — <command> did not run`
for permissions and `Question not answered — CompozyOS restarted before you answered · <question>`
for clarifications. The Web reads `GET …/sessions/{id}/interactions?status=canceled` only while an
undecided ask is on screen (5s control cadence), so a dock left open across a daemon restart
settles within one poll after CompozyOS returns; a window opened after the restart shows the
receipt on first load. Re-walk: pending permission → kill the daemon → restart → the dock clears,
the receipt appears, `GET …/interactions?status=canceled` agrees on the cause.

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
