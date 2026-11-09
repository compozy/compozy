# Harness token efficiency — final report

Date: 2026-11-09. Branch: `harness-token-efficiency`. Base: `e34e8332c`.
Implementation head: `1ba6a448b`. Brief commit: `4e206f01f`.

Seven workstreams have implementation commits. W6, W8, and W10 are blocked with no partial implementation or migration. **The inflated cost ledger is not fixed.** W9's requested actual Claude first-call token measurement remains unverified; its implementation and provider-option contracts pass. This is not a claim that all ten workstreams or all acceptance measurements are complete.

| Workstream | Status | Commit | Result |
| --- | --- | --- | --- |
| W1 | done | `703a89c27` | Optional global/per-agent Claude auto-compaction window; default unchanged |
| W2 | done | `6de5c90f9` | Stable lineage reuse; one first-message situation block |
| W3 | done | `a3414008e` | Omit exact skill files already exposed natively to Claude |
| W4 | done | `f4635c9c8` | Smaller runtime reference router with required guidance retained |
| W5 | done | `be622495c` | Smaller hosted descriptions; identifiers and schemas unchanged |
| W6 | blocked | — | Default output reduction conflicts with the current public compatibility window |
| W7 | done | `31eb6c52e` | Durable admission prevents replaying the same child-wake revision |
| W8 | blocked | — | Adapter cache deadline and compaction ownership are not exposed to the daemon |
| W9 | done; token measurement unverified | `1ba6a448b` | Lean Claude environment for system/triage and opted-in agents |
| W10 | blocked | — | Resume/reset counter baseline cannot be established through the current adapter contract |

## Paired binary measurements

Both binaries ran against an isolated Compozy home, provider home, workspace, port, and Unix socket. The provider-free ACP driver exercised the real daemon and hosted native tools. The fixture contained 100 workspace skills, 80 exposed as native Claude skill symlinks, plus the bundled Compozy skill. It used a general agent with the complete allowed native-tool policy.

The final base session was `sess-d07c27f9cc046d9e`; the final candidate session was `sess-ee3cf323aa920bc4`. Each began fresh and completed two turns. Counts below are Unicode characters, not provider tokens. Tagged-block counts include opening and closing tags. Startup plain text excludes the four runtime/situation/catalog blocks, trailing user text, and surrounding whitespace.

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

### W6 — blocked: stable default output

Current session tools expose stable full-session outputs. Dropping static fields from the default response would break existing no-argument callers. SD-013 requires keeping the old public shape for one release after a replacement ships when a lossless migration is unavailable. The current interface has no response-version negotiation that would identify compact-capable callers. Keeping the old default and adding an opt-in compact view would be a staged prerequisite, not the requested compact default or 50% result-size outcome; the brief prohibits shipping partial workstreams.

No production/test/doc contract files or migration were changed for W6. No W6-specific tests or lint were run. The real four-tool measurements above demonstrate unchanged behavior and the unmet target.

Open decision: authorize a staged replacement/deprecation release, or specify a negotiated response version and rollout contract. This workstream requires a public-surface compatibility transition, not a SQLite migration.

### W7 — redundant wake admission

Found that child-attention deduplication was in memory and could lose its remembered revision after restart/eviction. The wake bridge now checks the existing durable synthetic-wake ledger and submits the same `WakeEventID` as `MessageID`, letting queue admission enforce idempotency across races and replay. Ledger errors are surfaced; an unseen revision still receives its own wake.

Verification: T7 and L7 passed. Controlled delivery of one revision three times results in one admission; a later revision still admits once. No production-wide turn reduction is claimed. Existing monitor coalescing (bounded batch of 64), reply-watch guarantees, ordinary monitor delivery, and informative attention wakes were left unchanged. A trivial-looking response alone is not proof that a wake was redundant.

Compatibility: existing public wake/event shapes and durable queue identity are reused; no schema/data migration or new delay. No open design question. Historical wake frequencies were supplied by the brief; they were not remeasured from operator data.

### W8 — blocked: warm-cache timing and ownership

Read the documented native `idleCompaction` conditions before evaluating a pass-through: interactive terminal/VS Code/Desktop, automatic compaction enabled, one-hour cache TTL, feature-flag access, sufficient context, and approximately an hour idle. This is not an SDK guarantee. [Claude settings reference](https://code.claude.com/docs/en/settings-reference#idlecompaction).

The installed `@agentclientprotocol/claude-agent-acp` 0.102.0 independently implements SDK idle compaction in `dist/idle-compaction.js`: default delay 54 minutes, actual request-level cache TTL tracking, a late-fire guard, and its own `this.prompt(..., "idleCompaction")` call in `acp-agent.js`. Its context/delay environment variables exist, but the timer disappears when Compozy releases the process at 30 minutes. Enabling that timer would also introduce a second compaction owner and its 54-minute default intersects the observed 28–58 minute warm resumes.

The current daemon contract does not receive the authoritative last API request/cache-expiry deadline required to guarantee warm-cache compaction after those common resumes. A turn-end timestamp is insufficient: a long final tool execution may already have consumed much of the TTL. Rebinding a released process and sending `/compact` based only on idle time could trigger a full-price cold read.

No opt-in setting, timer, process-retention workaround, test, or migration was shipped. No W8 lint/check result or cache-saving measurement is claimed. The supplied resume rates (19% at 200k+, 26% at 400k+) and common warm 28–58 minute gaps were retained as design constraints.

Open dependency: expose authoritative cache timing from the adapter and coordinate the runtime-release lifecycle with the existing `RequestCompaction`/`session_compact` owner. Threshold and delay must then be configurable, off by default, and observed through session events.

### W9 — lean environment

Added optional per-agent `lean_environment`. System and triage sessions select the lean option; user and ordinary spawned sessions preserve their default unless their definition opts in. Claude options disable auto-memory and claude.ai connectors and set `strictMcpConfig`, while retaining Compozy-configured MCP, repository `CLAUDE.md`, native skills, built-in tools, settings hooks, and the established input-only title behavior.

Verification: T9 and L9 passed. Tests cover new/load/resume metadata, ordinary/default and explicit choices, one-shot role defaults, other providers, and preservation of repository instructions/tool/skill options. Contract generation, drift checks, client build/typecheck, and a forced uncached Web typecheck against the final client passed.

**Actual first-call Claude tokens before/after for triage/system were not measured.** The provider-free ACP lab cannot expose Claude's private system prompt or operator environment cost, and its mock command does not execute the Claude bridge. No savings percentage is claimed for W9. This required quantitative acceptance result remains unverified despite the completed implementation and focused contract coverage.

Compatibility: additive optional agent field co-shipped through API/CLI and generated SDKs; user-session default unchanged. Internal one-shot launch policy changes intentionally. No SQLite shape/data change or migration. No open design question; remaining verification needs an authorized provider-backed comparison with representative operator resources.

### W10 — blocked: cumulative-counter baseline

The root cause is confirmed: the ACP `Cost.amount` contract is cumulative, the installed Claude adapter forwards `total_cost_usd`, and the observer adds that value as though it were per turn. [ACP cost schema](https://agentclientprotocol.com/protocol/v1/schema#cost).

Claude's counter behavior also depends on runtime version and lifecycle: before CLI 2.1.277, SDK resumes began at zero; newer versions restore totals saved on normal exit. Conversation resets start another counter, and a crash result can carry zeroed totals. [SDK cost tracking](https://code.claude.com/docs/en/agent-sdk/cost-tracking).

The adapter does not expose an authoritative counter epoch/restored baseline at session load/resume to Compozy's usage owner. Its experimental structured usage request is only used to format a user `/usage` turn as Markdown, not a supported ACP baseline response. Detecting a numerical decrease cannot distinguish a reset whose first new value exceeds the previous counter, and assuming a zero baseline can double-count restored spend. Dropping the first resumed value would lose legitimate turn cost. Those heuristics cannot establish the required restart invariant.

No partial normalizer, historical invalidation-only migration, or provider-semantics guess was shipped. Other adapters' per-turn/cumulative semantics were not fully established; no claim is made about them. Observe/globaldb/migration coverage was therefore not changed or run for W10. Existing inflated totals remain present and may still be labeled actual; this is an unresolved defect.

Open dependency: establish adapter epoch/baseline and reset signals for every supported cost-producing adapter, then implement durable normalization and the lossless historical repair/unknown migration together. The migration must preserve all other token and attribution columns and prove fresh/reopen/equivalence behavior.

## Focused verification commands

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

Cross-surface impact: W1/W9 add configuration and agent DTO fields consumed through existing settings/API/CLI surfaces and generated clients; no Web redesign. W2/W3/W4 change prompt/catalog instructions only. W5 changes hosted description text while retaining routing/authentication/approval/schema contracts. W7 reuses the existing synthetic queue ledger and wake IDs. Official skill references and site docs accompany the changes. Workspace/profile isolation was exercised in the lab. No completed workstream changes SQLite shape or data, so no migration is required; the blocked W10 migration is explicitly absent.

Unverified: actual Claude first-call token reduction for W9; provider-side W1 compaction thresholds; paid/model-side token or dollar savings; real warm-cache deadlines and long-idle resumes; a production daemon restart during duplicate wake delivery; all non-Claude adapter cost semantics; W10 restart normalization/historical repair; full CI/gates/race/E2E and complete Web scenario walks.

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
