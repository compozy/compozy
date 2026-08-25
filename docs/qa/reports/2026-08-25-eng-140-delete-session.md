# QA Run Report — 2026-08-25 — ENG-140 session deletion windows

- **Scope:** End-to-end deletion of a session through the public HTTP and UDS surfaces, with verification that matching session windows close and the session cannot be reopened. The Web row-action walk was attempted separately.
- **Cadence tier:** targeted
- **Build:** working tree on `linear-eng-140`; direct Go binary built with `go build -o /tmp/compozy-eng140 ./cmd/compozy`
- **Environment:** fresh isolated QA lab `/Users/pedronauck/dev/qa-labs/compozy-eng-140-delete-session-20260825-003852-605532-lab`; manifest `/Users/pedronauck/dev/qa-labs/compozy-eng-140-delete-session-20260825-003852-605532-lab/qa-artifacts/qa/bootstrap-manifest.json`
- **Started:** 2026-08-25T00:38:52-03:00 · **Status:** closed with Web verification blocked

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Surface | Status | Evidence |
|---|---|---|---|---|---|---|
| 1 | CH-untested-016-15-theo | J-15 / RT-014 | Théo | HTTP API | Pass | `qa/logs/http-delete.txt` |
| 2 | CH-untested-016-15-theo | J-15 / RT-014 | Théo | UDS CLI | Pass | `qa/logs/uds-delete.txt` |
| 3 | CH-untested-016-15-theo | J-15 / RT-014 | Théo | Web | Blocked (needs human verify) | `qa/logs/web-blocker.txt` |

## Session Debriefs

- **HTTP:** Created an idle session and a public `app=session` window with the exact session id as `instance_key`. `DELETE /api/workspaces/ws_0d30ade5ff6eea76/sessions/sess-b11b8c63ba0e1eec` returned 204; an independent session read returned 404 and the public window list was empty.
- **UDS:** Created a second session and matching window. `compozy session remove sess-be2aa05b1f5b9735 --json` completed through UDS; independent window and session catalog reads showed no remaining window and `total=0` sessions.
- **Web:** `COMPOZY_WEB_API_PROXY_TARGET` was derived from the manifest, but `make web-dev` stopped in codegen because the cached `openapi-typescript` transform could not resolve the `typescript` package. No browser pass is claimed.

## What Was Fixed

- Session deletion now invokes the owning workspace window-manager stream only after catalog deletion succeeds.
- The reducer removes every exact `app=session` / `instance_key=session_id` placement, including tiled, floating, stacked, minimized, pinned, and cross-desktop placements, while preserving unrelated windows and an empty desktop.
- Reconciliation uses the existing close event/hook path, clears undo/redo resurrection history, retries durable tombstones with bounded attempts, and waits for hook readiness during boot.

## Runtime Errors Observed

No unexpected daemon error occurred in the HTTP or UDS walks. The Web startup blocker was the missing `typescript` package resolution described above.

## Human Verifications Needed

Re-run RT-014's Web row-menu confirmation/cancel path after the repository's Bun codegen dependency is restored. The full automated exit gate was intentionally not run because the ENG-140 request forbids `make gate-full` and `make verify`.

## Evidence

- HTTP: `/Users/pedronauck/dev/qa-labs/compozy-eng-140-delete-session-20260825-003852-605532-lab/qa-artifacts/qa/logs/http-delete.txt`
- UDS: `/Users/pedronauck/dev/qa-labs/compozy-eng-140-delete-session-20260825-003852-605532-lab/qa-artifacts/qa/logs/uds-delete.txt`
- Web blocker: `/Users/pedronauck/dev/qa-labs/compozy-eng-140-delete-session-20260825-003852-605532-lab/qa-artifacts/qa/logs/web-blocker.txt`
- Journey log: `/Users/pedronauck/dev/qa-labs/compozy-eng-140-delete-session-20260825-003852-605532-lab/qa-artifacts/qa/journey-log.jsonl`
- Teardown: `/Users/pedronauck/dev/qa-labs/compozy-eng-140-delete-session-20260825-003852-605532-lab/qa-artifacts/qa/teardown.json` (`clean: true`, no survivors)

## Compozy Impact Audit

- **Native tools:** No `compozy__*` tool IDs, toolsets, descriptors, schemas, digests, risk flags, diagnostics, or capability gates changed; the existing window command path is reused.
- **Extensibility and hooks:** No extension, registry, skill, resource, MCP, bridge, or `config.toml` surface changed. The existing `window.close` hook path remains observable, including after startup retry readiness.
- **Workspace data isolation:** The changed datum is a workspace/profile-scoped window aggregate. Session deletion passes the stored profile and workspace ids; the adapter resolves that profile and the reducer targets only the owning workspace. Tests cover another workspace and unrelated session/app identities.
- **Official Compozy skill:** No public tool id, CLI path, hook event, capability, extension resource, or memory/network/task semantic changed; `skills/compozy/` was checked and needs no update.

## Final Status

- **QA exit gate:** HTTP and UDS deletion walks passed; Web is blocked by the missing Bun dependency; full gate evidence is intentionally absent per the user instruction.
- **Coverage:** 2 of 3 required surfaces walked; 1 surface blocked with a concrete environment failure.
- **Verdict:** BLOCKED — production implementation and focused runtime checks are ready, but RT-014 remains `blocked-verify` until Web verification can run.
