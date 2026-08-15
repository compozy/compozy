import type { SessionPayload, SessionRuntimeEffective, SessionRuntimeSelection } from "../types";
import type { SessionPromptRuntimeSnapshot } from "../contexts/session-prompt-runtime-context-value";

export type SessionPromptCapability = "unknown" | "supported" | "unsupported";

type PromptCapabilityKey = "prompt_image" | "prompt_embedded_context";

function normalized(value: string | null | undefined): string {
  return value?.trim() ?? "";
}

function normalizedSpeed(value: "fast" | "normal" | null | undefined): "fast" | "normal" {
  return value ?? "normal";
}

function targetsSameRuntime(
  selected: SessionRuntimeSelection | SessionPromptRuntimeSnapshot,
  effective: SessionRuntimeEffective
): boolean {
  return (
    normalized(selected.provider) === normalized(effective.provider) &&
    normalized(selected.model) === normalized(effective.model) &&
    normalized(selected.reasoning_effort) === normalized(effective.reasoning_effort) &&
    normalizedSpeed(selected.speed) === normalizedSpeed(effective.speed)
  );
}

/**
 * ACP capabilities belong to the active agent binding, never the model picker.
 * Until the session has a live binding that matches its selected next-prompt
 * runtime, local UI gates must defer to the daemon's authoritative admission.
 */
export function sessionPromptCapability(
  session: SessionPayload,
  capability: PromptCapabilityKey,
  promptRuntime: SessionPromptRuntimeSnapshot | null = null
): SessionPromptCapability {
  const { runtime } = session;
  const effective = runtime.effective;
  const selected = promptRuntime ?? runtime.selected;
  const caps = runtime.acp_caps;

  if (
    session.state !== "active" ||
    runtime.status !== "ready" ||
    !effective ||
    !caps ||
    (selected && !targetsSameRuntime(selected, effective))
  ) {
    return "unknown";
  }

  return caps[capability] ? "supported" : "unsupported";
}
