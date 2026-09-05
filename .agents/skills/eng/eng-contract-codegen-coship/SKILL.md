---
name: eng-contract-codegen-coship
description: Contract co-ship for Compozy wire changes. Use when editing public DTOs, OpenAPI, JSON-RPC extension shapes, generated clients, or handler request/response semantics. Do not use for internal Go refactors or test-only changes that leave the wire contract unchanged.
trigger: implicit
---

# Contract Codegen Co-Ship

Ship changed wire contracts with their generated artifacts and affected consumers. Triggers include public DTOs under `internal/api/contract/` or `internal/api/spec/`, OpenAPI/generated clients, JSON-RPC extension shapes, and handler request/response/status/authentication/error semantics.

1. Identify the owning contract and compatibility regime. Preserve SD-013: public replacements auto-migrate losslessly where possible; otherwise retain the old shape for one release with warnings and a named removal release. Only documented experimental surfaces may skip that window. Internal consumers change together.
2. Use applicable sections of `references/coship-checklist.md` to trace consumers. Record impact once in the owning spec/task/PR using `docs/_memory/change-impact.md`; reuse existing decisions and group unaffected surfaces instead of producing a no-impact form per item.
3. Run root `make codegen` after changing the owning source. It also owns JSON-RPC Go-to-TypeScript generation. Inspect generated output and co-ship it with source; investigate unrelated drift or nondeterminism.
4. Update affected Web types, adapters, query keys/options, hooks, components, MSW fixtures, and stories. Import generated DTOs rather than maintaining copies. Read only the layers the changed shape reaches.
5. Update applicable site/protocol/config documentation and run `make cli-docs` for CLI changes. Document active migrations and deprecations; remove old public docs only when the corresponding surface is eligible for removal under SD-013.
6. Verify no generator drift with `make codegen-check` and use the affected root Turbo lint/typecheck/test/build lanes for Web/site changes. Reuse checks already covered by the root gate. Commit/push and PR completion follow root `make gate`/CI policy once at their delivery stage.

Complete when source, generated output, affected consumers, fixtures, docs, and required delivery evidence agree. A local task does not acquire a PR-publication requirement merely by loading this skill.

If a consumer typecheck fails, diagnose the actual mismatch; it does not automatically prove a mirrored DTO. For runner-only failures, inspect the failing CI lane and reproduce the relevant environment rather than assuming every failure is Linux race parity.
