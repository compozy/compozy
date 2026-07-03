---
title: Run TUI elapsed timer restored across all terminal outcomes
type: fix
---

The run TUI's elapsed **timer no longer disappears after a retried job succeeds**, and the fix now covers every terminal outcome — success, failure, and cancel — as well as remote tabs that attach after a job has already finished.

### What was wrong

The UI derived a job's elapsed time from a locally tracked start timestamp that the retry flow never seeded: retry attempts emit no fresh "job started" event, and a remote tab can bootstrap a job mid-retry. The authoritative duration the executor already computes was being discarded, so the timer showed blank on retry and roughly `00:00` on remote pre-attach.

### What changed

- The executor's authoritative job duration is now threaded into the UI and preferred over the locally tracked value, with the local start timestamp backfilled for coherence.
- Duration is carried on **all** terminal payloads — completed, failed, and cancelled — using a zero-guarded elapsed calculation (a job can be given up or canceled before any attempt starts).
- The remote run-job summary contract carries the duration too, with a start→terminal timestamp fallback for historical journals, so remote tabs that attach after completion show the correct elapsed time.

The result is a correct timer for every job across retry, failure, cancel, and remote-attach paths, with no persistence or schema migration required.
