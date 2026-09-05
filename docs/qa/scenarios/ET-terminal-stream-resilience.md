---
id: ET-terminal-stream-resilience
area: ET
title: Keep the terminal stream honest under many viewers and a bad connection
persona: Marina
journey: J-operate-integrated-terminal
expected: One stuck viewer never freezes or shrinks the terminal for anyone else; a dropped viewer resumes without duplicated or invented output, states any gap plainly, and a stale replay cursor restarts from a full snapshot instead of a silent partial history.
entry_points: Web dock Terminal app in several tabs; terminal attach-ticket then stream upgrade; catalog stream with Last-Event-ID; compozy terminal attach; compozy terminal get
qa_status: pass
bug_ids: BUG-20260826-terminal-cli-raw-mode
fix_status: fixed
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-review-r2-20260902-020216-937662-lab/qa-artifacts/qa/screenshots/marina-stream-controller-after-flood.png; /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-review-r2-20260902-020216-937662-lab/qa-artifacts/qa/screenshots/marina-stream-watcher-recovered.png; /Users/pedronauck/dev/qa-labs/compozy-terminal-shared-control-20260904-204013-041114-lab/qa-artifacts/web-reconnect.png; docs/qa/reports/2026-09-04-terminal-shared-control.md
last_report: docs/qa/reports/2026-09-04-terminal-shared-control.md
overlaps: ET-terminal-browser-lifecycle; ET-terminal-limits-capabilities
---

QA impact 2026-09-04: interactive viewers now share mutation rights and presence no longer carries a
controller. Reset to verify flow control, reconnect, resize, and explicit read-only attachments under
the shared-control contract.

qa-impact: 2026-09-01 deep-review round 2 changed stream draining, reconnect settlement, and
redacted-output backpressure behavior. Reset for a focused degraded-stream re-walk.

2026-09-02 re-walk: passed. Three browser viewers converged after a 6,000-line flood while one watcher
was offline; the watcher announced reconnecting, recovered through the final line, never gained control,
and never resized the `80×24` terminal. A disconnected controlling viewer did not strand the command.
Catalog cursor replay and snapshot reset were distinct, and single-use, expired, foreign-terminal, and
wrong-mode attach passes all failed before upgrade.

reset 2026-08-31: the stream client now settles as closed on exit or a gone terminal instead of reconnecting forever, and the reconnect line layers above the grid; the prior verdict predates that behavior.
Planned by integrated-terminal task 09 for the transport and flow-control guarantees, which
`ET-terminal-cli-public-contract` exercises only at the level of "the stream connects". This file owns
the multi-viewer and degraded-connection behaviour. Task 10 owns the walk, evidence, and verdict.

Walk:

1. Attach several interactive viewers to one busy terminal plus one explicit read-only presentation
   view; confirm every interactive viewer can write and the read-only view remains passive.
2. Stall one viewer while output floods and confirm the other viewers and the running program
   keep up; confirm the stalled viewer is either given a stated gap or disconnected with a stated
   reason, never silently fed stale bytes.
3. Confirm a slow acknowledging viewer can apply bounded flow control, while a viewer that stops
   acknowledging is demoted with the gap made visible rather than allowed to grow without bound.
4. Drop and restore the network for one viewer mid-command; confirm it resumes from where it left off
   with no duplicated and no missing acknowledged output.
5. Resize from two interactive viewers and confirm the last accepted size converges everywhere while
   the explicit read-only view never sends a resize.
6. Reconnect the catalog stream with a fresh cursor and then with a cursor older than the retained
   window; confirm the first replays and the second restarts from a full snapshot of the current list.
7. Reuse an attach ticket, use an expired one, and use one minted for a different terminal or mode;
   confirm each is refused before the connection is established.

2026-09-04 targeted re-walk: passed for the changed shared-control surface. Two browser viewers and two
CLI viewers received the same live output while writing independently. Detaching one CLI left the peer
writable, and a newly opened browser session replayed the terminal and wrote immediately. The unchanged
bounded-flow, stale-cursor, and attach-ticket guarantees retain the focused 2026-09-02 flood evidence
and canonical transport-suite coverage cited by the report.
