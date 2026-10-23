# QA Run Report — 2026-10-23 — Fast and reasoning application

- Scope: issue #914; provider-confirmed Fast and max/ultra reasoning.
- Cadence: targeted CLI/API/provider journey; one isolated daemon, sequential provider sessions.
- Baseline: `74f33b7d460866b8061544e7eb98b73dbe972901`.
- Status: PASS for the scoped provider journey; CI/reviews pending.
- Persona: Sol, a power user choosing the runtime for a short conversation.
- Evidence: [sanitized provider readbacks and turn events](../evidence/2026-10-23-issue-914-fast-reasoning/runtime-proof.json).

## Reproduction and root causes

The baseline Codex adapter accepted `gpt-6.1-sol`, `ultra`, and Fast. After a completed turn it advertised `reasoning_effort=ultra` and `fast-mode=on`; the effective runtime correctly said `ultra/fast/applied`. The suspected `fast-mode` matcher failure was not reproduced: normalized `fast-mode`, category `model_config`, and `off/on` already match.

The baseline Claude adapter accepted `claude-opus-5-5`, `max`, and Fast, then reported that Fast was disabled because the account did not have extra usage enabled. Its updated configuration had `effort=max` and `fast=off`, while Compozy still reported `fast/applied` in the effective runtime and turn events. `setConfigOptions` replaced options without reconciling speed resolution; session projections used copied requested fields. Equal-request prompt shortcuts also ignored later provider changes.

Native runtime schemas omitted `ultra` even though the domain and Web selector included it. This blocked native prompt/runtime selection, agent/fallback authoring, Goal runtime, Loop rules, and model-default curation before provider validation. Queued-input output schemas had the same omission. Subagent target schemas already use open effort identifiers.

A re-walk found that publishing observed speed through the persistence-facing `Info.Speed` violated the existing requested-speed invariant during stop. Detaching a process also discarded the last speed-resolution update; stop/release now retain it. The implementation keeps persisted intent unchanged and projects observations through `Info.EffectiveRuntime()`. No store invariant or test was relaxed.

## Research

| Source | Finding and consequence |
| --- | --- |
| Local Codex `codex-rs/protocol/src/openai_models.rs` | Effort includes `Max`, `Ultra`, and provider custom identifiers. Model metadata determines supported effort and service tiers. A static universal ceiling is not authoritative. |
| Local t3code `apps/server/src/provider/CodexProvider.ts` and `codexUsage.ts` | Uses native supported efforts/service tiers, maps Fast to priority, and reads applied thread settings. Requested configuration and provider observations are separate facts. |
| Local Superset `packages/shared/src/agent-models.ts` | Codex effort is passed as `-c model_reasoning_effort=ultra`; per-model options include ultra. Cursor embeds effort in a model variant. Compozy retains its stricter explicit rejection instead of Superset's fallback to default. |
| Local acpx `src/session/config-options.ts` | Consumes the returned configuration list and reconciles sibling desired settings. Compozy retains its stricter requirement for explicit current-value confirmation. |
| ACP SDK in use | `github.com/coder/acp-go-sdk` is replaced by the Compozy fork at `ebd00ee4ba87`. Both `SetSessionConfigOptionResponse` and `SessionConfigOptionUpdate` contain the full option set and current values; notifications can supersede earlier acknowledgements. |
| [Claude model configuration](https://code.claude.com/docs/en/model-config) and [settings reference](https://code.claude.com/docs/en/settings-reference) via Context7 | Fast and effort depend on the model/account. Live Claude advertised max but not ultra; its account-level Fast refusal was observable through ACP. No account setting was changed. |

## Provider × entry-surface matrix

“Shared path” means source tracing plus focused automated coverage, not a separate paid live turn for every cell. Configuration confirmation is ACP readback, not a claim about remote inference scheduling.

| Provider and entry surface | Accepted | Sent | Confirmed | Reported / evidence |
| --- | --- | --- | --- | --- |
| Codex, CLI prompt: ultra/Fast | Yes | `reasoning_effort=ultra`, `fast-mode=on` | Current values after completed turn | Baseline and fixed live readbacks agree: ultra/Fast/applied. |
| Codex, CLI prompt: max/normal | Yes | Exact max and off values | Current values after completed turn | Fixed live readback: max/normal/applied. |
| Codex, native `session_runtime_set`, then prompt without overrides | Yes, including ultra | Applied at next prompt; selecting alone does not start/configure ACP | ultra/on after turn | Live native registry call completed; transition `live_configuration`; selected and effective agree. |
| Codex, stop then prompt same session | Stored ultra/Fast retained | Reapplied on provider bind/load | ultra/on after completed turn | Live readback agrees; the logical session remains the same. |
| Claude, CLI prompt: max/Fast | Yes | `effort=max`, `fast=on` acknowledged initially | Provider later reports max/off | Fixed readback and assistant/done events: max/normal, Fast rejected/provider_rejected. Account does not permit Fast. |
| Claude, CLI prompt: ultra | Accepted by public input validation | Not sent as a supported option | Not advertised | Explicit `reasoning_effort_unsupported` with live valid choices; no false ultra claim. |
| Codex/Claude, Web composer and HTTP/UDS prompt | Model-specific catalog/selector; ultra already supported by Web vocabulary | Shared session runtime path | Same provider acknowledgement and notification handling | Source audit; no browser run under the machine-load restriction. |
| Codex/Claude, native prompt, agent/fallback defaults, Goal, Loop rules | Canonical schema widened additively to ultra; exact provider membership still required | Shared Start/ConfigureRuntime path; Goal/Loop intent is carried by the binder | ACP wire and runtime propagation suites | Schema tests cover native inputs; binder regression removes falsely applied unsupported speed. No live Goal/Loop was started. |
| Codex/Claude, replacement and automatic recovery | Existing selection retained | Exact ultra/Fast forwarded to the replacement Start call | Start negotiates against fresh capabilities | Existing replacement/recovery suites extended; live providers were not deliberately crashed. |
| Subagent targets | Existing unconstrained effort string accepts ultra | Target selection uses the same session creation/runtime path | Shared ACP negotiation | Source audit; no child agent created. |
| Cursor, all runtime entry surfaces | Exact advertised logical model configuration | Compiled to the CLI launch-model variant, not ACP effort/Fast setters | Existing launch strategy is unchanged | Focused launch-bound regression including a Cursor alias with conflicting ACP controls; no live Cursor turn. |
| Other ACP providers | Canonical input acceptance is not a support claim | Exact advertised option only | Explicit acknowledgement/current value required | Missing/ambiguous speed is unsupported; unadvertised effort rejects. No unsupported value is promoted to applied. |

## Implementation and tradeoffs

- Reconcile speed resolution whenever the provider replaces its ACP options, including notifications and sibling option responses.
- Keep requested intent in persisted session fields. Project the current effort/speed into HTTP/UDS, extension host reads, prompt events, and Loop speed observations through one session helper.
- Recheck provider effort/speed changes before taking the equal-selection prompt shortcut.
- Preserve the existing nonfatal unsupported-speed behavior. Unsupported speed has no claimed effective value; provider-observed off is normal with a rejected Fast request.
- Preserve every existing enum value and add ultra. Regenerate the native descriptor catalog and CLI reference; update official runtime guidance and provider docs.

## Validation

Focused Go tests passed for ACP configuration/notification/reapplication, native schema validation, session effective-vs-requested projection, replacement/recovery, and Loop binder speed propagation. The persistence re-walk passed: Claude reached stopped with verified process termination and retained requested Fast/rejected. Once live options are unavailable, effective speed is omitted rather than guessed. API/extension projection tests and focused syntax lint also passed.

The live lab used ordinary short arithmetic prompts, independent HTTP readbacks, the public native-tool invocation boundary, and stop/rebind. Raw local evidence stays outside the repository; the committed proof removes host paths and private session identities.

Full local gates, race suites, browser/E2E runs, and broad lint were explicitly prohibited for this assignment. CI owns those delivery gates. No PR/CI/reviewer completion is claimed by this report.

## Limits

Claude Fast could not remain enabled on this account. The refusal was verified; extra usage was not enabled. ACP current values prove adapter configuration, not hidden server-side effort budgets or scheduling. Cursor/other providers and browser presentation have source/automated evidence only. Lab teardown is recorded below.

## Local verification record

The focused binder run is recorded in `verify-focused.log` in the lab evidence directory. The broader local gate is deferred to CI by the explicit machine-load instruction. This is a targeted behavioral PASS, not release readiness.

The strict QA evidence audit passed with zero blockers and zero warnings. The manifest-scoped
teardown completed cleanly with zero survivors; only verified lab-owned processes were targeted.
