# Analysis: selector-runtime

Read-only exploration of the frontend runtime selector and session runtime-switch path.

## Scope

- Slice question: explain the Astra/Grok reasoning display and Cursor Grok 4.6 HTTP 500, then classify queue/session/window errors.
- Primary sources: `web/src/systems/{model-catalog,runtime,session}`, `internal/session`, and session/model-catalog API handlers.
- Sources read in full vs. sampled: selector mapper/state, session runtime hook/context/store, runtime API handler/status mapping, Cursor catalog parser/binding resolver, queue and window-manager handlers, and their canonical tests.

## Overview

There are two separate defects in the selector/runtime path. The API can publish a model with a non-empty public configuration matrix while omitting the top-level reasoning profile/default; the frontend then treats the matrix as authoritative for levels but cannot derive a default. More severely, selecting a Cursor logical model without a canonical `reasoning_effort` can reach `selectCursorTransportBinding`, which returns an untyped `fmt.Errorf` when no live binding matches. `StatusForSessionError` has no case for that error, so `PUT .../sessions/:id/runtime` becomes HTTP 500.

The screenshot's session/presence 404s and window-manager 409 are lifecycle/CAS signals. They do not arise from selector catalog mapping. The queue 400/404 family is also route/session admission state and must be debugged separately from model negotiation.

## Mechanisms / Patterns

- **Cursor logical model to transport binding:** `resolveCursorCatalogBinding` refreshes the live Cursor catalog and calls `selectCursorTransportBinding` for the logical model (`internal/session/manager_runtime_model_validation.go:44-75`). The selector matches exact reasoning, fast, and ACP option dimensions (`:193-253`). Empty reasoning only matches a binding with no reasoning (`:231-236`); it does not mean “choose a discovered default” unless `model.DefaultReasoningEffort` is populated (`:197-200`). Cursor rows generated from `cursor-agent models` do carry low/medium/high/xhigh and infer the display-name default (`internal/modelcatalog/live_model_rows_cursor.go:150-210`, `:253-264`).
- **500-producing untyped negotiation failure:** when the public/live model has no default and the request has `reasoning_effort:""`, all Cursor variants with effort are rejected and `selectCursorTransportBinding` returns plain `fmt.Errorf` for zero matches (`internal/session/manager_runtime_model_validation.go:206-222`). The runtime API passes manager errors directly to `StatusForSessionError` (`internal/api/core/handlers_session_runtime.go:42-53`); that mapper classifies `ErrInvalidRuntimeOverride`/provider negotiation but defaults unknown errors to 500 (`internal/api/core/session_workspace.go:188-208`, `:211-242`, `:168`). This is the concrete Grok 4.6 HTTP 500 chain. A binding mismatch or invalid advertised option should be a typed validation/negotiation error (HTTP 400/422), never an internal error.
- **Top-level reasoning profile can disappear:** merge resets `ReasoningEfforts` and `DefaultReasoningEffort`, then chooses the highest-priority non-models.dev profile (`internal/modelcatalog/reasoning_merge.go:5-22`). With no applicable provider strategy and no matching reasoning transport binding it clears the fields again (`:24-39`). The daemon's `effectiveCatalogReasoningApply` derives that map from provider config (`internal/daemon/model_catalog_reasoning.go:10-34`); built-in Cursor has no `Models.Reasoning.Apply` entry (`internal/config/provider_builtin.go:73-94` and Cursor entry around `:79-92`), so this is a fragile policy boundary for newly discovered providers. Public projection copies top-level fields and separately emits every transport binding as `configurations` (`internal/api/modelcatalogprojection/projection.go:21-49`, `:93-115`). Existing live Cursor parser tests expect Grok 4.6's efforts and default (`internal/modelcatalog/live_sources_test.go:374-390`), but there is no end-to-end assertion that the API payload preserves them after stored-row hydration/merge.
- **Frontend matrix fallback is incomplete:** `modelEfforts` uses `configurations ? configured : rawEfforts` (`web/src/systems/model-catalog/lib/to-runtime-selector-options.ts:97-109`). Any present matrix suppresses top-level `reasoning_efforts`; if the matrix has rows with no `reasoning_effort`, the selector loses valid raw levels. The default is accepted only from `default_reasoning_effort` and only when it is in the derived efforts (`:149-168`). A model with `supports_reasoning:true` and no derived levels renders provider-decides (`web/src/systems/runtime/components/runtime-selector/types.ts:191-210`), which explains a fixed/no-level Astra/Grok rail when payload capability fields are incomplete. Matrix rows that do contain reasoning efforts do derive levels, so the observed payload shape must be captured in a regression fixture rather than assumed from the presence of `configurations` alone.
- **Session context drops ACP option selections:** the session runtime context reconstructs effective/selected objects with provider/model/reasoning/speed but omits `acp_options` (`web/src/systems/session/contexts/session-prompt-runtime-context.tsx:17-59`, values extracted at `:66-74`). The hook and store otherwise normalize, compare, and persist ACP options (`web/src/systems/session/hooks/use-session-prompt-runtime.ts:29-39`, `:45-61`; `web/src/systems/session/stores/session-prompt-runtime-store.ts:161-189`, `:213-225`). A server-selected Cursor `thinking` value can therefore vanish on session hydration/refetch and later model switches can submit a different binding dimension.
- **Revision race on rapid picker changes:** every selector change aborts the previous request but sends the same `input.selectionRevision` until a successful response updates the store (`web/src/systems/session/hooks/use-session-prompt-runtime.ts:150-203`). If an aborted request has already committed, the next request can carry a stale expected revision and receive `ErrRuntimeSelectionConflict`/HTTP 409 (`internal/session/manager_runtime_selection.go:67-90`, `internal/api/core/session_workspace.go:211-242`). The hook only invalidates the exact session cache on 409 (`web/src/systems/session/hooks/use-session-runtime-selection.ts:26-33`), so rapid model/effort changes can appear as rollback/toast rather than converge.
- **Lifecycle errors are independent:** workspace-scoped session routes first resolve workspace and then call `Sessions.Status`; a missing/stale session or workspace mismatch is deliberately 404 (`internal/api/core/workspace_scope.go:101-120`, `:142-162`). The queue list validates the route before reading the queue (`internal/api/core/session_input.go:12-35`), so a stale session ID cannot be repaired by catalog changes. Window-manager commands use their own revision CAS and map `ErrRevisionConflict` to 409 (`internal/api/core/window_manager_handlers.go:42-86`; `internal/api/core/window_manager_errors.go:41-73`). The browser's `aria-hidden` focus warning is a UI focus lifecycle issue, outside model negotiation.

## Relevant Sources

- `internal/session/manager_runtime_model_validation.go:44-75,193-253` — Cursor catalog refresh, default fallback, exact binding match, untyped mismatch error.
- `internal/api/core/handlers_session_runtime.go:14-54` — runtime PUT decode, manager call, status mapping.
- `internal/api/core/session_workspace.go:142-242` — 404/400/422/409/500 classification.
- `internal/modelcatalog/live_model_rows_cursor.go:150-210,253-264` — Cursor effort extraction and default inference.
- `internal/modelcatalog/reasoning_merge.go:5-39` — effective profile reset and strategy/binding gate.
- `internal/api/modelcatalogprojection/projection.go:21-49,93-115` — public top-level profile vs. configurations projection.
- `web/src/systems/model-catalog/lib/to-runtime-selector-options.ts:97-109,142-169` — selector capability derivation and default sanitation.
- `web/src/systems/runtime/components/runtime-selector/types.ts:100-120,191-210` — effort/fast support and reasoning footer mode.
- `web/src/systems/session/contexts/session-prompt-runtime-context.tsx:17-59,66-128` — runtime hydration omission of ACP options.
- `web/src/systems/session/hooks/use-session-prompt-runtime.ts:150-235` — optimistic persistence, abort/revision behavior.
- `internal/modelcatalog/live_sources_test.go:295-402` — canonical Cursor parser/fixture coverage.
- `web/src/systems/model-catalog/lib/__tests__/to-runtime-selector-options.test.ts:87-161` and `web/src/systems/runtime/components/runtime-selector/__tests__/runtime-selector.test.tsx:491-573,603+` — current canonical frontend tests; no live Astra/Grok payload or matrix-without-efforts regression.

## Transferable Patterns

- **Keep public capability and transport binding truth aligned:** derive the public effort set/default from the same live binding matrix when top-level fields are absent, while preserving provider-default as an explicit empty request only when a unique default binding is known.
- **Type every admission failure:** wrap zero/multiple Cursor binding matches and unknown ACP options with `ErrInvalidRuntimeOverride` plus the provider-negotiation diagnostic so the runtime endpoint returns a client-actionable 400/422 and logs the requested dimensions.
- **Add one end-to-end catalog-to-runtime invariant:** the Cursor Grok 4.6 fixture should round-trip parser → store/merge → API payload → selector value and then select the unique `high`/`xhigh` binding; this catches omissions that parser-only tests miss.
- **Preserve `acp_options` through session context:** pass effective/selected ACP options into `runtimeInputFromValues` and include them in dependencies so server state and selector state remain equal.
- **Rebase or serialize runtime writes:** after aborting an in-flight mutation, use the latest acknowledged revision (or let one mutation queue behind the prior response) before sending the next selection.

## Risks / Mismatches

- Setting Cursor's provider-wide `ReasoningApply` to `acp_option` would make top-level efforts visible, but the Cursor runtime resolver uses transport aliases rather than a generic ACP option. Provider policy and transport binding selection should not be conflated; a regression must prove both catalog display and launch alias.
- Treating every `configurations` row as a reasoning level is unsafe: rows can describe only `fast` or `thinking`. Merge effort values from valid reasoning rows and raw fields; do not fabricate a level from a boolean option.
- Returning 422 for a stale live catalog may still leave the selector stale. The UI needs a refresh/retry path and the backend must report that the live catalog changed; hiding the error would recreate the HTTP 500 symptom.

## Open Questions

- The parent’s live GET observation (Cursor/Astra `configurations` present but top-level efforts/default omitted) needs one captured payload plus source/status rows to identify whether omission occurs in persisted hydration, merge profile selection, or API serialization. Parser and projection code alone both preserve the fields in their canonical fixtures.
- The exact queue 400 in the browser log is not attributable from the URL alone: GET queue routes normally map stale/mismatched sessions to 404, while 400 can come from missing/invalid route or prompt admission. Capture HTTP method and response body before changing queue logic.
- A concrete window-manager 409 response body would confirm the expected revision mismatch; its code path is independent and already typed.

## Evidence

- `internal/modelcatalog/live_sources_test.go`
- `internal/session/manager_runtime_model_validation.go`
- `internal/api/core/session_workspace.go`
- `web/src/systems/session/contexts/session-prompt-runtime-context.tsx`
- `web/src/systems/session/hooks/use-session-prompt-runtime.ts`
