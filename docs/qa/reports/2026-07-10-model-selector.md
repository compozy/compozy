# QA Report — Model Selector (2026-07-10)

**Cycle verdict:** PASS for the model-selector feature cycle.  
**Companion real-scenario verdict:** BLOCKED by QA-infrastructure defect `BUG-0027`; no collaboration evidence is fabricated.  
**Task 04 status:** COMPLETE; isolated runtime, web server, observer, and browser are torn down.  
**Whole-spec gate:** `make verify` intentionally not run here; `$cy-final-verify` owns the single fresh final invocation.

## Scope and environment

The cycle walked CH-028 through CH-036 against one isolated daemon, exercised the three RuntimeSelector mounts, live Claude and Codex ACP negotiation, HTTP/UDS/CLI/native catalog and agent surfaces, config lifecycle, provider Settings canary, public documentation, and the bundled AGH skill.

```json
{
  "schema_version": 1,
  "scenario_id": "model-selector-20260710-194713-914643",
  "manifest_path": "/Users/pedronauck/dev/qa-labs/agh-model-selector-20260710-194713-914643-lab/qa-artifacts/qa/bootstrap-manifest.json",
  "lab_root": "/Users/pedronauck/dev/qa-labs/agh-model-selector-20260710-194713-914643-lab",
  "qa_output_path": "/Users/pedronauck/dev/qa-labs/agh-model-selector-20260710-194713-914643-lab/qa-artifacts",
  "agh_home": "/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-a9ca00a24cc3/runtime",
  "http_base_url": "http://127.0.0.1:62339",
  "uds_socket": "/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-a9ca00a24cc3/runtime/aghd.sock",
  "web_url": "http://localhost:3001",
  "web_proxy_target": "http://127.0.0.1:62339",
  "browser": "agent-browser fallback; browser-use plugin cache unavailable",
  "playbook": "devtool-oss-launch",
  "teardown_evidence": "/Users/pedronauck/dev/qa-labs/agh-model-selector-20260710-194713-914643-lab/qa-artifacts/qa/teardown.json",
  "teardown_clean": true
}
```

The bootstrap smoke test and playbook validator passed after `BUG-0026` was fixed. The materialized playbook declared 8 differentiated agents, 5 channels, and 11 tasks.

## Charter verdicts

| Charter | Persona / journey | Verdict | Main evidence |
|---|---|---|---|
| CH-028 | Bruno / J-17 | PASS | manual session-create walk; screenshots; Playwright session-onboarding/provider-override |
| CH-029 | Bruno / J-18 | PASS | agent-create E2E; HTTP/UDS/CLI/AGENT.md read-back; inherited and explicit-override sessions |
| CH-030 | Lea / J-19 | PASS | manual first-run onboarding at Claude Sonnet 5 + Max; onboarding persistence E2E |
| CH-031 | Ada / J-20 | PASS | curated/all parity, CLI+HTTP+UDS+four native tools, curation, refresh/status, config, OpenAI-compatible list, docs/skill |
| CH-032 | Bruno / J-21 | PASS | live Claude canonical `claude-sonnet-5/max` -> ACP `sonnet/max`; focused and full runtime E2E |
| CH-033 | Marina / J-22 | PASS | 430x932 Settings canary, truthful source status, Back + refresh; `BUG-0025` retest |
| CH-034 | Sol / J-17 | PASS | named combobox/dialog/listbox/radiogroup and real favorite button in accessibility tree; keyboard/component contract and official E2E green |
| CH-035 | Ada / J-21 | PASS | identical typed 422 codes over HTTP/UDS and CLI diagnostics |
| CH-036 | Ada / J-18 | PASS | equivalent agent definitions via HTTP, UDS, CLI, and `agh__agent_create`; invalid native enum rejected before write |

All 29 J-17..J-22 tracker rows have a terminal `pass` verdict and point to this report. No native session-create or native agent read/update surface was invented.

## High-risk browser proof

The required highest-risk flow was driven through the documented fallback browser because the browser-use plugin cache was unavailable:

1. First-run onboarding selected Claude Sonnet 5 and Max and completed.
2. New Session on the general agent reopened the RuntimeSelector, selected the same tuple, and created `sess-60559bfa9795c41a`.
3. The persisted API row retained the canonical model ID; live ACP capability state reported transport `sonnet` and effort `max`.
4. Settings → Providers was walked on a 430x932 viewport, including Claude inspect, degraded source truth, navigation away, browser Back, and refresh.

Screenshots:

- `/Users/pedronauck/dev/qa-labs/agh-model-selector-20260710-194713-914643-lab/qa-artifacts/qa/screenshots/onboarding-claude-sonnet5-max.png`
- `/Users/pedronauck/dev/qa-labs/agh-model-selector-20260710-194713-914643-lab/qa-artifacts/qa/screenshots/session-create-claude-sonnet5-max.png`
- `/Users/pedronauck/dev/qa-labs/agh-model-selector-20260710-194713-914643-lab/qa-artifacts/qa/screenshots/session-created-claude-sonnet5-max.png`
- `/Users/pedronauck/dev/qa-labs/agh-model-selector-20260710-194713-914643-lab/qa-artifacts/qa/screenshots/settings-claude-mobile-truthful-status.png`

## Structured-surface results

- CLI, HTTP, and UDS returned the same Claude curated projection. `view=all` was a strict superset after curation excluded `claude-fable-5` from the curated view.
- CLI, HTTP, and native curation persisted hidden/deprecated/featured/default-effort intent. Restart rehydration preserved the result.
- `agh__provider_models_list|curate|refresh|status` were registered, callable, and successfully invoked with descriptor/schema digests and truthful availability diagnostics.
- `GET /api/openai/v1/models?provider_id=claude` returned the curated canonical IDs.
- HTTP, UDS, CLI, and `agh__agent_create` authored equivalent agent definitions with `reasoning_effort=max`; HTTP/UDS/CLI/AGENT.md fresh reads agreed.
- The agent default resolved to Max in `sess-d643ee43dd4744f2`; explicit Low won in `sess-67ce3ed24d8c637c`.

## Config lifecycle and deterministic errors

The isolated `[providers.claude.models.reasoning]` key was changed sequentially, never concurrently:

- `apply=none` + restart removed the working apply strategy and produced `reasoning_option_missing` before a prompt.
- Restoring `apply=acp_option` + restart restored Claude Max.
- Claude `minimal` produced `reasoning_effort_unsupported`.
- Off-contract `ultra` produced the same typed boundary code before provider startup.
- Codex `gpt-does-not-exist` produced `model_unavailable` and enumerated the seven canonical choices.

HTTP and UDS returned 422 for all three negotiation classes; `agh session new` preserved the same diagnostic codes. Claude's provider-owned custom option accepts arbitrary IDs by design, so its provisional custom-ID success is not an unknown-to-default fallback.

## Defects and fix-loop outcome

| Bug | Result | Verification |
|---|---|---|
| BUG-0024 | fixed in uncommitted worktree | live Claude + focused 8-test reasoning E2E + both official E2E gates |
| BUG-0025 | fixed in uncommitted worktree | 114 modelcatalog race tests + fresh CLI/status/mobile canary |
| BUG-0026 | fixed in uncommitted worktree | bootstrap smoke + real playbook validation + fresh manifest |
| BUG-0027 | open QA-infrastructure blocker | provider-backed operator transcript + strict evidence audit |

No commit SHA is recorded because the user explicitly requested that the verified worktree remain uncommitted.

## Real-scenario companion audit

The single Mateo Rivera kickoff reached live Claude session `sess-c0a90d2a22d46659`. Claude explored the lab, discovered evaluator metadata under the agent-visible workspace, called the comparison spoiled, and refused to manufacture multi-agent activity. That is the correct provider decision but invalidates the blind scenario.

The strict auditor exited 2 and truthfully reported 0/11 task runs, 0/12 peer messages, 0/3 review cycles, 0/3 disruption probes, no required deliverables, and no final `make verify` evidence. The last item is expected at this phase because the accepted whole-spec plan reserves exactly one `make verify` for the final gate. `BUG-0027` owns the contamination defect; the feature charter verdicts above come from independent browser, runtime, HTTP, UDS, CLI, native, config, and E2E evidence.

Audit artifacts:

- `/Users/pedronauck/dev/qa-labs/agh-model-selector-20260710-194713-914643-lab/qa-artifacts/qa/qa-audit-report.md`
- `/Users/pedronauck/dev/qa-labs/agh-model-selector-20260710-194713-914643-lab/qa-artifacts/qa/provider-attempt.json`
- `/Users/pedronauck/dev/qa-labs/agh-model-selector-20260710-194713-914643-lab/qa-artifacts/qa/model-selector-evidence.md`

## Gates

- `go test -race ./internal/session ./internal/config` — PASS.
- `go test -race ./internal/modelcatalog` — PASS, 114 tests.
- Focused provider-reasoning daemon E2E — PASS, 8 tests.
- `make test-e2e-runtime` — PASS: daemon 50, HTTP 8, UDS 14, harness 6.
- `make test-e2e-web` — PASS: 65/65.
- `make verify` — DEFERRED intentionally to the final whole-spec `$cy-final-verify` gate; it must run exactly once.
- QA teardown — PASS: `teardown.json` records `clean: true`, `survivors: []`, and stopped daemon PID `55789`.

## AGH Impact Audit

- **Native tools:** all four `agh__provider_models_*` descriptors, schemas, digests, availability and invocation paths passed; `agh__agent_create` enum and rejection passed; no native session-create or general agent-read/update surface exists.
- **Extensibility and hooks:** provider-model native toolset and bundled `skills/agh/` guidance passed; no extension/hook IDs changed in the QA fixes. Config lifecycle was exercised through `models.reasoning.apply` and curated-entry flags.
- **Workspace data isolation:** catalog/config data is global; agent definitions and sessions were workspace-scoped to `ws_9c974090799d4517`. HTTP, UDS, CLI, files, web and events all resolved the isolated workspace; no other workspace data appeared.
- **Official AGH skill:** `skills/agh/` references matched the shipped curated/all, authoring, and reasoning behavior.

## Decision for a human

`BUG-0027` needs a dedicated QA-infrastructure design: controller-only rubric/evidence state must live outside the provider session's readable workspace while business context and realistic seeds remain visible. Do not waive or game the collaboration thresholds.
