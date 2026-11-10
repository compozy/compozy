# Harness token efficiency — final report

Date: 2026-11-09. Branch: `harness-token-efficiency`. Base: `e34e8332c`.
Initial round 2 implementation: `09d60b814`; the review correction is recorded below. Brief commit: `4e206f01f`.

Eight workstreams have implementation commits. W10 now corrects cumulative accounting and ships historical repair migrations under the approved round 2 rule. W6 moved to [#1432](https://github.com/compozy/compozy/issues/1432) and W8 moved to [#1435](https://github.com/compozy/compozy/issues/1435), each for a separate pull request. Delivered scope is W1–W5, W7, W9 and W10. W9's requested actual Claude first-call token measurement remains unverified; its implementation and provider-option contracts pass. This is not a claim that all ten workstreams or all acceptance measurements are complete.

| Workstream | Status | Commit | Result |
| --- | --- | --- | --- |
| W1 | done | `703a89c27` | Optional global/per-agent Claude auto-compaction window; default unchanged |
| W2 | done | `6de5c90f9` | Stable lineage reuse; one first-message situation block |
| W3 | done | `a3414008e` | Omit exact skill files already exposed natively to Claude |
| W4 | done | `f4635c9c8` | Smaller runtime reference router with required guidance retained |
| W5 | done | `be622495c` | Smaller hosted descriptions; identifiers and schemas unchanged |
| W6 | moved to #1432 | — | Compact/full native-tool result views will ship separately |
| W7 | done | `31eb6c52e` | Durable admission prevents replaying the same child-wake revision |
| W8 | moved to #1435 | — | Adapter-coordinated idle compaction experiment and implementation will ship separately |
| W9 | done; token measurement unverified | `1ba6a448b` | Lean Claude environment for system/triage and opted-in agents |
| W10 | done in round 2 | `09d60b814` | Durable per-turn increments, zero-charge drops, migrated historical repair/unknown |

## Paired binary measurements

Both binaries ran against an isolated Compozy home, provider home, workspace, port, and Unix socket. The provider-free ACP driver exercised the real daemon and hosted native tools. The fixture contained 100 workspace skills, 80 exposed as native Claude skill symlinks, plus the bundled Compozy skill. It used a general agent with the complete allowed native-tool policy.

The final base and candidate runs each began with a fresh session and completed two turns. Counts below are Unicode characters, not provider tokens. Tagged-block counts include opening and closing tags. Startup plain text excludes the four runtime/situation/catalog blocks, trailing user text, and surrounding whitespace.

| Metric | Base | Candidate | Reduction |
| --- | ---: | ---: | ---: |
| Whole first prompt | 41,950 | 24,854 | 40.8% |
| Whole second prompt | 11,770 | 1,344 | 88.6% |
| Situation block on second turn | 11,296 | 870 | 92.3% |
| Situation blocks in first prompt | 2 | 1 | 50.0% |
| Startup skill catalog | 10,263 | 2,583 | 74.8% |
| Startup plain text | 14,007 | 6,724 | 52.0% |

All four tool calls succeeded in both final runs. The mean result content stayed effectively unchanged; W6's 50% target was not met. The one-character create difference is incidental runtime data, not a reduction attributable to a feature.

| Native tool | Base content | Candidate content | Base raw JSON | Candidate raw JSON |
| --- | ---: | ---: | ---: | ---: |
| `session_status` | 13,114 | 13,114 | 13,092 | 13,092 |
| `session_create` | 1,586 | 1,585 | 1,564 | 1,563 |
| `session_archive` | 11,714 | 11,714 | 11,692 | 11,692 |
| `session_spawn` | 10,482 | 10,482 | 10,460 | 10,460 |
| Mean | 9,224 | 9,223.75 | 9,202 | 9,201.75 |

Raw JSON uses compact serialization of `tool_result.raw_output`. The content column is the model-facing content string. The enclosing serialized `tool_result`, which includes both representations and escaping, is recorded separately in `measurements.json`; it is not silently mixed with content size.

Preliminary lab attempts exposed fixture mistakes: missing model metadata and a `session_create` argument named `agent_name` instead of `agent`. Both were repaired, then both binaries were rerun with new sessions. Preliminary failures are retained in local diagnostics and excluded from the final successful paired measurements.

## Workstream details

### W1 — auto-compaction window

Added optional integer `providers.claude.auto_compact_window` and agent `auto_compact_window`, including config layering, validation, cloning, creation/editing/duplication, settings DTOs, API/CLI boundaries, and generated clients. New/load/resume options carry `CLAUDE_CODE_AUTO_COMPACT_WINDOW` as a complete decimal token count, including the input-only path. Unset sends nothing.

Accepted range: 100000–1000000, further capped by the provider at the model's window. A bare environment-variable number means tokens. The documented native 1M default is about 967k; leaving `autoCompactWindow` unset lets Claude select its model-tuned window. No default was changed. Retired `session.compaction` archive-on-load behavior remains separate.

Verification: T1 and L1 below passed; code generation and drift checks passed; generated client and Web typechecks passed. No model-side compaction at the threshold was run. Before/after: absent setting remains absent; configured counts reach all three ACP lifecycle requests. This is configuration behavior, not measured monetary savings.

Compatibility: additive public config/agent/settings/CLI DTO fields; generated OpenAPI and both SDKs co-shipped. No SQLite shape/data change and no migration. No open design question.

### W2 — situation reuse

Separated stable lineage from volatile session fields in prompt rendering. Existing section-signature invalidation handles policy changes, new ACP sessions, completed compactions, and context drops. ACP prompt assembly removes the redundant startup situation block only when it has registered live situation evidence; unrelated user text and uncertain delivery retain the safe full context.

Verification: T2 and L2 passed. The focused large-policy fixture measured 12,506 first-delivery versus 849 later characters; the paired daemon run measured 11,296 versus 870 on turn two. The first prompt contains one situation block. Native and fallback delivery/error paths are covered.

Compatibility: internal prompt format only. `/agent/context` and `AgentContextPayload` are unchanged. No migration. No open design question.

### W3 — native skill exposure

Checks the winning regular `SKILL.md` file with `os.SameFile` against Claude's native exposure roots. Matching names alone do not suppress a skill. The check is not cached, so removed or repointed exposures restore catalog entries. The registry and `skill_view` retain every authorized skill; other providers retain their catalog behavior.

Verification: T3 and L3 passed, including homonyms, removal/repointing, current registry state, and non-Claude behavior. The controlled 100-skill fixture shrank from 9,371 to 2,331 characters; the real paired startup catalog shrank from 10,263 to 2,583 (74.8%).

Compatibility: internal advertisement policy only; no tool identifiers, permissions, skill content, or public DTO changes. No migration. No open design question. Native Claude's own skill loading was not executed in the provider-free lab.

### W4 — startup instructions

Replaced duplicated router tables/index prose with compact routing bullets. Preserved reference loading, full retained-result pagination, monitor waits, withheld-capability boundaries, and Task authority requirements. All reference destinations remain reachable.

Verification: T4 and L4 passed. Router body: 13,044 to 5,761 characters (55.8%). Paired first-prompt plain text: 14,007 to 6,724 (52.0%), exceeding the 40% target.

Compatibility: editorial official skill and startup text; reference paths and tool IDs preserved. No migration. No open design question.

### W5 — hosted tool footprint

Removed repeated toolset/tag/search-hint boilerplate and a generic calling instruction from hosted descriptions while preserving authored descriptions, titles, and canonical IDs. Search metadata remains available. Provider-visible names, permission matching, routing, schemas, and eager/deferred policy are unchanged, so no compatibility alias is needed.

Verification: T5 and L5 passed. Across native descriptors, hosted description bytes fell from 104,119 to 53,170 (48.9%); eager description bytes fell from 4,728 to 3,178 (32.8%). These are description-byte measurements, not total protocol payload or tokenizer measurements.

Compatibility: descriptive metadata only; existing stable identifiers and input/output schemas are unchanged. No migration. No open design question.

### W6 — moved to #1432: compact/full result views

Current session tools expose stable full-session outputs. Dropping static fields from the default response would break existing no-argument callers. SD-013 requires keeping the old public shape for one release after a replacement ships when a lossless migration is unavailable. The current interface has no response-version negotiation that would identify compact-capable callers. Keeping the old default and adding an opt-in compact view would be a staged prerequisite, not the requested compact default or 50% result-size outcome; the brief prohibits shipping partial workstreams.

No production/test/doc contract files or migration were changed for W6. No W6-specific tests or lint were run. The real four-tool measurements above demonstrate unchanged behavior and the unmet target.

The operator moved this workstream to [#1432](https://github.com/compozy/compozy/issues/1432), covering compact/full result views for all native tools in a separate PR. Its public-surface compatibility transition is outside this branch.

### W7 — redundant wake admission

Found that child-attention deduplication was in memory and could lose its remembered revision after restart/eviction. The wake bridge now checks the existing durable synthetic-wake ledger and submits the same `WakeEventID` as `MessageID`, letting queue admission enforce idempotency across races and replay. Ledger errors are surfaced; an unseen revision still receives its own wake.

Verification: T7 and L7 passed. Controlled delivery of one revision three times results in one admission; a later revision still admits once. No production-wide turn reduction is claimed. Existing monitor coalescing (bounded batch of 64), reply-watch guarantees, ordinary monitor delivery, and informative attention wakes were left unchanged. A trivial-looking response alone is not proof that a wake was redundant.

Compatibility: existing public wake/event shapes and durable queue identity are reused; no schema/data migration or new delay. No open design question. Historical wake frequencies were supplied by the brief; they were not remeasured from operator data.

### W8 — moved to #1435: adapter-owned idle compaction

The operator moved W8 to [#1435](https://github.com/compozy/compozy/issues/1435): experiment first, then a separate PR. The earlier round 2 proposal approved bounded retention for adapter-owned warm-cache compaction. The remaining blocker is technical: no authoritative armed/eligibility/deadline lifecycle is exposed by the verified Claude adapter. No daemon compaction or partial retention heuristic shipped. See the round 2 W8 section below for the complete survey, source evidence, exact prerequisite and unrun checks. The earlier off-by-default proposal is superseded.

### W9 — lean environment

Added optional per-agent `lean_environment`. System and triage sessions select the lean option; user and ordinary spawned sessions preserve their default unless their definition opts in. Claude options disable auto-memory and claude.ai connectors and set `strictMcpConfig`, while retaining Compozy-configured MCP, repository `CLAUDE.md`, native skills, built-in tools, settings hooks, and the established input-only title behavior.

Verification: T9 and L9 passed. Tests cover new/load/resume metadata, ordinary/default and explicit choices, one-shot role defaults, other providers, and preservation of repository instructions/tool/skill options. Contract generation, drift checks, client build/typecheck, and a forced uncached Web typecheck against the final client passed.

**Actual first-call Claude tokens before/after for triage/system were not measured.** The provider-free ACP lab cannot expose Claude's private system prompt or operator environment cost, and its mock command does not execute the Claude bridge. No savings percentage is claimed for W9. This required quantitative acceptance result remains unverified despite the completed implementation and focused contract coverage.

Compatibility: additive optional agent field co-shipped through API/CLI and generated SDKs; user-session default unchanged. Internal one-shot launch policy changes intentionally. No SQLite shape/data change or migration. No open design question; remaining verification needs an authorized provider-backed comparison with representative operator resources.

### W10 — completed in round 2

Commit `09d60b814` normalizes cumulative reports, preserves raw evidence across restart and repairs historical costs through session migration 11 and global migration 199. The operator subsequently approved zero charge on a counter drop, retaining the lower raw report as the next baseline. Accepted undercount risks are one turn per drop and an undetectable high/equal first report after a counter restart. See the complete round 2 implementation, survey, measurements, checks and migration notes below; the round 1 blocker is resolved under that decision.

## Round 1 focused verification commands

All commands below are from this worktree. Each listed final test/lint command passed with zero warnings/errors. Earlier failures were repaired before commit: formatting/lint issues, a temporary AgentDef size increase, DTO mapping errors, and missing generated client build output. W7's original broader selector included `internal/session/inputqueue` with no matching tests; the final T7 deliberately targets the two owning suites and passes without that warning.

```sh
# T1 — config layering/validation, ACP new/load/resume, API/settings projection
go test ./internal/config ./internal/acp -run 'TestAutoCompactWindow|TestClaudeAutoCompactWindow|TestStartDisablesClaudeProviderInternalBackgroundWakes' -count=1
go test ./internal/config ./internal/acp ./internal/api/core ./internal/settings -run 'TestAutoCompactWindow|TestToolConfigPathPolicy|TestClaudeAutoCompactWindow|TestAgentPayloadEffectiveRuntime|TestProviderSettings' -count=1
# L1
COMPOZY_GO_LINT_SCOPES='./internal/config ./internal/acp ./internal/settings ./internal/api/core ./internal/api/contract ./internal/cli ./internal/session' make go-lint

# T2 — prompt lifecycle and section reuse
go test ./internal/situation ./internal/acp -run 'TestAugment|TestRenderPrompt|TestPromptCompactsDeliveredSections' -v -count=1
go test ./internal/situation -run TestAugmentCompactsRepeatedSituationSections -v -count=1
# L2
COMPOZY_GO_LINT_SCOPES='./internal/situation ./internal/acp' make go-lint

# T3 — native exposure and catalog behavior
go test ./internal/daemon ./internal/skills -run 'TestComposedAssemblerFiltersProviderNativeSkillsAtStartup|TestHarnessSkillInjection|TestResolveSessionNativeSkillRoots|TestNewSkillsCatalogAugmenterUsesCurrentRegistryStatePerPrompt|TestSkillsCatalogAugmenterFiltersBeforeCatalogSignature|TestBuildCatalog' -v -count=1
# L3
COMPOZY_GO_LINT_SCOPES='./internal/daemon' make go-lint

# T4 — router/reference contract and startup delivery
go test ./skills ./internal/daemon ./internal/acp -run 'TestLoadContentReturnsSkillBody|TestBundledReferencesAreEmbeddedAndReadable|TestComposedAssemblerAssembleStartupLoadsBundledToolsSectionDescriptor|TestComposedAssemblerRuntimeEnvelopeEndTurnGuidance|TestPromptCompactsDeliveredSections' -v -count=1
# L4
COMPOZY_GO_LINT_SCOPES='./skills' make go-lint

# T5 — hosted descriptor schemas and routing
go test ./internal/mcp ./internal/toolmeta -run 'TestHostedProxy|TestApplyHostedTools|TestHostedNativeToolNames' -v -count=1
# L5
COMPOZY_GO_LINT_SCOPES='./internal/mcp' make go-lint

# T7 — durable replay and existing informative-wake delivery
go test ./internal/daemon ./internal/session -run 'TestSessionWakeBridge|TestMonitorSessionWaker|TestSpawnWake|TestEnqueueSyntheticWakeRunsOnceWithoutADeliveryStream' -count=1
# L7
COMPOZY_GO_LINT_SCOPES='./internal/daemon' make go-lint

# T9 — lean runtime/config and changed boundary consumers
go test ./internal/config ./internal/acp ./internal/session ./internal/api/core ./internal/cli -run 'TestAgentLeanEnvironment|TestClaudeLeanEnvironment|TestSessionLeanEnvironment|TestAgentPayloadEffectiveRuntime|TestAgentCreate|TestAgentUpdate' -count=1
# L9
COMPOZY_GO_LINT_SCOPES='./internal/config ./internal/acp ./internal/session ./internal/api/core ./internal/api/contract ./internal/cli' make go-lint

# W1/W9 owning generators and generated consumers
make codegen
make codegen-check
bun x turbo run build --filter=@compozy/client --only
bun x turbo run typecheck --filter=@compozy/client --filter=./web --only
bun x turbo run typecheck --filter=./web --only --force

# Final branch whitespace validation
git diff --check e34e8332c..HEAD
```

`make go-lint` ran the named packages under normal and integration build tags, with zero issues. No full suite, race suite, repository-wide lint, `make gate`, `make gate-full`, or E2E was run. Generated files changed through `make codegen`. The temporary base source archive initially caused `codegen-check` to detect a foreign sqlc import under `.tmp`; removing that task-owned archive and rerunning produced a clean pass.

Before each implementation commit, staged checks ran with `bunx --no-install lint-staged --no-stash --no-revert`; commits bypassed the hook's automatic stashing only after those checks passed. No prohibited destructive Git command was used.

## QA artifacts and impact

Local evidence root:
`.tmp/harness-token-efficiency/compozy-harness-token-efficiency-20261109-211259-251277-lab/qa-artifacts/qa/`.
It contains `bootstrap-manifest.json`, `measurements.json`, final base/candidate histories and ACP diagnostics, `candidate-api-usage-turns.json`, `focused-checks.json`, `journey-log.jsonl`, `verification-report.md`, `qa-audit-report.json`, and `teardown.json`. Diagnostics remain local and are not committed because they contain ephemeral runtime metadata.

Built the base binary from `git archive e34e8332c`, the candidate from the implementation tree, and `./internal/testutil/acpmock/cmd/acpmock-driver`. `python3 .tmp/harness-lab.py base run` and the corresponding `candidate run` both exited zero. A separate UDS GET of the candidate usage-turns endpoint returned two prompt records; the later situation span independently reports 870 bytes.

The targeted evidence auditor passed with zero blockers/warnings:

```sh
python3 .agents/skills/eng/eng-real-scenario-qa/scripts/audit-qa-evidence.py --qa-output-path .tmp/harness-token-efficiency/compozy-harness-token-efficiency-20261109-211259-251277-lab/qa-artifacts --strict --explain
python3 .agents/skills/eng/eng-qa-bootstrap/scripts/teardown-qa-env.py --manifest .tmp/harness-token-efficiency/compozy-harness-token-efficiency-20261109-211259-251277-lab/qa-artifacts/qa/bootstrap-manifest.json --json
```

Final teardown reports `clean: true`, no surviving processes, and no resource errors. Its first attempt hit `EPERM` because the helper used `unlink` on a real lab `.codex/skills` directory rather than a symlink. Removing only that task-owned temporary directory and rerunning the helper resolved it. The repository helper was not changed.

Affected QA scenario documents:

- `MS-provider-settings-model-delta-roundtrip.md` — W1/W9 configuration and launch contracts.
- `RT-spawn-budget-context.md` — W2 prompt lineage and first delivery.
- `ET-skill-session-source-injection.md` — W3 exact native exposure behavior.
- `ET-compozy-official-skill-discovery.md` — W4 router/reference reachability.
- `ET-compozy-native-tool-invocation.md` — W5 hosted descriptor compatibility.
- `RT-session-spawn-wake.md` — W7 duplicate revision admission.

The focused suites and provider-free paired run verify the changed contracts above; complete Web/manual walks of every scenario were not performed. No production profile, root checkout, operator configuration, credentials, historical ledger, or live provider transcript was mutated. No push or PR was performed, and `docs/_memory/change-impact.md` was not edited.

Cross-surface impact: W1/W9 add configuration and agent DTO fields consumed through existing settings/API/CLI surfaces and generated clients; no Web redesign. W2/W3/W4 change prompt/catalog instructions only. W5 changes hosted description text while retaining routing/authentication/approval/schema contracts. W7 reuses the existing synthetic queue ledger and wake IDs. Official skill references and site docs accompany the changes. Workspace/profile isolation was exercised in the lab. Round 1 did not change SQLite shape/data. Round 2 W10 adds session/global migrations and a monetary projection upgrade, detailed below.

Unverified: actual Claude first-call token reduction for W9; provider-side W1 compaction thresholds; paid/model-side token or dollar savings; real warm-cache deadlines and long-idle resumes; a production daemon restart during duplicate wake delivery; live non-Claude adapter monetary behavior; production-ledger repair (isolated W10 restart/migration tests now pass); full CI/gates/race/E2E and complete Web scenario walks.

## Exact changed files by workstream

The following manifest is derived from each implementation commit; it includes all generated files. Blocked workstreams changed no files.

### W1 — 703a89c27

```text
docs/qa/scenarios/MS-provider-settings-model-delta-roundtrip.md
internal/acp/claude_session_meta.go
internal/acp/client_start_contract_test.go
internal/api/contract/agent_definitions.go
internal/api/contract/agent_observe_payloads.go
internal/api/contract/settings_collection_payloads.go
internal/api/core/agent_conversions.go
internal/api/core/agent_create.go
internal/api/core/agent_identity_authoring.go
internal/api/core/conversions_settings_catalog.go
internal/api/core/settings_provider_request.go
internal/cli/agent_mutate_requests.go
internal/config/agent.go
internal/config/agent_clone.go
internal/config/agent_create.go
internal/config/agent_edit.go
internal/config/agent_resource.go
internal/config/agent_runtime_options.go
internal/config/agent_validation.go
internal/config/config_test.go
internal/config/merge.go
internal/config/merge_provider.go
internal/config/provider.go
internal/config/provider_clone_helpers.go
internal/config/provider_merge.go
internal/config/provider_resolve.go
internal/config/provider_resolved_validation.go
internal/config/provider_validation.go
internal/config/tool_surface.go
internal/config/tool_surface_test.go
internal/session/provider_runtime_config.go
internal/settings/collection_provider_write.go
internal/settings/collection_settings_map.go
internal/settings/models_provider.go
internal/settings/provider_models_projection.go
openapi/compozy.json
packages/site/content/docs/configuration/config-toml.mdx
sdk/client/src/generated/openapi/compozy-openapi.d.ts
sdk/client/src/generated/sdk/types.gen.ts
sdk/go/client/types_001_gen.go
sdk/go/client/types_002_gen.go
sdk/go/client/types_003_gen.go
sdk/go/client/types_004_gen.go
sdk/go/client/types_005_gen.go
sdk/go/client/types_006_gen.go
sdk/go/client/types_007_gen.go
sdk/go/client/types_008_gen.go
sdk/go/client/types_009_gen.go
sdk/go/client/types_010_gen.go
sdk/go/client/types_011_gen.go
sdk/go/client/types_012_gen.go
sdk/go/client/types_013_gen.go
sdk/go/client/types_014_gen.go
sdk/go/client/types_015_gen.go
sdk/go/client/types_016_gen.go
sdk/go/client/types_017_gen.go
sdk/go/client/types_018_gen.go
sdk/go/client/types_019_gen.go
sdk/go/client/types_020_gen.go
sdk/go/client/types_021_gen.go
sdk/go/client/types_022_gen.go
sdk/go/client/types_023_gen.go
sdk/go/client/types_024_gen.go
sdk/go/client/types_025_gen.go
sdk/go/client/types_026_gen.go
sdk/go/client/types_027_gen.go
sdk/go/client/types_028_gen.go
sdk/go/client/types_029_gen.go
sdk/go/client/types_030_gen.go
sdk/go/client/types_031_gen.go
skills/compozy/references/configuration.md
```

### W2 — 6de5c90f9

```text
docs/qa/scenarios/RT-spawn-budget-context.md
internal/acp/client_prompt_contract_test.go
internal/acp/prompt_sections.go
internal/situation/render.go
internal/situation/service_test.go
```

### W3 — a3414008e

```text
docs/qa/scenarios/ET-skill-session-source-injection.md
internal/daemon/composed_assembler_test.go
internal/daemon/skill_injection_policy.go
skills/compozy/references/tools-and-skills.md
```

### W4 — f4635c9c8

```text
docs/qa/scenarios/ET-compozy-official-skill-discovery.md
skills/bundled_test.go
skills/compozy/SKILL.md
```

### W5 — be622495c

```text
docs/qa/scenarios/ET-compozy-native-tool-invocation.md
internal/mcp/hosted_proxy.go
internal/mcp/hosted_proxy_test.go
skills/compozy/references/tools-and-skills.md
```

### W7 — 31eb6c52e

```text
docs/qa/scenarios/RT-session-spawn-wake.md
internal/daemon/session_wake_bridge.go
internal/daemon/session_wake_bridge_test.go
skills/compozy/references/runtime-operations.md
```

### W9 — 1ba6a448b

```text
docs/qa/scenarios/MS-provider-settings-model-delta-roundtrip.md
internal/acp/claude_session_meta.go
internal/acp/client_start_contract_test.go
internal/acp/client_test_support_test.go
internal/acp/types.go
internal/api/contract/agent_definitions.go
internal/api/contract/agent_observe_payloads.go
internal/api/core/agent_conversions.go
internal/api/core/agent_create.go
internal/api/core/agent_identity_authoring.go
internal/cli/agent_mutate_requests.go
internal/config/agent.go
internal/config/agent_clone.go
internal/config/agent_create.go
internal/config/agent_edit.go
internal/config/agent_resource.go
internal/config/agent_runtime_options.go
internal/config/config_test.go
internal/config/provider_resolve.go
internal/session/manager_start_env.go
internal/session/manager_start_test.go
openapi/compozy.json
packages/site/content/docs/agents/providers.mdx
sdk/client/src/generated/openapi/compozy-openapi.d.ts
sdk/client/src/generated/sdk/types.gen.ts
sdk/go/client/types_001_gen.go
sdk/go/client/types_002_gen.go
sdk/go/client/types_005_gen.go
sdk/go/client/types_006_gen.go
skills/compozy/references/agent-definitions.md
```

## Round 2 — adapter surveys and delivery evidence

Round 2 implements the operator-approved counter heuristic and historical repair in W10. The W8 adapter survey identified an observation-contract gap; W8 is now tracked separately in #1435. W6 remains outside this branch and is tracked in #1432. The round 2 brief is `docs/prompts/20261109-1845_harness-token-efficiency-round2.md`.

### W10 — committed: per-turn monetary accounting

Commit: `09d60b814` (`fix: normalize reported session costs and repair historical totals`).

Usage is normalized in the session prompt owner before canonical persistence, observers, transcript projection, and stream delivery. Both individual and coalesced event paths use the same function. For each currency, the previous raw report comes from bounded pages of the canonical event ledger, including archived turns; it does not depend on the rebuildable `token_usage` projection. A turn fixes that baseline so repeated usage notifications and the final notification do not subtract twice. Raw amounts remain available as `ReportedCostAmount` in canonical evidence and `reported_cost_amount` in the per-turn projection.

ACP cumulative reporting is the default for every provider, including unknown adapters. Provider capability data explicitly identifies the verified cumulative adapters and has an explicit `per_turn` mode for exceptions. No builtin adapter was proven to report per-turn monetary amounts, so none is falsely declared an exception. Token counters are not normalized by this change.

The operator decision of 2026-11-10 supersedes the original restart rule: charge `reported - previous` for a non-decreasing report and zero for a lower report. Preserve every raw report and use the new, lower amount as the next turn's baseline. This avoids recharging accumulated spend when a restored counter lags. **Accepted limitations:** the drop turn is not counted (undercount of at most one turn per drop); a restart whose first report equals or exceeds the previous total remains indistinguishable from continuation. Neither rule proves exact invoice reconciliation. The raw baseline survives manager restart and runtime reattachment; explicitly per-turn adapters are unchanged.

#### Migration and compatibility

- Session migration **11** adds the raw report column, derives turn increments in canonical order per currency, and rewrites the cost portion of canonical usage payloads while retaining the original amount. Other event fields, IDs, sequence, archive state, token values and timestamps remain intact. Existing rewind snapshots are marked stale for their owning rebuild path.
- Transcript projection version **4** replays affected cost-bearing assistant entries, preserving message identity and unrelated projection state. Persisted transcript parts therefore stop showing the cumulative report as a turn cost.
- Global migration **199** adds a durable repair marker only when agent-reported aggregates exist. Before opening the runtime, the migration owner captures the original agent-reported session and daily targets, then invalidates and repairs only those targets in one global transaction. It opens available retained session ledgers through their normal guarded/migrating owner and recomputes complete, attributable completed-turn totals. Subscription-included, estimated and all other non-agent-reported aggregate cost fields remain exactly as stored. Daily totals require matching day/profile/workspace/agent and turn count. For the targeted agent-reported aggregates, missing, unreadable, pending-clear, partial, mixed-currency or otherwise uncertifiable evidence becomes `unknown` with no amount/source. All non-cost aggregate columns are preserved.
- One global transaction commits repaired totals and the completion marker. An interrupted open retries the pending repair; session migrations are idempotent. A fresh database has no repair work. Prior migration bytes/checksums are unchanged; the new migrations, Atlas sums and sqlc artifacts ship together.
- Public CLI/HTTP/UDS/tool identifiers and DTO shapes are unchanged. `cost_amount` keeps its promised per-turn meaning; incorrect values are corrected. No compatibility alias or deprecation period is needed. No config key changed. Existing profile/workspace guards remain the data boundary. `compozy__session_usage`, CLI usage/context, transcript messages, prompt streams and task cost views consume the corrected owners; they do not add independent subtraction.
- Generated persistence files were produced through the owning generators. `make codegen` passed. Appending the data tails initially exposed checksum validation before regeneration; the Atlas Goose-directory checksum API regenerated only the two owned manifests, followed by `make codegen`. No released migration was edited. `make codegen-check` was not run in round 2.

#### Measurements

These are deterministic accounting fixtures, not fresh measurements of operator billing or token savings.

| Fixture | Previous additive result | Correct recorded turns | Correct total |
| --- | ---: | --- | ---: |
| Restored counter across manager restart: `1, 3, 4` | 8 | `1, 2, 1` | 4 |
| Lower counter across runtime restarts: `1, 3, 0.5, 2` | 6.5 | `1, 2, 0, 1.5` | 4.5 |
| Lagging restored counter, live and migrated: `10, 12, 11.5, 12.5` | 46 | `10, 2, 0, 1` | 13 |
| Explicit per-turn reports: `1, 3, 0.5` | 4.5 | unchanged | 4.5 |
| Missing historical ledger with stored aggregate 6.5 | `actual`, 6.5 | unavailable | `unknown`, no amount |

The brief's production example (59,163.13 stored versus a maximum raw counter of 300.9) was not migrated on the operator's installation. A maximum alone is not the correct total when counters reset; this implementation recomputes the ordered increments instead.

#### W10 adapter cost survey

Inventory: `internal/config/provider_builtin.go`, 15 builtin provider IDs across 10 adapter families. Source inspection was performed on 2026-11-09. “Unknown” means the reviewed primary documentation does not establish monetary reporting semantics; it does not mean the adapter is incapable of reporting cost. All such cases follow ACP's cumulative contract if a monetary report arrives. The [ACP schema](https://agentclientprotocol.com/protocol/v1/schema#cost) defines cumulative session cost and also permits non-monetary units such as credits.

| Builtin provider(s) / adapter | Established monetary behavior | CompozyOS handling / evidence |
| --- | --- | --- |
| `claude` / claude-agent-acp 0.102.0 | Cumulative: forwards SDK `total_cost_usd`; resume may restore or reset the counter | Explicit cumulative. [Adapter result handling](https://github.com/agentclientprotocol/claude-agent-acp/blob/main/src/acp-agent.ts), [SDK cost lifecycle](https://code.claude.com/docs/en/agent-sdk/cost-tracking). Installed distribution was also inspected. |
| `opencode` / `opencode acp` | Cumulative session dollars in `usage_update`; turn token usage is a separate shape | Explicit cumulative. [ACP documentation source](https://github.com/anomalyco/opencode/blob/main/services/site/src/docs/content/cli/acp.mdx), [ACP turn adapter](https://github.com/anomalyco/opencode/blob/main/packages/cli/src/acp/turn.ts). |
| `codex` / codex-acp | Reviewed usage update reports context used/size, without `cost` | Default cumulative if later supplied; preserve absent cost. [Event handler](https://github.com/agentclientprotocol/codex-acp/blob/main/src/CodexEventHandler.ts). |
| `gemini` / Gemini CLI ACP | Reviewed emitter reports context used/size, without `cost` | Default cumulative if later supplied. [ACP session emitter](https://github.com/google-gemini/gemini-cli/blob/main/packages/cli/src/acp/acpSession.ts). |
| `qwen-code` / Qwen Code ACP | Reviewed live usage emitter reports context used/size, without `cost` | Default cumulative if later supplied. [MessageEmitter](https://github.com/QwenLM/qwen-code/blob/main/packages/cli/src/acp-integration/session/emitters/MessageEmitter.ts). |
| `pi`, `openrouter`, `zai`, `moonshot`, `vercel-ai-gateway`, `xai` / pi-acp | `toUsageUpdate` reports context used/size, without monetary `cost`; backend billing is not automatically ACP cost | All six use the ACP default if cost is supplied. [Session emitter](https://github.com/svkozak/pi-acp/blob/main/src/acp/session.ts). |
| `kimi-cli` / `kimi acp` | Unknown monetary contract in reviewed public ACP documentation | ACP default; no invented per-turn exception. [ACP reference](https://moonshotai.github.io/kimi-code/en/reference/kimi-acp). |
| `copilot` / Copilot CLI ACP | Unknown monetary contract; a `/usage` command does not establish `Cost.amount` semantics | ACP default. [ACP server reference](https://docs.github.com/en/copilot/reference/copilot-cli-reference/acp-server). |
| `cursor` / Cursor ACP | Unknown monetary contract in reviewed ACP documentation | ACP default. [ACP reference](https://cursor.com/docs/cli/acp). |
| `agy` / Antigravity ACP | Unknown monetary contract in reviewed integration documentation | ACP default. [Official ACP integration](https://antigravity.google/docs/ide/extensions/acp). |

#### W10 focused commands and results

All final commands below passed. No package reported “no tests to run”. Earlier failed iterations are not passing evidence: a missed coalesced usage path, incomplete telemetry fixture setup, timestamp string-format comparison, a transcript fixture without an assistant message, and lint findings were repaired before the commit. Existing assertions on preservation were retained.

```sh
# T10a: live/restarted accounting, observer provenance, old/fresh/reopen/equivalent
# migration streams, nullable usage, and transcript monetary reprojection.
go test ./internal/session ./internal/store ./internal/store/sessiondb ./internal/store/globaldb ./internal/observe \
  -run 'TestPromptCostAccounting|TestProductionMigrationStreams|TestGlobalDBTokenCostMigration|TestGlobalDBUsageCostRepair|TestSessionDBUsageCostMigration|TestSessionDBRecordTokenUsageStoresNullableFieldsAsNULL|TestTranscriptProjectionUpgrade|TestOnAgentEventProjectsTruthfulCostProvenance' -count=1

# T10b: existing integration suites, real SQLite and lifecycle owners, fake ACP I/O.
go test -tags=integration ./internal/session ./internal/task \
  -run '^(TestManagerIntegrationFullLifecycle|TestTaskManagerRunDetailUsesPersistedRuntimeDataIntegration)$' -count=1

# T10c: existing public readers and provider usage decoding.
go test ./internal/cli ./internal/api/core ./internal/acp \
  -run '^(TestSessionUsageCommandPreservesCostProvenance|TestSessionUsageEndpoint|TestWriteUsageChangedEvents|TestTokenUsageParsing)$' -count=1

# L10: both normal and integration-tag lanes, 0 issues.
COMPOZY_GO_LINT_SCOPES='./internal/config ./internal/acp ./internal/session ./internal/store ./internal/store/globaldb ./internal/store/sessiondb ./internal/transcript' make go-lint

make codegen
git diff --check
```

Source formatting used `golangci-lint fmt` on the explicit changed non-generated Go files. Markdown formatting used `oxfmt` on the changed official reference and docs where supported. The commit hook was not invoked because it runs repository-wide `make fmt` and automatic stash; equivalent applicable focused checks above were run directly. This avoided both unrelated mutation and forbidden destructive Git.

#### W10 release-note text

> Fixed cumulative provider costs being added repeatedly as per-turn spend. Session, transcript, CLI, task and daily cost views now use turn increments, charging zero for a counter drop and using its retained raw report as the next baseline. The upgrade preserves raw reports and rebuilds agent-reported historical costs from retained session ledgers. Included and estimated costs remain unchanged. Missing or incomplete evidence for agent-reported costs is shown as unknown; token counts, turn counts and timestamps are preserved. A drop omits that one turn's cost (at most one turn per drop). A counter restart whose first report equals or exceeds the previous total can still undercount because it cannot be distinguished from continuation.

### W8 — survey retained for #1435: adapter-owned eligibility/deadline observation

Status: **blocked, no implementation commit**. The operator decision is not pending: the requested policy is on by default, with an off switch, bounded retention and no Compozy-issued compaction. No partial retention rule, placeholder capability, new setting or migration was shipped.

The installed Claude ACP adapter 0.102.0 already owns SDK idle compaction. Its `IdleCompaction` class records request time and TTL, evaluates current merged settings, rate-limit state, context size and busy state, then arms or skips its private timer. The host callback surface exposes only `isBusy`, `contextTokens`, `settings`, `compact` and `log`. Arm/disarm/skip/failure details are emitted as logger text; no structured client eligibility/deadline transition is emitted. [Adapter idle-compaction implementation](https://github.com/agentclientprotocol/claude-agent-acp/blob/main/src/idle-compaction.ts), [host integration](https://github.com/agentclientprotocol/claude-agent-acp/blob/main/src/acp-agent.ts).

The `_claude/sdkMessage` extension can forward raw SDK responses, but it does not forward the adapter timer's armed state or the current merged settings decision. Reconstructing this state from selected usage fields would duplicate a private policy and can retain a session that the adapter will not compact. Retaining all large Claude sessions until minute 54 would violate the explicit unchanged-release requirement for wrong TTL, disabled settings, rate limits and other ineligible states. Parsing diagnostic logs as lifecycle authority would violate the repository's structured transition ownership rule.

A second integration gap is visible locally: `internal/acp/compaction.go` invalidates prompt occupancy on terminal compaction before emitting the event, but `AgentProcess.emitPromptEvent` in `internal/acp/agent_process_prompt.go` drops delivery when no active Compozy prompt exists. An adapter-owned idle turn therefore needs a durable idle-runtime event route as well as the release hold; forwarding only active prompt events cannot satisfy the requested outcome visibility.

**Exact missing prerequisite:** an adapter-supported structured observation/query contract for current eligibility, armed deadline, disarm/lapse and compaction completion/failure, with session/operation correlation. The release owner can then consume generic provider capability data and enforce a bounded hold without reimplementing the adapter's private decisions. The idle event route must persist outcomes and preserve existing prompt-section invalidation. This prerequisite is outside the present Compozy repository; no upstream adapter mutation was authorized or published.

#### W8 adapter cache / compaction survey

The same 15 provider IDs are covered. Backend cache policies are not sufficient evidence that an ACP adapter has a warm-cache timer. “Not verified” below means no such contract was established from the reviewed primary sources; it is not a universal claim that future or private builds lack the feature.

| Builtin provider(s) / adapter | Cache expiry information | Compaction mechanism, conditions and controls | Release-contract finding / sources |
| --- | --- | --- | --- |
| `claude` / claude-agent-acp 0.102.0 | Adapter distinguishes observed 1h and 5m cache creation; unknown TTL is ineligible. Deadline is measured from the first main-thread response sighting, not Compozy turn end. | Default timer 54m; fires only for 1h TTL, context >=200k (floor100k), idle/not awaiting input, rate status unknown or allowed, not already compacted, and enabled settings/env. Skips >60s after deadline. Controls: `CLAUDE_AGENT_ACP_IDLE_COMPACT_DELAY_MS`, `CLAUDE_CODE_IDLE_COMPACT_MIN_TOKENS`, `idleCompaction`, `autoCompactEnabled`, `DISABLE_COMPACT`, `DISABLE_AUTO_COMPACT`. Settings env overrides launch env for CLI flags/threshold. | Verified timer; private eligibility and deadline, logger-only arm/disarm. [Adapter implementation](https://github.com/agentclientprotocol/claude-agent-acp/blob/main/src/idle-compaction.ts). Native [Claude setting](https://code.claude.com/docs/en/settings-reference#idlecompaction) targets attended CLI/VS Code/Desktop sessions; the adapter implements the SDK path separately. |
| `codex` / codex-acp | No adapter-exposed cache-expiry deadline established; actual cache retention depends on model/backend policy. | Context-driven auto/async compaction controls exist; `model_auto_compact_token_limit` is a context threshold, not an idle cache deadline. | No verified warm-idle eligibility/deadline contract. [Codex configuration schema](https://github.com/openai/codex/blob/main/codex-rs/core/config.schema.json), [context-window owner](https://github.com/openai/codex/blob/main/codex-rs/core/src/session/context_window.rs). |
| `gemini` / Gemini CLI ACP | Effective backend cache TTL/deadline is not exposed by the reviewed ACP usage emitter. | Context pressure compression via `model.compressionThreshold`; no verified warm-idle timer contract. | Existing release policy remains applicable. [Settings](https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/settings.md), [ACP session](https://github.com/google-gemini/gemini-cli/blob/main/packages/cli/src/acp/acpSession.ts). |
| `opencode` / OpenCode ACP | Provider-dependent prompt cache; reviewed ACP contract does not publish a warm-idle expiry deadline. | Automatic context-limit compaction, manual request, and experimental `background_compaction` starting before the context threshold. Background summary generation can reuse cache but is driven by context growth. | Background compaction is not evidence of an idle-expiry timer. No verified release-hold contract. [Compaction documentation](https://opencode.ai/docs/compaction/). |
| `qwen-code` / Qwen Code ACP | Backend-dependent caching; no authoritative idle expiry deadline in reviewed ACP usage updates. | `autoCompactThreshold` controls context pressure. `clearContextOnIdle.toolResultsThresholdMinutes` controls pruning old tool results; pruning is not a warm-cache summarization request. | No verified adapter idle-compaction lifecycle contract. [Settings](https://qwenlm.github.io/qwen-code-docs/en/users/configuration/settings/), [cache-sharing design](https://github.com/QwenLM/qwen-code/blob/main/docs/design/chat-compression-cache-sharing.md). |
| `pi`, `openrouter`, `zai`, `moonshot`, `vercel-ai-gateway`, `xai` / pi-acp | TTL depends on the selected Pi backend/model; reviewed ACP emitter exposes no expiry deadline. | Adapter delegates manual `/compact` and automatic compaction control to Pi; no warm-idle timer contract established. | No verified eligibility/deadline signal for any of these six provider IDs. [Adapter README](https://github.com/svkozak/pi-acp/blob/main/README.md), [session integration](https://github.com/svkozak/pi-acp/blob/main/src/acp/session.ts). |
| `kimi-cli` / Kimi ACP | Effective prompt cache deadline not established in reviewed public ACP/session docs. | Automatic compression when context requires it and manual `/compact`; no verified warm-idle timer or controls. | No release-hold contract established. [Sessions guide](https://github.com/MoonshotAI/kimi-cli/blob/main/docs/en/guides/sessions.md), [ACP reference](https://moonshotai.github.io/kimi-code/en/reference/kimi-acp). |
| `copilot` / Copilot CLI ACP | No ACP cache TTL/deadline guarantee established. | Automatic background context compaction and manual `/compact`; documented context thresholds are not idle deadlines. | No verified warm-idle lifecycle contract. [Context management](https://docs.github.com/en/copilot/concepts/prompting-and-context/context-management), [ACP reference](https://docs.github.com/en/copilot/reference/copilot-cli-reference/acp-server). |
| `cursor` / Cursor ACP | No authoritative cache TTL/deadline found in reviewed ACP documentation. | No warm-cache idle compaction conditions/controls established for this ACP adapter. | Unknown, not inferred from other Cursor surfaces. [ACP reference](https://cursor.com/docs/cli/acp). |
| `agy` / Antigravity ACP | No authoritative cache TTL/deadline found in reviewed ACP integration documentation. | No warm-cache idle compaction conditions/controls established for the shipped ACP server. | Unknown, not inferred from the interactive product. [Official integration](https://antigravity.google/docs/ide/extensions/acp). |

W8 measurements: no after measurement or savings claim. The source proves why a 30m Compozy idle release can pre-empt the Claude adapter's 54m timer; it does not prove eligibility for a specific operator session. The supplied 28–58m warm resumes and later-resume proportions remain input evidence, not a new measurement.

W8 files changed: report only. Compatibility/migrations: no runtime/public/config/schema change. W8 focused hold/lapse/off-switch tests and provider-backed shortened-delay run: **not run**, because no safe implementation of the required authoritative hold contract is available. The existing `session.supervision.idle_release_after` behavior is unchanged. Config/site/skill documents do not advertise an unimplemented on-by-default policy.


### Round 2 remaining questions and unverified work

- No additional operator decision is needed for W10. Its accepted zero-charge drop and undetectable restart limitations remain documented; no reset epoch is invented.
- W8 moved to #1435 for an experiment and separate PR; W6 moved to #1432 for compact/full native-tool result views. Neither is part of this delivery.
- No production user database was modified or production billing series remeasured. Migration proof uses isolated SQLite fixtures, including real owner open/reopen and fresh-versus-replay equivalence.
- No paid Claude warm-idle experiment, no live adapter matrix, and no browser/HTTP/UDS QA walkthrough was run in round 2. Existing integration suites ran against real storage/lifecycle owners with a fake ACP boundary. The three cost QA scenarios were updated, not marked newly walked.
- No `make codegen-check`, repository-wide/uncached typecheck, gate, full/race/E2E suite, CI, push or PR was run in round 2. Round 1 receipts remain explicitly historical. All listed round 2 focused checks passed before commit.
- `docs/_memory/change-impact.md` and the root checkout were left untouched. W10's affected official skill/site/QA docs shipped with the implementation. W8 introduces no partial implementation.

### W10 exact changed files — 09d60b814

```text
docs/qa/scenarios/ET-cli-session-usage-context.md
docs/qa/scenarios/RT-session-cost-provenance.md
docs/qa/scenarios/TA-task-run-cost-provenance.md
internal/acp/types.go
internal/config/provider_usage.go
internal/session/manager_prompt_batch.go
internal/session/manager_prompt_contract_test.go
internal/session/manager_prompt_persistence.go
internal/session/manager_prompt_runtime_loop.go
internal/session/manager_prompt_state.go
internal/session/usage_cost.go
internal/store/globaldb/automation_attribution_migration.go
internal/store/globaldb/global_db_cost_migration_test.go
internal/store/globaldb/queries/usage_cost_repair.sql
internal/store/globaldb/schema/definitions/20_sessions.sql
internal/store/globaldb/schema/migrations/00199_schema.sql
internal/store/globaldb/schema/migrations/atlas.sum
internal/store/globaldb/sqlcgen/models.go
internal/store/globaldb/sqlcgen/usage_cost_repair.sql.go
internal/store/globaldb/usage_cost_migration.go
internal/store/sessiondb/queries/session_core.sql
internal/store/sessiondb/queries/transcript_projection.sql
internal/store/sessiondb/schema/migrations/00011_schema.sql
internal/store/sessiondb/schema/migrations/atlas.sum
internal/store/sessiondb/schema/schema.sql
internal/store/sessiondb/session_db.go
internal/store/sessiondb/session_db_test.go
internal/store/sessiondb/session_token_usage.go
internal/store/sessiondb/session_writer.go
internal/store/sessiondb/sqlcgen/models.go
internal/store/sessiondb/sqlcgen/session_core.sql.go
internal/store/sessiondb/sqlcgen/transcript_projection.sql.go
internal/store/sessiondb/transcript_projection_upgrade.go
internal/store/sessiondb/transcript_projection_upgrade_test.go
internal/store/types_event.go
internal/transcript/projection_types.go
packages/site/content/docs/sessions/index.mdx
skills/compozy/references/runtime-operations.md
```

## Round 2 review correction — preserve unaffected provenance and ledger integrity

The review identified that the original migration invalidated included/estimated costs that were never inflated. This revision corrects **unreleased migration 199 in place**, as explicitly authorized; no migration 200 is added. Its Atlas checksum and generated sqlc queries are regenerated through their owners. Session migration 11 is unchanged.

### Changed behavior and coverage

- Migration 199 now creates pending work only for `cost_source = 'agent_reported'`. The repair captures the original session and daily target sets before invalidation. Invalidation and recomputation run in the same global transaction, before a runtime is exposed; interruption rolls that transaction back and leaves the durable pending marker for retry.
- Both selection and updates are restricted to those original targets. A daily row is not selected merely because a reconstructed session has a matching day/profile/workspace/agent key. Every other row retains `total_cost`, `cost_currency`, `cost_status` and `cost_source`, including NULL values.
- The existing `TestGlobalDBUsageCostRepair` suite now proves preservation after upgrade and repeated reopen for `included/none`, `estimated/catalog_config`, `estimated/models_dev`, `estimated/builtin`, and `unknown/none`. The fixtures also contain reported ledger amounts to catch accidental provenance replacement during repair, plus a protected estimated daily row matching an actually reported session. Existing assertions still require reported history to recompute to 5 and missing reported history to become unknown.
- Example preservation: catalog estimate USD 2.75, models.dev estimate EUR 3.25, builtin estimate USD 4.25, and subscription-included NULL cost remain unchanged in both aggregate tables. Previously this implementation incorrectly cleared them to unknown. Token counts, timestamps and turn counts remain covered.

### Canonical payload-byte integrity audit

The event table has **no persisted per-event payload signature, digest column or hash chain**. That does not mean nothing ever hashes event bytes. The actual dependencies reviewed are:

| Mechanism | Bytes/fence covered | Why the migration is compatible / correction |
| --- | --- | --- |
| `internal/store/sessiondb/schema/schema.sql`, `session_event_write.go` | IDs, sequences and event identity; `sessionEventsHaveIdenticalIdentity` compares content on a duplicate deterministic append | IDs/sequences/timestamps/archive flags stay unchanged. Duplicate-content checks remain enabled. Runtime idempotent permission/input and restart lifecycle events do not carry the monetary usage rewritten by migration 11. A stale conflicting payload is still rejected; no collision check is weakened. |
| `internal/session/manager_conversation_rewind.go:conversationActivePrefixDigest` | SHA-256 of queried event rows, including their content, across the stop/commit phase of one in-flight rewind | This digest is held in memory, not persisted across daemon restart. Migration occurs on database open before the runtime can admit rewind; a migrated ledger supplies both reads. The persisted rewind `request_hash` hashes request/session/message/epoch/generation/sequence identity, not event payload bytes. |
| `internal/store/sessiondb/session_conversation_rewind.go` and `session_conversation_rewind_baseline.go` | Durable generation/max-sequence fences and a derived replay snapshot | Sequence/generation fences remain intact. Migration 11 marks covered cost-bearing rewind snapshots stale, so their existing owner rebuilds from canonical evidence. Transcript projection version 4 similarly rebuilds affected usage parts. |
| `internal/session/manager_clear_manifest.go`, `manager_clear_family_operation.go`, `internal/store/sessiondb/family_file.go` | Persisted SHA-256 of entire database/WAL/SHM/journal files during interrupted clear recovery | **A real byte dependency was found and fixed.** The global cost repair now refuses to open a family with a clear manifest, committed marker, or any clear backup. Opening could migrate/checkpoint and invalidate these hashes before the session manager recovers it. Its reported aggregate becomes unknown as unavailable; recovery evidence remains untouched for the established clear owner. Six focused cases prove byte-for-byte database and recovery-artifact preservation. |
| `internal/store/sessiondb/owner_snapshot.go`, `owner_retry.go`, migration Atlas manifests | SQLite WAL checksums/structural integrity; migration-source checksums | Normal SQLite transactional writes maintain database/WAL integrity. Atlas checksums cover migration source files, not individual event payloads; only authorized unreleased migration 199's checksum changes. |

The append-only ledger remains authoritative for runtime writes and replay. Migration 11 is an explicit, versioned data-representation migration, not a runtime overwrite path or a replacement of ledger truth with projection truth. It changes the meaning of the cost field to the accepted per-turn contract while retaining the original amount as `ReportedCostAmount`; all other JSON values and row identity remain preserved. `json_set` may change JSON formatting, so this is **semantic preservation with raw monetary evidence retained, not a promise of byte-identical historical JSON**. No permanent payload signature needs rewriting. The one durable whole-file hash dependency is protected by the new pending-clear availability guard.

### Focused verification for the correction

All commands below passed; lint reported zero issues in both normal and integration-tag lanes:

```sh
# Regenerate the authorized unreleased migration checksum with Atlas's Goose
# directory checksum API, then run the repository's owning generators.
go run ./.tmp/w10-atlas-hash.go && make codegen

go test ./internal/store/globaldb ./internal/store ./internal/store/sessiondb \
  -run '^(TestGlobalDBUsageCostRepair|TestGlobalDBTokenCostMigration|TestProductionMigrationStreams|TestSessionDBUsageCostMigration|TestTranscriptProjectionUpgrade)$' -count=1

go test -tags=integration ./internal/session ./internal/store/sessiondb \
  -run '^(TestClearConversationFailureRecovery|TestConversationRewindRestartsSameSessionFromRetainedPrefix|TestSessionDBAppendEventIfAbsent)$' -count=1

COMPOZY_GO_LINT_SCOPES='./internal/store/globaldb' make go-lint
git diff --check
```

The checksum helper uses `sqltool.NewGooseDir`, `dir.Checksum` and `migrate.WriteSumFile` from Atlas; it is a local generator invocation, not hand-edited checksum text. The helper is retained under ignored `.tmp/` from the original work. Only migration 199 and the manifest root digest changed; earlier migration checksum entries and session migration 11 did not change.

The reviewer separately reported running migration 11 on a copy of a real 62 MB ledger: 420 turns summed to 300.88, equal to its final report, in under 0.1 s. This is **reviewer-provided evidence**, not a run performed by this implementation session.

Changed files in this correction:

```text
docs/prompts/20261109-1726_harness-token-efficiency-report.md
internal/store/globaldb/global_db_cost_migration_test.go
internal/store/globaldb/queries/usage_cost_repair.sql
internal/store/globaldb/schema/migrations/00199_schema.sql
internal/store/globaldb/schema/migrations/atlas.sum
internal/store/globaldb/sqlcgen/usage_cost_repair.sql.go
internal/store/globaldb/usage_cost_migration.go
```

Not verified: production-wide global repair, live-provider billing, CI, full/race/E2E suites or browser/HTTP/UDS walkthroughs. No `make codegen-check` or global/uncached typecheck ran. At this correction checkpoint, W8 code was unchanged and no push or PR had been performed; delivery evidence is recorded separately below. The pending-clear guard is tested with real SQLite files and synthetic recovery artifacts; existing clear-recovery and rewind integration checks passed, but no process was deliberately crashed during an upgrade.


## Delivery preparation and main integration

The approved delivery scope is W1–W5, W7, W9 and W10. W6 moved to #1432; W8 moved to #1435, each for its own PR. Publication cleanup removes lab session identifiers and private host locations from this report and the touched QA evidence references; unpublished local artifacts are labeled as such.

Integrated `origin/main` at `5f48b457f` with a merge, preserving history. Source conflict resolution retains both the permission-excluded ACP option projection from main and the new auto-compaction/lean-environment settings. The wake QA scenario retains both changes. SDK conflicts were regenerated with `make codegen`; no generated SDK was resolved by hand. Main's final migration versions remain global 198 and session 10, so this branch's global 199 and session 11 remain consecutive and unchanged.

Post-merge focused verification:

```sh
make codegen
go test ./internal/acp ./internal/api/core ./internal/api/spec ./internal/config ./internal/session ./internal/store/globaldb -run '^(TestPermissionDispatchOrdering|TestAgentPayloadEffectiveRuntimeUsesRequestedWorkspaceConfig|TestDocumentTracksRequiredFieldsAndEnums|TestAutoCompactWindow|TestAgentLeanEnvironment|TestClaudeAutoCompactWindow|TestClaudeLeanEnvironment|TestPromptCompactsDeliveredSections|TestManagerAttentionHookRunsAfterCanonicalCommitAndFailsOpen|TestFinishPromptPumpSpawnWakeCancelSuppression|TestPromptCostAccounting|TestGlobalAutomationRunProfileMigrationTail|TestGlobalDBUsageCostRepair)$' -count=1
go test -tags=mage ./magefiles -run '^(TestIntegrationUnifiedPartition|TestIntegrationObservationIdentity|TestPartitionGoShardItems|TestCensusUpdateBootstrap)$' -count=1
go test ./internal/session -run '^(TestSessionStartDropsUnrestrictedAgentModeUnderRestrictedPermissions|TestPromptRuntimeKeepsRestrictedSessionAfterRuntimeChange|TestPromptRuntimeRejectsUnrestrictedModeForPersistedRestrictedSession|TestSessionLeanEnvironment)$' -count=1
go test -tags=integration ./internal/session ./internal/store/sessiondb -run '^(TestClearConversationFailureRecovery|TestConversationRewindRestartsSameSessionFromRetainedPrefix|TestSessionDBAppendEventIfAbsent)$' -count=1
bun x turbo run build --filter=@compozy/client --only
bun x turbo run test --filter=compozy-web --only -- src/systems/runtime/components/runtime-selector/__tests__/runtime-selector.test.tsx src/systems/session/components/__tests__/session-continue-dialog.test.tsx src/systems/os/apps/settings/__tests__/settings-window-nav.test.tsx
bun x turbo run test --filter=@compozy/site --only -- components/landing/__tests__/landing.test.tsx components/site/__tests__/site-footer.test.tsx lib/__tests__/marketplace-catalog.test.tsx lib/__tests__/public-heading-hierarchy.test.tsx lib/__tests__/public-install-contract.test.ts lib/__tests__/static-route-metadata.test.ts
bun x turbo run test --filter=@compozy/ui --only -- src/components/__tests__/sheet.test.tsx
bun x turbo run test --filter=@compozy/extension-sdk --only -- src/__tests__/integration.test.ts
bun x turbo run test --filter=@compozy/lint-plugins --only -- __tests__/test-file-location.test.mjs
COMPOZY_GO_LINT_SCOPES='./internal/acp ./internal/config ./internal/session ./internal/api/core ./internal/api/contract ./internal/api/spec ./internal/store/globaldb ./magefiles ./sdk/go/client' make go-lint
bun x oxlint --deny-warnings desktop/src/deep-links/__tests__/deep-link.test.ts lint-plugins/__tests__/test-file-location.test.mjs lint-plugins/test-file-location.mjs packages/site/components/landing/__tests__/landing.test.tsx packages/site/components/site/__tests__/site-footer.test.tsx packages/site/lib/__tests__/marketplace-catalog.test.tsx packages/site/lib/__tests__/public-heading-hierarchy.test.tsx packages/site/lib/__tests__/public-install-contract.test.ts packages/site/lib/__tests__/static-route-metadata.test.ts packages/ui/src/components/__tests__/sheet.test.tsx sdk/typescript/src/__tests__/integration.test.ts web/e2e/fixtures/__tests__/runtime-seed.test.ts web/src/systems/agent/index.ts web/src/systems/agent/lib/agent-effective-runtime.ts web/src/systems/os/apps/settings/__tests__/settings-window-nav.test.tsx web/src/systems/runtime/components/runtime-selector/__tests__/runtime-selector.test.tsx web/src/systems/runtime/components/runtime-selector/index.ts web/src/systems/runtime/components/runtime-selector/runtime-advanced-options-model.ts web/src/systems/runtime/components/runtime-selector/runtime-advanced-options.tsx web/src/systems/runtime/components/runtime-selector/runtime-selector.tsx web/src/systems/runtime/components/runtime-selector/use-runtime-selector.ts web/src/systems/runtime/index.ts web/src/systems/session/components/__tests__/session-continue-dialog.test.tsx web/src/systems/session/components/session-continue-fields.tsx web/src/systems/session/hooks/use-session-continue-dialog.ts
bun x oxfmt --check desktop/src/deep-links/__tests__/deep-link.test.ts lint-plugins/__tests__/test-file-location.test.mjs lint-plugins/test-file-location.mjs packages/site/components/landing/__tests__/landing.test.tsx packages/site/components/site/__tests__/site-footer.test.tsx packages/site/lib/__tests__/marketplace-catalog.test.tsx packages/site/lib/__tests__/public-heading-hierarchy.test.tsx packages/site/lib/__tests__/public-install-contract.test.ts packages/site/lib/__tests__/static-route-metadata.test.ts packages/ui/src/components/__tests__/sheet.test.tsx sdk/typescript/src/__tests__/integration.test.ts web/e2e/fixtures/__tests__/runtime-seed.test.ts web/src/systems/agent/index.ts web/src/systems/agent/lib/agent-effective-runtime.ts web/src/systems/os/apps/settings/__tests__/settings-window-nav.test.tsx web/src/systems/runtime/components/runtime-selector/__tests__/runtime-selector.test.tsx web/src/systems/runtime/components/runtime-selector/index.ts web/src/systems/runtime/components/runtime-selector/runtime-advanced-options-model.ts web/src/systems/runtime/components/runtime-selector/runtime-advanced-options.tsx web/src/systems/runtime/components/runtime-selector/runtime-selector.tsx web/src/systems/runtime/components/runtime-selector/use-runtime-selector.ts web/src/systems/runtime/index.ts web/src/systems/session/components/__tests__/session-continue-dialog.test.tsx web/src/systems/session/components/session-continue-fields.tsx web/src/systems/session/hooks/use-session-continue-dialog.ts
git diff --check
```

These final commands passed. Go lint reported zero issues in normal, integration, SDK-client and mage lanes; frontend lint reported zero warnings/errors on the 25 incoming JS/TS files. Web passed 151 cases, UI 4 and the lint plugin 12. The extension SDK result reused the matching Turbo cache (3 cases); it is not a fresh runtime execution. Site initially lacked its generated install template and content, then passed after the owning generators completed.

One limited Desktop selection remains non-green locally: `bun x turbo run test --filter=@compozy/desktop --only -- src/deep-links/__tests__/deep-link.test.ts` passed all 13 cases but exited 1 because its selected-file branch coverage was 78.37%, below the existing global 80% threshold. The imported Desktop change is only the test-location annotation; no production Desktop code changed. The coverage threshold was not weakened and the full Desktop lane remains CI-owned.

During Site prerequisite diagnosis, one invocation omitted `--only`, which unintentionally scheduled the root `codegen-check` dependency. That terminal was canceled after identifying the graph; its check ended with exit 141 and is not passing validation. The corrected test commands above retain `--only`. No repository-wide lint, full/race/E2E suites or uncached/repository-wide typechecks were run in this delivery round. Live-provider token savings, production-wide repair and new browser/HTTP/UDS QA walks remain unverified. PR/CI/reviewer evidence will be recorded at the published head.


## Pre-publication integration review corrections

The independent Claude Opus 5.5 review of the integrated branch found two valid defects. Both were reproduced in their existing owning suites before changing production code:

- **Archived transcript migration:** a rewound conversation retains archived cost events whose old transcript entry has been removed. The monetary projection upgrade incorrectly tried to rebuild that removed entry and refused to open the session. The owning SQL query now selects active events only. `TestTranscriptProjectionUpgrade` opens a real schema-10 fixture with archived monetary history through migration 11 twice, requiring the ledger evidence to remain archived and the visible transcript to remain empty. The regression failed with a missing assistant-entry corruption error before the query fix.
- **Stop during batched cost normalization:** a stop that closed the recorder during the baseline read became a persistence failure. The batch path now applies the existing post-stop discard policy to that read error, matching the individual-event path. `TestStoppedSessionDiscardsLatePromptEvents` exercises the real stop transition at the recorder boundary and requires the terminal reason/failure to remain unchanged with one post-stop marker. It failed with a transport/persistence failure before the fix.

The query's sqlc output was regenerated with `make codegen`. Neither migration SQL nor migration checksums changed in this correction.

The first scoped lint/integration attempts could not compile because entries in the shared Go build cache disappeared; the standard-library sources still existed. The repeat uses a worktree-local Go build cache rather than modifying the shared cache or relaxing checks. The earlier pre-push hook also timed out during its Go lint tier after formatting and JS/TS lint passed; this is not a passing hook receipt.


Focused regression checks passed for all three packages, and the recovery/rewind integration selection passed for both packages. Scoped lint reported zero issues in normal and integration lanes. Commands:

```sh
make codegen
go test ./internal/store/sessiondb ./internal/store/globaldb ./internal/session -run '^(TestTranscriptProjectionUpgrade|TestSessionDBUsageCostMigration|TestGlobalDBUsageCostRepair|TestStoppedSessionDiscardsLatePromptEvents|TestPromptCostAccounting)$' -count=1
GOCACHE="$PWD/.tmp/go-build-delivery" go test -tags=integration ./internal/session ./internal/store/sessiondb -run '^(TestClearConversationFailureRecovery|TestConversationRewindRestartsSameSessionFromRetainedPrefix|TestSessionDBAppendEventIfAbsent)$' -count=1
GOCACHE="$PWD/.tmp/go-build-delivery" GOLANGCI_LINT_CACHE="$PWD/.tmp/golangci-delivery" COMPOZY_GO_LINT_SCOPES='./internal/session ./internal/store/sessiondb' make go-lint
git diff --check
```

The test-shape checker accepts the transcript upgrade suite. It reports seven pre-existing direct-test shapes and one pre-existing discarded error in the larger lifecycle suite; comparison against the prior committed file confirms these findings predate the added subtest. The new behavior is covered in the existing owning suites without relaxing any assertion or adding a new test file.


## PR #1437 — first CI and external review round

Published ready at `f74617459` with `ci:full`. The independent Claude reviewer approved that head and reran all 16 archived-transcript/post-stop cases. The publication hook timed out twice during Go lint after its formatting and JS/TS checks passed; publication used its documented skip option with the successful scoped lint receipts above. The hook is not counted as green.

The first CI Preflight passed generated-artifact and installer validation, then rejected two uses of the retired product name in the reduced-environment reference. Both now say CompozyOS. Its two failed Go shards also exposed stale contract expectations: the open/reopen suite still expected session migration 10, and the historical OpenAPI comparison lacked the six exact schemas receiving the approved optional harness settings. The tests now require migration 11 and allow only `auto_compact_window` / `lean_environment` on those declared components; existing fields, requiredness and schema comparisons remain enforced.

Greptile's two findings were reproduced before fixing production code:

- **Canonical migration baselines:** unreleased session migration 11 now derives prior-turn reports from canonical `usage`/`done` events, grouped by turn and currency in sequence order. A missing `token_usage` row no longer clears a retained event cost or inflates the next turn. Existing projected monetary rows are reconciled against the same reports without changing their token fields. The canonical suite includes reports `1, 3, 4, 0.5, 2` with the second projection row absent and now requires costs `1, 2, 1, 0, 1.5` after upgrade and repeated reopen under the subsequent 2026-11-10 operator decision. The unpublished migration checksum was regenerated through Atlas, followed by `make codegen`; global migration 199 is unchanged.
- **Live native root identity:** retain original absolute native root paths and perform exact-file checks through those paths on every catalog decision. Remove cached symlink targets and the root-only suppression shortcut. A removed or repointed `.claude/skills` restores injection; a newly restored exact exposure suppresses it again. Existing native-root tests now create the actual files whose exposure they assert.

Focused verification passed, with zero normal/integration lint findings:

```sh
go run ./.tmp/w10-atlas-hash.go && make codegen
GOCACHE="$PWD/.tmp/go-build-delivery" go test ./internal/store/sessiondb ./internal/store/globaldb ./internal/store ./internal/daemon ./internal/api/spec -run '^(TestSessionDBUsageCostMigration|TestTranscriptProjectionUpgrade|TestOpenSessionDBAppliesBaselineAndRepeatedBootIsIdempotent|TestGlobalDBUsageCostRepair|TestProductionMigrationStreams|TestMigrationSchemaEquivalence|TestComposedAssemblerFiltersProviderNativeSkillsAtStartup|TestHarnessSkillInjectionFilterSuppressesOnlyWinningNativePresetRoots|TestResolveSessionNativeSkillRoots|TestDocumentPreservesSchemaSemantics)$' -count=1
GOCACHE="$PWD/.tmp/go-build-delivery" GOLANGCI_LINT_CACHE="$PWD/.tmp/golangci-delivery" COMPOZY_GO_LINT_SCOPES='./internal/daemon ./internal/store/sessiondb ./internal/api/spec' make go-lint
bash scripts/check-product-language.sh skills/compozy/references/agent-definitions.md
git diff --check
```

The test-shape checker passes the two daemon suites and the API addition suite. Its file-wide Setenv/Parallel warning on `session_db_test.go` is unchanged from the committed file: the environment-changing subtest is serial, and the independent migration subtests do not mutate the environment. No broader local suites, codegen drift gate or E2E were run for this round. CI and external review of the correction head remain pending.

## PR #1437 — first CodeRabbit review corrections

CodeRabbit reviewed `f74617459`. Its missing-projection cost finding was already fixed by `46c918028`, including the real schema-10 upgrade with an absent projection row. Greptile subsequently reviewed `46c918028` and reported both earlier findings fixed. The remaining valid CodeRabbit findings were reproduced in the existing owning suites before the production fixes:

- **Unconfirmed startup delivery:** assembling the first request used to mutate the retained startup snapshot. A failed or canceled send followed by a retry without live augmentation therefore lost both snapshots. Startup text and its delivery spans now come from one request-local copy; only successful, non-canceled delivery advances the sent marker. Tests cover unsent preparation, request failure, cancellation, and successful replacement of the entire startup snapshot without replaying it on the next turn.
- **Cost-bearing user entries:** the monetary projection upgrade required every affected entry to be an assistant entry, although canonical user events can carry usage metadata. It now resolves the persisted entry identity and uses the existing projector for that kind, while still rejecting a missing identity. Repeated read-only upgrade/reopen preserves the user entry and its stable identity. The existing assistant and archived-history cases still pass.
- **Single-setting provider write:** a PUT containing only `auto_compact_window` was rejected as empty. The shared API decoder now includes that field in its emptiness check; the canonical handler suite requires HTTP 200 and the exact forwarded window.
- **Review assertions and clarity:** description-size comparisons now build their historical baseline independently and reject repeated discovery boilerplate. Invalid windows require specific diagnostics; canonical migration checks require all expected events and non-nil monetary evidence. Wake tests read stub state under its mutex, and the admission path separates errors from already-admitted success. Native exposure lookup rejects path-like skill names before joining them to a native root.

No public schema or migration bytes changed in this round, so no generated artifact needed regeneration. No token/character savings were remeasured; the earlier paired measurements remain evidence for the original successful-delivery path, not a new provider-backed benchmark.

Focused tests passed in all six packages, the native-root selection passed, and real clear-recovery/rewind integration passed in both packages. Scoped lint reported zero issues in normal and integration lanes, including the final native-name guard:

```sh
GOCACHE="$PWD/.tmp/go-build-delivery" go test ./internal/acp ./internal/api/core ./internal/config ./internal/daemon ./internal/mcp ./internal/store/sessiondb -run '^(TestPromptCompactsDeliveredSections|TestPromptAttachesSystemPromptDeliveryMetadata|TestSettingsCollectionHandlersDelegateValidPayloads|TestAutoCompactWindow|TestSessionWakeBridge|TestHostedProxyHelpers|TestSessionDBUsageCostMigration|TestTranscriptProjectionUpgrade)$' -count=1
GOCACHE="$PWD/.tmp/go-build-delivery" go test ./internal/daemon -run '^(TestComposedAssemblerFiltersProviderNativeSkillsAtStartup|TestHarnessSkillInjectionFilterSuppressesOnlyWinningNativePresetRoots|TestResolveSessionNativeSkillRoots)$' -count=1
GOCACHE="$PWD/.tmp/go-build-delivery" go test ./internal/acp -run '^(TestStartFinalizesRuntimeGuidance|TestPromptMaintenancePreservesStartupDelivery|TestPromptPrependsSystemPromptOnce)$' -count=1
GOCACHE="$PWD/.tmp/go-build-delivery" go test -tags=integration ./internal/session ./internal/store/sessiondb -run '^(TestClearConversationFailureRecovery|TestConversationRewindRestartsSameSessionFromRetainedPrefix|TestSessionDBAppendEventIfAbsent)$' -count=1
GOCACHE="$PWD/.tmp/go-build-delivery" GOLANGCI_LINT_CACHE="$PWD/.tmp/golangci-delivery" COMPOZY_GO_LINT_SCOPES='./internal/acp ./internal/api/core ./internal/config ./internal/daemon ./internal/mcp ./internal/store/sessiondb' make go-lint
GOCACHE="$PWD/.tmp/go-build-delivery" GOLANGCI_LINT_CACHE="$PWD/.tmp/golangci-delivery" COMPOZY_GO_LINT_SCOPES='./internal/daemon' make go-lint
git diff --check
```

The test-shape checker reports no new findings: each modified suite was compared with its prior committed version. Existing file-wide/legacy-shape findings remain unchanged; the daemon, hosted-tool and transcript-upgrade suites pass outright. CI on `46c918028` had completed Preflight (including generators), Go lint, Desktop and React Doctor successfully when inspected; those results do not certify the next correction head. Full/race/E2E validation, live-provider savings and production-wide repair remain CI-owned or unverified as specified above.

## PR #1437 — provider preservation and wake retry review round

Greptile and CodeRabbit found that accepting a window-only PUT was insufficient: the settings writer still replaced the provider table, losing authored command, endpoint, display name and home policy. The existing settings-service suite reproduced that loss against a real isolated configuration file. A window-only edit now validates and classifies the existing provider with just that scalar changed, then edits only `auto_compact_window` in the overlay. Full provider PUT replacement semantics remain intact. The regression covers both a built-in provider override and a custom provider, repeated writes, and preserved raw model curation.

CodeRabbit also found that a failed durable wake lookup left the caller's in-memory reservation intact. The existing session and bridge suites reproduced the suppressed retry and missing error classification. The bridge now marks this pre-dispatch failure as retryable; the session owner releases only that event's reservation and eviction entry. Every retry still checks durable admission. Successful/admitted wakes and errors with uncertain dispatch outcomes retain duplicate suppression. No automatic retry loop or extra wake turn was introduced.

The API contract suite independently asserts the optional integer type of `auto_compact_window` and optional boolean type of `lean_environment` on their six affected schemas. These assertions supplement the narrow historical compatibility allowance without copying requiredness or types from the current schema.

Focused verification commands:

```sh
GOCACHE="$PWD/.tmp/go-build-delivery" go test ./internal/settings ./internal/api/spec ./internal/api/core -run '^(TestProviderSettingsUsesMergedCatalogProjection|TestCollectionMutationsProviderAndHook|TestCollectionMutationsCodexNativeProviderOverlay|TestProviderACPMethodMutationSemantics|TestProviderSecretOnlyMutationStoresVaultSecret|TestProviderSecretMutationRejectsInvalidProviderConfigWithoutStoringSecrets|TestDocumentTracksRequiredFieldsAndEnums|TestDocumentPreservesSchemaSemantics|TestSettingsCollectionHandlersDelegateValidPayloads)$' -count=1
GOCACHE="$PWD/.tmp/go-build-delivery" go test ./internal/session ./internal/daemon -run '^(TestManagerDispatchSpawnWake|TestSessionWakeBridge|TestSpawnWakeEventContract|TestSpawnWakeReasonForBadge|TestEnqueueSyntheticWakeRunsOnceWithoutADeliveryStream)$' -count=1
GOCACHE="$PWD/.tmp/go-build-delivery" go test -tags=integration ./internal/settings ./internal/session -run '^(TestProviderSettingsUsesMergedCatalogProjection|TestProviderOverlayDeleteRevealsBuiltinFallbackMetadataCorrectly|TestConfigApplyServiceProviderOverlayForBuiltinRequiresRestart|TestConfigApplyServiceAppliesProviderModelOnlyChangesLive|TestConfigApplyServiceCuratesProviderModelsLive|TestFinishPromptPumpSpawnWakeCancelSuppression)$' -count=1
GOCACHE="$PWD/.tmp/go-build-delivery" GOLANGCI_LINT_CACHE="$PWD/.tmp/golangci-delivery" COMPOZY_GO_LINT_SCOPES='./internal/session ./internal/daemon' make go-lint
GOCACHE="$PWD/.tmp/go-build-delivery" GOLANGCI_LINT_CACHE="$PWD/.tmp/golangci-delivery" COMPOZY_GO_LINT_SCOPES='./internal/settings ./internal/api/spec' make go-lint
git diff --check
```

The focused and integration selections passed. Both scoped lint selections passed with zero normal/integration issues. A final settings fixture expansion exposed one trailing blank line, corrected before the successful final lint run. The wake suites pass the test-shape checker; the settings and API suites retain exactly their pre-existing findings (13 and 1). This round changes no public schema or migration bytes and therefore requires no generator run. No new provider-backed token savings, browser QA or production-wide repair measurements were made.

The unverified symlink-skip suggestion was not applied: existing Windows filesystem contracts explicitly require a runner capable of creating symlinks (`internal/fileutil/nofollow_windows_test.go`). Setup failures remain visible instead of silently skipping the catalog invariant. CodeRabbit also emitted a generic docstring-coverage warning without identifying missing functions; no coverage threshold was changed, and this warning is not represented as passing verification.

CI run `42728543950` on `c6f3493a3` failed the SQLite paired experiment because baseline trial 0 had a transcript-write deadline failure, leaving its tail comparison incomplete. The four head trials had zero operation failures, no invalid trials and no throughput regressions; the remaining three complete tail ratios had median 0.6911. The failure is retained, not averaged away. The 1 GB fixture's all-column digest, observe results and sequence fences matched after upgrade/reopen, and the separate real-daemon thirty-session job passed. These are CI receipts for the earlier head, not approval of the next head. The acceptance criteria and baseline are unchanged; the next push reruns this lane.


## PR #1437 — CI repair-path confinement

CodeRabbit approved `a026344ab` after reviewing the latest provider/wake fixes; Greptile reported 5/5 with no new actionable findings. The full Go lint job subsequently found `G703` on the recovery-marker lookup in `requireCostRepairLedgerReady`. The package-scoped local lint had not surfaced that cross-package taint path.

The repair preflight now opens the configured sessions directory with `os.OpenRoot` and checks recovery markers and ledger existence through relative `Root.Lstat` calls. It no longer performs these checks with absolute `os.Lstat` calls or the earlier inline suppression. The directory handle is always closed, joining cleanup errors with the primary error. The canonical session opener still owns ledger migration and identity validation. Recovery-marker refusals, missing-history behavior and monetary provenance are unchanged.

`TestGlobalDBUsageCostRepair` now rejects a session-directory symlink leading to another data home at the preflight boundary and verifies that the outside ledger bytes remain unchanged. The existing six clear-recovery marker cases and complete/missing-history upgrade/reopen cases still pass. The new check lives in the existing owning suite.

```sh
GOCACHE="$PWD/.tmp/go-build-delivery" go test ./internal/store/globaldb -run '^(TestGlobalDBUsageCostRepair|TestGlobalDBTokenCostMigration)$' -count=1
GOCACHE="$PWD/.tmp/go-build-delivery" GOLANGCI_LINT_CACHE="$PWD/.tmp/golangci-delivery" COMPOZY_GO_LINT_SCOPES='./internal/store/globaldb' make go-lint
python3 .agents/skills/eng/eng-test-conventions/scripts/check-test-conventions.py internal/store/globaldb/global_db_cost_migration_test.go
git diff --check
```

All focused checks passed; normal and integration lint reported zero issues. The first lint attempt required the repository's `errors.AsType` convention in the new assertion; that was corrected before the passing run. No schema, migration or generated bytes changed. Full-repository taint analysis remains CI-owned and must confirm the new head; the earlier approval and local package result are not substitutes for that result. No new live-provider or production-data measurements were performed.

## Operator correction — zero charge on cumulative counter drops (2026-11-10)

The operator replaced the original full-report charge on a counter drop after reviewing 1,281 real sessions with monetary reports: 27 drops occurred in 24 sessions, and 17 landed at least halfway up the previous amount. A restored counter can lag rather than start at zero. In three independently migrated ledger copies, two totals matched the final raw counter; the third produced 242.55 against roughly 180 because the old drop rule charged a 62.40 report again after 66.73. These aggregate observations were supplied by the operator; this executor did not access those private ledgers.

`turnCost` and both monetary expressions in unreleased session migration 11 now return zero on a drop, retain the raw report and use the lower amount as the next baseline. Non-decreasing reports remain increments, and explicitly per-turn adapters retain their reported amounts. Global repair has no second normalization rule: it sums the already normalized canonical ledger costs. Its fixture now uses the same zero-charge drop and expects 4.5 for `1, 3, 0.5, 2`. Included/estimated/other cost provenance remains untouched; global migration 199 does not change. Atlas regenerated the session checksum, and `make codegen` passed.

The existing prompt suite restarts the manager before every subsequent turn, including both sides of a drop. Both live and schema-10-to-11 migration/reopen coverage assert `1, 3, 0.5, 2 → 1, 2, 0, 1.5` and lagging restore `10, 12, 11.5, 12.5 → 10, 2, 0, 1`, preserving the raw amounts. The migration suite retains its missing-projection case. Report examples, release notes, the superseded brief, official skill reference, site docs and the three affected QA scenarios now describe this decision. The accepted limitation is an uncounted drop turn (at most one turn per drop); a restart whose first report equals or exceeds the previous amount remains indistinguishable from continuation.

Verification, with an isolated Go build cache:

```sh
go run ./.tmp/w10-atlas-hash.go && make codegen
go test ./internal/session ./internal/store/sessiondb ./internal/store/globaldb ./internal/store -run '^(TestPromptCostAccounting|TestSessionDBUsageCostMigration|TestTranscriptProjectionUpgrade|TestGlobalDBUsageCostRepair|TestGlobalDBTokenCostMigration|TestProductionMigrationStreams|TestMigrationSchemaEquivalence)$' -count=1
go test -tags=integration ./internal/session ./internal/store/sessiondb -run '^(TestClearConversationFailureRecovery|TestConversationRewindRestartsSameSessionFromRetainedPrefix|TestSessionDBAppendEventIfAbsent)$' -count=1
COMPOZY_GO_LINT_SCOPES='./internal/session ./internal/store/sessiondb ./internal/store/globaldb' make go-lint
bun x turbo run test --filter=compozy-web --only -- src/systems/agent/components/__tests__/agent-editors.test.tsx -t Permissions
bun x turbo run test --filter=compozy-web --only -- src/systems/agent/components/__tests__/agent-editors.test.tsx
```

All selected Go packages and real recovery/rewind integration passed; normal and integration lint reported zero issues. The test-shape helper passed the global migration file; its prompt-contract and session-database findings were byte-identical to HEAD, with no new finding. The changed skill guidance was reviewed statically; triggers, metadata and reference links are unchanged. Formatting passed for the matched documentation files. Root Turbo executed uncached: 15 Permissions tests passed, then all 66 agent-editor tests passed. The initial integration-tagged selection also passed prompt-accounting and transcript-upgrade cases; the exact recovery selection above is the evidence for recovery and rewind.

### Existing Web CI failure

[CI run 42733804439, Frontend (web)](https://github.com/compozy/compozy/actions/runs/42733804439/job/129558466604) failed the default-reach round-trip test after 1,004 ms while the editor still showed its loading skeleton (9,233 other tests passed). The aggregate Verify failure was caused by this frontend lane. The branch changes no `web/` files relative to main `5f48b457f`; its generated TypeScript client changes only optional, erased types. The editor's draft is loaded by the unchanged `useAgentSettingsPage → useAgent → fetchAgent` path, against a synchronous MSW agent handler. No changed Go DTO serialization or runtime client code executes in that test.

Both focused runs above passed, including the whole owning file. They also emitted the same existing MSW warnings for unhandled ancillary requests seen in CI; these are not a warning-free Web receipt. Evidence supports an intermittent harness/timing failure, with a load-sensitive cause still unproven. Duplicate searches found no matching issue; [#1445](https://github.com/compozy/compozy/issues/1445) records the evidence and investigation criteria. No test, timeout, coverage threshold or frontend production code was changed. The next accounting push supplies a second full-CI sample.

No new provider-backed token/billing measurement, private-ledger migration, production-wide repair timing or browser/HTTP/UDS walk was performed. Full, race, E2E, repository-wide lint/typecheck and codegen drift gates remain CI-owned. Final delivery still requires all checks concluded without failures, renewed review coverage on the new head and zero unresolved threads.
