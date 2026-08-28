import type { AgentPayload } from "../types";
import type { RuntimeSpeed } from "@/lib/api-contract";
import {
  normalizeRuntimeACPSelections,
  type RuntimeACPOptionSelection,
  type RuntimeSelectorValue,
} from "@/systems/runtime";

type AgentACPOption = NonNullable<AgentPayload["acp_options"]>[number];
type EffectiveACPOption = NonNullable<
  NonNullable<AgentPayload["effective_runtime"]>["acp_options"]
>[number];

export function runtimeACPSelections(
  options: readonly (AgentACPOption | EffectiveACPOption)[] | undefined
): RuntimeACPOptionSelection[] | undefined {
  return normalizeRuntimeACPSelections(options);
}

export interface AgentRuntimeOverrideValue {
  provider: string;
  model: string;
  reasoningEffort: string;
  speed: string;
  acpOptions?: readonly RuntimeACPOptionSelection[];
}

export function hasAgentRuntimeOverride(value: AgentRuntimeOverrideValue): boolean {
  return Boolean(
    value.provider.trim() ||
    value.model.trim() ||
    value.reasoningEffort ||
    value.speed ||
    (value.acpOptions?.length ?? 0) > 0
  );
}

export function normalizeRuntimeSpeed(value: string | null | undefined): RuntimeSpeed | "" {
  return value === "normal" || value === "fast" ? value : "";
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
