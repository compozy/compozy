# Native Agy ACP reasoning controls — 2026-10-21

Current verdict: **FAIL for the requested ACP self-report outcome.** Native selection and outbound
model routing passed, but no tested configuration made the model report all three selected levels.
Effective backend reasoning remains unverified; no authoritative telemetry was found.
This investigation changes no production code, provider configuration, or authentication method.

## Fresh self-report inference round

The installed official ACP **1.4.1** completed **28 prompts** with the same neutral question:

```text
What is your reasoning effort? Please answer in one short sentence. Do not use tools.
```

The first 24 cases independently tested Argon and Flash at Low/Medium/High using four native controls:
family plus `thinking_level`, concrete model suffix, startup `AGY_ACP_DEFAULT_MODEL`, and legacy
`session/set_model`. Four further Argon Low/High cases tested experimental effort metadata on
`session/new` or `session/prompt`. All 28 routed the intended suffixed model ID.

| Model | ACP Low | ACP Medium | ACP High | Native CLI Low / Medium / High |
| --- | --- | --- | --- | --- |
| Argon | `0.20 (low)` | `0.20 (low)` | `0.20 (low)` | `low (0.05)` / `medium (0.20)` / `high (1.00)` |
| Flash | `0.50 (medium)` | `0.50`, usually labeled Medium | `0.50 (medium)` | `low` / `medium (0.50)` / `high (1.00)` |

One Flash Medium legacy-setter answer was numeric-only; it is retained as label-unknown. Seven
alternative option names and four alternative values were rejected with `-32602`. Unsupported
`_meta.antigravity` thinking fields did not change the symptom. The ACP advertises only `plan`,
`logout`, and `usage`; `/effort` was not supplied as ordinary ACP prompt text.

The CLI control matrix recorded version **1.4.4**. All six matrix inferences plus an initial Argon
High control matched their requested labels. A separate native `/effort` query returned `high`
with zero token usage. CLI success is a positive control, not a correction to the ACP implementation.
Earlier CLI authentication trouble did not block these successful inference controls.

There were 28 independent ACP prompts and 34 native model log entries; repeated entries for two
Flash turns are not independent prompts. Only final answers and allowlisted model IDs were retained.
Both executable hashes still match the recorded matrices. The self-report numbers are observations,
not authoritative backend effort telemetry or a proposed shared numeric scale.

Current evidence and manifest:
`/home/vscode/dev/qa-labs/compozy-agy-native-effort-inference-20261021-111717-990598-lab/qa-artifacts/qa/`.
Retained files include the four ACP `matrix-*.json` results, `parameter-probes.json`,
`matrix-native-cli.json`, `native-cli-control.json`, `final-assertions.json`, and `verification-report.md`.
Final assertions passed. The strict evidence audit reports zero blockers and warnings; targeted
teardown records `clean: true`. The evidence-audit PASS does not change the failed ACP outcome.

Cross-surface impact: not applicable — investigation and evidence only; no native-tool,
extensibility/config, workspace-isolation, Web, or official-skill behavior changed.

## Earlier routing-only controls and live evidence

The unchanged official ACP 1.4.1 server accepted `session/set_config_option` for model family
`gemini-4-argon`, followed by `thinking_level=low|medium|high`. It also accepted the corresponding
concrete model ID and legacy `session/set_model`; all route through the same model application handler.

| Requested level | Acknowledged level | Model observed at the native OAuth proxy | Inference |
| --- | --- | --- | --- |
| Low | `low` | `gemini-4-argon-low` | Completed; correct arithmetic answer |
| Medium | `medium` | `gemini-4-argon-medium` | Completed; correct arithmetic answer |
| High | `high` | `gemini-4-argon-high` | Completed; correct arithmetic answer |

Three fresh account-backed sessions used the same bounded arithmetic prompt and an independent integer
oracle. Native debug-to-stderr logging retained only allowlisted model IDs; raw logs and thought chunks
were discarded. There was no custom relay, request modification, executable patch, credential copy,
new login, or prompt-based effort instruction. A prior Low inference also completed but its attempt
stopped at a missing-log measurement boundary, so four test inferences were sent in total. Test
conversations remain in the provider-owned history; existing conversations were not changed or deleted.

## Source findings and limits

- The server rejects launch flags `--effort`, `--model`, and `--thinking_level`. Those CLI model/effort
  flags belong to the separate native `agy` executable, not this ACP server.
- `AGY_ACP_DEFAULT_MODEL` changes only the initial selection; it is not another inference control.
- The shipped `model_selection.py:257-264` documents server-side resolution from effort-qualified
  OAuth model IDs. A missing `thinkingConfig` alone therefore does not prove the suffix was ignored.
  The earlier unpublished upstream draft has been qualified; model self-reports are not telemetry.
- `agent-platform` is an official alternative that maps a selected suffix to the base Argon model
  plus explicit endpoint `thinking_level`. It requires separate Google Cloud access and quota/billing;
  no Agent Platform credentials were configured, so that path was source-inspected only.
- Compozy already uses the supported native family/effort mechanism. Independent suffix tests in the
  fresh round did not fix the self-report symptom. Web flows and project test/lint gates were not rerun.

The inspected ACP executable SHA-256 is
`57e7c4c505fdc8787993d4ff868274f44fd7aa4e4c82cf424fece3bf6c487643`.
Its bundled `server.py:2675-2879` owns selection, `server.py:4025-4069` constructs literal or mapped
targets, and `server.py:4581-4615` distinguishes personal OAuth from Agent Platform.

## Sources and retained evidence

- [Official registry](https://github.com/agentclientprotocol/registry/blob/main/antigravity-acp/agent.json)
  still published 1.4.1 on the fresh fetch.
- [ACP session configuration](https://agentclientprotocol.com/protocol/v1/session-config-options).
- [Official authentication methods](https://antigravity.google/docs/ide/extensions/zed/).
- [Native CLI model/effort flags](https://antigravity.google/docs/cli/headless.md).
- [Native CLI effort command](https://antigravity.google/docs/cli/commands/effort).
- [Argon API thinking levels](https://aistudio.google.com/learn/gemini-4-argon-developer-guide).

Earlier routing-only lab evidence and manifest:
`/home/vscode/dev/qa-labs/compozy-agy-native-effort-investigation-20261021-105857-403468-lab/qa-artifacts/qa/`.
The directory retains `native-effort-results.json`, `attempt-1-native-effort-results.json`,
`source-findings.json`, and the standalone probe. The complete research note is
`/home/vscode/agy-acp-native-effort-investigation-2026-10-21.md`.

The final assertions confirm all three observed native model IDs and that the executable hash is
unchanged. The strict evidence audit reports no blockers or warnings; `teardown.json` records
`"clean": true`. No lab process remains. QA reports are excluded by the repository formatter's
ignore rules; no project-wide test/lint/typecheck gate was run for this documentation-only result.
