/**
 * Wire types for the calls and mailbox surfaces.
 *
 * The daemon's OpenAPI spec builder inlines every schema, so there are no named
 * `components["schemas"]["Call*"]` entries to import. Everything below derives
 * from `operations[...]` through the shared contract helpers — nothing here is
 * hand-mirrored, and a daemon-side rename breaks the build rather than drifting.
 *
 * `CHILD_STATES` is the only local view union: the wire has no `child_state`
 * field yet, so the web must not invent parked/gone from session stop reasons.
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
/** One-call admission body; batch-only fields are rejected at this boundary. */
type SingleCallRequest<T> = T extends { target: unknown; prompt: string } ? T : never;
export type CreateSingleCallRequest = SingleCallRequest<CreateCallRequest>;
export type CreateCallResponse = OperationResponse<"createCallWorkspace", 201>;
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

export type StopSessionDrainResponse = OperationResponse<"stopSession", 200>;

// --- Closed contract unions -------------------------------------------------

/** Nine call states, taken from the list item the daemon already closes. */
export type CallState = CallPayload["state"];

/** How an admitted result arrived. `extracted` never renders as `returned`. */
export type CallVerdict = NonNullable<CallPayload["verdict"]>;

/** The public delivery receipts. The runtime does not model read/seen. */
export type CallDelivery = SendCallMessageResponse["delivery"];

const CALL_STATE_BY_VALUE = {
  queued: "queued",
  running: "running",
  completed: "completed",
  "invalid-result": "invalid-result",
  "completed-without-result": "completed-without-result",
  failed: "failed",
  canceled: "canceled",
  timeout: "timeout",
  expired: "expired",
} as const satisfies Record<CallState, CallState>;

export const CALL_STATES = Object.values(CALL_STATE_BY_VALUE);

const CALL_VERDICT_BY_VALUE = {
  returned: "returned",
  extracted: "extracted",
  repaired: "repaired",
} as const satisfies Record<CallVerdict, CallVerdict>;

export const CALL_VERDICTS = Object.values(CALL_VERDICT_BY_VALUE);

const CALL_DELIVERY_BY_VALUE = {
  attention: "attention",
  "delivered-into-turn": "delivered-into-turn",
  woke: "woke",
  queued: "queued",
  failed: "failed",
} as const satisfies Record<CallDelivery, CallDelivery>;

export const CALL_DELIVERIES = Object.values(CALL_DELIVERY_BY_VALUE);

/** Daemon-owned delegated-child lifecycle projected through the session catalog. */
export const CHILD_STATES = ["running", "parked", "gone"] as const;

export type ChildState = (typeof CHILD_STATES)[number];
