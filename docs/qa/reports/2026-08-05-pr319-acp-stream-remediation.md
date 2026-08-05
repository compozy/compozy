# QA Run Report — 2026-08-05 — PR #319 ACP stream remediation

- **Scope:** Review remediation for issue #315 and PR #319
- **Cadence tier:** targeted
- **Build:** `codex/issue-315-acp-stream-disconnect` working tree
- **Environment:** isolated bootstrap lab at `/Users/pedronauck/dev/qa-labs/compozy-pr319-acp-stream-remediation-20260805-220911-045783-lab`
- **Started:** 2026-08-05T22:09:11-03:00
- **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | structured session operator | desktop / wifi-fast / en-US | CH-acp-stream-disconnect-recovery |

## Flows in Scope

- `J-15` — operate and recover a session consistently through CLI, HTTP, UDS, persisted history, and runtime diagnostics.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-acp-stream-disconnect-recovery | J-15 / RT-acp-stream-disconnect-recovery | Ada | Network Tour | Pass | #315 | PR #319 remediation batch |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-acp-stream-disconnect-recovery — Ada

- **Ran:** 2026-08-05T21:55:57-03:00 → 2026-08-05T21:56:14-03:00.
- **Observed:** The real daemon and ACP subprocess fixture preserved the partial HTTP response and emitted one typed `process_exit` terminal with exit code 23. The event-linked crash bundle already contained the exit code and stderr when the event was read. CLI JSONL printed the retained frames and exited nonzero; the native prompt returned `tool_backend_failed`/`backend_dead`.
- **Recovery:** The failed session stopped within the scenario bound, then a new explicit prompt restarted the same session without replaying the failed turn and completed normally.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-pr319-acp-stream-remediation-20260805-220911-045783-lab/qa-artifacts/qa/evidence/acp-stream-remediation/runtime-public-surfaces.log`.
- **Result:** `RT-acp-stream-disconnect-recovery` passed across HTTP/SSE, CLI JSONL, UDS/native, persistence, process diagnostics, and explicit recovery.

## What Was Fixed

- Terminal-less ACP stream closure is now a typed transport failure in the session owner. It persists already delivered chunks, emits an Error terminal, and stops the unusable runtime instead of treating EOF as completion.
- HTTP/SSE, CLI, and native-tool consumers independently reject terminal-less EOF. Malformed SSE JSON keeps its decode cause instead of being replaced by a generic incomplete-stream error.
- Fatal process and transport failures reuse the normal session-stop preparation, including lifecycle hooks and prompt setup cleanup. The manager then stops and waits for the process, writes the status-complete crash bundle, and only then persists and delivers the public Error event. Final classification reuses that same path, so clients never observe an event-linked bundle before known exit evidence is present.
- Daytona forwards the remote exit code. Signal exits omit Unix's `-1` sentinel while preserving the signal; Windows-style numeric exits remain available. Signal extraction is limited to the published Darwin/Linux Unix targets, with a no-signal fallback for other Go targets.

## Paper Cuts

The QA tracker originally recorded PASS even though the native assertion failed. This report now treats
the executable assertion as canonical. The corrected retest replaced that stale evidence and passed.

## Runtime Errors Observed

None in the corrected walk. One discarded harness attempt used an overlong temporary path and could
not create the daemon socket; it never entered the product scenario and left no surviving process.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- The public Error event and final classification must share one deterministic bundle path. Process evidence must be complete before the event becomes observable; an eventual rewrite is too late for clients that open the path immediately.
- A fatal prompt failure must enter the same pre-stop, stopping, cleanup, and post-stop lifecycle as an operator or daemon stop. The process-evidence barrier may own finalization, but it cannot bypass lifecycle preparation.
- A `[DONE]` sentinel proves transport termination, not successful ACP completion. Consumers must also observe a semantic Done or Error terminal.
- A deterministic subprocess fixture remains necessary because a live provider cannot safely be instructed to disconnect at an exact response byte boundary.

## Web/Docs Impact

- `web/`: no source impact. The existing adapters already consume terminal error streams; the remediation changes no UI contract or copy.
- `packages/site`: no new documentation contract. The PR's existing lifecycle and debugging documentation already describes partial-output retention, explicit recovery, and process diagnostics.
- Config lifecycle: no keys, defaults, overlays, validation, or examples changed.
- QA impact: the existing scenario passed its corrected real-subprocess retest.

## Compozy Impact Audit

- **Native tools:** `compozy__session_prompt` retains its ID and schema. Its consumer defense rejects empty or partial EOF as a clear backend failure, while a proven subprocess exit projects `tool_backend_failed`/`backend_dead`. The canonical native-tool suite and real daemon integration cover both failure modes.
- **Extensibility and hooks:** no tool descriptors, capability gates, extension resources, bridge SDKs, MCP sidecars, hook payloads, registries, or config lifecycle changed. Hosted/native projections reuse the same corrected session stream.
- **Workspace data isolation:** partial events and crash evidence remain session-scoped inside the existing workspace-owned store and owner-only log directory. HTTP, UDS, CLI, SSE, and native paths keep their existing workspace/session authorization and add no global cache or cross-workspace datum.
- **Official Compozy skill:** no additional change. The PR already documents partial-output retention, process diagnostics, and explicit no-replay recovery in `skills/compozy/references/runtime-operations.md`; this batch closes implementation gaps without changing that public contract.

## Final Status

- **Behavioral verdict:** PASS — the real ACP exit-23 path satisfies the public-surface and recovery contract.
- **Focused regression suite:** 31 race-enabled session tests passed, including explicit prompt cancellation, terminal-less EOF, process exit, lifecycle hooks, and crash-bundle invariants.
- **Strict QA audit:** the content-frozen report, journey log, runtime transcript, final gate record, and teardown record are audited together before publication.
- **Teardown:** the isolated lab is required to finish with `teardown.json` reporting `"clean": true` and no surviving process or socket.
- **Automated workstream gate:** the content-fingerprinted `make gate-full` record is produced after this report's final mutation and cited in the PR evidence.
- **Readiness:** ready when the strict audit and current full-gate record both pass.
