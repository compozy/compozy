---
name: agh-contract-codegen-coship
description: Contract co-ship for AGH wire changes. Use when editing public DTOs, OpenAPI, JSON-RPC extension shapes, generated clients, or handler request/response semantics. Do not use for internal Go refactors or test-only changes that leave the wire contract unchanged.
trigger: implicit
---

# Contract Codegen Co-Ship

Ship one atomic wire-contract bundle. This file owns the sequence;
`references/coship-checklist.md` owns the inventory.

## Procedures

**Step 1: Detect the Trigger**

1. Inspect the staged or about-to-be-edited files. Triggers (any one match):
   - `internal/api/contract/**.go`
   - `internal/api/spec/**.go`
   - `openapi/agh.json`
   - `openapi/compozy-daemon.json`
   - `web/src/generated/**`
   - a public handler's request, response, status, authentication, or error shape changes even before a contract file is edited
2. If no path or semantic trigger fires, this skill does not apply.

*Done when:* every changed wire shape and owning source is named, or the skill is explicitly ruled out with evidence.

**Step 2: Plan the Co-Ship Bundle**

1. Read `.agents/skills/agh/agh-contract-codegen-coship/references/coship-checklist.md` in full.
2. Mark every applicable source, generated, backend, Web, site, CLI, UDS, native-tool, extension, config, QA, and official-skill item before editing.
3. Record explicit no-impact evidence for checklist branches that do not apply.

*Done when:* every applicable checklist item has one owning file or verification command and no public surface is left implicit.

**Step 3: Run Codegen Locally**

1. Execute `make codegen` from the repo root. This regenerates `openapi/agh.json` and `web/src/generated/agh-openapi.d.ts`.
2. If the contract is a JSON-RPC extension shape (not REST), the Go→TS generator also runs through `make codegen` (see `internal/codegen/openapits/generate.go`).
3. Inspect the generated diff. Generated files MUST commit alongside source.

*Done when:* generated artifacts are deterministic, reviewed, and synchronized with their owning source.

**Step 4: Update Web Consumers**

1. For each affected `web/src/systems/<system>/`:
   - Open `types.ts`. Remove DTOs that simply mirror the regenerated TypeScript types — import from `@/generated/agh-openapi` or the local re-export instead.
   - Open `adapters/`. Update the typed wrappers if the request/response shape changed.
   - Open `query-keys.ts` and `query-options.ts` if response shape affects keys.
   - Open `hooks/`. Update query/mutation hook return types and rollback logic.
   - Open `components/`. Update presentational props if exposed shape changed.
2. Update MSW fixtures and Storybook stories to match the new contract.

*Done when:* every affected consumer imports the canonical generated shape and its runtime/test fixtures agree.

**Step 5: Update Site Documentation**

1. CLI verb changes: regenerate `make cli-docs` so `packages/site/content/runtime/cli/` reflects the new flags / commands.
2. HTTP endpoint changes: update `packages/site/content/runtime/<area>/` MDX files and any protocol documentation.
3. Configuration key changes: update `packages/site/content/runtime/configuration/` MDX files.
4. Removed CLI verbs / endpoints / config keys: delete the doc pages in the same change (no "deprecated" markers).

*Done when:* public docs and agent-manageable surfaces describe only the shipped hard-cut contract.

**Step 6: Verify the Bundle**

1. Run `make codegen-check`. Output MUST be clean (no drift).
2. Run `make bun-lint` from the repository root.
3. Run `bunx turbo run typecheck test build --filter=./web` from the repository root when Web consumers are affected.
4. Run `bunx turbo run typecheck test build --filter=./packages/site` from the repository root when site content or generated inputs are affected.
5. Confirm every applicable co-ship checklist item passes.
6. Reserve the single full `make verify` for the completion gate after source freeze.

*Done when:* codegen has no drift, affected Turbo lanes are green, the checklist is complete, and exactly one fresh full monorepo gate is scheduled or complete for the finished task.

## Error Handling

- **`make codegen` shows large diffs unrelated to your change:** something else is out-of-date in the source tree. Investigate before committing — generated drift hides regressions.
- **`make codegen-check` fails after `make codegen`:** there's a non-deterministic generator. Inspect the generator's output ordering, file formatting, JSON normalization. Use semantic JSON comparison and run formatter before write/check (lesson `docs/_memory/analysis/analysis_local_runs.md` issue #6).
- **Web typecheck fails on imported generated type:** the consumer is mirroring instead of importing. Move it to use the generated type and delete the duplicate.
- **`packages/site` build breaks because a documented page references a removed verb:** delete the page; don't add a "see new verb" stub.
- **CI passes locally but fails on a runner:** Linux-race CI parity issue. Reproduce with `act workflow_dispatch -W .github/workflows/ci.yml -j verify --container-architecture linux/amd64`.
