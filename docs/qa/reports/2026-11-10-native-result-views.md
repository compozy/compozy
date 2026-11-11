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
capabilities and supervision; loops resolved runtime; merge-gate lineage spawn depth;
agent-definitions appearance. OpenDesign reads retained fields and needs no selector change.
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

Set `S10_QA_OUTPUT` to the isolated bootstrap manifest’s `qa_output_path`.

```sh
bun install
make codegen
go test ./internal/tools/builtin -run 'Test(NativeResult|SingleResultContracts|ResultViewDeclaration)' -count=1
go test ./internal/daemon -run 'TestNative(ResultViewContracts|SessionResultViews)' -v -count=1
go test ./internal/mcp -run TestHostedResultViewCatalogSize -v -count=1
go test ./internal/testutil/e2e -run '^TestSeedConfigPersistsProviderUsageOverlay$' -count=1
COMPOZY_S10_LAB_MANIFEST="$S10_QA_OUTPUT/qa/bootstrap-manifest.json" go test -tags=integration ./internal/daemon -run '^TestDaemonE2ENativeResultViews$' -v -count=1
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

The lab evidence root is `$S10_QA_OUTPUT/qa`, resolved from the bootstrap manifest.
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
python3 .agents/skills/eng/eng-real-scenario-qa/scripts/audit-qa-evidence.py --qa-output-path $S10_QA_OUTPUT --strict --explain
python3 .agents/skills/eng/eng-qa-bootstrap/scripts/teardown-qa-env.py --manifest "$S10_QA_OUTPUT/qa/bootstrap-manifest.json"
```

Audit fixes: `skills/compozy/references/tasks-and-orchestration.md:48`,
`runtime-operations.md:67,1650`, `loops.md:778`, `agent-definitions.md:336`, and
`.compozy/agents/merge-gate/AGENT.md:22`. All request full
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

## S10 corrective self-review

Reviewed the slice against integration follow-ups `24f9e24f8` and `b7ba7f741`.
The generated catalog was not changed in this pass.

- P2: the migration field inventory reflected populated fixtures, missing optional
  fields, and incorrectly marked approval `profile_archived` as full-only. The release
  note now follows projection fields and explicitly preserves gate prompts, review
  `delivery_id`, archived-profile state and the conditional skill diagnostics rule.
  `session_list` also retains folder and health fields.
- P2: native search discovery uses `output_views` digests, not `has_result_views`.
  Corrected the official reference. Native list keeps `has_result_views` and has no
  description; tool info stays whole.
- P2: CLI examples omitted the invoke envelope and the `tool` wrapper on info.
  Corrected the jq paths, native descriptor `id`, and the moved-field diagnostic path.
  Full schemas can be open; the documentation no longer promises all fields are declared.
  Error tables distinguish dispatcher detail from redacted CLI/HTTP messages.
- P2: E2E discovery checked only JSON substrings and full invocation only nonempty output.
  It now asserts both actual schemas/digests, default-contract equality and full lineage
  policy, rejects a non-string selector, reuses compact IDs, and compares full create
  and status against unchanged records, plus archive lineage against its pre-archive value. Hosted text counts include preview plus
  structured JSON, matching the hosted result projection; they are not token counts.
- P2: committed QA commands contained an operator-specific absolute lab path. Replaced
  it with `S10_QA_OUTPUT`, resolved from the bootstrap manifest; examples use synthetic IDs. Root lineage omits `kind`, and permission prompts use
  the canonical `waiting-for-auth` badge.
- P3: OpenDesign inspection and output review read fields compact keeps. Reverted those
  full selectors. The general Loop authoring instruction now reads the retained input
  contract/version without requesting full.

Retained scope additions: merge-gate reads `lineage.spawn_depth`; agent identity docs read
appearance; runtime docs read ACP catalogs and supervision; reply-watch docs read
`reply_watches`. Those fields require full. Root/site migration parity requires updating
MIGRATION_GUIDE.md. The config seed repair persists disabled provider usage, and the ACP
fixture turn is consumed by the full-view Loop journey. Its actual path is
`internal/testutil/acpmock/testdata/multi_agent_fixture.json`.

UT-014 plus the existing `TestSingleResultContracts` cover exclusive classification and
orphan single-result entries; UT-015 checks the exact fields specified by the contract.
No duplicate assertion was added. The three public CLI/process journeys remain together
in the daemon E2E suite behind `integration && !windows`, matching adjacent RuntimeHarness
CLI suites; the direct in-process lifecycle suite remains separate.

### Hosted discovery cost finding

The measured S10 full-policy catalog is 1,087,936 bytes versus 744,687 before the feature
(+343,249, 46.1%). `hostedMCPTool` in `internal/mcp/hosted_proxy.go` sets OutputSchema;
`reconcileHostedTools` registers that tool with the MCP SDK. The private cache middleware
only adds cache scope/TTL and the protocol middleware does not prune tools/list. Both
schemas therefore reach the managed provider's MCP client. This repository does not
prove whether each external provider forwards outputSchema into model context.

A cheaper advertisement would be an object-only output schema with detailed contracts
available through tool info, retaining selected-view validation inside dispatch. That
would sacrifice client-side detailed output validation and change the accepted MCP
contract, so it is a finding for a separate decision, not an internal/mcp edit in S10.

The corrective test run first exposed two invalid newly added comparisons: raw JSON schema
bytes differ by indentation in CLI output, and session detail adds fields not emitted by
the historical create/archive handlers. Schema equality now compares decoded values and
exact digests. Record equality accounts for the explicit decorations in
`internal/api/core/authored_context_access.go` and `session_profile_owners.go`;
`origin/main:internal/daemon/native_session_mutation.go` and `native_session_archive.go`
both used `SessionPayloadFromInfo` without those decorators. The original status equality
and 80% byte threshold remain, with an additional text-character threshold.

Archive returns `sessionInfoFromCatalog` from `internal/session/archive.go`, while
status can return live session metadata. The catalog intentionally lacks live workspace
path and permission mode and has its own update time. The archive journey therefore
compares the persisted full lineage against its pre-archive value and checks archive
state; it does not equate the catalog snapshot with live session detail.

Corrective verification:

- `go test ./internal/tools/builtin -run 'Test(NativeResult|SingleResultContracts|ResultViewDeclaration)' -count=1`: passed (0.476s).
- `go test -tags=integration ./internal/daemon -run '^TestDaemonE2ENativeResultViews$' -v -count=1`: all three journeys passed (36.421s). The RuntimeHarness owns its temporary home, port, socket and teardown; no provider credentials or dev server were used.
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run --build-tags=integration ./internal/daemon --new-from-rev=c936360e1`: passed, 0 issues after the focused formatter correction.
- `bash scripts/verify-migration-guide-parity.sh`: passed.
- `bunx turbo run generate:content typecheck lint --filter=./packages/site --only`: passed, 3 tasks (5.66s), zero warnings/errors.
- Release-note defaults compared with the integration catalog: exactly 99 tools, no missing tools and no single-result tools. Catalog unchanged.

The corrective run's structured full/compact byte samples were status 16,841/1,203,
create 2,131/955, spawn 11,692/160 and archive 15,095/1,358. The projected hosted text
adds the 21-character preview to each result. These are single-run samples, not averages.

### S12 — ADR-004 hosted schema removal

The discovery cost finding above is superseded by accepted ADR-004. The hosted proxy
now omits outputSchema for every native tool. Measured on the S12 checkout: 288 tools,
99 view adopters; tools/list before 1,088,135 B, after 347,863 B (740,272 B removed),
versus the historical pre-feature baseline 744,687 B. Model-facing name + description +
inputSchema is 207,320 B and unchanged; these are JSON bytes, not model tokens.
The earlier 1,087,936 B observation belongs to its earlier head.

Server.AddTool in MCP Go SDK v1.8.0 preserves handler Content and StructuredContent
without requiring OutputSchema. Real MCP + registry dispatch cases in TestHostedResultViews
cover both views, typed errors and truncation. TestHostedToolResultContract also retains
all-native-tool truncation/artifact recovery coverage. No external provider run is claimed.

Advertising audit: internal/session/pi_native_tools.js registers the same hosted MCP
server through Pi registerMcpServer; provider_runtime_pi_bridge.go passes connection
configuration, not schemas. internal/acp/session_mcp.go also passes MCP servers;
internal/mcpbridge/bridge.go forwards the upstream tools unchanged, so hosted native
schemas are absent there too. internal/mcp/serve_projection.go exposes separate
compozy_host__ Host API tools with input schemas only. External MCP descriptors in
internal/mcp/tool_descriptor.go and extension SDK descriptors retain their own output
schemas; these are outside ADR-004. CLI/HTTP/UDS descriptor contracts stay available.

S12 verification (local, scoped):

- `go test ./internal/mcp -count=1`: pass (6.470s).
- `go test ./internal/mcp -run 'TestHostedResultViews|TestHostedResultViewCatalogSize|TestReconcileHostedTools|TestHostedToolResultContract' -count=1 -v`: pass, including real MCP/registry round trips and catalog measurement.
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run --allow-parallel-runners --timeout 5m ./internal/mcp/...`: 0 issues. The installed binary was built with Go 1.26; running the pinned version with Go 1.27.3 resolves that toolchain mismatch.
- `python3 .agents/skills/eng/eng-test-conventions/scripts/check-test-conventions.py internal/mcp/hosted_proxy_test.go` and the same command for `hosted_proxy_result_test.go`: pass.
- `bunx turbo run generate:openapi generate:sdk generate:install --filter=./packages/site --only` and `bunx turbo run generate:content --filter=./packages/site --only`: pass; generated runtime assets remain untracked/ignored.
- `bunx turbo run lint test typecheck --filter=./packages/site --only`: pass, 54 test files / 397 tests, all three tasks successful. The first run lacked generated local assets and exposed a console-language fence in the touched result-views page; generators and a bash fence with commented sample output repaired those failures without changing tests.
- `git diff --check`: pass.

No repository-wide gates, race suite, E2E or external provider walk ran, as required by the slice brief. The canonical catalog was not regenerated. Integration/CI and release publication remain conductor-owned.

### S12 self-review follow-up

Reviewed `762f8c808..010b9c828` against ADR-004 and S12. No production result-path
regression was found. The former schema builder returned nil exactly when the selected
internal contract was empty; `requiresStructured` preserves that decision. Without a
contract, plain text stays unstructured, object results still carry structured content,
and truncation still creates a continuation. Typed and partial errors return before
this flag is consulted. Fingerprints include the flag so registration changes cannot
retain a handler with the wrong fallback behavior.

Findings repaired in the existing hosted proxy suite:

- P2 coverage gap: catalog size and omission assertions could pass with missing tools,
  duplicate identities or lost result selectors. Assert complete listing membership,
  no continuation cursor, exact descriptor input schemas and each view selector.
- P2 coverage gap: the re-registration test counted registrations without observing
  returned content; truncation did not assert artifact text/identity, and partial
  errors only had helper-level coverage. Verify nil/declared/nil contract transitions
  through real MCP, exact artifact continuation text, and partial errors through MCP
  plus dispatch in both views.
- P3 inaccurate comment: SDK v1.8.0 permits non-object structured content. The moved
  helper comment now describes Compozy's preserved projection, not an MCP restriction.

The only hosted registration path constructs schema-free tools. SDK `Server.callTool`
and `ClientSession.CallTool` do not condition structured result delivery on an output
schema. Stdio/HTTP serializers, protocol middleware and the ACP HTTP bridge do not
remove structured content. Pi registers that same MCP server; `mcp serve` builds the
separate input-only Host API tool definitions. No external provider walk is claimed.

The artifact-recovery scenario and result-artifacts page were correctly changed because
truncation recovery must continue without an advertised schema. The session-message
scenario now obtains its validation contract through tool info; its dated pre-S12
verification remains historical evidence. The earlier 1 MB catalog measurements in this
report are explicitly superseded by the S12 section, not current-size claims. Removed
schema validation cases tested the deleted widening/rebasing; artifact byte recovery,
result values, error flags and partial-result assertions remain covered.

Follow-up checks: `go test ./internal/mcp -count=1` passed (6.166s); the focused
`TestReconcileHostedTools|TestHostedResultViews|TestHostedResultViewCatalogSize` run
passed with 288 tools, 99 selectors and the same 347,863 / 207,320 byte measurements.
The pinned Go 1.27.3 `golangci-lint@v2.15.0` command above reports 0 issues; the test-shape
checker and `git diff --check` pass. Site source did not change in this follow-up, so
its prior scoped Turbo receipts remain applicable.

## R1 — Final review repairs

The repair worktree started clean at `0374f8f0dbbc72dc411e71f3bc4df55cab2a88c0`.
Fable F2–F4 and Astra F1 were addressed without changing the accepted projections or ADR-004.
Fable F1 was only partially resolved in R1; the R1b section records the remaining repairs.

- F1: share the existing Loop JSON Schema classifier with the reference resolver and its
  available-field diagnostics. Apply the keyword guard only to recognized schemas carrying
  `type` or `additionalProperties`; shorthand field maps retain direct lookup. The existing
  refs suite now covers plain `type` fields, nested map-valued `type`, transform `any` fields,
  undeclared paths, real JSON Schema keywords and declared properties.
- F2: native instructions explicitly select full for reply watches, missing-root diagnostics,
  PR lookup, heartbeat source schemas, process/monitor/linked-work summaries, spawn permissions,
  subagent models and skill provenance. Site session/skill pages and the descriptor example
  identify the full view where required.
- F3: remove `result_bytes` and `result_ref` from both run-list migration entries and from
  `continuation_run` on review submit. Rechecked all 99 adopting tools against full/compact
  schema paths (including local `$ref` definitions), populated/empty fixture results and the
  projection owners. No other migration-field correction was required. Shared window-manager
  full schemas declare unused alternate endpoint fields; those are not fields moved by these
  handlers. `desktop_clients` never returns attachment tokens (the service strips them), so no
  token migration entry was added. Skill diagnostics remain conditional, as already documented.
- F4: delete the twelve nil-only fixture families and their aggregator entries. The canonical
  all-tool contract suite still requires fixtures for every descriptor with views.
- Safety 6: the dispatcher suite checks the check-in audit preview and both started/completed
  event input digests retain `result_view`, while the handler receives the stripped input.
- Safety 4: the shared HTTP/UDS handler suite sends forged snake-case and Go-style wire view/pin
  fields, checks a successful response, and verifies neither reaches the registry request.

Red/green evidence: `go test ./internal/loop/dsl/refs -run '^TestNamespaceSchemaNamedFields$'
-count=1` failed before the production edit in all three shorthand cases (`unresolvable_path`),
then passed after the fix. The first broader Loop run caught an overly broad classifier guard
on shorthand maps such as `items`; narrowing the guard repaired production behavior. All Loop
packages subsequently passed. No assertions were weakened.

Verification commands (all from the worktree root):

```sh
go test ./internal/loop/... ./internal/tools/... -count=1
go test ./internal/api/core -count=1
go test ./internal/daemon -run '^Test(NativeResultViewContracts|NativeSessionResultViews|NativeToolHookResultViews|NativeCorrectedOutputSchemas|ToolApprovalResultViews|CheckInResultViewApproval|BoundResultViewApprovalToken)$' -count=1
go test -tags=integration ./internal/daemon -run '^Test(NativeResultViewsIntegration|LoopResultViewsIntegration)$' -count=1
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run ./internal/loop/... ./internal/tools/... ./internal/api/core ./internal/daemon
bunx turbo run typecheck test --filter=./packages/site
bunx turbo run generate:content typecheck test --filter=./packages/site --only
bunx turbo run test --filter=./packages/site --only -- --maxWorkers=2
git diff --check
```

The Go tests passed, including the real SQLite/session lifecycle and persisted Loop integration
checks. Test-shape heuristics passed for all four changed test files. Focused Oxfmt checks passed.
The initial site attempt lacked installed workspace dependencies; `bun install --frozen-lockfile`
restored them without manifest/lock changes, and the normal Turbo graph then passed (397 tests).
The final bounded site rerun passed content generation and typecheck after the last prose edits.
Its test run had one SDK index test exceed the unchanged 5s timeout (396 passed); the full
site suite was repeated with two workers to reduce contention, without changing assertions
or timeout configuration.
The installed lint binary was built with Go 1.26 and could not analyze this Go 1.27 module;
the repository-pinned lint version was run from source with the current toolchain instead.
A first lint pass overlapped removal of the temporary audit probe. The next pass reported
two pre-format Golines findings; focused formatting was applied and the final stable-file
pass completed with `0 issues`.
The linker emits its existing macOS `-bind_at_load` deprecation message when building the lint tool.

Impact follows the existing Native Tool Result Views audit in `docs/_memory/change-impact.md`:
no additional native tool, wire, hook/config or workspace-isolation contract changes; the first Loop
repair restored only the shorthand `type` plus non-keyword sibling cases then tested.
The R1b evidence below supersedes that incomplete compatibility claim. Official skill and site consumers are corrected above.
No database migration, generated-source edit, external-provider run, full gate, race suite or E2E
run belongs to this repair slice. The conductor owns final-head CI and integrated scenario walks.

Pre-commit tasks passed via `bunx lint-staged --no-stash`; commit-message validation passed via
`printf '%s\n' 'fix: repair native result view review findings' | bunx commitlint`.
The commit hook is not repeated because its default invocation creates a Git stash; the same tasks
were run explicitly without that backup, honoring this slice's Git restrictions.


## R1b — Provenance-aware reference resolution

Base: `cea0ff9646de31579aba5c1fee9970082179ff08`. Read both completed Fable/Astra re-reviews.
This section supersedes R1's broad compatibility claim; it establishes the listed shapes and
existing focused suites, not universal compatibility of arbitrary user definitions.

- Fable R1-N1 / Astra Fable F1: `NodeSchema.OutputJSONSchema` carries the producer's known
  schema kind. Descriptor schemas are always JSON Schema; transforms construct an explicit
  object schema from their actual map keys; authored action/ask schemas pass through the same
  `normalizeLoopSchema` used by output validation. Reference resolution never consults a keyword
  allowlist. The existing runtime authored-schema classifier moves back to its runtime owner,
  unchanged. Source `produces` declarations and watch-event field descriptions remain field
  maps: source execution does not run them through output-schema validation. Existing source
  and mixed field-description traversal is preserved. Normalized/descriptor modes propagate
  through current, previous and best output paths and diagnostics.
- Astra R1-N1: six added tool-schema cases cover `$comment` on scalar and closed object schemas,
  `$ref`, `patternProperties`, `else`, and `x-compozy-free-form`. These reject metadata children
  without expanding the authored-schema classifier's keyword set. All earlier strict view cases
  remain green.
- Fable R1-N2: default `branch`, `pull_request`, and `board_column` are documented separately
  from full-only `pull_request_lookup`.
- Fable R1-N3: the HTTP/UDS example again describes the complete HTTP/UDS descriptors. No view
  selector applies to those routes; native compact search retains view digests, while compact
  list uses `has_result_views` (the existing native section already documents that distinction).
- Fable R1-N4 / Astra R1-N2: lifecycle supervision and native lineage origin-field instructions
  now select full. Repeated the skill/site scan for explicit native tool IDs and named output
  fields, using the generated compact schemas (resolving local `$ref` definitions), complete
  payload fixtures and projection owners. Further corrections cover `parent_title`, source poll
  timestamps/`last_item_count`, effective permission/workspace-access reads, and Loop run-detail
  snapshot fields. Input parameters, descriptions of internal state computation, CLI/HTTP-only
  instructions and retained compact fields were not switched to full.
- Fable R1-N5: scoped the previous claim and recorded concrete comparisons below.

The test invariant is agreement between reference lint and the owning runtime shape. The canonical
suite is `linter_test.go`: `TestLinterOutputShapeRuntimeAgreement` calls the real transform executor
or `ValidateActionStructured` before checking references. It runs every authored case independently
for `produces`, `expect`, and `output_schema`; no provider or validator is mocked. `produces` is a
node's declared shape, not a guarantee that every executor validates it; the runtime comparison
here exercises the shared declared-schema validator used for structured output and amendments.
The lower-level refs fixtures now explicitly mark their JSON Schema provenance, retaining the same
path assertions. No assertion or runtime schema validation was weakened.

### Shape comparison

`origin/main` was frozen at `766345807802b7056b0fc8a026c84de70f23ea84`. The same test was run
with all original `internal/loop` sources/tests supplied through a read-only Go overlay, plus the
new comparison case. The scalar/open-warning assertions intentionally fail against that old lint;
its logged observations, and the same runtime validator/executor assertions, produce this table.

| Source | Shape and reference | origin/main lint | Runtime observation | Lint now |
| --- | --- | --- | --- | --- |
| Transform map | `{type}` → `type` | Accepted | Executor returns `type: "bug"` | Accepted |
| Transform map | `{type, title, description}` → each key | Accepted | Executor returns exactly those keys | Accepted |
| Transform map | `{properties, name}` → each key | Accepted | Executor returns both keys | Accepted |
| Authored: produces / expect / output_schema | `{properties: object, name: string}` → both keys | Accepted | Shared validator accepts `{"properties":{},"name":"name"}` | Accepted |
| Authored: produces / expect / output_schema | `{properties: {a: string}, id: string}` → `properties.a`, `id` | Accepted | Validator requires the nested object and `id` | Accepted |
| Authored: produces / expect / output_schema | Same nested shape → root `a` | Accepted | Missing in the original sample; optional extra root `a` is valid and resolves | Accepted with warning (R1c) |
| Authored: produces / expect / output_schema | `{type: array, items: string, name: string}` → each key | Accepted | Validator accepts `{"type":[],"items":"items","name":"name"}` | Accepted |
| Authored: produces / expect / output_schema | `{type: object}` → `type` | Accepted, no warning | Validator accepts `{"type":"value"}`; object is open | Accepted with cannot-be-proven warning |
| Authored: produces / expect / output_schema | `{type: string}` → `type` | Accepted | Validator accepts a structured JSON string; rejects `{"type":"value"}` | Accepted with warning (R1c) |
| Authored: produces / expect / output_schema | `{type: string, description: string}` → `type`, `description` | Accepted | Same string validation; description is schema metadata | Accepted with warning (R1c) |
| Tool descriptor JSON Schema | Annotated scalar / closed object metadata children | Old direct fallback accepted metadata | Scalar has no children; closed object has no undeclared properties | Rejected in all six new cases |

Scalar observations refer to the structured JSON validation path. Text-only agent output still
uses the existing final-object extraction path; this repair does not change it. The table does not
claim that source hints are runtime validators or that arbitrary additional output fields are
rejected by an open schema. The R1c section supersedes R1b's classification of historically
accepted authored references as lint errors; the table above is updated to the current behavior.

### Red, green and verification

The tests were authored before production edits. After correcting the test consumer to use a valid
run-agent prompt rather than a transform without its required map, the corrected tests were replayed
against the immutable base production files with a Go overlay, avoiding any Git revert. Baseline:
15 of 21 shape/source subtests failed (keyword transforms, properties/array shorthand, open object),
and all six new tool-metadata cases failed. The six scalar negative controls already passed on the
base. After the fix, all 21 shape/source subtests and all 39 view-reference subtests passed; the full
`internal/loop/...` suite passed as well. Intermediate broader tests caught source/watch-event hints
being interpreted as normalized schemas; their producer mode was corrected, preserving their
existing runtime and lint behavior.

Commands and outcomes:

```sh
# Baseline replay: expected failure, cases above.
go test -overlay=/tmp/ntrv-r1b-base-overlay.json ./internal/loop -run '^Test(LinterOutputShapeRuntimeAgreement|LinterResultViewReferences)$' -count=1 -v
# origin/main comparison: expected differences, runtime assertions passed.
go test -overlay=/tmp/ntrv-r1b-origin-overlay.json ./internal/loop -run '^TestLinterOutputShapeRuntimeAgreement$' -count=1 -v
# Passed all packages; verbose output also records every shape/path observation.
go test ./internal/loop/... -count=1 -v
# Passed contracts and real persisted Loop/session integration.
go test ./internal/daemon -run '^Test(NativeResultViewContracts|LoopResultViews.*)$' -count=1
go test -tags=integration ./internal/daemon -run '^Test(NativeResultViewsIntegration|LoopResultViewsIntegration)$' -count=1
# Final pass: 0 issues. The earlier pass caught duplicate array literals, replaced with a constant.
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run ./internal/loop/...
bunx turbo run generate:content typecheck --filter=./packages/site --only
bunx turbo run test --filter=./packages/site --only -- --maxWorkers=2
git diff --check
```

Content generation/typecheck passed; the test-only Turbo run passed all 397 tests in 54 files.
A first combined Turbo command incorrectly forwarded the Vitest
worker flag to Velite; the commands above separate generation/typecheck from test-only arguments.
The refs test-shape check passed. The linter file checker reports the existing untouched
`TestLinterShouldRejectDeletedNoProgressParameters` inline-case diagnostic; the identical diagnostic
was reproduced on the base file. The new table-driven cases use parallel `Should ...` subtests.
Focused formatting and lint are the checks for these edits; no unrelated test was rewritten.

The cross-surface audit remains the existing Native Tool Result Views record in
`docs/_memory/change-impact.md`: internal lint/source provenance and corrected official skill/site
consumers only. No wire DTO, native ToolID, config, hook, data-isolation, migration or generated-source
change. No external provider, global gate, race or E2E run; no push or PR from this slice.


## R1c — Preserve authored reference acceptance

Base: `bd9a78a551bb5d16776c11ae45be6b51b03e5290`. Authored-schema references are user state;
this feature does not authorize tightening their acceptance. The three previously rejected rows
are now accepted with the existing `unresolvable_path` code at warning severity. The warning text is
`node "shape" output path ".nodes.shape.output.<field>" cannot be proven`. No new code was added.
Tool descriptor schemas have no authored fallback and retain their strict behavior. Transform
output fields retain their behavior. No runtime validation or executor behavior changes.

### Runtime evidence

Before editing production code, the existing `TestLinterOutputShapeRuntimeAgreement` suite was
extended to evaluate the disputed references against actual runtime values. These assertions passed
while nine shape/source subtests failed on lint rejection or missing warnings at the base:

- For the sample `{"properties":{"a":"a"},"id":"id"}`, direct template access to root `a`
  fails with a missing-map-key error. However, the schema permits additional properties: adding
  `"a":"root"` passes the real schema validator, and the identical template then renders `root`.
  Calling this reference universally unresolvable was incorrect.
- For the validated structured string `"value"`, direct access to `type` or `description` fails:
  those fields cannot be read from a string. `{type: string, description: string}` still describes
  a string, not an object with either field. The object payload remains rejected by the validator.
- `refs.RenderTemplateString` uses `missingkey=error`, so none of the failing evaluations silently
  renders an empty result. Both `TransformActionExecutor.Execute` mapping forms, `template` and
  `from`, return `ErrActionMaterialization` for these samples. The coordinator maps that error to
  `action_materialization_failed`; control-node materialization likewise marks the output failed
  (`coordinator_action.go:actionMaterializationFailure`, `control_plan.go:materializationFailedOutput`).
- This does **not** prove every such definition hard-fails the entire Loop. Root `a` can exist as
  shown; `produces` is descriptive and is not automatically used as run-agent `output_schema`
  (`action_runagent.go` reads and validates `spec.OutputSchema`, not `node.Produces`); and failure
  policy/routing determines the run outcome. An unguarded evaluation of the missing/scalar paths
  tested above fails the consuming node. No empty-value coercion or runtime workaround was added.

### Updated compatibility table

Each authored row runs independently for `produces`, `expect`, and `output_schema`, and checks
current, `previous`, and `best` references. Main is the frozen `766345807802b7056b0fc8a026c84de70f23ea84`.
The warning column describes lint; runtime errors above remain unchanged.

| Source | Shape → path | origin/main lint | Runtime evidence | Lint now / warning |
| --- | --- | --- | --- | --- |
| Transform map | `{type}` → `type` | Accepted | Executor returns that key | Accepted; no warning |
| Transform map | `{type,title,description}` → each | Accepted | Executor returns each key | Accepted; no warning |
| Transform map | `{properties,name}` → each | Accepted | Executor returns both keys | Accepted; no warning |
| Authored (all three sources) | `{properties: object,name:string}` → both | Accepted | Validator accepts both fields | Accepted; no warning |
| Authored (all three sources) | `{properties:{a:string},id:string}` → `properties.a`, `id` | Accepted | Valid nested values | Accepted; no warning |
| Authored (all three sources) | Same shape → root `a` | Accepted | Missing sample fails evaluation; valid extra root `a` resolves | Accepted; cannot-be-proven warning |
| Authored (all three sources) | `{type:array,items:string,name:string}` → each | Accepted | Valid three-field object | Accepted; no warning |
| Authored (all three sources) | `{type:object}` → `type` | Accepted | Open object accepts and resolves it | Accepted; cannot-be-proven warning |
| Authored (all three sources) | `{type:string}` → `type` | Accepted | Direct evaluation against validated string fails | Accepted; cannot-be-proven warning |
| Authored (all three sources) | `{type:string,description:string}` → both metadata names | Accepted | Direct evaluation against validated string fails | Accepted; cannot-be-proven warning |
| Authored (all three sources) | Closed object metadata; nested string metadata; array metadata; keyword collisions | Accepted | Values are not guaranteed by those contracts | Accepted; cannot-be-proven warning |
| Tool descriptor JSON Schema | Scalar/closed-object metadata, including annotations | Historically admitted by old metadata lookup | Not output fields | Rejected, unchanged from R1b |

### General acceptance check

The fix is not a list of exceptions for those three rows. `NodeSchema.AuthoredOutput` retains the
raw converted declaration for every authored producer (`produces`, Goal/run-agent `output_schema`,
and ask `expect`, including source declarations). If normalized resolution rejects a path, the
field-map resolver replays the exact main ordering: declared `properties` lookup, direct-key lookup,
then array item traversal. A successful replay accepts the path and marks it unproven. The same
check is used for current/previous/best paths and result-contract fields. Paths rejected by both
remain errors. Descriptor-only tool outputs never receive this raw declaration.

Checked the permissive branch against `git show 766345807:internal/loop/dsl/refs/namespace.go`:
map/typed-map/slice/string handling and the empty-schema rule have the same acceptance behavior.
This preserves main acceptance by construction for the raw authored shapes, without claiming
that a finite test set enumerates every user definition. A separate main-source Go overlay ran the
same 33 shape/source cases: all expected acceptance/error classifications matched; the intentional
failures were only assertions that main should emit the new warnings (main emitted none).
Runtime assertions passed in that comparison too. Twelve added cases broaden the check beyond the
three reported rows: authored closed-object metadata, nested scalar metadata, array metadata, and
keyword collisions, each across all three authored producer locations. Missing undeclared fields
that main rejected remain negative controls. The 39 strict tool/view cases are unchanged.

Verification commands:

```sh
# Before production edits: expected failure in nine disputed authored cases; runtime assertions passed.
go test ./internal/loop -run '^TestLinterOutputShapeRuntimeAgreement$' -count=1 -v
# Main comparison: expected warning-only assertion differences, no acceptance classification mismatch.
go test -overlay=/tmp/ntrv-r1b-origin-overlay.json ./internal/loop -run '^TestLinterOutputShapeRuntimeAgreement$' -count=1 -v
# Full focused Loop packages, including 33 shape/source cases and 39 tool/view cases.
go test ./internal/loop/... -count=1 -v
go test -tags=integration ./internal/daemon -run '^Test(NativeResultViewsIntegration|LoopResultViewsIntegration)$' -count=1
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run ./internal/loop/...
git diff --check
```

The test-shape heuristic reports the same untouched inline-case diagnostic on
`TestLinterShouldRejectDeletedNoProgressParameters` documented in R1b; the new table cases retain
parallel `Should ...` subtests. No site content changed, so no Turbo rerun is needed in this round.
No global gate, race, E2E, external-provider run, push or PR. The existing change-impact audit remains
applicable: authored lint compatibility and QA evidence only; no wire, config, hook or data migration.

Final R1c results: all six Loop packages passed; the shape suite passed 33 source/shape subtests
(with current/previous/best checks inside each), and the unchanged tool/view suite passed 39.
The two named daemon integrations passed together (4.026s). Focused lint finished with `0 issues`;
one earlier long-line finding was corrected before that pass. `git diff --check` passed.
No remaining repair item is open. Pre-commit tasks are executed with `--no-stash`, preserving the
session's Git restriction; commit-message validation uses the same repository convention.
