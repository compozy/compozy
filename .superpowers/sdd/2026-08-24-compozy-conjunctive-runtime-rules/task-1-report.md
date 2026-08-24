# Task 1 Report: Validate the Conjunctive Selector Grammar

## Implementation

- Updated `internal/loop/runtime_validation.go` so runtime selectors accept exactly `id`, `type`, `complexity`, or `type + complexity`.
- Kept `id` exclusive: `id + type`, `id + complexity`, and all three fields return `selector_collision`.
- Empty selectors still return `selector_required`; existing unknown-field, empty-runtime, model, reasoning, and speed validation remains unchanged.
- Added `TestValidateRuntimeRulesShouldAcceptTypeComplexityConjunction` in `internal/loop/runtime_validation_test.go` with table coverage for all accepted and rejected shapes.

## Tests and evidence

RED (pre-production change, selector-count implementation):

```text
rtk env TMPDIR="$(rtk mktemp -d -p /home/francisross/tmp-builds batuta-task1-red-exact.XXXXXX)" go test ./internal/loop -run TestValidateRuntimeRulesShouldAcceptTypeComplexityConjunction -count=1
--- FAIL: TestValidateRuntimeRulesShouldAcceptTypeComplexityConjunction
    runtime_validation_test.go:280: ValidateDefinitionRuntime() error = loop: runtime validation failed for runtime_rules[0].match="": selector_collision
FAIL
```

GREEN:

```text
rtk env TMPDIR="$(rtk mktemp -d -p /home/francisross/tmp-builds batuta-task1-green-exact.XXXXXX)" go test ./internal/loop -run TestValidateRuntimeRulesShouldAcceptTypeComplexityConjunction -count=1
ok   github.com/compozy/compozy/internal/loop  0.009s

rtk env TMPDIR="$(rtk mktemp -d -p /home/francisross/tmp-builds batuta-task1-package-final.XXXXXX)" go test ./internal/loop -count=1
ok   github.com/compozy/compozy/internal/loop  12.730s
```

`rtk git diff --check` also passed before commit.

## Self-review

- Diff is limited to the two requested files.
- Public wire shape is unchanged; no new fields or compatibility aliases were added.
- All new tests use `Should ...` names and `t.Parallel()` where safe.
- Commit: `ba17822 feat: allow domain complexity matches`.

## Concerns

None for Task 1. Broader runtime rule selection behavior and platform wiring remain outside this task and are deferred to later tasks.

## Compozy Impact Audit

- Native tools: Checked `internal/tools`, native tool descriptors, and tool capability surfaces conceptually; no `compozy__*` IDs, schemas, digests, or capability gates are touched by selector validation. No impact.
- Extensibility and hooks: Checked runtime registries, extensions, hooks, skills/capabilities, tools/resources, bridge SDKs, MCP sidecars, and config lifecycle surfaces; this is local validation of existing `RuntimeMatch` fields and adds no extension or hook surface. No impact.
- Workspace data isolation: Scope is definition/config validation before runtime selection; no workspace, session, agent, store, CLI, HTTP, UDS, SSE, cache, event, or `workspace_id` propagation changes. No impact.
- Official Compozy skill: No `skills/compozy/` surface is involved in grammar validation. No impact now; official skill updates are deferred to Task 4 if required.
