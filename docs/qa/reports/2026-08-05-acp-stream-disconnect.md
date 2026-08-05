# QA Run Report — 2026-08-05 — ACP stream disconnect

- **Scope:** Issue #315 provider-process disconnect during final-response streaming
- **Cadence tier:** targeted
- **Build:** `codex/issue-315-acp-stream-disconnect` working tree · **Environment:** isolated bootstrap lab at `/Users/pedronauck/dev/qa-labs/compozy-acp-stream-disconnect-20260805-194318-831236-lab`; real daemon, SQLite, HTTP, UDS, CLI, and native-tool registry with a deterministic ACP subprocess fault fixture
- **Started:** 2026-08-05T16:43:18-03:00 · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | structured session operator | desktop / wifi-fast / en-US | CH-acp-stream-disconnect-recovery |

## Flows in Scope

- `J-15` — operate and recover a session consistently through CLI, HTTP, UDS, persisted history, and runtime diagnostics (`../journeys/J-15-operate-session-via-cli-api.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-acp-stream-disconnect-recovery | J-15 / RT-acp-stream-disconnect-recovery | Ada | Network Tour | Pass | #315 | working tree |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-acp-stream-disconnect-recovery — Ada

- **Ran:** 2026-08-05T16:46:23-03:00 → 2026-08-05T16:46:42-03:00 (box respected: yes)
- **Findings:** The HTTP stream preserved the first assistant chunk and emitted a terminal error after the ACP subprocess exited. Session detail and persisted history classified the failure as `process_exit`; crash bundle v2 recorded exit code 23. A different explicit prompt restarted the provider process in the same Compozy session, completed normally, and preserved both outputs without replay or duplication. The UDS-backed CLI JSONL path printed the partial chunk and error frame before returning a nonzero process status. `compozy__session_prompt` returned HTTP 502 with `tool_backend_failed` and `backend_dead`.
- **Bugs filed/updated:** none; this walk verifies issue #315's fix.
- **Scenarios settled:** RT-acp-stream-disconnect-recovery → pass.
- **Paper cuts:** none.
- **Surprises:** none.
- **Suggested next charter:** repeat against a future provider-owned deterministic Codex fault hook if Codex exposes one; a live provider cannot currently be ordered to disconnect at an exact byte boundary safely.

## What Was Fixed

### Issue #315: ACP disconnect loses the active session during final streaming

- **Symptom:** A Codex ACP peer disconnect after partial final output left the session stopped without useful process diagnostics, while CLI JSONL could return success at stream EOF.
- **Root cause:** Fatal prompt handling stopped the session process before its natural wait result was collected, and client stream consumers treated EOF as successful without requiring a terminal stream event.
- **Fix:** Preserve the natural process wait result behind a one-second exit boundary, capture exit code/signal/stderr in crash diagnostics, require terminal stream completion, return terminal errors to CLI and native-tool callers, and recover only after a new explicit prompt.
- **Regression test:** `internal/session/manager_prompt_contract_test.go`, `internal/subprocess/process_test.go`, `internal/cli/client_test.go`, `internal/cli/session_test.go`, `internal/daemon/native_tools_test.go`, and `internal/daemon/daemon_acpmock_faults_integration_test.go` failed on the old behavior and pass on this working tree.
- **Retested:** J-15 through HTTP, UDS-backed CLI JSONL, `compozy__session_prompt`, persisted transcript, runtime failure projection, crash bundle diagnostics, and same-session explicit recovery.

## Paper Cuts

None observed.

## Runtime Errors Observed

The intentionally injected ACP disconnect surfaced as a terminal stream error and `process_exit`; no unrelated runtime errors were observed.

## Human Verifications Needed

None planned.

## Decisions for a Human

None.

## Learnings

- The fault must be injected at the provider-process boundary to distinguish it from a client-only stream interruption.
- Automatic replay is not a recovery mechanism here: the provider may already have completed external side effects before disconnecting.
- Production-parity deviation: the walk uses a deterministic ACP subprocess fixture instead of live Codex so it can force an exact mid-stream exit. The daemon, process supervision, SQLite persistence, HTTP, UDS, and CLI paths are production implementations.

## Web/Docs Impact

- `web/`: no source change. Existing session runtime adapters and components already consume terminal `error` frames with `failure.kind: "process_exit"`; their canonical tests already cover the exact `peer disconnected before response` projection.
- `packages/site`: updated `content/docs/sessions/lifecycle.mdx` and `content/docs/guides/debug-a-failed-session.mdx` for partial-output retention, nonzero clients, diagnostics, and the explicit recovery boundary. Generated CLI docs are unchanged because no command or flag changed.
- Config lifecycle: no keys, defaults, overlays, validation, or examples changed.
- QA impact: added and walked `RT-acp-stream-disconnect-recovery` to `pass`.

## Compozy Impact Audit

- **Native tools:** `compozy__session_prompt` now returns `tool_backend_failed` with `backend_dead` when its drained prompt stream terminates with `process_exit`. Tool IDs, toolsets, descriptors, schemas, digests, risk flags, availability diagnostics, and capability gates are unchanged. Canonical native-tool and daemon integration suites cover the failure.
- **Extensibility and hooks:** the native registry and any hosted projection reuse the corrected call path. Extension manifests, contributed tools/resources, bridge SDKs, MCP sidecars, registries, and `session.post_stop` hook payloads are unchanged; the wire event and failure kinds already existed. No config lifecycle change.
- **Workspace data isolation:** exit code and signal are session-process facts written only to that session's owner-only crash bundle. Partial events remain in the existing workspace-scoped session store, and CLI/HTTP/UDS/native reads keep their existing workspace and session checks. No global cache, shared list datum, or cross-workspace event path was added.
- **Official Compozy skill:** updated `skills/compozy/references/runtime-operations.md` with persisted partial-output, nonzero CLI/native failures, diagnostics, and the no-automatic-replay boundary.

## Final Status

- **Exit gate (full automated suite):** PASS — `make gate-full`; Final verify evidence: `/Users/pedronauck/dev/qa-labs/compozy-acp-stream-disconnect-20260805-194318-831236-lab/qa-artifacts/qa/evidence/acp-stream-disconnect/final-make-verify.log`
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1 of 1 in-scope journey sessions complete; HTTP, UDS-backed CLI, native tool, and runtime/store diagnostics covered. Live Codex deterministic fault injection is disclosed above.
- **Verdict:** PASS — ready for review; the controlled provider-process disconnect is recoverable at the next explicit prompt and does not lose delivered output.
