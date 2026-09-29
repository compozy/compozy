# BUG-20260928-derive-stopped-source-turn-in-progress: Continuing or forking a stopped session claims a turn is still in progress

- **Status:** fixed
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Théo
- **Journey Step:** J-15-operate-session-via-cli-api, derive branch (continue/fork of an idle source)
- **Scenarios:** RT-session-lineage-upgrade; ET-web-session-continue; ET-web-session-fork-from-here; ET-cli-session-continue
- **Found:** 2026-09-28 · **Report:** docs/qa/reports/2026-09-28-session-continue-fork-exec-c.md

## Summary

Théo stops a session (or the daemon stops it), then continues or forks it. Every turn in that session has finished, but the preview and the derive outcome say `source_turn_in_progress: true`. The Web dialogs then show "A turn is still in progress; it will not be carried over." on a session that has no running turn. The fork dialog also drops its epoch/generation/max-sequence fences for such a source, because it treats the source as running.

## Reproduction

- **Charter:** CH-session-derive-interrupt · **Tour:** Interrupt Tour
- **Environment:** desktop / CLI + HTTP / en-US, isolated Lab B (acpmock agent `up-alpha`)

1. `compozy session new --agent up-alpha --name "branch stop repro"`, then `compozy session prompt <id> "hello root"` (turn ends with `done`).
2. `compozy session stop <id>`.
3. `GET /api/workspaces/<ws>/sessions/<id>/derive/preview` (or `compozy session continue <id> --agent <other>` / `compozy session fork <id>`).

**Expected:** `source_turn_in_progress: false`, because every user turn is settled and the session is stopped.
**Actual:** `{"message_count":2,…,"source_turn_in_progress":true,…}`. Continue and fork of a stopped pre-feature root also record `source_turn_in_progress: true` in their `derived` outcome and receipt.

## Evidence

- `docs/qa/evidence/2026-09-28-session-continue-fork-upgrade/preview-stopped-source-turn-in-progress.json` (branch-created stopped session `sess-9253da0b0506654e`).
- `docs/qa/evidence/2026-09-28-session-continue-fork-upgrade/walk-summary.json` → `continue_derived` / `fork_derived` for the pre-feature root `sess-2dac0818ae8c1d30`.
- Independent read: `compozy session history <id> -o json` shows the last turn as `[session.stop_escalated, session_stopped, transcript_marker.created]` with no `done`/`error`, and `session status` reports `state: stopped`, `active_prompt: false`.
- Dedup: `grep -rli 'source_turn_in_progress\|turn is still in progress' docs/qa/bugs/` found nothing.

## Root-cause hypothesis (not fixed by this walk)

`lastSettledTurn` (`internal/session/derive_cut.go`) groups every event by `turn_id` and sets `laterOpen` when any turn after the last settled one lacks a terminal event (`done`, `error`, or a prompt-settlement marker). The stop path records `session.stop_escalated` / `session_stopped` (and a stop `transcript_marker`) under a daemon lifecycle turn id with none of those terminals, so a stopped session always has an "open" later turn. The same applies to the `hook.dispatch.start` lifecycle turn on a session that has never been prompted. `derive_snapshot.go:140-142` copies that into `snapshot.sourceTurnInProgress`. Likely fix: only turns that carry a user or prompt-originated event count as open, or stop/lifecycle events count as terminal. Owning suite: the `lastSettledTurn` cases in `internal/session/derive_test.go` plus a stopped-source case in the derive preview/manager tests.

## Fix

Fixed by task_08 part B1 (uncommitted at report time). `deriveTurns` (`internal/session/derive_cut.go`) now marks a turn as a
prompt turn only when it carries a prompt-stream event (`user_message`, `synthetic_reentry`, `prompt_delivery`, agent output,
`done`/`error`); `lastSettledTurn` reports a later turn in progress only for an open prompt turn. Lifecycle-only turn ids
(hook dispatch, `session.stop_escalated`, `session_stopped`, stop markers) no longer count, so a stopped or never-prompted
session is not "in progress" and its carried context no longer ends with the aborted-turn note. A prompt turn that never reached
a terminal event (crash mid-turn) still counts. ADR-005's settlement rule is unchanged. Regression:
`TestLastSettledTurn/Should_not_flag_lifecycle-only_turns_of_a_stopped_or_unprompted_session_as_in_progress`
(`internal/session/derive_test.go`).

## Verification

Part B1 lab (`compozy-session-continue-fork-b1-20260929-010817-219689-lab`), rebuilt daemon: preview of the stopped Codex source
`sess-b585138e835a4e38` (last turn = `session.stop_escalated`, `session_stopped`) → `source_turn_in_progress: false`
(`docs/qa/evidence/2026-09-28-session-continue-fork-b1/cx-src-preview-stopped-fixed.json`; before the fix the same read returned
`true` with 1351 replay bytes, after 988 — the aborted-turn note is gone). A never-prompted session previews `false`
(`unprompted-preview.json`). A live running turn still reports `true` (`long-fork-whole.json`), and the preview returns to `false`
after cancel. The pre-feature root of Lab B was torn down before the fix and was not re-walked; the same code path serves it.
