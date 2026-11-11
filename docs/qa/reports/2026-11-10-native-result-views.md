# QA Run Report — 2026-11-10 — Native tool result views

- Scope: S10 closure of #1432; targeted CLI/API/runtime journeys.
- Build: feat/native-tool-result-views-s10, base 282f90421 plus owned changes.
- Environment: isolated bootstrap lab native-result-views-s10-20261111-000039-415039;
  provider-free ACP subprocess driver, no copied credentials or live provider calls.
- Status: PASS for the targeted result-view scope.

## Session Matrix & Results

| Charter | Journey / Scenario | Persona | Tour | Status |
|---|---|---|---|---|
| CH-native-result-views | ET-native-result-views / E2E-001–003 | Ada | Feature | Pass |
| CH-native-result-views | ET-cli-tool-invoke-structural-handles (view-specific handles) | Ada | Feature | Pass |
| CH-native-result-views | RT-session-spawn-wake (view-specific spawn result) | Ada | Feature | Pass |

## Session Debriefs

The first harness attempt exposed a missing required `ttl_seconds` in the new journey input,
not a production defect. Status measured 16,820 full / 1,206 compact bytes (92.83%);
create measured 2,119 / 955 (54.93%). The attempt stopped at spawn and made no completion claim.
The report was created after that attempt; this is a reporting-order deviation. The corrected
walk starts with the matrix above pending. Broader historical wake/redaction scenarios are
not settled by the bounded result-view walk.

## Verification

UT-014 `TestNativeResultClassification`, UT-015 `TestNativeResultCatalog`, existing
`TestSingleResultContracts` and `TestResultViewDeclaration`: passed.
`TestNativeResultViewContracts` and `TestNativeSessionResultViews`: passed.
`TestHostedResultViewCatalogSize`: passed, 288 tools / 99 with views, 1,087,936 bytes.
Its 1,015,805-byte synthetic no-output-views comparison retains current injected inputs
and compact defaults; it is not the original pre-adoption catalog (744,687 bytes).

## Consumer audit

Search roots: skills/, extensions/, .agents/, .compozy/agents and internal/ prompt templates.
Moved-field reads repaired: tasks-and-orchestration reply watches; runtime-operations ACP
capabilities and supervision; loops resolved runtime and full inspection; open-design-review
inspection/node output detail; merge-gate lineage spawn depth; agent-definitions appearance.
Runtime confirmation instructions read runtime.effective, which compact retains. Merge-gate
lineage identity remains in compact. Automation Job, Trigger and LoopTarget in
internal/automation/model/types.go contain session/Task/Loop targets and event input mappings;
there are no native output references to migrate.

## Final Status

The final public-surface run passes all three subcases (57.75 seconds; Go package 61.456 seconds).
Site content generation, typecheck and lint pass, with zero warnings or errors.
Strict evidence audit and teardown receipts are recorded below.

## Harness repairs and validation limitations

`TestSeedConfigPersistsProviderUsageOverlay` owns the invariant that a provider-free
lab's persisted config keeps usage collection disabled. The runtime log exposed that
`configSeedFile` omitted ProviderUsage, so ConfigSeed.Mutate lost the requested value.
The helper now persists it. Earlier attempts did make background signed-out usage probes;
no operator credential files were copied and this was not live-provider acceptance.

The test-conventions heuristic flags pre-existing cases in builtin_test.go and
config_seed_test.go; added cases use the canonical Should subtests. The new E2E file passes.
The first installed linter used Go 1.26 and refused the repo's Go 1.27.3 target; the
repo-pinned v2.15.0 linter is invoked with `go run` under the project toolchain.
The first Turbo typecheck selected the existing root codegen-check dependency, which
failed migration parity. Both guide sources now match; affected-package checks use
`--only` after content generation so repository-wide checks stay with CI.

The full-record parity probe initially compared against three fields the HTTP detail
route adds independently (`working_dir`, `worktree_binding`, `process_summary`).
`origin/main:internal/daemon/native_tool_session_calls.go` returns only
`SessionPayloadWithQueue` plus profile owners; `internal/api/core/session_detail.go:46`
and `session_processes.go:340` add those API-only decorations. The probe now excludes
exactly those API-only fields and compares the rest of the unchanged record. This
corrects an invalid newly authored assertion; no production contract or threshold changed.

## Review

In-session review runtime confirmed through daemon status: codex / gpt-6-astra / high,
selection revision 1 (live configuration, same ACP session). Reviewed the owned diff,
classification/catalog comparison, consumer changes, new E2E journey, lab helper repair,
and docs against the S10 contracts. Review tightened full-record equality, made non-completed
tool outcomes fail, corrected the Loop example's dependency edge, and replaced the repeated
session migration rows with one explicit shared-record field list. No acceptance threshold
or existing regression assertion was relaxed.

The scope is the affected CLI/agent result-view behavior in the two existing scenarios;
the complete historical Cora UI/wake matrix remains outside this walk. The provider-free
ACP process is an I/O fixture, not a live provider acceptance claim. Daemon lifecycle is
owned and joined by RuntimeHarness; targeted bootstrap teardown independently checks survivors.

## Exact commands

```sh
bun install
make codegen
go test ./internal/tools/builtin -run 'Test(NativeResult|SingleResultContracts|ResultViewDeclaration)' -count=1
go test ./internal/daemon -run 'TestNative(ResultViewContracts|SessionResultViews)' -v -count=1
go test ./internal/mcp -run TestHostedResultViewCatalogSize -v -count=1
go test ./internal/testutil/e2e -run '^TestSeedConfigPersistsProviderUsageOverlay$' -count=1
COMPOZY_S10_LAB_MANIFEST=/Users/pedronauck/dev/qa-labs/compozy-native-result-views-s10-20261111-000039-415039-lab/qa-artifacts/qa/bootstrap-manifest.json go test -tags=integration ./internal/daemon -run '^TestDaemonE2ENativeResultViews$' -v -count=1
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run ./internal/tools/builtin ./internal/testutil/e2e --new-from-rev=HEAD
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run --build-tags=integration ./internal/daemon --new-from-rev=HEAD
bunx turbo run generate:content typecheck lint --filter=./packages/site --only
bash scripts/verify-migration-guide-parity.sh
```

CI owns full, race and complete E2E suites; only the three new E2E subcases ran locally.
The release tag is not assigned in this slice. The conductor should tie #1462's next-release
removal to the actual shipping tag, without guessing a version here.

## Final real-run measurements

Structured result bytes and Unicode character counts are equal for these ASCII records.
This is one provider-free lab sample, not a statistical benchmark. The historical values
come from the spec; the full/compact columns are paired measurements in the current lab.

| Tool | Historical default bytes | Current full bytes/chars | Compact bytes/chars | Reduction |
|---|---:|---:|---:|---:|
| session_status | 13,114 | 16,821 | 1,207 | 92.82% |
| session_create | 1,585 | 2,119 | 955 | 54.93% |
| session_archive | 11,714 | 15,082 | 1,358 | 91.00% |
| session_spawn | 10,482 | 11,679 | 160 | 98.63% |

Full status preserves the unchanged native content; all 99 populated full projections
are also checked by the owning contract suite. Create has no size threshold.

The lab evidence root is `/Users/pedronauck/dev/qa-labs/compozy-native-result-views-s10-20261111-000039-415039-lab/qa-artifacts/qa`.
It contains `bootstrap-manifest.json`, `journey-log.jsonl`, `verification-report.md`,
`result-views-e2e.log`, `result-views-contract.log`, `result-views-builtin.log`,
`result-views-seed.log`, and `result-views-mcp.log`. Journey entries are retrospective,
reconstructed from the passing final test log; they are not claimed as live observations.

## Audit and cleanup receipts

`qa-audit-report.json`: PASS, zero blockers and warnings. `teardown.json`: clean=true,
matched=[], killed=[], survivors=[]; RuntimeHarness had already stopped its daemon.
No lab daemon or dev server remains running. Focused Go lint reports zero issues
for builtin, the seed helper and daemon integration tests.

```sh
python3 .agents/skills/eng/eng-real-scenario-qa/scripts/audit-qa-evidence.py --qa-output-path /Users/pedronauck/dev/qa-labs/compozy-native-result-views-s10-20261111-000039-415039-lab/qa-artifacts --strict --explain
python3 .agents/skills/eng/eng-qa-bootstrap/scripts/teardown-qa-env.py --manifest /Users/pedronauck/dev/qa-labs/compozy-native-result-views-s10-20261111-000039-415039-lab/qa-artifacts/qa/bootstrap-manifest.json
```

Audit fixes: `skills/compozy/references/tasks-and-orchestration.md:48`,
`runtime-operations.md:67,1650`, `loops.md:185,778`, `agent-definitions.md:336`,
`.compozy/agents/merge-gate/AGENT.md:22`, and
`extensions/open-design/skills/open-design-review/SKILL.md:12,53`. All request full
when reading moved fields. No additional moved-field reads were found in bundled
`.agents` procedures or internal agent prompt templates.

The PR inventory is kept in the authorized workspace-root spec directory:
`.compozy/tasks/native-tool-result-views/qa/pr-inventory.md`. It includes 288 rows,
99 measured populated fixtures (largest full sample per tool), single-result reasons,
and the complete digest-change table against origin/main. Single-result byte sizes
are explicitly unmeasured: those tools keep their result shape and have no paired
view fixture. The table does not invent production measurements for those tools.

The documented hook, MCP and Loop contracts also pass:

```sh
go test ./internal/tools ./internal/loop ./internal/mcp -run 'Test(RuntimeRegistryDispatchResultViews|HostedResultViews|LinterResultViewReferences|ActionRegistryResultViewPins)$' -count=1
```

Results: tools 0.589s, loop 0.883s, mcp 1.485s.
