# Upstream model and reasoning capability discovery

Date: 2026-09-09

This slice compares the current OpenAI Codex app-server, Cursor, xAI, ACP, and
OpenCode contracts with the local competitor sources under `.resources/`. The
central finding is that model support, selectable effort values, default effort,
reasoning output visibility, and the transport payload are separate pieces of
metadata. A selector must be built from the provider's current per-model
advertisement; model names and a global effort ladder cannot be the authority.

## OpenAI Codex

The official [Codex app-server documentation](https://learn.chatgpt.com/docs/app-server)
explicitly tells clients to call `model/list` before rendering model or
personality selectors. The response includes `id`, `model`, `displayName`,
`hidden`, `supportedReasoningEfforts`, `defaultReasoningEffort`, input
modalities, personality support, and the default marker. `includeHidden` is
available when a complete catalog is needed; `isDefault` is the picker default,
not a reasoning default.

The checked-in upstream implementation confirms the contract:

- `.resources/codex/codex-rs/app-server-protocol/src/protocol/v2/model.rs:53-63`
  defines paginated `model/list` parameters and `includeHidden`.
- `.resources/codex/codex-rs/app-server-protocol/src/protocol/v2/model.rs:92-120`
  defines the per-model capability fields, and `:136-151` defines each
  reasoning option as a value plus description.
- `.resources/codex/codex-rs/app-server/src/models.rs:13-24` obtains the
  account/provider catalog through `ThreadManager::list_models` and filters
  hidden entries only for the default picker. `:27-79` maps the upstream
  `ReasoningEffortPreset` list and default into the wire response without
  rebuilding an effort ladder from the model name.
- `.resources/codex/codex-rs/models-manager/src/manager.rs:416-513` shows the
  refresh policy. `OnlineIfUncached` may use a cache; `Online` fetches again;
  remote entries are merged by exact model slug, and visible ChatGPT catalogs
  can be authoritative. `:516-553` rejects a cache whose client version or
  provider/auth identity no longer matches.

The local Codex bundle already contains the reported Astra capability. In
`.resources/codex/codex-rs/models-manager/models.json:4-65`, `gpt-6-astra`
advertises `low`, `medium`, `high`, `xhigh`, `max`, and `ultra`, including a
description for every value. It also says `supports_reasoning_summaries` at
`:170-171`. This bundled file currently says Astra's default is `low` (`:40`),
while the parent task's live Codex 0.153.4 `model/list` probe reported Astra's
default as `medium`. That discrepancy is evidence that the runtime,
account-scoped `model/list` result must win over a bundled/static resource.
The same probe reported Sol with `low` as its default and Astra/Sol with
`ultra`; Luna did not advertise `ultra`. Those values must be treated as the
observed runtime response, not as rules inferred from IDs.

The Codex wire type is deliberately forward-compatible. In
`.resources/codex/codex-rs/protocol/src/openai_models.rs:55-70`,
`ReasoningEffort` includes `Ultra` and `Custom(String)`; `:73-86` serializes
the exact wire string; `:139-155` parses all known values and preserves any
non-empty future value as `Custom`. `turn/start` accepts an `effort` override at
`.resources/codex/codex-rs/app-server-protocol/src/protocol/v2/turn.rs:230-235`,
and the same file explicitly says at `:253-256` that proactive multi-agent
behavior uses `effort: "ultra"`. Thus `ultra` is accepted by the current Codex
protocol. Whether a selected model may use it remains a per-model
`supportedReasoningEfforts` question.

Compozy's native Codex probe is on the right transport path: it launches a
short-lived app-server, initializes JSON-RPC, reads every `model/list` page,
and protects cursor loops at `internal/modelcatalog/live_sources_codex.go:40-138`.
However, the projection at `:163-184` currently normalizes efforts through the
global canonical vocabulary. `internal/modelcatalog/live_model_rows.go:259-265`
and `internal/reasoning/effort.go:55-62` will drop future provider strings that
Codex itself would preserve as `Custom`. Keep the raw non-empty advertised
value (or an explicit forward-compatible value type) and retain the provider's
description. A global enum can still provide display ordering for known values,
but must not decide whether an unknown advertised value is supported.

The offline bootstrap remains a fallback only: `internal/config/provider_reasoning.go:167-208`
contains three curated Codex models and a hardcoded `none` through `max` ladder,
with `medium` as the static default. A successful live `model/list` observation
must replace or outrank that bootstrap row, so the bootstrap cannot hide newly
released models or erase a provider-specific effort such as `ultra`.

## Cursor and xAI

The official [Cursor CLI parameter reference](https://cursor.com/docs/cli/reference/parameters)
provides `--model`, `--list-models`, and the `agent models` command. The CLI
reference establishes model discovery and selection, but it does not define a
universal reasoning ladder. Cursor's richer [Cloud Agents `/v1/models`
contract](https://prod.cursor.com/docs/cloud-agent/api/endpoints) returns each
model's ID, display name, aliases, parameters, and concrete `params`/variant
combinations. The create-agent endpoint says to use only `id`/`params` pairs
returned by `/v1/models`, including per-model reasoning effort or context-window
parameters. This is the correct shape for Cursor: model options are a matrix,
not a single global effort list.

The local Synara implementation demonstrates this end to end. Its ACP extension
`cursor/list_available_models` is documented and projected from each model's
own config options at `.resources/synara/apps/server/src/provider/acp/CursorAcpSupport.ts:423-535`.
It extracts the model-specific effort select, default value, context-window
options, thinking toggle, and fast mode. Tests encode parameterized Grok IDs
such as `grok-4.5[effort=high,fast=false]` and
`grok-4.6[effort=high,fast=false]` at
`.resources/synara/apps/server/src/provider/acp/CursorAcpSupport.test.ts:800-892`.
This explains why a static provider-wide “reasoning supported” flag loses
Cursor's real capabilities.

The official [xAI reasoning documentation](https://docs.x.ai/developers/model-capabilities/text/reasoning)
is explicit about the distinction:

- `grok-4.6` and `grok-4.5` support `reasoning_effort`; the default is `high`
  when omitted and reasoning cannot be disabled.
- The valid values are `low`, `medium`, and `high`; `xhigh` is available on
  Grok 4.6 and later. Grok 4.5 does not support xhigh and treats it as high.
- xAI's Chat SDK/OpenAI-compatible Chat shape calls the field
  `reasoning_effort`, while its OpenAI-compatible Responses shape uses
  `reasoning: { effort: ... }`.
- For `grok-4.20-multi-agent`, the same `reasoning.effort` controls agent count,
  not reasoning depth. Reasoning support, effort values, output summaries, and
  the meaning of an effort value are therefore separate metadata.

Synara also records the transport limitation instead of pretending every ACP
agent supports live mutation. `.resources/synara/docs/archive/RECAP-grok-provider.md:21-25`
and `.resources/synara/apps/server/src/provider/acp/GrokAcpSupport.ts:216-229`
state that its current Grok ACP advertises model state but does not implement
`session/set_config_option`, so model and effort are process-start arguments and
model changes restart the process. The test at
`.resources/synara/apps/server/src/provider/acp/GrokAcpSupport.test.ts:74-95`
verifies `grok-4.6` plus `--reasoning-effort xhigh`. A Compozy adapter must
discover whether a provider supports live config, and use startup/restart
semantics when it does not.

## ACP session configuration

The stable [ACP Session Config Options specification](https://agentclientprotocol.com/protocol/v1/session-config-options)
defines model, model configuration, and thought-level options as agent-owned
selects. `session/new` returns the complete set of options and their current
values. `session/set_config_option` must return the complete resulting state;
the specification gives changing model-dependent reasoning options as the
example. Agents can also send `config_option_update` with the complete state
when a fallback, rate limit, or other runtime event changes availability.

The local ACPX docs agree with this model: `.resources/acpx/skills/acpx/SKILL.md:179-199`
and `.resources/acpx/docs/session-control.md:51-61` say current Codex ACP
releases advertise separate `model` and `reasoning_effort` options, save only
accepted selections, and reconcile or remove an effort after a model switch.
`.resources/acpx/agents/Codex.md:7-12` documents the same behavior and the
legacy combined model form for adapters that do not advertise config options.
The client path at `.resources/acpx/src/acp/client.ts:1424-1489` forwards the
literal config value and records the complete returned option list. It does not
need to know an enum for `ultra`; it must validate against the adapter's
advertised values and surface the adapter's rejection.

## OpenCode discovery and metadata

OpenCode separates the human-readable model list from its capability metadata.
The command implementation at
`.resources/opencode/packages/opencode/src/cli/cmd/models.ts:8-45` prints one
`provider/model` line per model by default. With `--verbose`, it prints the
model's JSON metadata after each line; `--refresh` refreshes the cache. The docs
at `.resources/opencode/packages/web/src/content/docs/cli.mdx:306-335` describe
the same flags. A parser that reads only default output can discover IDs but
cannot learn reasoning values.

The metadata schema at `.resources/opencode/packages/core/src/models-dev.ts:52-76`
has an independent `reasoning: boolean` and optional `reasoning_options`. An
effort option carries an arbitrary string array; other models can expose a
toggle or a numeric token budget. `reasoning_options` may be absent or an empty
array, and those states must not be collapsed into “model has no reasoning”.
The source is `https://models.opencode.ai/api.json`, with a five-minute disk
cache and explicit forced refresh at `:160-181` and `:217-260`.

As observed from that endpoint on 2026-09-09, the records were:

| provider/model                   | reasoning | advertised effort values                    |
| -------------------------------- | --------- | ------------------------------------------- |
| `xai/grok-4.5`                   | true      | low, medium, high                           |
| `xai/grok-4.6`                   | true      | low, medium, high, xhigh                    |
| `xai/grok-4.20-multi-agent-0309` | true      | low, medium, high, xhigh                    |
| `xai/grok-build-0.1`             | true      | no selectable values in this catalog record |
| `openai/gpt-6-astra`             | true      | low, medium, high, xhigh, max               |

The endpoint is a catalog source, not proof that the current account or
provider credentials can use every row. OpenCode turns the metadata into
provider-specific variants at `.resources/opencode/packages/opencode/src/provider/provider.ts:1265-1320`.
Its `reasoningVariants` implementation preserves the distinction between
missing and empty options at `.resources/opencode/packages/opencode/src/provider/transform.ts:1704-1734`,
then maps `reasoning_effort` to the selected SDK/provider transport at `:1769-1826`.
The ACP layer builds the effort select only for the selected model's variants
at `.resources/opencode/packages/opencode/src/acp/config-option.ts:31-112` and
rejects values absent from those variants at
`.resources/opencode/packages/opencode/src/acp/service.ts:400-455`.

## Implementation implications

1. Store a per-provider, per-model capability record with the exact model ID,
   display name, visibility, `supportsReasoning`, advertised effort IDs and
   descriptions, default effort, summary/visibility support, and transport
   bindings. Use `(provider, model ID)` as the key; do not classify by words in
   the display name.
2. Keep four predicates separate: reasoning is available; selectable values are
   known; a default is supplied; reasoning text/summary can be returned. An
   empty effort list can coexist with `reasoning=true`.
3. Treat provider advertisements as authoritative for the active session. On a
   model change, replace the complete dependent option state, preserve the
   current effort only if the new list still contains it, otherwise select the
   advertised default or leave the provider-default sentinel. Publish the
   complete state after the change.
4. Keep effort IDs as strings at the catalog boundary. Preserve unknown,
   non-empty values from Codex/ACP/OpenCode for forward compatibility; known
   values may receive canonical labels and ordering. Never synthesize `ultra`,
   `xhigh`, or `none` merely because a model family name suggests them.
5. Bind transport payloads per provider/model. Examples are Codex `turn/start`
   `effort`, xAI Chat `reasoning_effort`, xAI Responses `reasoning.effort`, and
   Cursor parameter arrays. The same UI label can have different backend
   meaning, especially for multi-agent models.
6. For ACP, read and retain the complete `configOptions` from `session/new`,
   `session/set_config_option`, and `config_option_update`. Detect missing live
   mutation support and use a process restart/reprobe path instead of issuing a
   request that the adapter cannot implement.
7. For subprocess catalogs, expose structured output where available. OpenCode
   needs `--verbose` (or its JSON source) for capabilities; Cursor's CLI list
   alone is insufficient for the richer parameter matrix. Keep raw discovery
   source/version/freshness and a redacted error for diagnostics, with a
   clearly marked stale fallback.
8. Test model transitions and forward compatibility with fixtures that include
   Astra `ultra`, Luna without `ultra`, Grok 4.5 versus 4.6, a reasoning model
   with an empty option list, a model whose effort controls agent count, and an
   unknown future effort string. Assert the rendered option state and outgoing
   provider payload separately.

## Remaining uncertainty

- Codex runtime catalogs are account/auth and release dependent. The live
  `model/list` response should override the checked-in bundle; the Astra default
  discrepancy above is a concrete example.
- Cursor's public CLI reference documents listing and selection, while the
  authenticated `/v1/models` API documents the full parameter/variant matrix.
  Which surface is available depends on the running Cursor adapter and auth
  mode, so capability discovery must record the source used.
- OpenCode's public catalog describes model/provider metadata but does not prove
  entitlement or connectivity for a particular user's credentials. It is useful
  enrichment and fallback, while provider/ACP/runtime discovery should carry
  availability authority.
