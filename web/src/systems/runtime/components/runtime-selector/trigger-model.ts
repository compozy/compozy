import type { RuntimeSpeed } from "@/lib/api-contract";

import {
  reasoningEffortLabel,
  reasoningEffortPosition,
  resolveReasoningState,
  type RuntimeModelOption,
  type RuntimeProviderOption,
  type RuntimeSelectorValue,
  type RuntimeSelectorVariant,
} from "./types";

export interface RuntimeTriggerInput {
  value: RuntimeSelectorValue;
  provider: RuntimeProviderOption | undefined;
  model: RuntimeModelOption | undefined;
  variant: RuntimeSelectorVariant;
  needsAuth: boolean;
  modelPlaceholder: string;
  speed: RuntimeSpeed | undefined;
}

export interface RuntimeTriggerMeter {
  position: number;
  hollow: boolean;
}

export interface RuntimeTriggerView {
  compact: boolean;
  providerKind: string;
  modelName: string;
  /** Present only when the model exposes selectable levels and the variant has room. */
  meter: RuntimeTriggerMeter | null;
  showFast: boolean;
  /** Sign-in or availability problem; provider-managed settings are a normal state. */
  warningLabel: string | null;
  /** Spoken value: provider/model, reasoning, speed, and warning, in reading order. */
  valueSummary: string;
}

function warningLabelFor(needsAuth: boolean, model: RuntimeModelOption | undefined): string | null {
  if (needsAuth) return "Provider needs sign in";
  return model?.availability === "unavailable" ? "Model unavailable" : null;
}

// `||` not `??`: an unset model is "" (not nullish), so the placeholder must
// still win — otherwise the trigger renders blank in the no-model state.
function triggerModelName(input: RuntimeTriggerInput): string {
  const { model, value, provider, modelPlaceholder } = input;
  const fallback =
    provider?.runtime_strategy === "provider_managed" ? "Provider managed" : modelPlaceholder;
  return model?.name || value.model || fallback;
}

// The meter mirrors the slider: the model default fills the bars while the
// wire value is ""; only a level-less model renders the hollow zero state.
function triggerMeter(input: RuntimeTriggerInput): {
  meter: RuntimeTriggerMeter | null;
  spoken: string | null;
} {
  const reasoning = resolveReasoningState(input.model);
  if (reasoning.mode !== "levels" || input.variant === "compact") {
    return { meter: null, spoken: null };
  }
  const currentEffort = input.value.reasoning_effort || reasoning.defaultEffort;
  if (currentEffort === "") {
    return { meter: { position: 0, hollow: true }, spoken: "reasoning provider default" };
  }
  return {
    meter: { position: reasoningEffortPosition(currentEffort), hollow: false },
    spoken: `reasoning ${reasoningEffortLabel(currentEffort)}`,
  };
}

export function runtimeTriggerView(input: RuntimeTriggerInput): RuntimeTriggerView {
  const { value, provider, variant } = input;
  const compact = variant === "compact";
  const providerName = provider?.name || value.provider;
  const modelName = triggerModelName(input);
  const { meter, spoken: reasoningSpoken } = triggerMeter(input);
  const showFast = input.speed === "fast" && !compact;
  const warningLabel = warningLabelFor(input.needsAuth, input.model);

  const summaryParts = [
    compact ? providerName : `${providerName} / ${modelName}`,
    reasoningSpoken,
    showFast ? "fast speed requested" : null,
    warningLabel?.toLowerCase() ?? null,
  ].filter((part): part is string => part !== null);

  return {
    compact,
    providerKind: provider?.runtime_provider ?? provider?.id ?? value.provider,
    modelName,
    meter,
    showFast,
    warningLabel,
    valueSummary: summaryParts.join(", "),
  };
}
