# QA Run Report — 2026-09-01 — Loop route selection

- **Scope:** Honor same-pass route skips before evaluating later coordinator-owned gates or actions.
- **Cadence tier:** targeted
- **Base:** `34208e9990622ee62e9a5cf114386273ae6abfa0` · **Build:** `304059507bbeff0213b1d516cccbd5be7939bb03` · **Environment:** isolated integration harness using the real CLI, HTTP API, UDS, and Loop runtime with deterministic ACP fixtures
- **Started:** 2026-09-01T20:00:00Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Developer/operator | Local isolated runtime / en-US | Exclusive route history |

## Flows in Scope

- Exclusive route selection and durable route history.

## Session Matrix & Results

| # | Journey / Scenario | Persona | Status | Issue | Fix commit |
|---|---|---|---|---|---|
| 1 | `LP-exclusive-route-history` | Bruno | Fixed | [`BUG-20260901-unselected-route-gate-executes`](../bugs/BUG-20260901-unselected-route-gate-executes.md) | `30405950` |

## Session Debriefs

- The owning regression evaluates a selected `c_verify` gate before a lexically later unselected `z_reject` gate. Only `c_verify` ran, only the selected action was queued, and `z_reject` settled `route_not_taken`.
- The public route-history integration journey created and ran the Loop through HTTP, read it through CLI over UDS, observed no verdict for the unselected gate, preserved its `route_not_taken` output, and settled `done`.
- The rest of `TestDaemonE2ELoopGenerationFeedbackShouldConvergeAndBound` ran as the adjacent runtime canary. The harness owned process cleanup and exited without a persistent QA lab or daemon.

## What Was Fixed

- **Root cause:** the planner iterated stale cloned generation outputs after route selection had marked canonical outputs skipped.
- **Fix:** reload each candidate from canonical state before deciding whether it remains pending.
- **Regression:** `TestCoordinatorRunnerShouldNotEvaluateUnselectedRouteGate`; the daemon route-history integration case now proves the same gate shape across HTTP and CLI/UDS.

## Runtime Errors Observed

- None.

## Verification Refresh — 2026-09-02

- `CGO_ENABLED=1 go test -race ./internal/loop/... -run '^TestCoordinatorRunnerShouldNotEvaluateUnselectedRouteGate$' -count=1` — pass.
- `CGO_ENABLED=1 go test -race -tags=integration ./internal/daemon -run '^TestDaemonE2ELoopGenerationFeedbackShouldConvergeAndBound$' -count=1` — pass in 46.781 seconds; the route-history journey and its adjacent generation-feedback canaries all passed.
- `bunx turbo run generate:content --filter=./packages/site` and `bunx turbo run typecheck --filter=./packages/site` — pass.
- `bunx turbo run build --filter=./packages/site` — pass and prerendered `/docs/loops/dsl-reference`; it also repeated an existing Turbopack filesystem-tracing warning from `packages/site/lib/marketplace-bridges.ts`, outside this route change.
- Public-path boundary: this refresh started the run over HTTP and read it independently through the CLI over UDS. Web and native-tool Loop status consume the unchanged run-detail contract; their adapters and schemas did not change in this fix.

## Human Verifications Needed

- None.

## Final Status

- **Exit gate:** focused race, full daemon feedback integration, Fumadocs generation, site typecheck, site build, and the final scoped `make gate` passed.
- **Issues:** Blocks-Completion 1 fixed and verified · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 targeted Loop route journey.
- **Verdict:** ready pending exact-head provider CI.
