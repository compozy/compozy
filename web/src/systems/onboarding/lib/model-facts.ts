import type { RuntimeModelOption } from "@/systems/runtime";

/**
 * One fact about the selected model. `value` is the emphasised token (a price);
 * `label` is the words that give it meaning.
 */
export interface OnboardingModelFact {
  id: string;
  value: string | null;
  label: string;
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

/**
 * First-run facts for the selected model. Only the price earns a line here —
 * context size, tool support, and reasoning levels live in the model picker.
 * An unknown price is omitted rather than rendered as zero.
 */
export function onboardingModelFacts(model: RuntimeModelOption | undefined): OnboardingModelFact[] {
  const input = model?.cost_input ?? null;
  const output = model?.cost_output ?? null;
  if (input === null || output === null) return [];
  return [
    {
      id: "price",
      value: `$${formatRate(input)} in / $${formatRate(output)} out`,
      label: "per million tokens",
    },
  ];
}
