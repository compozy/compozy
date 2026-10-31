---
id: RT-session-later
area: RT
title: Park a session tree in Later and bring it back by date, prompt, or Back from Later
persona: Théo
journey: J-archive-session-without-deleting
expected: Move to Later stops the session, its child sessions, monitors, pending wakes, processes, and terminals, then parks the tree without deleting history; parked sessions leave the default list, board, and counts and appear in Later with reason, return date, parked time, and pending messages; a return date fires once, survives daemon restart, and lands the session in Needs you with a notification; an operator prompt resumes it; an agent message queues with disposition later and does not resume it.
entry_points: Web session list Later view and Kanban Later bar; CLI session later/resume-later/list --later; HTTP+UDS later routes; native tools session_later/session_resume_later/session_prompt
qa_status: pending
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-session-archive-catalog; ET-web-sessions-board-paging-archive
---

Later is not archive: it parks work you mean to return to. It must never delete or rewrite session
history, transcript, attachments, or the worktree binding. Forks and continues of the session are
not children and are not parked with it.

## Park a tree

1. Start a session, spawn a child session from it, register a monitor timer on it, and open a
   terminal that runs a long command from it.
2. Move the session to Later with a reason and a return date one hour out (web dialog,
   `compozy session later <id> --reason ... --return-at ...`, or `compozy__session_later`).
3. Expect: the session and child are `stopped` and parked; the monitor is canceled; the terminal
   and its process are gone; the child carries no reason or date of its own.
4. Expect: both leave the default session list, the Kanban columns, and the project counts; the
   sidebar Later toggle shows the count and the Later view lists the session with its reason,
   return date, and parked time; the board Later bar lists it per project and in All projects.

## Messages and resume

1. From another session, `compozy__session_prompt` the parked session. Expect disposition `later`,
   the session stays parked and stopped, and its Later card shows one pending message.
2. `compozy__session_status` resume attempts by agents and wakes do not start it.
3. Send a prompt from the web composer or `compozy session prompt`. Expect the session leaves
   Later, resumes, and the queued agent message is delivered after the operator prompt.

## Return date

1. Park a session with a return date a minute out, then restart the lab daemon before it passes.
2. Expect: after restart it is still parked; when the date passes it leaves Later, appears in
   Needs you as "Back from Later", and a notification arrives. Children stay parked.
3. Park a session with a return date in the past. Expect it comes back at once.

## Back from Later and archive

1. `compozy session resume-later <id>` returns it stopped to the normal list; children stay parked
   unless `--with-children` is passed.
2. Archiving a parked session succeeds and clears Later.
