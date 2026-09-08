# L-029 — QA labs must tear down processes, not just isolate them

**Class:** Workflow / QA hygiene
**Date discovered:** 2026-07-09 (operator report: machine freezing under accumulated dead QA processes)
**Evidence sources:** orphaned lab roots on the operator machine + skill-text audit

## Context

Task-driven QA runs (`.compozy/tasks/*` executions using `eng-qa-bootstrap` → `eng-real-scenario-qa` / `qa-execution`) repeatedly left long-lived processes behind after the QA verdict was written: isolated `compozy` daemons (each supervising ACP agent subprocesses and MCP sidecars), tmux servers on lab sockets, `make web-dev` Vite servers, `observe-runtime.py` watchers (30-minute windows), and browser sessions. Accumulated across cycles, they froze the operator's machine (2026-07-09). After a reboot, three orphaned short runtime roots were still present at `$TMPDIR/compozyqa-*` (loops-refac task-16 cycle and earlier), and six stale labs were discoverable under `~/dev/qa-labs/compozy-*-lab/`.

## Root cause

The QA toolchain solved **state isolation** (L-009: unique `COMPOZY_HOME`, ports, tmux sockets) but never defined **process lifecycle ownership**. No skill step, script, or make target stopped what a QA pass started:

- `bootstrap-qa-env.py` provisioned labs but had no teardown counterpart; the manifest recorded paths and ports but no PIDs.
- `eng-worktree-isolation` Step 6 explicitly instructed leaving the lab "in place for forensic inspection" — conflating file forensics (harmless) with live processes (harmful).
- `eng-real-scenario-qa` and `qa-execution` completion checklists gated on evidence and reports, never on process hygiene; `cy-final-verify` gates on `make verify`, which cannot see orphaned daemons.

Isolation without teardown converts every QA pass into permanent background load: each lab's daemon keeps its supervision loops, SQLite handles, and subprocess trees alive indefinitely.

## Rule

> Every QA lab or isolated runtime envelope ends with a process teardown on every terminal path (pass, fail, blocked, abort). Files may stay for forensics; processes never do. A QA/task completion claim with orphaned lab processes is a blocking failure.

## Operationalization

- Register long-lived lab processes in `<QA_OUTPUT_PATH>/qa/pids/` when spawning them.
- On every terminal path, run the bootstrap manifest's `TEARDOWN_COMMAND` or `make qa-reap`; retain `<QA_OUTPUT_PATH>/qa/teardown.json` with `clean: true`.
- `eng-qa-bootstrap` and its teardown helper own stop ordering, survivor detection, and signal escalation. Follow those current mechanics rather than duplicating them here.
- An active loop may retain its lab; the owner ending it performs teardown. Reading this lesson alone does not require launching a lab.

## Detection signals

- Mounting fan noise / memory pressure after QA cycles; `ps` showing `compozy daemon`, tmux servers on non-default sockets, or Vite/watcher processes with lab paths in their command lines.
- `$TMPDIR/compozyqa-*` or `~/dev/qa-labs/compozy-*-lab/` roots with `daemon.lock` PIDs that are still alive.
- A QA report whose manifest lacks a `teardown` block.

## Source

- Operator incident report, 2026-07-09 (machine freezes attributed to accumulated dead QA processes).
- Orphaned roots observed post-reboot: `$TMPDIR/compozyqa-6873f014d4f1`, `compozyqa-07cd636345d5`, `compozyqa-a80910e5fc8a`; six stale labs under `~/dev/qa-labs/`.
- Pre-fix skill text: `eng-worktree-isolation` Step 6 ("left in place for forensic inspection"); absence of any teardown step in `eng-qa-bootstrap`, `eng-real-scenario-qa`, `qa-execution`.
- Fix: `teardown-qa-env.py`, `make qa-reap`, `TEARDOWN_COMMAND` contract, and skill updates (this change).
- Related: [L-009](L-009-concurrent-worktree-deadlock.md) (state isolation — the half that existed).
