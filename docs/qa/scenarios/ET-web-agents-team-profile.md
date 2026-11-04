---
id: ET-web-agents-team-profile
area: ET
title: Agents list, agent home and focused editors
persona: Bruno
journey: J-31
expected: /agents shows one card per agent in Always-on then On-demand sections, in Cards and Rows; the agent's home shows identity edited in place, where it's working as live cards, activity, findings, recent work and Setup; the Model row is the runtime selector itself, a pick saves at once and Use default returns to "Uses your default"; Tools opens a side panel; Instructions, Heartbeat, Memory and Where it works open focused pages; old tab and section links land on the matching panel or page, and old model links land on the home; no surface prints a slug; a mention of an agent is a reference that starts nothing
entry_points: web /agents; /agents/$name; /agents/$name/what-it-does; /agents/$name/check-ins; /agents/$name/memory; /agents/$name/where
qa_status: blocked-verify
bug_ids:
fix_status: fixed
retest_status: pending
fix_commits: f7a8bbf3d, 4a6fef558, 2e2487068, 11e90692f, 2f917de93, 9d9147a9f, 10183640d, 301b10320, f5d9906aa, 1300ed91f, 1a9b61852, 7fc9e2389, 6b13d0dec, e53e4593a, 11042786b, b280406e0, a64b1c981
evidence: docs/qa/reports/2026-11-01-stays-on-agents.md; .compozy/tasks/always-on-agent/qa/pr-evidence.md
last_report: docs/qa/reports/2026-11-03-vocabulary-alignment-qa.md
overlaps: ET-web-agent-detail-tab-parity; ET-web-agent-fleet-listing-rows
---

Added by always-on F2 (S2, S4 to S9, S12, S13) on 2026-11-01. Slice walk: an isolated daemon with
two projects, one Global and two project instances, screenshotted in Chromium in both themes on
2026-11-01. Activity, check-in, occurrence and away routes weren't in the daemon yet, so those states
were seen as their honest empty copy, not with data. Walk them again once B3 lands.

Steps:

1. Open Agents under All projects. Same slug in two projects reads as one card. Same slug with
   different definitions reads as two cards, each labeled by origin.
2. Filter by Where it works, Model and Made by, and use the Needs you, Working and Paused tabs.
   Switch between Cards and Rows; the groups stay the same.
3. Open an always-on agent. Edit the name, the description and the picture in the header. The slug,
   route and sessions don't change. Save each picture kind: a generated face on Auto, the same face
   with a picked color, an icon with a color, and an emoji. Each save succeeds with no error toast
   and survives a reload.
4. Pause and resume a place, add it to another project, and remove it. Removing keeps its sessions.
5. Under Setup, pick a model in the Model row. It saves at once, with no panel, Save or
   Command field, and survives a reload. Choose Use default; the row reads "Uses your default"
   again. Open Tools, save a change, close with unsaved edits and confirm the guard.
6. Open Instructions. An agent with no SOUL.md leads with Starting instructions, which stay editable.
7. Open Heartbeat. Change How often, When and What it looks at, then open Advanced and confirm the
   file. Turn the heartbeat on and off per project.
8. Open Memory. Add, edit and remove a note, go back to a version, and force a conflict by writing
   through the CLI while editing. Choose Keep mine, then repeat and choose Use theirs.
9. Open an extension-owned agent. The definition is read-only with its owner named; places and
   memory stay editable.
10. Mention an agent in the composer. The chip shows its avatar and name, and nothing starts.

Check-in history validation: after two persisted wake events, read the instance's
`check-ins` CLI command and `compozy__agent_instance_check_ins` with `limit=1`.
Follow `next_cursor` and confirm newest-first ordering, no duplicate rows, and an
empty final cursor. An instance in another workspace must never expose those events.
The HTTP and native reads share the same persisted-event reader. The browser walk
remains pending; SQLite-backed handler and repository tests cover the read path.
11. Set `agents.always_on.max_active_instances` to 25 with 10 agents running. Every picker still
    offers its rows and the create screen keeps Where it works. Set it to the running count and the
    meter shows that number; the daemon's limit is what stops a start.
12. Pause a place with a pending failure report. Its card shows Resume, or Can't resume when the
    definition is gone, keeps the triangle, and never says Pause or Working. Set
    `agents.heartbeat.fatigue_limit` to 3 and fail three checks: every message says 3.
13. Open Heartbeat for an agent with no HEARTBEAT.md. It reads all day with all five sources, and
    turning checks on writes no file. Turn every source off and save: the file says `include: []`
    and the page reopens with all of them off.
14. Switch to another profile, add an agent there, then open All profiles. Rows from both profiles
    appear, and pause, Heartbeat and Memory act on each row's own profile.

QA impact 2026-11-03 (#1199 vocabulary, v0.5.1): the setup section reads Setup, and its rows and page
titles read Instructions, Model, Heartbeat, Memory, Tools and Permissions. The URLs `what-it-does`
and `check-ins` are unchanged, so old links open the renamed pages. The sections read Always-on and
On-demand. Not walked; S4 walks it on the integration head.

2026-11-01 walk: list, profile, Memory, Where it works, pause, resume, add, remove, cap and the all-day Check-ins reading passed for agents with a project. Blocked: Global-only and On-demand agents can't open What it does or Check-ins, because authoring needs a workspace ID (open defect 1 in the report). Saves, extension-owned agents, All profiles and mentions weren't exercised.

B5 regression journey: under All projects, open Instructions and Heartbeat for a Global-only
always-on agent and an On-demand agent without any project placement. Read and save both files.
With no HEARTBEAT.md, verify all day and all five sources. Switch profiles, select the same global agent
name, save content and check that history and rollback remain within that profile. The global
definition and sidecar files remain shared; persisted revisions and snapshots belong to each profile.
User-scope HTTP read/write/history/rollback, two-profile SQLite authoring tests and the isolated
browser walk passed. Both agent kinds opened both editors under All projects. SOUL UI saves
survived reload; missing HEARTBEAT showed 00:00 to 24:00 with all five sources checked. Evidence:
`reports/2026-11-01-stays-on-b5-remediation.md`.

PR 1134 regression checks:

- With same-named definitions in two projects and profiles, each card, editor, report source,
  task owner, automation and session mention uses its recorded owner. Activity pulses sum only
  the definition's resolved projects. Hover or focus a card title and confirm the scoped detail
  read starts before navigation.
- Start a dry run for instance A, switch to B and start another. Complete A last; B retains its
  own result. Repeat two requests for B in reverse completion order.
- Create from another project's picker. A successful first start opens that project; Decide
  later and a failed start open the saved definition's project or User scope.
- Mention a display name containing parentheses and a newline; the transcript chip preserves
  the full label. No session starts as a side effect.
- Read check-ins through HTTP and native tools; bounded details survive the shared projection.

Spec-cycle B1 check-in impact: for a workspace instance with `context.include: [work_log]`,
write `.compozy/work-log.md` with a `[ ]` entry older than 30 minutes. Check-ins triage receives
`work:<id>` once, suppresses an unchanged successful digest within MaxInterval, and retries a
failed triage. User/global instances collect nothing. A symlink outside the workspace is never
read; overflow produces `work:log-truncated`. Backend execution evidence and remaining release
walks are tracked in `docs/qa/reports/2026-11-02-spec-cycle/B1.md`; the editor change belongs to F1.

Issue 1204 regression: break an always-on agent's `appearance` in its AGENT.md by hand (drop
`version` and `color`). `/agents` and the agent's home still open and its places still list, on
the default face, with an `agent_appearance_invalid` warning on the definition read. Picking a new
face in the header saves and replaces the broken block.
Issue 1219 regression: duplicating that agent without an appearance override succeeds; the copy
shows the default face for its new name and the source keeps its broken block.

Appearance background regression (#1208 review): saving an icon or emoji with an omitted
`background` or explicit `plate` succeeds. An authored `background: none` is preserved on disk,
but reads fall back to the default face with an `agent_appearance_invalid` warning; writes of a new or changed
unsupported appearance are rejected with a message naming the optional `plate` rule.
Repeat for both icon and emoji. Picking a supported face repairs the saved block.

Large telemetry regression (#1283): use an isolated profile with at least 1,000,000 retained
`event_summaries`, mostly belonging to another agent, and several Always-on members. Open Agents
in Cards and Rows. Each visible group fetches its members' pulses with one
`POST /api/agents/activity`, preserving the definition's resolved workspace set. Open a sparse
agent's profile and revisit/refocus both views. Confirm no status-499 bursts, no sustained daemon
CPU increase, and p95 below 100 ms for the sparse agent activity read. Switch profile and use
same-named definitions in different projects: pulses and cost must stay within the authorized
profile and requested definition scopes. An empty authorized scope returns empty arrays.
Automated evidence: `TestInstanceActivity`, `TestAgentInstanceActivityReach`, and
`agent-team-page.test.tsx`; interactive desktop CPU verification remains a separate release walk.
