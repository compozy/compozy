---
id: ET-web-agents-team-profile
area: ET
title: Agents list, agent home and focused editors
persona: Bruno
journey: J-31
expected: /agents shows one card per agent in Stays on then On demand sections, in Cards and Rows; the agent's home shows identity edited in place, where it's working as live cards, activity, findings, recent work and how it's set up; Model and What it can use open side panels; What it does, Check-ins, Memory and Where it works open focused pages; old tab and section links land on the matching panel or page; no surface prints a slug; a mention of an agent is a reference that starts nothing
entry_points: web /agents; /agents/$name; /agents/$name/what-it-does; /agents/$name/check-ins; /agents/$name/memory; /agents/$name/where
qa_status: blocked-verify
bug_ids:
fix_status: fixed
retest_status: pending
fix_commits: f7a8bbf3d, 4a6fef558, 2e2487068, 11e90692f, 2f917de93, 9d9147a9f, 10183640d, 301b10320, f5d9906aa, 1300ed91f, 1a9b61852, 7fc9e2389, 6b13d0dec, e53e4593a, 11042786b, b280406e0, a64b1c981
evidence: docs/qa/reports/2026-11-01-stays-on-agents.md; .compozy/tasks/always-on-agent/qa/pr-evidence.md
last_report: docs/qa/reports/2026-11-01-stays-on-agents.md
overlaps: ET-web-agent-detail-tab-parity; ET-web-agent-fleet-listing-rows
---

Added by stays-on F2 (S2, S4 to S9, S12, S13) on 2026-11-01. Slice walk: an isolated daemon with
two projects, one Global and two project instances, screenshotted in Chromium in both themes on
2026-11-01. Activity, check-in, occurrence and away routes weren't in the daemon yet, so those states
were seen as their honest empty copy, not with data. Walk them again once B3 lands.

Steps:

1. Open Agents under All projects. Same slug in two projects reads as one card. Same slug with
   different definitions reads as two cards, each labeled by origin.
2. Filter by Where it works, Model and Made by, and use the Needs you, Working and Paused tabs.
   Switch between Cards and Rows; the groups stay the same.
3. Open a stays-on agent. Edit the name, the description and the picture in the header. The slug,
   route and sessions don't change.
4. Pause and resume a place, add it to another project, and remove it. Removing keeps its sessions.
5. Open Model and What it can use. Save a change. Close with unsaved edits and confirm the guard.
6. Open What it does. An agent with no SOUL.md leads with Starting instructions, which stay editable.
7. Open Check-ins. Change How often, When and What it looks at, then open Advanced and confirm the
   file. Turn check-ins on and off per project.
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
11. Set `agents.stays_on.max_active_instances` to 25 with 10 agents running. Every picker still
    offers its rows and the create screen keeps Where it works. Set it to the running count and the
    meter shows that number; the daemon's limit is what stops a start.
12. Pause a place with a pending failure report. Its card shows Resume, or Can't resume when the
    definition is gone, keeps the triangle, and never says Pause or Working. Set
    `agents.heartbeat.fatigue_limit` to 3 and fail three checks: every message says 3.
13. Open Check-ins for an agent with no HEARTBEAT.md. It reads all day with all five sources, and
    turning checks on writes no file. Turn every source off and save: the file says `include: []`
    and the page reopens with all of them off.
14. Switch to another profile, add an agent there, then open All profiles. Rows from both profiles
    appear, and pause, Check-ins and memory act on each row's own profile.

2026-11-01 walk: list, profile, Memory, Where it works, pause, resume, add, remove, cap and the all-day Check-ins reading passed for agents with a project. Blocked: Global-only and On demand agents can't open What it does or Check-ins, because authoring needs a workspace ID (open defect 1 in the report). Saves, extension-owned agents, All profiles and mentions weren't exercised.

B5 regression journey: under All projects, open What it does and Check-ins for a Global-only
Stays on agent and an On demand agent without any project placement. Read and save both files.
With no HEARTBEAT.md, verify all day and all five sources. Switch profiles, select the same global agent
name, save content and check that history and rollback remain within that profile. The global
definition and sidecar files remain shared; persisted revisions and snapshots belong to each profile.
User-scope HTTP read/write/history/rollback, two-profile SQLite authoring tests and the isolated
browser walk passed. Both agent kinds opened both editors under All projects. SOUL UI saves
survived reload; missing HEARTBEAT showed 00:00 to 24:00 with all five sources checked. Evidence:
`reports/2026-11-01-stays-on-b5-remediation.md`.
