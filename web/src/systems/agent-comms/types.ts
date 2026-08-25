/**
 * Wire types for the calls and mailbox surfaces.
 *
 * The daemon's OpenAPI spec builder inlines every schema, so there are no named
 * `components["schemas"]["Call*"]` entries to import. Everything below derives
 * from `operations[...]` through the shared contract helpers — nothing here is
 * hand-mirrored, and a daemon-side rename breaks the build rather than drifting.
 *
 * Only the closed view unions at the bottom are declared locally: they are the
 * `_dx.md` vocabulary, which the wire carries as bare `string`.
 */
import type { OperationQuery, OperationRequestBody, OperationResponse } from "@/lib/api-contract";

// --- Calls: reads -----------------------------------------------------------

export type CallsListResponse = OperationResponse<"listCallsWorkspace", 200>;

/** One call record. `getCall` returns the same projection as a list item. */
export type CallPayload = CallsListResponse["items"][number];

/**
 * The exact filters the daemon accepts. Named verbatim from the contract —
 * `child_session_id` is the Received direction, `root_session_id` scopes one
 * delegation tree, `agent` scopes one definition.
 */
export type CallsListQuery = NonNullable<OperationQuery<"listCallsWorkspace">>;

export type CallResultResponse = OperationResponse<"getCallResultWorkspace", 200>;

/** The whole ask, when the inline `prompt_preview` was bounded. */
export type CallPromptResponse = OperationResponse<"getCallPromptWorkspace", 200>;

/** A result that arrived after the call settled — evidence, never a state change. */
export type CallSupersededResponse = OperationResponse<"getCallSupersededWorkspace", 200>;

// --- Calls: mutations -------------------------------------------------------

export type CreateCallRequest = OperationRequestBody<"createCallWorkspace">;
export type CreateCallResponse = OperationResponse<"createCallWorkspace", 201>;
export type CreateCallBatchResponse = OperationResponse<"createCallWorkspace", 200>;
export type CancelCallRequest = OperationRequestBody<"cancelCallWorkspace">;
export type CancelCallResponse = OperationResponse<"cancelCallWorkspace", 200>;

// `await` and `publish` are runtime operations with no operator surface in this
// app — awaiting is what the CLI and the native tool do, and `_uiux.md` S2 lists
// exactly three call-detail controls: cancel, call again, message child. Types
// and adapters for them are deliberately absent rather than unused.

/** The one error envelope every calls/messages route returns. */
export type CallErrorPayload = OperationResponse<"getCallWorkspace", 404>;

/** Roster rows the daemon attaches to a `call_agent_unknown` refusal. */
export type CallErrorRosterEntry = NonNullable<CallErrorPayload["available"]>[number];

// --- Messages ---------------------------------------------------------------

export type CallMessagesListResponse = OperationResponse<"listCallMessagesWorkspace", 200>;
export type CallMessagePayload = CallMessagesListResponse["items"][number];
export type CallMessagesListQuery = NonNullable<OperationQuery<"listCallMessagesWorkspace">>;
export type SendCallMessageRequest = OperationRequestBody<"sendCallMessageWorkspace">;
export type SendCallMessageResponse = OperationResponse<"sendCallMessageWorkspace", 202>;

// --- Subtree drain ----------------------------------------------------------
//
// "Stop subtree" is the session stop route carrying `{subtree: true, reason}`.
// The body is optional — omitting it stops only the named session and answers
// 204 — so the drain response is the 200 branch, and the two are read apart by
// status rather than by guessing from the payload.

export type StopSessionRequest = OperationRequestBody<"stopSession">;
export type StopSessionDrainResponse = OperationResponse<"stopSession", 200>;

// --- View unions (the `_dx.md` vocabulary) ----------------------------------

/**
 * A call is always in exactly one of these nine. The wire types them as
 * `string`; `toCallState` in `lib/call-state.ts` is the only narrowing seam.
 */
export const CALL_STATES = [
  "queued",
  "running",
  "completed",
  "invalid-result",
  "completed-without-result",
  "failed",
  "canceled",
  "timeout",
  "expired",
] as const;

export type CallState = (typeof CALL_STATES)[number];

/** How an admitted result arrived. `extracted` never renders as `returned`. */
export const CALL_VERDICTS = ["returned", "extracted", "repaired"] as const;

export type CallVerdict = (typeof CALL_VERDICTS)[number];

/**
 * The public delivery receipts. Internal transport states never surface, and no
 * read/seen state exists anywhere — the runtime does not model one.
 */
export const CALL_DELIVERIES = ["delivered-into-turn", "woke", "queued", "failed"] as const;

export type CallDelivery = (typeof CALL_DELIVERIES)[number];

/** A child session is working, resting, or gone. There is no fourth. */
export const CHILD_STATES = ["running", "parked", "gone"] as const;

export type ChildState = (typeof CHILD_STATES)[number];
