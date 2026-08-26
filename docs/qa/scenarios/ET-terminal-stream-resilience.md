---
id: ET-terminal-stream-resilience
area: ET
title: Keep the terminal stream honest under many viewers and a bad connection
persona: Marina
journey: J-operate-integrated-terminal
expected: One stuck viewer never freezes or shrinks the terminal for anyone else; a dropped viewer resumes without duplicated or invented output, states any gap plainly, and a stale replay cursor restarts from a full snapshot instead of a silent partial history.
entry_points: Web dock Terminal app in several tabs; terminal attach-ticket then stream upgrade; catalog stream with Last-Event-ID; compozy terminal attach; compozy terminal get
qa_status: untested
bug_ids: BUG-20260826-terminal-cli-raw-mode
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-20260826-074528-452132-lab/qa-artifacts/qa/test-e2e-runtime-after-fix.log; docs/qa/reports/2026-08-26-integrated-terminal.md
last_report: docs/qa/reports/2026-08-26-integrated-terminal.md
overlaps: ET-terminal-browser-lifecycle; ET-terminal-limits-capabilities
---

Planned by integrated-terminal task 09 for the transport and flow-control guarantees, which
`ET-terminal-cli-public-contract` exercises only at the level of "the stream connects". This file owns
the multi-viewer and degraded-connection behaviour. Task 10 owns the walk, evidence, and verdict.

Walk:

1. Attach several viewers to one busy terminal — at least one that controls it and several that only
   watch — and confirm the watchers never acquire or disturb control.
2. Stall one watching viewer while output floods and confirm the other viewers and the running program
   keep up; confirm the stalled viewer is either given a stated gap or disconnected with a stated
   reason, never silently fed stale bytes.
3. Confirm a slow controlling viewer is the only one that can throttle the program, and that a viewer
   which stops acknowledging is demoted with the gap made visible rather than allowed to grow without
   bound.
4. Drop and restore the network for one viewer mid-command; confirm it resumes from where it left off
   with no duplicated and no missing acknowledged output.
5. Resize from the smallest controlling viewer and confirm watchers cannot shrink the terminal.
6. Reconnect the catalog stream with a fresh cursor and then with a cursor older than the retained
   window; confirm the first replays and the second restarts from a full snapshot of the current list.
7. Reuse an attach ticket, use an expired one, and use one minted for a different terminal or mode;
   confirm each is refused before the connection is established.
