import { AGENT_COMMS_ERROR_CODES, type AgentCommsErrorCode } from "../adapters/agent-comms-api";

const CALL_CREATE_FAILURE_COPY: Partial<Record<AgentCommsErrorCode, string>> = {
  call_expect_invalid:
    "That answer shape isn't usable. Provide an example of the answer, or a full JSON Schema.",
  call_agent_unknown: "There is no agent by that name here.",
  call_prompt_empty: "A call always carries work — say what you want done.",
  call_target_expired:
    "That helper sat idle past its limit and left. Calling the agent again starts a fresh one.",
  call_children_cap: "This caller already has as many helpers as it may run at once.",
  call_depth_exceeded: "Delegation is already as deep as it goes here.",
  call_workspace_denied: "That agent belongs to another workspace.",
};

const CALL_MESSAGE_FAILURE_COPY: Partial<Record<AgentCommsErrorCode, string>> = {
  message_target_blocked:
    "That helper is waiting on a decision from you. Answer it on its own screen first — a message cannot approve a pending permission.",
  message_rate_limited: "Too many messages in the last minute. Try again shortly.",
  message_duplicate: "That exact message just went out. The original is already on its way.",
  message_too_large: "That message is longer than the runtime accepts. Shorten it and send again.",
  message_pending_cap:
    "That helper already has as many undelivered messages as it can hold. Wait for it to work through them.",
  call_target_expired:
    "That helper sat idle past its limit and left. Ask the agent again to start a fresh one.",
  call_target_denied: "That helper is outside your lineage, so you cannot message it.",
  call_workspace_denied: "That helper belongs to another workspace.",
};

const FALLBACK_CREATE_COPY = "The call was not accepted.";
const FALLBACK_MESSAGE_COPY = "The message did not go out.";

function copyFor(
  table: Partial<Record<AgentCommsErrorCode, string>>,
  fallback: string,
  code: string | null
): string {
  if (code === null) return fallback;
  const known = (AGENT_COMMS_ERROR_CODES as readonly string[]).includes(code)
    ? table[code as AgentCommsErrorCode]
    : undefined;
  return known ?? fallback;
}

export function callCreateFailureCopy(code: string | null): string {
  return copyFor(CALL_CREATE_FAILURE_COPY, FALLBACK_CREATE_COPY, code);
}

export function callMessageFailureCopy(code: string | null): string {
  return copyFor(CALL_MESSAGE_FAILURE_COPY, FALLBACK_MESSAGE_COPY, code);
}
