import type { OperationRequestBody } from "@compozy/client";

export type {
  OperationId,
  OperationResponse,
  OperationRequestBody,
  OperationQuery,
  OperationPath,
} from "@compozy/client";

type PromptRuntimeSelection = NonNullable<OperationRequestBody<"sendSessionPrompt">["runtime"]>;

export type ReasoningEffort = NonNullable<PromptRuntimeSelection["reasoning_effort"]>;

/**
 * Provider-neutral ACP runtime speed (PR #267). "normal" is the daemon default;
 * surfaces omit it from POST bodies and send only an explicit "fast" request.
 * The daemon resolves the request at prompt dispatch (`speed_resolution`:
 * applied | unsupported | rejected) — there is no pre-session capability
 * signal, so UI treats this as intent, never as a guaranteed mode.
 */
export type RuntimeSpeed = NonNullable<PromptRuntimeSelection["speed"]>;

/** Validate the wire identifier; the selected model owns the allowed values. */
export function isReasoningEffort(value: string): value is ReasoningEffort {
  return value.length > 0 && !/[\s\p{Cc}]/u.test(value);
}
