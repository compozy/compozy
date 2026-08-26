---
id: ET-terminal-redaction-boundaries
area: ET
title: Keep secrets, clipboard escapes, and window titles out of everything a terminal retains
persona: Dora
journey: J-supervise-agent-terminal
expected: A redacted answer never echoes and never appears in scrollback, journal, recording, spill artifact, event payload, or log — only a length marker survives; a program cannot read or write the clipboard or inject a title through terminal output; and everything a recording or artifact keeps has already passed the secret scrub.
entry_points: Terminal input request with hidden entry; compozy terminal respond; compozy__terminal_request_input; terminal recording download; terminal spill artifact download; terminal hook payloads; daemon logs
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-terminal-agent-handoff-input; ET-terminal-journal-fail-closed; RT-secret-redaction-boundary
---

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
