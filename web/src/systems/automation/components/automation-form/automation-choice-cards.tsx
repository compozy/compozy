import { Lock, type LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { RadioCard, cn } from "@compozy/ui";

export interface AutomationChoice<T extends string> {
  value: T;
  icon: LucideIcon;
  title: string;
  description: ReactNode;
  /** Why the choice can't be picked; renders the card disabled with a lock line. */
  lockedReason?: string;
}

interface AutomationChoiceCardsProps<T extends string> {
  label: string;
  choices: readonly AutomationChoice<T>[];
  value: T;
  onChange: (value: T) => void;
  testIdPrefix: string;
}

/** A row of three `RadioCard`s (Starts, Does); a locked card says why instead of describing. */
export function AutomationChoiceCards<T extends string>({
  label,
  choices,
  value,
  onChange,
  testIdPrefix,
}: AutomationChoiceCardsProps<T>) {
  return (
    <div aria-label={label} className="grid grid-cols-1 gap-2 sm:grid-cols-3" role="radiogroup">
      {choices.map(choice => {
        const locked = choice.lockedReason !== undefined;
        return (
          <RadioCard
            className={cn("h-full", locked && "cursor-not-allowed opacity-60")}
            data-testid={`${testIdPrefix}-${choice.value}`}
            description={
              locked ? (
                <AutomationLockLine>{choice.lockedReason}</AutomationLockLine>
              ) : (
                choice.description
              )
            }
            disabled={locked}
            icon={choice.icon}
            iconWellSize="lg"
            key={choice.value}
            onSelect={() => onChange(choice.value)}
            selected={choice.value === value}
            title={choice.title}
            titleClassName="whitespace-normal"
          />
        );
      })}
    </div>
  );
}

/** `🔒 Locked` / `🔒 Only scheduled automations can create tasks`. */
export function AutomationLockLine({ children }: { children: ReactNode }) {
  return (
    <span className="inline-flex items-center gap-1.5 text-faint">
      <Lock aria-hidden="true" className="size-3 shrink-0" />
      {children}
    </span>
  );
}
