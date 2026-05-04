# Task Memory: task_02.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Establish the daemon-web browser contract artifact and deterministic typed-client codegen flow in the current `agh` repo without expanding into later handler implementation work.
- Pre-change signal: the repo only has the existing AGH-wide contract (`openapi/agh.json` -> `web/src/generated/agh-openapi.d.ts`), so task 02 is not complete until the parallel daemon-web artifact/types/client surface exists and is checked by automation.

## Important Decisions
- Add a parallel `compozy-daemon` OpenAPI/codegen surface instead of replacing the existing `agh` contract, because the current web app still imports `agh-openapi` and the daemon-web endpoints from the techspec are not implemented in this repo yet.
- Keep the root `codegen` / `codegen-check` entrypoints and extend them to cover the new daemon-web types so later tasks get one deterministic workflow rather than package-specific ad hoc commands.
- Refactor `cmd/agh-codegen` to route through an internal `runWithPaths(...)` helper so temp-path tests can verify the real CLI behavior without mutating checked-in generator outputs.

## Learnings
- The current repo already has deterministic OpenAPI/type generation through `cmd/agh-codegen`, `magefile.go`, `Makefile`, and root/web `codegen` scripts; task 02 should extend that path instead of inventing a second workflow.
- `web/src/lib/api-client.ts` and `web/src/lib/api-contract.ts` are shared AGH helper surfaces today, so daemon-web support needs additive exports to avoid breaking the existing app.
- The existing root and `web` package scripts already exposed the required `codegen` / `codegen-check` entrypoints; task 02 only needed to widen the shared generation/check implementation and add coverage/assertions around those scripts.
- Task-specific coverage now meets the requirement: `internal/codegen/openapits` 80.0%, `internal/e2elane` 91.7%, `cmd/agh-codegen` 80.4%, and `web/src/lib/api-client.ts` 100% in the targeted Vitest coverage run.

## Files / Surfaces
- `cmd/agh-codegen/main.go`
- `cmd/agh-codegen/main_test.go`
- `openapi/compozy-daemon.json`
- `magefile.go`
- `internal/codegen/openapits/generate.go`
- `internal/codegen/openapits/generate_test.go`
- `internal/e2elane/command_wiring_test.go`
- `turbo.json`
- `web/src/generated/compozy-openapi.d.ts`
- `web/src/lib/api-client.ts`
- `web/src/lib/api-contract.ts`
- `web/src/lib/api-client.test.ts`
- `web/src/lib/daemon-api-contract.test.ts`

## Errors / Corrections
- The task docs describe future daemon-web endpoints that are not present in the current repo yet. Treat task 02 as contract/codegen groundwork only and keep the new surface additive so existing AGH routes and consumers remain stable.

## Ready for Next Run
- Completed. Later daemon-web tasks can consume `daemonApiClient`, daemon operation helper types, and the checked-in `compozy-daemon` artifact through the shared root/web `codegen` and `codegen-check` scripts without adding another generation path.
