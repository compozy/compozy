import { RadioCard } from "@agh/ui";

import type { LoopReattemptStrategy } from "../../lib/loop-config-draft";
import { MonoTag } from "../mono-tag";

interface LoopConfigureStrategyProps {
  value: LoopReattemptStrategy;
  disabled?: boolean;
  onChange: (strategy: LoopReattemptStrategy) => void;
}

interface StrategyCard {
  value: LoopReattemptStrategy;
  label: string;
  testIdSuffix: string;
  isDefault: boolean;
  description: string;
}

/** The two runtime re-attempt strategies (ADR-009); `failed_only` is the runtime default. */
const STRATEGY_CARDS: StrategyCard[] = [
  {
    value: "failed_only",
    label: "failed-only",
    testIdSuffix: "failed-only",
    isDefault: true,
    description:
      "Re-runs only the work that failed the review gate, plus its downstream steps. Handled work carries forward; the next tick re-fetches anything still unresolved.",
  },
  {
    value: "full_body",
    label: "full-body",
    testIdSuffix: "full-body",
    isDefault: false,
    description:
      "Re-runs the whole body from scratch each generation. Safer when steps share hidden state, but costs more tokens.",
  },
];

/**
 * The re-attempt strategy cards (§4.7.3): two selectable cards writing the `reattempt_strategy`
 * config key, built on the shared `RadioCard` affordance. Configure-time only — the common
 * run flow never sees it (ADR-009).
 */
export function LoopConfigureStrategy({ value, disabled, onChange }: LoopConfigureStrategyProps) {
  return (
    <div
      className="grid grid-cols-1 gap-2.5 sm:grid-cols-2"
      data-testid="loop-configure-strategy"
      role="radiogroup"
      aria-label="Re-attempt strategy"
    >
      {STRATEGY_CARDS.map(card => (
        <RadioCard
          key={card.value}
          data-testid={`loop-configure-strategy-${card.testIdSuffix}`}
          aria-label={card.label}
          selected={value === card.value}
          disabled={disabled}
          onSelect={() => onChange(card.value)}
          title={
            <span className="flex items-center gap-2">
              <span className="font-mono">{card.label}</span>
              {card.isDefault ? (
                <MonoTag className="rounded-xs bg-success-tint px-1.5 py-0.5 text-pill-group-badge text-success">
                  default
                </MonoTag>
              ) : null}
            </span>
          }
          titleClassName="whitespace-normal"
          description={card.description}
        />
      ))}
    </div>
  );
}
