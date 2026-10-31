# QA Run Report — 2026-10-31 — Later: park sessions

- Scope: issue #1076, Move to Later, Back from Later, return dates, agent messages to parked
  sessions, and the Web Later surfaces.
- Cadence: targeted.
- Build: working tree on `feat/issue-1076-later`.
- Environment: two isolated labs (temporary home, HTTP listener, UDS) from `eng-qa-bootstrap`;
  real daemon built from the branch, Claude Code provider on Haiku 5.5 for the agent steps, built
  Web bundle served by the daemon, headless Chromium.
- Status: PASS for RT-session-later; PR CI and bot reviews remain delivery gates.

## Session Matrix

| Charter | Persona | Journey / scenario | Tour | Status |
| --- | --- | --- | --- | --- |
| Park a tree, restart, return by date, messages and resume | Théo, CLI + agent | J-archive-session-without-deleting / RT-session-later | Feature | PASS |
| Later toggle, Later view, Move to Later dialog, Kanban Later bar | Théo, Web | RT-session-later | Feature | PASS |

## Walk

1. A parent session used native tools to start `sleep 3600` in a terminal, register a 2-hour
   timer monitor, and spawn a child session.
2. `compozy session later <parent> --reason ... --return-at <now+4m>` returned in about 3s. The
   parent and child were `stopped` and parked (child without reason or date), the monitor list was
   empty, the session process list was empty, and no `sleep 3600` process remained. The default
   list was empty and `--later` returned both with a total of 2.
3. Another session called `compozy__session_prompt` on the parked parent: disposition `later`, the
   parent stayed stopped, and its Later state showed one pending message.
4. The lab daemon was restarted before the return time: both sessions were still parked.
5. At the return time the catalog stream delivered `operator_notification` "Back from Later" with
   the reason, the parent left Later with `returned_from_later_at`, and the child stayed parked.
6. Re-parked, then `compozy session prompt` from the operator: the parent left Later, resumed, and
   the queued agent message was delivered.
7. A past `--return-at` came back at once; archiving a parked child archived it and cleared Later;
   `resume-later` returned it stopped.
8. Web: the sidebar showed the Later toggle with its count beside Archive; the Later view listed
   the parked session with its reason, return date, and parked time; Move to Later from the row
   menu opened the dialog, and after confirming, the session left the list, the count rose to 2,
   and the CLI showed it stopped and parked with the reason. The Kanban showed the returned
   session in Needs you as "Back from Later" and the Later bar with 2 at the bottom.

## Limits

- Esc did not leave the Later view when focus was outside the list; the toggle did. This matches
  the Archive toggle.
- On resume the queued agent message drained before the operator prompt, so the operator prompt
  was delivered as steering into that turn.
