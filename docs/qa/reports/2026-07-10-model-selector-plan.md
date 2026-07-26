# QA Plan — model-selector (2026-07-10)

**Cycle:** model-selector (unified Runtime Selector + truthful model catalog / curation / reasoning).
**Tier:** Targeted + e2e-web + e2e-runtime (feature program spanning web MVP + backend catalog/reasoning + agent surfaces).
**Scope source:** `.compozy/tasks/model-selector/_spec.md` §7 (invariants), §11 (agent surfaces), §12 (Web/Docs impact); tasks 01 & 02 QA-impact flags.
**Planning only** — execution is the later isolated real-user QA pass; this cycle mints/reconciles scenarios, journeys, and charters and assigns no verdicts.

## What changed (dogfood surface)

- **Web:** one unified `RuntimeSelector` (button-group trigger + single popup: search/refresh, provider rail, grouped model list with availability + capability chips, reasoning footer) replaces three divergent leaf pickers on session-create, agent-create (now with reasoning), and onboarding (grid deleted). Favorites/recents (browser-local), custom model ID, needs-auth states, keyboard/a11y.
- **Backend:** truthful curated catalog (curation columns + curated view; `view=curated|all`), name-heuristic reasoning deleted, per-provider ACP reasoning-apply strategies (fail-loud), settings/catalog projection unified, single reasoning enum, API/OpenAPI co-ship.
- **Agent surfaces:** `agh provider models list/set/refresh/status`, `POST .../models/curate`, native `provider_models_list/curate/refresh/status`, deterministic error codes; agent create/read now carry `reasoning_effort`.
- **Config:** `[providers.<id>.models.reasoning] apply=acp_option|none`; `[[providers.<id>.models.curated]]` gains `deprecated|hidden|featured|release_date`.
- **Docs + bundled skill:** `content/runtime/core/agents/{model-catalog,providers}.mdx`, `configuration/config-toml.mdx`; bundled `skills/agh/` updated for the new CLI verb, `view` param, curation fields, and the fail-loud reasoning error.

## Spec erratum (agent update)

There is **no general public agent-definition update API** in this MVP. The runtime exposes agent **create** (`POST /api/agents`) over HTTP/UDS/CLI and the native `agh__agent_create` tool, and agent **read** (`GET /api/agents[/:name]`) over HTTP/UDS/CLI/web plus AGENT.md file persistence and default-inheritance into `StartOpts` — but **no native agent read/get/list/update tool** and no `PATCH/PUT` agent-update route. `agh__agent_create` is create-only. Task wording implying "agent update" is treated as an erratum: this cycle covers **create + read (projection / file persistence / inheritance)** only and fabricates no update coverage. `RT-029` (create) and `RT-028` (list/get) carry that scope under J-18 / CH-029.

## Journeys (flows before matrix)

| Journey | Name | Persona(s) | Charter(s) |
|---|---|---|---|
| J-17 | Start a session through the unified runtime selector | Bruno, Sol | CH-028, CH-034 |
| J-18 | Author + read-back an agent whose runtime now carries reasoning | Bruno, Ada | CH-029, CH-036 |
| J-19 | Choose a default runtime during onboarding | Lea | CH-030 |
| J-20 | Curate the model catalog from structured agent surfaces | Ada | CH-031 |
| J-21 | Claude reasoning applied truthfully, end to end | Bruno, Ada | CH-032, CH-035 |
| **J-22 (canary)** | Provider settings & display surfaces stay truthful | Marina | CH-033 |

Each journey file carries a Mermaid flow (entry → actions → branch points → side effects → **true end state**) with ≥1 abandonment/error path. Every flow reaches its declared `true_end_state` through an explicit fresh read/reopen/restart terminal: session created → reopen dialog + fresh-read the session transcript (J-17); agent created → fresh HTTP/UDS/CLI/AGENT.md read-back → new-session inheritance (J-18); onboarding persisted → fresh settings re-read → later session default (J-19); curation → daemon-restart rehydration → cross-surface readback, plus `model_not_found` deterministic terminal (J-20); reasons at the advertised effort → restart-safe seeded subsets, with the fail-loud 422 terminals (J-21); no-regression → navigate-back/refresh/fresh re-read (J-22).

## Adjacent canary (required) — justification

**J-22 / CH-033 (Marina, Back-Button Tour)** is the adjacent canary. The catalog/settings-projection unification (task_01 §5.4) and the new KindIcon marks (task_02) touch shared read surfaces the feature was **not** supposed to change — the provider list/inspect/edit, the model-catalog status card, and the display-only echoes (session list, task execution profile, provider logos). The canary walks those unchanged surfaces to prove the refactor stayed invisible (single projection, GET→PUT no-op, truthful stale states, consistent labels/logos). Back-Button Tour is the matrix first-choice for Settings and a genuine regression vector here (navigate away/back + refresh must preserve the projected state).

## Coverage matrix — surface → scenario id → charter

Every public CLI/HTTP/UDS/native-tool/web/docs/config surface named in `_spec §11–§12` maps to ≥1 scenario file and ≥1 charter.

| Surface | Entry point | Scenario(s) | Charter |
|---|---|---|---|
| Web · session-create | session-create dialog (RuntimeSelector) | RT-063, RT-064, RT-065, RT-066, RT-067, RT-010 | CH-028 |
| Web · session-create a11y | keyboard + screen reader | RT-068 | CH-034 |
| Web · agent-create | wizard RuntimeStep (+reasoning) | RT-069, RT-070 | CH-029 |
| Web · onboarding | default-model step (grid deleted) | RT-071, RT-004 | CH-030 |
| Web · settings/display (canary) | providers inspect/edit/status; display echoes | MS-028, MS-058 | CH-033 |
| HTTP · list | `GET .../models?view=curated\|all` | MS-042, MS-053, MS-055 | CH-031 |
| HTTP · session create (+reasoning) | `POST /api/sessions` | RT-010, RT-061, RT-063 · fail-loud RT-062 | CH-028, CH-032, CH-035 |
| HTTP · agent create | wizard POST + direct structured create (+reasoning_effort) | RT-069, RT-029 | CH-029, CH-036 |
| HTTP · agent read | `GET /api/agents[/:name]` (projection) | RT-028 | CH-029 |
| HTTP · curate | `POST .../models/curate` | MS-054 | CH-031 |
| HTTP · model source status | `GET .../models/status` | MS-044 | CH-031 |
| HTTP · openai-compat | `GET /api/openai/v1/models` | MS-045 | CH-031 |
| UDS parity | same applicable agent/catalog routes and session-negotiation errors over UDS | MS-055, RT-028, RT-029, RT-062 | CH-031, CH-029, CH-035, CH-036 |
| CLI | `agh provider models list/set/refresh/status`; `agh agent create/info`; `agh session new` | MS-042, MS-043, MS-054, MS-055, MS-044, RT-028, RT-029, RT-062 | CH-031, CH-029, CH-035, CH-036 |
| Native tools | `provider_models_list/curate/refresh/status`; native tool registry (ET-049) — no native agent read/update tool | MS-042, MS-054, MS-055, MS-043, MS-044, ET-049 | CH-031 |
| Reasoning apply (session) | ACP `set_config_option` ordering | RT-061 (happy) · RT-062, MS-057 (fail-loud) | CH-032, CH-035 |
| Config lifecycle | `providers.<id>.models.reasoning`; curated flags | MS-056 | CH-031 |
| Docs (site) | published model-catalog / providers / config-toml pages | ET-053 | CH-031 |
| Bundled AGH skill | `skills/agh/` (new verb, `view`, curation fields, reasoning error) | ET-053 | CH-031 |

## Journey × five-dimension coverage matrix (canonical taxonomy)

Dimensions from `qa-report/references/taxonomy.md`: **1 Journeys · 2 Functional · 3 Experiential · 4 Edge/error/empty · 5 Cross-cutting.** Cells point at the covering scenario/charter, or record a justified skip.

| Journey | 1 Journeys | 2 Functional | 3 Experiential | 4 Edge / error / empty | 5 Cross-cutting |
|---|---|---|---|---|---|
| J-17 | CH-028 create→topbar shows choices | wire body: provider present, empty omitted (RT-063/RT-065) | CH-034 a11y walk (Sol) | custom-ID `model_unavailable` (RT-065); needs-auth disabled row (RT-067) | responsive desktop/mobile proven by the RuntimeSelector Storybook desktop+mobile capture matrix (task_02 deliverable); reset-on-switch across providers (RT-064) |
| J-18 | CH-029/CH-036 create→agent default resolved into StartOpts (RT-070) | reasoning threads web wizard + structured create→persist→fresh read (RT-069/RT-029/RT-028) | CH-029 wizard walk (Bruno) plus CH-036 structured authoring walk (Ada), each in its canonical persona | off-contract effort rejected at build/native-schema boundary; unknown name → 404 (RT-028) | create parity CLI/HTTP/UDS/`agh__agent_create` (RT-029/CH-036); read parity CLI/HTTP/UDS/web + AGENT.md (RT-028/CH-029) — no native agent read tool |
| J-19 | CH-030 onboarding advances; default runtime persisted (RT-071) | commit folds model + default reasoning (RT-071); auth section gates bound_secret | first-run friction lens (Lea) | bound_secret missing-env-var error; grid-deleted empty path | *skip: single-surface onboarding; product makes no cross-device continuity promise here — recorded* |
| J-20 | CH-031 curate→reflected across surfaces (MS-054) | list/set/refresh/status round-trip; curated-default vs `view=all` (MS-042/053/055/044/056) | CH-031 agent-experiential (Ada): structured-output parseability, deterministic diagnostics, perceived tool-call latency across CLI/native | `model_not_found`; no alias resolution (canonical IDs only); ET-049 invalid mutating call reaches validation | CLI==HTTP==UDS==native parity + config lifecycle (MS-055/056) |
| J-21 | CH-032 session reasons at the advertised effort (RT-061) | ACP ordering; explicit `none` RPC vs empty default no-RPC; truthful source badge | reasoning-meter truthfulness (no fabricated levels) | CH-035 fail-loud: `reasoning_option_missing`/`_unsupported`/`model_unavailable`→422 (RT-062/MS-057) | deterministic session-negotiation code parity across CLI/HTTP/UDS; no native session-create tool exists |
| J-22 | CH-033 read surfaces stay truthful (MS-028) | projection == catalog; unchanged GET→PUT no-op; empty `curated[]` clears/fails-closed | display-echo + KindIcon consistency (Marina) (MS-058) | degraded catalog shows stale honestly, no crash/empty firehose | regression canary across shared components; Back-Button nav/refresh preserves projection |

**Regression hot spots (`_spec §7`) → charter home:** 7.1 truthful efforts → CH-032; 7.4 curated-by-default → CH-031 (+ CH-028 browse); 7.5 fail-loud → CH-035; 7.6 reset-on-switch → CH-028; 7.9 empty-omitted → CH-028 (+ CH-029 build boundary).

## Reconciliation (task 01/02 flags + orphans → this cycle)

- **Orphans reconciled (journey + persona set, `qa_status=untested`, planning-only):** `ET-049` → J-20/Ada (native curation toolset agent-manageable); `MS-028` → J-22/Marina (provider settings CRUD canary); `MS-044` → J-20/Ada (model source status verb); `RT-028` → J-18/Bruno (agent read projection); `RT-029` → J-18/Ada (structured agent create). Verified each had an empty `journey` before reconciliation; placeholder personas were replaced with canonical names.
- **New journey-derived rows:** `MS-058` → J-22/Marina (display-only echo consistency canary); `ET-053` → J-20/Ada (public docs + bundled AGH skill are executable guidance, not unowned gates).
- **Prior model-selector rows retained:** RT-063..RT-072, MS-053..MS-057, and the linked task_01 rows (MS-042/043/045/053/054, RT-004/010/061/062).
- **Charter fixes:** all tours map to one canonical tour and all timeboxes are 30/60/90; CH-033 owns only its J-22 canary rows; CH-034 applies the accessibility lens through Feature Tour; CH-035 owns session-negotiation failures only and no longer invents a native session-create path; CH-036 gives Ada the structured HTTP/UDS/CLI/`agh__agent_create` leg of J-18 without forcing Bruno to act as a native agent.
- **Tracker integrity:** `state.csv` parses at 16 fields/row (365 rows), is canonically sorted by `id`, has no duplicate/malformed rows, and leaves no in-scope (J-17..J-22) row without a charter. No id renumbered; only journey-derived planning data changed.

## Completeness (Step 7)

- [x] Every in-scope journey (J-17..J-22) has a Mermaid flow, a true end state, and ≥1 abandonment/error path.
- [x] Every in-scope journey has ≥1 charter with an assigned canonical persona (J-17→CH-028/034, J-18→CH-029/036, J-19→CH-030, J-20→CH-031, J-21→CH-032/035, J-22→CH-033).
- [x] Every in-scope scenario row (29) has a stable id, a linked journey, and `qa_status=untested` (planning only — no verdicts assigned).
- [x] Every charter names persona, journey, scenario ids, exact surfaces/entry points, `evidence_to_capture`, and executable `exit_criteria`, with exactly one canonical tour and a 30/60/90 timebox.
- [x] Every touched public surface (§11–§12) appears as an `entry_point` in ≥1 scenario file and in the surface→scenario→charter matrix above, including CLI list/set/refresh/status, HTTP curated/all + sessions + agent create/read, UDS parity, native list/curate/refresh/status (incl. `agh__provider_models_refresh`/`_status`), config lifecycle, docs pages, and the bundled AGH skill.
- [x] The five canonical taxonomy dimensions were swept per journey (matrix above); J-18 (wizard / draft / read-back) and J-20 (agent-experiential) now carry their own experiential coverage, leaving one deliberate skip — J-19 cross-device continuity (single-surface onboarding; no cross-device promise) — recorded with reasoning.
- [x] The adjacent canary journey is named (J-22) and justified with its charter (CH-033).
- No `TC-*` cases, per-round `qa/` trees, or `verification-report.md` artifacts introduced; no execution evidence or verdict rows fabricated.

**Handoff:** the later isolated real-user QA pass bootstraps a fresh lab, walks CH-028..CH-036 in persona (browser-use for the CH-032 max-reasoning web flow), runs `make test-e2e-runtime` + `make test-e2e-web`, exercises the CLI/HTTP/UDS/native + config matrix, registers any defects into `docs/qa/bugs/`, verdicts the scoped rows, and writes the dated run report with the machine-readable QA bootstrap block.
