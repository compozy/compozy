import type { AgentPayload } from "../types";
import type { RuntimeACPOptionSelection, RuntimeSelectorValue } from "@/systems/runtime";

type AgentACPOption = NonNullable<AgentPayload["acp_options"]>[number];
type EffectiveACPOption = NonNullable<
  NonNullable<AgentPayload["effective_runtime"]>["acp_options"]
>[number];

export function runtimeACPSelections(
  options: readonly (AgentACPOption | EffectiveACPOption)[] | undefined
): RuntimeACPOptionSelection[] | undefined {
  if (!options || options.length === 0) return undefined;
  const selections: RuntimeACPOptionSelection[] = [];
  for (const option of options) {
    const id = option.id.trim();
    if (id.length === 0) continue;
    const valueID = option.value_id?.trim();
    if (valueID) {
      selections.push({ id, value_id: valueID });
      continue;
    }
    if (typeof option.bool_value === "boolean") {
      selections.push({ id, bool_value: option.bool_value });
    }
  }
  return selections.length > 0 ? selections : undefined;
}

export function resolveAgentRuntimeValue(agent: AgentPayload | undefined): RuntimeSelectorValue {
  const effective = agent?.effective_runtime;
  const acpOptions = runtimeACPSelections(effective?.acp_options ?? agent?.acp_options);
  return {
    provider: effective?.provider?.trim() || agent?.provider?.trim() || "",
    model: effective?.model?.trim() || agent?.model?.trim() || "",
    reasoning_effort: effective?.reasoning_effort ?? agent?.reasoning_effort ?? "",
    ...(acpOptions ? { acp_options: acpOptions } : {}),
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
