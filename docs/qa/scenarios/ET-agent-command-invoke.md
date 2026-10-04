---
id: ET-agent-command-invoke
area: ET
title: Invoke a discovered command through structured surfaces
persona: Ada
journey: J-operate-command-palette
expected: CLI, HTTP/UDS, and native-tool discovery, inspection, client targeting, invocation, and approval status return one workspace-bound terminal result without duplicate execution; every refusal (unknown id, invalid arguments, unavailable context, no attached shell, multiple clients, already running) is a structured error carrying the same reason text the UI shows.
entry_points: compozy cmd-palette list|inspect|invoke|clients; compozy approvals show|resolve|cancel; compozy__cmd_palette_list|invoke; GET /api/cmd-palette/commands|clients (HTTP + UDS); POST /api/cmd-palette/commands/{id}/invoke (HTTP + UDS); GET /api/tools/approvals/{id} (HTTP + UDS); POST /api/tools/approvals/{id}/resolve|cancel (HTTP + UDS); GET /api/cmd-palette/stream (HTTP + UDS)
qa_status: untested
bug_ids: BUG-20261004-palette-lists-closed-clients
fix_status: fixed
retest_status: pass
fix_commits: 259d7142c
evidence: docs/qa/evidence/2026-10-02-untested/profile-client-disconnect-listed-after-close.json; docs/qa/evidence/2026-10-02-untested/profile-client-disconnect-invoke-closed.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-agent-palette-config-parity; ET-palette-inline-args-confirmation; ET-palette-registry-driven-root
---

Flagged by command-palette task 01. Task 12 owns the first real-user walk and verdict.

Walk (task_11 plan):

1. `cmd-palette list -o json` — full catalog with ids, availability + reasons, bindings, argument
   schemas; `--available=false` shows verbatim reasons; without `--client`, client-context commands
   report "requires an attached shell" regardless of attachment count.
2. `inspect` a destructive command — destructive flag, confirmation copy, execution policy, and
   risk match the listing.
3. `invoke` with valid args → `status: ok` + result; missing required → exit 2 invalid_arguments
   naming fields; unknown id → exit 1 command_not_found; a context-unavailable command → the same
   reason string the UI row shows.
4. Invoke a UI-effecting command with zero clients → no_attached_shell; with two attached clients
   and no `--client` → multiple_clients listing both ids; `cmd-palette clients` is the targeting
   source of truth; with `--client` it lands in that client.
5. Invoke a destructive command → 202 approval_pending with a stable approval id; `approvals show`
   tracks pending → terminal; approve → exactly one execution; deny/cancel → terminal without
   effect; a duplicate invoke while pending → already_running until the terminal outcome releases
   the guard.
6. Repeat list/invoke through `compozy__cmd_palette_list|invoke` — output parity with CLI/HTTP,
   same approval and no-shell gates.

Expected evidence: CLI/HTTP/native transcripts for each error class beside the matching UI reason,
the approval lifecycle transcript (pending → terminal, exactly-once), and the multiple_clients /
no_attached_shell captures.

QA 2026-10-04: the adjacent attached-client leg fails. A closed browser tab remains in the public
targeting inventory and invoking it returns runtime_unavailable. The full native/approval/refusal
matrix remains unwalked; this finding does not establish its other outcomes.


The attached-client defect is verified in 259d7142c: closing a tab removes its destination,
the survivor receives automatic invocation, reload reconnects with the same identity, and zero
clients return no_attached_shell. The full native, approval and other refusal matrix remains
Pending in the cycle; qa_status is untested for those remaining legs.
