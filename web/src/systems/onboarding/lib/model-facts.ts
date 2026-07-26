import type { RuntimeModelOption } from "@/systems/runtime";

/**
 * One fact about the selected model. `value` is the emphasised token (a number,
 * a price, a harness name); `label` is the word that gives it meaning. A fact
 * with no `value` is a plain capability statement ("tool calls").
 */
export interface OnboardingModelFact {
  id: string;
  value: string | null;
  label: string;
}

/** `acp` spawns a signed-in CLI; `pi_acp` talks to an API with a bound key. */
const HARNESS_LABELS: Record<string, string> = {
  acp: "CLI",
  pi_acp: "API key",
};

function formatContextWindow(tokens: number): string {
  if (tokens >= 1_000_000) {
    const millions = tokens / 1_000_000;
    return `${Number(millions.toFixed(1))}M`;
  }
  if (tokens >= 1_000) return `${Math.round(tokens / 1_000)}k`;
  return String(tokens);
}

/**
 * Two decimals read cleanly for the common rates, but a configured sub-cent rate
 * must never round to `0` — that would state a price the provider does not charge.
 */
function formatRate(rate: number): string {
  const rounded = Number(rate.toFixed(2));
  if (rounded > 0 || rate === 0) return String(rounded);
  return `<0.01`;
}

/** Input/output rates read as one price, so only the pair carries the currency. */
function formatRatePair(input: number, output: number): string {
  return `$${formatRate(input)}/${formatRate(output)}`;
}

function reasoningFact(model: RuntimeModelOption): OnboardingModelFact | null {
  // `none` is not a selectable stop in the selector, so it never counts as a level.
  const levels = model.efforts.filter(effort => effort !== "none").length;
  if (levels > 0) {
    return {
      id: "reasoning",
      value: String(levels),
      label: `reasoning level${levels === 1 ? "" : "s"}`,
    };
  }
  if (model.supports_reasoning) return { id: "reasoning", value: null, label: "reasoning on" };
  return null;
}

/**
 * Ordered facts for the selected model, each present only when the runtime
 * actually reports it — an unknown context window or price is omitted rather
 * than rendered as zero.
 */
export function onboardingModelFacts(
  model: RuntimeModelOption | undefined,
  harness: string | null
): OnboardingModelFact[] {
  const facts: OnboardingModelFact[] = [];
  if (model) {
    const contextWindow = model.context_window ?? null;
    if (contextWindow !== null && contextWindow > 0) {
      facts.push({ id: "context", value: formatContextWindow(contextWindow), label: "context" });
    }
    const input = model.cost_input ?? null;
    const output = model.cost_output ?? null;
    if (input !== null && output !== null) {
      facts.push({ id: "price", value: formatRatePair(input, output), label: "per Mtok" });
    }
    if (model.supports_tools) facts.push({ id: "tools", value: null, label: "tool calls" });
    const reasoning = reasoningFact(model);
    if (reasoning) facts.push(reasoning);
  }
  const harnessKey = harness?.trim().toLowerCase() ?? "";
  if (harnessKey.length > 0) {
    facts.push({
      id: "harness",
      value: HARNESS_LABELS[harnessKey] ?? harnessKey,
      label: "harness",
    });
  }
  return facts;
}
