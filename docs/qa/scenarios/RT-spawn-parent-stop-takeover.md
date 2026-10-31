---
id: RT-spawn-parent-stop-takeover
area: RT
title: Resume and take over a spawned worker after its parent stops
persona: Ada
journey: J-15
expected: When an orchestrator that spawned a worker (also into another workspace) stops, the reaper stops the worker as a resumable stop. Prompting the worker resumes it and later reaper sweeps leave it running. Stopping it and choosing Take over session (or `session reparent --top-level`) turns it into a top-level user session with the full row menu; subagents and running workers are refused.
entry_points: compozy spawn; compozy__session_spawn; compozy session prompt/stop; compozy session reparent --top-level; compozy__session_reparent; DELETE /api/workspaces/{workspace_id}/sessions/{session_id}/parent; Web session row menu Take over session
qa_status: pass
bug_ids: GH-1095
fix_status: fixed
retest_status: pass
fix_commits:
evidence: internal/session/manager_lifecycle_contract_test.go (TestSharedSessionStopOperation startup stop); internal/session/spawn_test.go (TestManagerSpawnStoppedParentRules); internal/session/session_reparent_test.go (TestManagerAdoptSession take-over); web/src/systems/session/components/__tests__/session-list.test.tsx; isolated acpmock lab walk recorded in the issue-1095 PR
last_report:
overlaps: RT-spawn-ttl-cleanup; RT-session-list-row-actions
---

Spawn a worker from a user session in workspace X into workspace Y with the default
`auto_stop_on_parent`. Prompt it once, then stop the parent. Within one reaper sweep the worker is
stopped with `spawn_reaper:parent_stopped` and a canceled (not process) failure.

Prompt the worker again. It resumes and stays running across at least two reaper sweeps; its
lineage keeps the parent, TTL, and permission narrowing, with `auto_stop_on_parent: false`.

While it runs, `session reparent <id> --top-level` is refused with
`session_reparent_source_running`. Stop it and repeat: the worker becomes `type=user` with a root
lineage, no TTL, auto-stop, creator wake, or narrowing, and the next prompt resumes it. In the web
session list, the stopped worker's row menu offers **Take over session** only (no Rename, Continue,
or Fork); after the take-over the toast reads "You took over …", the row moves to the top level, and
the menu offers Rename, Continue with another agent, Fork, Nest under, and Clear conversation.

Regression baseline 2026-10-31 (isolated acpmock lab, `origin/main` build): the resumed worker was
stopped again by the next reaper sweep, and `--top-level` was refused with "only user sessions can
be made top-level". A launch still starting when the reaper fires settled as a `process` failure
(`session: startup canceled for …`), which marks the runtime dead and refuses every resume; the
take-over reclassifies such legacy failures as a canceled stop.

QA completion 2026-10-31: the branch build passed every step above in an isolated lab (unique
home, port, and socket) through the CLI, HTTP session reads, and the served web bundle.
