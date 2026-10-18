import { Pill, RadioCard } from "@compozy/ui";

import type { LoopReattemptStrategy } from "../../lib/loop-config-draft";

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

const STRATEGY_CARDS: StrategyCard[] = [
  {
    value: "failed_only",
    label: "Retry what failed",
    testIdSuffix: "failed-only",
    isDefault: true,
    description: "Re-runs only the steps that failed review, and anything after them.",
  },
  {
    value: "full_body",
    label: "Start over each round",
    testIdSuffix: "full-body",
    isDefault: false,
    description: "Re-runs every step each round. Safer, but uses more tokens.",
  },
  {
    value: "halt",
    label: "Stop and wait for me",
    testIdSuffix: "halt",
    isDefault: false,
    description: "Stops after a failed round until you start it again.",
  },
];

export function LoopConfigureStrategy({ value, disabled, onChange }: LoopConfigureStrategyProps) {
  return (
    <div
      className="grid grid-cols-1 gap-2.5 sm:grid-cols-3"
      data-testid="loop-configure-strategy"
      role="radiogroup"
      aria-label="If a round fails"
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
              <span>{card.label}</span>
              {card.isDefault ? (
                <Pill size="xs" tone="neutral">
                  Default
                </Pill>
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
