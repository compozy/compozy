---
id: ET-web-agents-team-profile
area: ET
title: Agents list, agent home and focused editors
persona: Bruno
journey: J-31
expected: /agents shows one card per agent in Stays on then On demand sections, in Cards and Rows; the agent's home shows identity edited in place, where it's working as live cards, activity, findings, recent work and how it's set up; Model and What it can use open side panels; What it does, Check-ins, Memory and Where it works open focused pages; old tab and section links land on the matching panel or page; no surface prints a slug; a mention of an agent is a reference that starts nothing
entry_points: web /agents; /agents/$name; /agents/$name/what-it-does; /agents/$name/check-ins; /agents/$name/memory; /agents/$name/where
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
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
