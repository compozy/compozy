Goal (incl. success criteria):

- Review `.compozy/tasks/extensibility/_techspec.md` and all ADRs under `.compozy/tasks/extensibility/adrs/` to identify architectural gaps, design smells, inconsistencies, and concrete improvement opportunities.
- Success means producing a review with severity-ranked findings backed by exact document references, plus open questions where the design is still under-specified.

Constraints/Assumptions:

- Follow `AGENTS.md` and `CLAUDE.md` instructions for this repository.
- Scope stayed at design/spec level; no production code changes in this session.
- Required skills in use: `architectural-analysis`, `brainstorming`, `cy-create-techspec`, `cy-final-verify`.
- Existing ledgers are read-only context from other sessions; do not modify them.

Key decisions:

- Treat the tech spec and ADRs as the primary source of truth and evaluate cross-document consistency, operational completeness, rollout safety, and testability.
- Focus findings on architecture-level risks, not copy edits.
- Keep the feature scope intact while fixing coherence issues by reshaping lifecycle, enablement, and integration semantics rather than removing capabilities.
- Model `exec` extensibility as explicit operator opt-in via `--extensions`.
- Keep skill packs in scope, but deliver them through `internal/setup` and agent preflight rather than a host-only runtime merge.
- Keep provider registration in scope, but realize it as command-scoped registry overlays loaded from manifests without requiring subprocess startup.
- Align Host API memory operations to Compozy's actual file-backed memory model and task creation to a typed core service instead of a nonexistent CLI command.

State:

- Completed after fresh `make verify`.

Done:

- Read repository instructions and scanned existing ledgers for cross-agent awareness.
- Loaded the `architectural-analysis` skill to structure the audit.
- Located the extensibility tech spec and ADR set under `.compozy/tasks/extensibility/`.
- Read the full tech spec and ADR-001 through ADR-007.
- Cross-checked the proposed design against current code in `internal/core/plan`, `internal/core/run/{executor,exec}`, `internal/core/memory`, `internal/core/provider`, `internal/core/agent`, `internal/cli/skills_preflight.go`, `skills/embed.go`, and `pkg/compozy/events`.
- Identified major contradictions around lifecycle timing, exec-path coverage, skill-pack delivery, and Host API assumptions versus current surfaces.
- Delivered the review to the user with severity-ranked findings and concrete fixes.
- Updated `.compozy/tasks/extensibility/_techspec.md` to:
  - move extension bootstrap earlier than planning
  - add `exec --extensions` opt-in semantics
  - route skill packs through setup/preflight instead of runtime-only merge
  - align Host API task/memory semantics to the current codebase
  - introduce provider overlay resolution for command bootstrap
  - document operator-local enablement and best-effort observer delivery
- Updated ADR-001, ADR-002, ADR-003, ADR-004, ADR-005, ADR-006, and ADR-007 for consistency with the revised technical approach.
- Ran fresh repository verification successfully with `make verify`.

Now:

- Prepare the final handoff with file references and verification evidence.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None currently blocking.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-10-MEMORY-extensibility-review.md`
- `.compozy/tasks/extensibility/_techspec.md`
- `.compozy/tasks/extensibility/adrs/adr-001.md`
- `.compozy/tasks/extensibility/adrs/adr-002.md`
- `.compozy/tasks/extensibility/adrs/adr-003.md`
- `.compozy/tasks/extensibility/adrs/adr-004.md`
- `.compozy/tasks/extensibility/adrs/adr-005.md`
- `.compozy/tasks/extensibility/adrs/adr-006.md`
- `.compozy/tasks/extensibility/adrs/adr-007.md`
- `internal/core/plan/prepare.go`
- `internal/core/run/executor/execution.go`
- `internal/core/run/exec/exec.go`
- `internal/core/run/run.go`
- `internal/core/memory/store.go`
- `internal/core/provider/registry.go`
- `internal/core/agent/registry_specs.go`
- `internal/cli/root.go`
- `internal/cli/commands.go`
- `internal/cli/skills_preflight.go`
- `pkg/compozy/events/bus.go`
- `skills/embed.go`
- Verification: `make verify`
- Commands: `find`, `sed`, `nl`, `rg`, `make verify`
