# Workflow Memory

Keep only durable, cross-task context here. Do not duplicate facts that are obvious from the repository, PRD documents, or git history.

## Current State
- Task 02 (Phase 1: File-level splits) is complete with all required same-package file reorganizations verified by `make verify`.
- Task 03 (Phase 2: Domain restructuring) is complete with `prompt` reduced to prompt-building concerns, parsing ownership moved into `tasks` and `reviews`, and the full repository gate passing.
- Task 05 (Phase 4: DRY and generics consolidation) is complete with the shared content-block engine, generic CLI/kernel/setup helpers, runtime-config chain collapse, ACP parameter objects, and a clean `make verify`.

## Shared Decisions
- Task and review parsing now belong to `internal/core/tasks` and `internal/core/reviews`; future package work should not move that logic back into `prompt`.
- Default review-provider registry composition now lives in `internal/core/providerdefaults` rather than a generic `providers` package.
- Preparation journal shutdown now belongs to `internal/core/plan` via `ClosePreparationJournal`.
- Task 04 established `internal/core/run/ui` as the owner of the Bubble Tea TUI, event adapter, and validation-form code; remaining `run` callers should depend on it only through narrow bridges or shared interfaces.
- Task 04 established `internal/core/run/{exec,executor,transcript}` plus `internal/core/contentconv` as the runtime ownership boundary; future runtime changes should land in those focused packages instead of re-expanding root `run`.
- Task 04 also established `internal/core/migration` as the owner of workflow artifact migration logic; root `core` keeps only a thin forwarding shim.
- Task 04 introduced `internal/core/run/internal/{runshared,acpshared,runtimeevents}` as the shared internal layer that prevents `run -> subpackage -> run` cycles while keeping root `run` as a compatibility facade for CLI-facing preflight and persisted-exec helpers.
- Task 05 established `internal/contentblock` as the single owner of shared content-block encode/decode/validation logic for both internal runtime models and public session event payloads.
- Task 05 collapsed the kernel runtime-config chain onto `model.RuntimeConfig`; future run/prepare command changes should update `core.Config.RuntimeConfig()` and the command-local runtime helpers instead of reintroducing intermediary runtime field structs.
- Task 05 converted `internal/setup` agent support to declarative path specs plus shared `selectByName` selection; future agent additions should extend the declarative table, not reintroduce closure-based specs.
- Task 05 replaced ACP session setup/update high-arity helpers with request/config structs in `internal/core/run/internal/acpshared`; future runtime plumbing changes should extend those parameter objects rather than adding new positional parameters.

## Shared Learnings
- Incremental package-scoped verification caught split-induced import drift quickly; the final Task 02 tree passes `fmt`, `lint`, `test`, and `build`.
- The Phase 1 split boundaries are now established across `internal/core/model`, `internal/cli`, `internal/core/run`, `internal/core/agent`, `internal/core/workspace`, `pkg/compozy/events/kinds`, and `pkg/compozy/runs`.
- Trying to move `DefaultRegistry` into `internal/core/provider` creates an import cycle because `provider/coderabbit` already depends on the base `provider` package; keep that composition in a separate package.
- The package split uncovered a genuine race in tests that swap process stdio; the durable fix is to serialize the small set of stdio-mutating tests instead of relying on package timing.
- Generic helpers remained easiest to land when the behavior-sensitive edges stayed explicit: `[]string` cloning is opt-in via `applyConfig` transforms, and setup selector error phrasing stayed aligned with the pre-refactor wording.

## Open Risks
- Later runtime cleanup should continue respecting the `run/internal/*` layering and the new ACP request/config objects so the root `run` facade does not regrow positional setup logic.

## Handoffs
