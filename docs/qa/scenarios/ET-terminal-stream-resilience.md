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
last_report: docs/qa/reports/2026-10-17-issue-774-terminal-workspace-stall.md
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
3. Confirm that neither acknowledging nor dropping viewers can stall the running process. A viewer
   that stops consuming remains bounded and receives a GAP or an explicit disconnect; it cannot
   prevent public terminal reads from advancing. Slow browser parsing under a sustained flood
   must also bound the client render backlog and held snapshot tail by bytes and frame count. Confirm snapshot recovery
   replaces discarded work, keeps input gated through replay completion, and renders current
   output once the viewer catches up. Consecutive server trims must preserve cumulative GAP metadata.
4. Drop and restore the network for one viewer mid-command; confirm it resumes from where it left off
   with no duplicated and no missing acknowledged output.
5. Resize from two interactive viewers and confirm the last accepted size converges everywhere while
   the explicit read-only view never sends a resize.
6. Reconnect the catalog stream with a fresh cursor and then with a cursor older than the retained
   window; confirm the first replays and the second restarts from a full snapshot of the current list.
7. Reuse an attach ticket, use an expired one, and use one minted for a different terminal or mode;
   confirm each is refused before the connection is established.
8. Open a short-lived approved agent terminal while its catalog refresh is in flight. Let the process
   exit before the attachment completes; confirm the retained output and exit bar appear, and a late
   catalog response cannot restore the running state. Repeat in the all-profiles catalog without
   changing another profile's rows or another workspace's cache.

9. Start a long-running command that continuously writes output in workspace A. Switch to workspace B
   for at least 30 seconds and observe increasing output sequence through `terminal get` or native
   `terminal_read`/`terminal_wait` in A. Return to A and confirm current output renders through the
   existing replay/GAP/snapshot path. Repeat with the terminal minimized and the browser backgrounded.
   Repeat switches while attachment is opening; canceled requests must not become HTTP 500, and the
   final active view must reconnect. No slow-consumer timeout may hold the PTY on any reattach.

QA impact 2026-10-17: issue #774 removes PTY backpressure from viewer ACKs and uses drop-mode Web
attachments. Targeted isolated re-walk passed: 33 seconds in another workspace with advancing public
read sequences, minimized/backgrounded views, current output on return, and repeated no-ACK full
replays. Overflow preserves ATTACHED before GAP; replay demotion cannot re-enter the session lock.
See the current report for receipts and platform/evidence-publication limits. Unchanged historical
evidence below remains historical only.

2026-09-04 targeted re-walk: passed for the changed shared-control surface. Two browser viewers and two
CLI viewers received the same live output while writing independently. Detaching one CLI left the peer
writable, and a newly opened browser session replayed the terminal and wrote immediately. The unchanged
bounded-flow, stale-cursor, and attach-ticket guarantees retain the focused 2026-09-02 flood evidence
and canonical transport-suite coverage cited by the report.

2026-09-07 targeted regression: the canonical catalog suite reproduces a late REST response
overwriting a live exit and now passes for concrete and aggregate profiles with workspace isolation.
The unchanged terminal-agent E2E-003 journey passed against the real daemon in
[CI run 34144138424](https://github.com/compozy/compozy/actions/runs/34144138424), including retained
output, the exit bar, and journal discovery. See `docs/qa/reports/2026-09-07-ci-recovery.md`.

QA verification 2026-10-06: the existing E2E-014 alternate-screen TUI now builds in
the runner environment before launching in the isolated operator home, avoiding a
read-only Go module cache during teardown. Three focused real-browser runs passed
resize, watcher agreement, primary-screen restoration and cleanup. This is a focused
slice, not a new full-scenario verdict; official-shard and CI evidence are tracked in
`docs/qa/reports/2026-10-05-dependency-upgrades.md`.


All-projects A4 backend coverage (2026-10-21): Read the local `/api/terminals/catalog-stream?all_workspaces=true` across projects and profiles. Reconnect with its selection-bound cursor, then widen the selection: expect a fresh snapshot with `reset: true`, including terminals created before the previous fence. Close/delete events identify workspace and profile. The private Gateway returns 404 for both terminal aggregate routes.
Targeted evidence: `docs/qa/reports/2026-10-21-all-projects-user-scope.md`. The integrated Web walk remains owned by the orchestration QA slice.

QA impact 2026-11-02 (responsiveness audit): ring retention now reuses owned byte buffers and
limits the newline preference search to its existing 256-byte window. Reconnect cursors,
UTF-8/escape boundaries, typed redacted markers and immutable returned replays remain the contract.
The append improvement does not imply faster full replay: immutable full replay adds one copy.

Focused evidence: [terminal performance audit](../reports/2026-11-01-performance/terminal-terminal.md).
`TestRingReplayContract` (`internal/terminal/ring_test.go`), `TestSessionTailReadContract` and
`TestSessionAttachReplayAndResizeContract` (`internal/terminal/manager_test.go`) passed with
`-race` through the bounded runner. The latter includes a stopped-consuming read/write viewer
while output continues; this is controlled runtime-contract coverage, not a browser flood walk.

Existing current-head CI targets, identified here but not executed by this note:

- `web/e2e/__tests__/terminal.spec.ts`: `E2E-014: alternate-screen TUI reflows, matches a watcher, and restores primary`.
- The same file: `E2E-020: zsh prompt > redraws each key with shell integration disabled` and
  `E2E-020: zsh prompt ❯ redraws each key with shell integration disabled` (redraw and reconnect).
- `web/e2e/__tests__/terminal-agent.spec.ts`: `E2E-004: two browser contexts write to one agent terminal concurrently`.
- `internal/api/core/terminal_wire_integration_test.go`: `TestTerminalWireShouldCompleteRealLifecycle`
  supplies the existing real wire/runtime integration target alongside the browser selection.

No browser re-walk is claimed for this audit. The prior scenario verdict remains historical;
the owning root/CI run must attach its fresh receipt before claiming the changed journey passed.
