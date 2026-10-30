# QA Run Report — 2026-10-29 — session-processes

- **Scope:** process discovery, ownership, explicit cleanup, keep across restart, and deleted-owner cleanup.
- **Cadence tier:** targeted. **Runtime verdict:** PASS.
- **Build:** feat/session-processes; backend working tree plus frontend through c984a2688.
- **Environment:** isolated lab `session-processes-20261029-234903-302882`, backend port 62890, Web port 62891.
- **Walk:** 2026-10-30T00:08:01Z–00:27:29Z. Teardown reports `clean: true` with no survivors.
- **Parity:** acpmock/launcher authorized by the accepted brief; OS processes, daemon, SQLite, CLI and API are real. This is not live-LLM provider proof. Broad gates and E2E belong to CI.

## Personas and flows

Théo (Power User), desktop, local network, en-US. Journey `J-session-processes`, charter
`CH-session-processes`. The fixture supplies deterministic ACP behavior and launches ordinary
OS commands; discovery and termination use production code.

## Session matrix

| Leg | Result | Evidence under lab qa-artifacts/qa |
|---|---|---|
| Inspect and terminate | PASS: protected sleep attributed by lineage; Node by env; port 50619; stop with terminate removed both identities and closed the port | process-walk.jsonl |
| Keep and restart | PASS: sleep 7962 and Node 7967 retained the same start times after daemon restart; HTTP server on 50701 still returned 200 | server-before-restart.json, server-after-restart.json, process-walk.jsonl |
| Orphan cleanup | PASS: aggregate returned both retained identities; explicit termination returned terminated; both PIDs exited and port 50701 refused connections | kept-processes-terminated.json |
| Deleted owner | PASS: deleting sess-9f0384be435dbffb with keep retained sleep 3257 and Node 3270 in the aggregate; aggregate termination verified exit and closed port 50399 | deleted-owner-processes.json, deleted-owner-terminated.json |
| Native terminal shutdown | PASS: public CLI created term-aa2a4f73557c; OS identity captured PID 52240, parent 32369; graceful lab daemon stop removed that PID | terminal-shutdown-verified.json |
| Web | PASS for live chip, popover, port, lineage hint, and stop choices in both themes; no page errors reported by F | /tmp/session-processes-shots/live-chip-*.png, live-popover-*.png, live-stop-dialog-*.png |
| Teardown | PASS: manifest-scoped cleanup, no surviving lab processes, no operator runtime mutation | teardown.json |

The live Web walk inspected the dialog and canceled it; API/CLI performed the destructive lifecycle
legs. Archive/delete and left-running visual states were captured in Storybook. Full Web end-to-end
execution, including `E2E-PROC-001`, is a CI gate and is not claimed as locally run.

## Focused engineering evidence

- `go test -race ./internal/procutil ./internal/sessionprocess ./internal/tools/builtin`: owning suites passed (separate final invocations after corrections). Exact identity, reused groups, startup protection, real env attribution and durable protected-binary lineage/reload are covered.
- Focused API/CLI/daemon checks passed, including policy body/query encoding and process ownership. The final UDS regex selected no tests; an earlier targeted UDS run passed. No claim of full UDS suite execution.
- Migration streams/equivalence and OpenAPI contract checks passed. Linux and Windows cross-builds passed; Windows is explicitly unsupported for process inspection.
- `make codegen-check` passed after regenerating the embedded linter with the pinned Bun 1.4.0 binary. No dependency update.
- Scoped Go lint reported zero issues. Test-shape checks found no added findings; existing findings in older suites were left untouched. The new lineage suite handles cleanup errors explicitly.
- The procutil niceness helper had competing cancellation and explicit cleanup owners. Its context now leaves process cleanup to the existing explicit owner; the full race suite passed without weakening assertions.
- F: root Turbo Web typecheck and focused Vitest passed (824 tests in the adapter-inclusive run), process controls/lib rerun 19/19, oxlint clean, React Doctor 100/100 before the final small adapter/display fixes.
- No local `make gate` or full E2E run: the accepted delivery brief assigns them to CI.

## Measured scan cost

Apple M4 Max, approximately 1,145 OS processes: tree scan 0.171 ms wall / 0.253 ms CPU;
environment scan 8.245 ms wall / 9.267 ms CPU. At 250 ms and 5 s respectively, these correspond
to approximately 0.101% and 0.185% of one core. These are scanner microbenchmarks, excluding
port/CWD enrichment and the service owner lookup; they are not total daemon CPU measurements.

## Findings and limitations

macOS protected binaries that detach and change PGID before the first observation cannot be
attributed. Daemon shutdown preserves detached descendants and closes native terminals; PTY
restart survival is out of scope. Legacy unknown markers cannot establish profile ownership.

Review repaired empty-workspace User-scope queries and aggregate batches above 100 targets.
Command display uses case-sensitive path matching for Linux and case-sensitive APFS safety.

F observed an existing light-theme destructive Button contrast issue in Storybook. No tokens were
changed. Initial live popovers predate the argv enhancement. Later argv captures predate removal
of case-folding and must not be presented as the exact final row label; use the unchanged chip/stop
dialog and Storybook row layout as delivery evidence. Vite did not pick up two module edits, so F
explicitly refreshed those modules for the later captures.

## Final status

PASS for the bounded runtime walk. The strict evidence audit passed with zero blockers, and manifest-scoped teardown
reported clean with no survivors. Both artifacts are recorded with the lab. PR readiness remains contingent on current-head CI and review completion.
