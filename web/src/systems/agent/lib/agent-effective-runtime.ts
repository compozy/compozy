import type { AgentPayload } from "../types";
import type { RuntimeSelectorValue } from "@/systems/runtime";

export function resolveAgentRuntimeValue(agent: AgentPayload | undefined): RuntimeSelectorValue {
  const effective = agent?.effective_runtime;
  return {
    provider: effective?.provider?.trim() || agent?.provider?.trim() || "",
    model: effective?.model?.trim() || agent?.model?.trim() || "",
    reasoning_effort: effective?.reasoning_effort ?? agent?.reasoning_effort ?? "",
  };
}

export function inheritedAgentRuntimeFields(agent: AgentPayload): string[] {
  const sources = agent.effective_runtime?.sources;
  if (!sources) return [];

  const inherited: string[] = [];
  if (sources.provider !== "agent") inherited.push("provider");
  if (sources.model && sources.model !== "agent") inherited.push("model");
  if (sources.reasoning_effort && sources.reasoning_effort !== "agent") {
    inherited.push("reasoning");
  }
  return inherited;
}
