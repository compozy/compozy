---
title: Loop inputs that know what they point at
type: feature
---

A Loop input can declare the kind of thing it accepts, and every surface that edits inputs now validates against the real workspace catalog before anything starts. A wrong agent name, a retired skill, or an unsupported model is caught at the field that caused it instead of failing deep inside a run. (#427, #438)

- New input types: `agent` (an exact agent name), `ref` with a closed `ref.kind` of `skill`, `loop`, `worktree`, `session`, `workspace`, or `secret`, and `runtime` (`{ provider?, model?, reasoning? }`, accepting exact custom model IDs). A string-like input may still declare `enum`, and those choices take precedence over a catalog picker.
- Effective values resolve one field at a time: run input, then workspace config, then global config, then the definition default. The daemon validates the winning value immediately before a dry run or a run, including entity existence and runtime support.
- A failure starts no run, creates no task and no external action, and returns the same `input_validation` payload — `{ loop, field, kind?, value?, origin, reason }` — over HTTP, UDS, CLI, native tools, and the web form, which attaches the reason to that field.
- The same typed controls appear wherever Loop inputs are edited: the run form, scheduled automation, event-trigger mappings, fork and amend flows, and human-request answers annotated with `x-compozy-kind`. Every surface submits the exact stored identifier — a display label is never treated as a reference.
- The run form reuses the canonical runtime selector instead of free-text provider, model, and reasoning fields. From the CLI, a runtime input also accepts the compact `provider/model@reasoning` form, with `-` leaving provider or model unset.
- `compozy loop run` prompts in an interactive terminal only for supported required inputs still missing after defaults; `--no-prompt` fails instead, and structured or non-interactive input never prompts, so scripts stay deterministic.
- Secret inputs expose Vault reference names and metadata only — a secret value never enters a catalog or an error payload. Two native tools back the new pickers: `compozy__agent_list` and metadata-only `compozy__vault_list`.
- `params.runtime` binds a declared `type: runtime` input through an exact reference such as `{{ .inputs.worker_runtime }}`. Provider, model, and reasoning are validated at bind time, while literal runtime objects keep compile-time typo detection, and the resolved runtime records `input` provenance.
- If a saved reference is no longer listed, the field keeps the exact value visible so the daemon returns a field-level error instead of silently replacing it.

```yaml
inputs:
  reviewer: { type: agent, default: code_reviewer }
  release_token: { type: ref, ref: { kind: secret } }
  worker_runtime:
    { type: runtime, default: { provider: codex, model: gpt-5.5-codex, reasoning: high } }
```
