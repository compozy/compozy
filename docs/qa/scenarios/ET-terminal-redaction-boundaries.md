---
id: ET-terminal-redaction-boundaries
area: ET
title: Keep secrets, clipboard escapes, and window titles out of everything a terminal retains
persona: Dora
journey: J-supervise-agent-terminal
expected: A redacted answer never echoes and never appears in scrollback, journal, recording, spill artifact, event payload, or log — only a length marker survives; a program cannot read or write the clipboard or inject a title through terminal output; and everything a recording or artifact keeps has already passed the secret scrub.
entry_points: Terminal input request with hidden entry; compozy terminal respond; compozy__terminal_request_input; terminal recording download; terminal spill artifact download; terminal hook payloads; daemon logs
qa_status: pass
bug_ids: BUG-20260901-private-passphrase-session-composer; BUG-20260902-private-input-shell-leak
fix_status: fixed
retest_status: pass
fix_commits: pending-remediation-batch
evidence: /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-review-r2-20260902-020216-937662-lab/qa-artifacts/qa/screenshots/dora-recorded-private-input.png; /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-review-r2-20260902-020216-937662-lab/qa-artifacts/qa/logs/stream-lease-redaction-session.md; docs/qa/reports/2026-09-01-integrated-terminal-review-r2.md
last_report: docs/qa/reports/2026-09-01-integrated-terminal-review-r2.md
overlaps: ET-terminal-agent-handoff-input; ET-terminal-journal-fail-closed; RT-secret-redaction-boundary
---

qa-impact: 2026-09-01 deep-review round 2 changed OSC/DCS filtering and redacted-output drain
ordering. Reset for a focused secret-retention and hostile-escape re-walk.

Planned by integrated-terminal task 09 for the containment guarantees that no task 06–08 scenario
owned end to end. `ET-terminal-agent-handoff-input` confirms a hidden answer is delivered; this file
confirms that nothing the daemon retains ever holds it. Task 10 owns the walk, evidence, and verdict.

Walk:

1. Answer a hidden input request with a recognisable secret while a recording is running, then search
   the live scrollback, the journal row, the downloaded recording, any spill artifact, the emitted
   event payloads, and the daemon log for that value; only a length marker may appear anywhere.
2. Confirm nothing is echoed on screen during the hidden entry, and that after the answer the shell's
   own echo behaviour is exactly what it was before.
3. Reject a hidden request and confirm the same containment applies to the rejection path.
4. Run a program that tries to read and to write the system clipboard through terminal output and
   confirm neither direction reaches the operator's clipboard, for the controller and for a watcher.
5. Confirm the filtered bytes are the same bytes everywhere: what a reattaching viewer replays, what a
   recording plays back, and what the byte counters report must agree after that program ran.
6. Run a program that emits a hostile window title — control characters and an over-long string — and
   confirm the title shown in the window, the tab, and the terminal list is sanitised and bounded.
7. Have an agent read the same screen and confirm the model-facing read additionally neutralises
   escape sequences that could carry instructions, and marks the content as untrusted.
8. Write a secret-shaped value into a recording and a spill artifact and confirm both are scrubbed
   before they hit disk, stored under the workspace's own protected directory, and refuse to resolve
   through a symbolic link that points outside it.

2026-09-02 re-walk: passed after remediation. Hidden answers persisted only length markers; the raw
canaries were absent from live and durable surfaces. Hostile OSC title, OSC52, and DCS payloads were
removed while visible text survived consistently. The saved recording and spill artifact were
scrubbed, mode `0600`, workspace-contained, and the artifact endpoint refused a forged external
symbolic link.

2026-09-12 issue 628 verification slice:

9. Start `bash --noprofile --norc` inside the Terminal and type `abc` one character at a time.
   Each character appears without a trusted hidden-input marker. Editors and pagers using the
   same raw no-echo mode have the same automatic classification, regardless of foreground group.
10. Run a canonical no-echo reader and confirm automatic length-only redaction still applies.
11. Run a raw no-echo reader that never prints its input. Submit an explicitly redacted request;
    confirm one delivery and length-only journal/stream metadata. Re-enable echo before answering
    another request and confirm rejection with zero bytes delivered. Echo must remain program-owned.

Raw mode alone cannot identify secrets. Ordinary typing into raw programs is retained as ordinary
input; use explicit redaction for a trusted secret reader. Program-rendered text is output and is
not made private by marking its input redacted.

Issue 628 targeted replay passed on macOS: `TestUnixPTYHardening` exercised real nested bash and
canonical/raw no-echo readers; `TestSessionTypingGrantAndInputRequestLifecycle` verified explicit
raw answers, length-only journal/stream data, and echo-enabled rejection. The existing real
`TestTerminalWireShouldCompleteRealLifecycle` HTTP/WebSocket run typed `abc` one character at a
time in nested bash without a redaction opcode. Existing secret-output-sink tests passed with
`-race`. This slice changes no Web layout or hostile-output filtering behavior. Linux and Windows
runtime parity is owned by the PR CI lanes.
