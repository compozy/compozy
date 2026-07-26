# Feature Story Exploration Summary

## Research Question

go over every single feature in this app create a user story with expected behaviour based on the code keep a single canonical spreadsheet tracking the features status
- when done switch loop to testing every user story and documenting all errors
- when done fix every logistical error or ux error
- test every user behaviour again post fix

<critical>use $agent-exploration with 5 agents opus in different parts of the project, all of them doing the same prompt to find out user stories for the spreadsheet first</critical>
<critical>just include ./internal/* and web/ here in the scope</critical>

<obs>you can use docs from the .compozy/* docs/* also to understand what was build and get ideas for here about scope and stories</obs>

## Slice Map

| Slice | Slice question | One-line finding |
|---|---|---|
| 01 - runtime-sessions | What runtime, workspace, agent, provider, onboarding, sandbox, and session behaviors exist? | Found 42 stories covering daemon/doctor status, onboarding, workspace CRUD, the full session lifecycle, prompt streaming, provider/auth probes, agent identity/context/spawn, authored soul/heartbeat, sandbox profiles, and web runtime scoping. |
| 02 - tasks-automation | What task orchestration, task-run lifecycle, scheduler, observe, automation, and webhook behaviors exist? | Found 62 stories covering task CRUD, draft/publish/start/enqueue, run lifecycle/recovery/reviews, dashboard/inbox/triage, scheduler controls, agent claim/lease flows, automation jobs/triggers/runs, webhook delivery, and automation settings. |
| 03 - extensibility-tools | What skill, extension, bundle, resource, hook, MCP, and native-tool behaviors exist? | Found 51 stories covering skills and marketplace, extension lifecycle/provenance/trust, bundle activation, resource CRUD, tool registry/toolsets/approval/invocation, hook taxonomy/runs/settings, MCP settings/auth, hosted MCP, and agent-native agh__ tool IDs. |
| 04 - network-bridges | What AGH Network, bridge, delivery, and notification behaviors exist? | Found 46 stories covering network status/settings/channels/threads/directs/peers/work/send/inbox, bridge CRUD/lifecycle/health/targets/secrets/test-delivery, task bridge subscriptions, and notification presets. |
| 05 - memory-settings | What memory, settings, vault, model catalog, support, logs, filesystem, and knowledge behaviors exist? | Found 52 stories covering memory/knowledge CRUD/search/decisions/dreams/extractor/providers, settings sections/collections/apply/reload/restart, vault write-only secrets, model catalog/OpenAI models, support bundles, logs, and filesystem browse. |

Canonical spreadsheet: `.compozy/tasks/feature-stories/feature-status.csv` contains 253 story rows with inventory, QA, error, fix, retest, overlap, evidence, and notes columns.

## Convergences

- **Shared handler, multi-surface parity** appears in all five analyses: user-facing behavior is generally implemented once in `internal/api/core` and registered through HTTP and UDS, with web and CLI/native tools consuming those routes where available. This makes HTTP/UDS parity a default QA lane. See `01_analysis_runtime-sessions.md`, `02_analysis_tasks-automation.md`, `03_analysis_extensibility-tools.md`, `04_analysis_network-bridges.md`, and `05_analysis_memory-settings.md`.
- **Agent-manageability is a real product invariant, not just backend plumbing.** Sessions, tasks, tools, hooks, bundles, skills, network, bridges, memory, settings, vault, and model catalog all expose machine-readable API/UDS/native-tool paths, though a few gaps remain. See the agent-kernel stories in `01_analysis_runtime-sessions.md`, task lease stories in `02_analysis_tasks-automation.md`, native tool stories in `03_analysis_extensibility-tools.md`, and network/bridge CLI surfaces in `04_analysis_network-bridges.md`.
- **Status code plus body assertions are required.** Most domains map domain errors into precise 400/401/403/404/409/410/422/501/503 responses with structured bodies. The testing phase should not accept status-only checks. See runtime error mapping in `01_analysis_runtime-sessions.md`, task/webhook/review mappings in `02_analysis_tasks-automation.md`, extension/tool/resource mappings in `03_analysis_extensibility-tools.md`, bridge target diagnostics in `04_analysis_network-bridges.md`, and memory/settings/vault mappings in `05_analysis_memory-settings.md`.
- **Secret and token redaction is a cross-cutting QA invariant.** Prompt streams, provider probes, claim tokens, bridge/vault secrets, webhook/auth tokens, tool approvals, logs, SSE, and model catalog errors all need explicit leak checks. See `01_analysis_runtime-sessions.md`, `02_analysis_tasks-automation.md`, `03_analysis_extensibility-tools.md`, `04_analysis_network-bridges.md`, and `05_analysis_memory-settings.md`.
- **Web coverage is narrower than API/agent coverage in several domains.** Backend/agent surfaces are richer for network send kinds, bundles, resources, MCP auth, memory promote/reindex/reset/reload, logs, and some automation runs. This is often intentional, but the QA spreadsheet must not treat "no web route" as "no feature." See `03_analysis_extensibility-tools.md`, `04_analysis_network-bridges.md`, and `05_analysis_memory-settings.md`.
- **Config lifecycle is a high-risk UX path.** Settings mutations often produce apply records and restart banners; HTTP privileged gates differ from UDS local-trust behavior. This recurs in runtime, automation, network, skills/extensions, and memory/settings. See `01_analysis_runtime-sessions.md`, `02_analysis_tasks-automation.md`, `03_analysis_extensibility-tools.md`, `04_analysis_network-bridges.md`, and `05_analysis_memory-settings.md`.

## Divergences

- **HTTP and UDS auth are intentionally asymmetric.** HTTP wraps selected mutations with privileged guards, while UDS does not. This is expected local-trust behavior, but spreadsheet rows must track which transport was tested. See `01_analysis_runtime-sessions.md`, `02_analysis_tasks-automation.md`, `03_analysis_extensibility-tools.md`, and `05_analysis_memory-settings.md`.
- **Some backend-complete features have no obvious web route.** Bundles, generic resources, MCP OAuth auth, several memory maintenance operations, OpenAI-compatible model list, network advanced send kinds, and some automation-run/review views are API/CLI/native-tool first. See `03_analysis_extensibility-tools.md`, `04_analysis_network-bridges.md`, and `05_analysis_memory-settings.md`.
- **Some endpoints are truthful stubs or capability-gated empties.** Memory recall trace always returns a "not materialized" 404; dream status is intentionally empty; extractor/session-ledger/provider operations can return 501 when components are not wired. These are not automatically bugs. See `05_analysis_memory-settings.md`.
- **Some visible UX copy may overstate mechanics.** The network UI advertises "live message flow" while the web network surfaces appear poll-driven rather than SSE-driven. See `04_analysis_network-bridges.md`.
- **There are potential partial-surface gaps.** Network channel/peer message handlers appear implemented but unrouted; notification preset update has API/web support but no confirmed CLI verb; MCP trust-root and hosted-MCP reachability need more confirmation. See `04_analysis_network-bridges.md` and `03_analysis_extensibility-tools.md`.
- **Several rows intentionally overlap and need one testing owner.** The CSV records overlap notes for sandbox profiles, filesystem browse, bundle network settings, hooks/MCP settings, network/automation/skills settings, and task bridge notifications.

## Risks & Open Questions

- Should implemented but unrouted `NetworkChannelMessages` and `NetworkPeerMessages` be wired to HTTP/UDS or deleted? Evidence: `04_analysis_network-bridges.md`.
- Does web network "live message flow" copy need changing to reflect polling, or is polling considered live enough by product language? Evidence: `04_analysis_network-bridges.md`.
- Which default daemon build components wire `DreamTrigger`, `MemoryExtractor`, `MemoryProviders`, and `MemorySessionLedger`? This determines expected 200 vs 501/empty behavior for MS-016..MS-023. Evidence: `05_analysis_memory-settings.md`.
- Does the CLI/native surface include provider login, notification preset update, single-peer detail, MCP auth/trust-root exact config targets, and all near-duplicate resource tool IDs? Evidence: `01_analysis_runtime-sessions.md`, `03_analysis_extensibility-tools.md`, and `04_analysis_network-bridges.md`.
- What is the canonical webhook HMAC signing recipe for TA-060, and where is it documented or implemented? Evidence: `02_analysis_tasks-automation.md`.
- Does sending a `say` message with a fresh thread id implicitly create a thread root, or must another mechanism create it first? Evidence: `04_analysis_network-bridges.md`.
- Are Daytona sandbox profiles selectable without config, and what UX should the web show when Daytona is unavailable? Evidence: `01_analysis_runtime-sessions.md`.
- Which overlap rows should be skipped during QA versus tested as independent views? The CSV marks overlap candidates but keeps them visible so the testing loop can make an explicit decision.

## Recommended Next Steps

1. **Freeze the CSV as the single working tracker** before QA: update `qa_status`, `error_status`, `documented_errors`, `fix_status`, and `retest_status` in `.compozy/tasks/feature-stories/feature-status.csv`; do not create parallel trackers. Supported by all five slice files.
2. **Start QA with highest-risk end-to-end journeys**: onboarding/workspace/session prompt streaming (`01_analysis_runtime-sessions.md`), task create/run/review/scheduler (`02_analysis_tasks-automation.md`), skills/extensions trust and visibility (`03_analysis_extensibility-tools.md`), network channel/thread/direct and bridge secrets (`04_analysis_network-bridges.md`), and settings restart/vault/memory write decisions (`05_analysis_memory-settings.md`).
3. **Use both HTTP/web and UDS/agent lanes for sampled parity.** Each slice emphasizes shared handlers and agent-manageability, so at least one representative story per domain should be exercised over both transports.
4. **Document every discovered error in the CSV first.** Use `documented_errors` for concrete reproduction/evidence and keep `fix_status=Not started` until a fix begins.
5. **Resolve overlap/uncertainty rows before exhaustive QA.** Prioritize rows marked `Blocked by uncertainty` and rows with `duplicate_or_overlap` notes so the testing loop does not waste time double-testing the same behavior.
6. **Treat redaction and detached execution as mandatory invariants.** Prompt streams, task claim tokens, vault/bridge secrets, provider probe output, logs, and settings/model errors need explicit leak checks.
7. **After fixes, retest the same CSV rows.** The final phase should set `retest_status=Passed` only with fresh evidence, not just a green unit test.

## Index

- `.compozy/tasks/feature-stories/analysis/01_analysis_runtime-sessions.md`
- `.compozy/tasks/feature-stories/analysis/02_analysis_tasks-automation.md`
- `.compozy/tasks/feature-stories/analysis/03_analysis_extensibility-tools.md`
- `.compozy/tasks/feature-stories/analysis/04_analysis_network-bridges.md`
- `.compozy/tasks/feature-stories/analysis/05_analysis_memory-settings.md`
- `.compozy/tasks/feature-stories/feature-status.csv`

