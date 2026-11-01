import { humanizeAgentSlug } from "./agent-identity";
import { GLOBAL_SCOPE } from "./agent-instance";
import type { AgentAppearancePayload, AgentInstancePayload, AgentPayload } from "../types";

/**
 * The daemon's row so far carries less than `_dx.md` promises. Anything it leaves out reads as
 * unknown, so a surface never claims a state or prints `undefined`.
 */
export function normalizeAgentInstance(
  raw: Partial<AgentInstancePayload> & Pick<AgentInstancePayload, "id" | "agent_name" | "state">
): AgentInstancePayload {
  const workspaceId = raw.workspace_id ?? "";
  return {
    ...raw,
    workspace_id: workspaceId,
    label: raw.label || humanizeAgentSlug(raw.agent_name),
    scope: raw.scope || workspaceId || GLOBAL_SCOPE,
    resolved_appearance: raw.resolved_appearance ?? (null as unknown as AgentAppearancePayload),
    activity: raw.activity ?? "unknown",
    needs_attention: raw.needs_attention ?? false,
  };
}

/** The definition is gone but its instances remain: enough of it to explain what happened. */
export function agentFromOrphanInstance(instance: AgentInstancePayload): AgentPayload {
  return {
    name: instance.agent_name,
    label: instance.label,
    kind: "stays_on",
    provider: instance.provider ?? "",
    prompt: "",
    origin: "global",
    definition_digest: "",
    resolved_appearance: instance.resolved_appearance,
  } as unknown as AgentPayload;
}
