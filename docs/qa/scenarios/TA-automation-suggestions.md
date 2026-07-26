---
id: TA-automation-suggestions
area: TA
title: Review and resolve consent-first automation suggestions
persona: Bruno
journey: J-24
expected: A fresh workspace lists 3–5 workspace-owned pending Job proposals up to the positive configured cap; Create job accepts exactly one proposal through normal Job validation and the lifecycle-command guard, persists a schedulable dynamic Job, and removes the proposal; Dismiss durably latches another proposal across reload; no suggestion crosses workspace boundaries.
entry_points: Web `/jobs`; `*/workspaces/{workspace_id}/automation/suggestions*`; CLI `automation suggestions`; native `agh__automation_suggestions_*`; config `automation.suggestions.pending_cap`
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: `internal/automation/suggestion_catalog.go`; `internal/automation/manager_suggestion.go`; `internal/store/globaldb/global_db_automation_suggestions.go`; `web/src/systems/automation`
last_report:
overlaps: TA-052
---

Exercise first-list catalog seeding, one accepted Job through its first scheduler fire, one durable
dismissal across reload, configured-cap enforcement, concurrent acceptance, lifecycle-command
rejection, structured parity, and workspace isolation. This file flags the new behavior for the next
QA cycle; no QA replay has run.

Phase C planning 2026-07-19: persona normalized to Bruno; settles US-008 (A1, ADR-007; Safety
Invariant 16). Only the `catalog` source emits in this program.

Forensic evidence contract (SD-006) — each item cites timestamp, exact command, observed output:

- Fresh-workspace seed listing (3–5 pending, workspace-scoped) and the suggestions-card screenshot.
- Accept → job row + first scheduler fire; the CAS conflict (`ErrSuggestionResolved`) under
  concurrent resolution.
- The dismissal latch proven by a re-emission attempt, the pending-cap enforcement at insert, and
  the rejected lifecycle-command prefill (nothing persisted).
