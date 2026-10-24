import { Check } from "lucide-react";
import { useRef } from "react";

import { RadioCard, SkeletonText } from "@compozy/ui";

const CHOICE_GRID_CLASS = "grid gap-2 p-3 @min-settings-takeover:grid-cols-3";

/** One placeholder card per rung of the three-way choices this group serves. */
const CHOICE_SKELETON_CARDS = ["first", "second", "third"] as const;

export interface SettingsChoiceOption<V extends string> {
  value: V;
  /** Humanized name ("Ask first"). */
  name: string;
  /** One-line consequence. */
  description: string;
}

export interface SettingsChoiceGroupProps<V extends string> {
  options: ReadonlyArray<SettingsChoiceOption<V>>;
  value: V;
  onChange: (next: V) => void;
  ariaLabel: string;
  "data-testid"?: string;
}

/** Choice cards with one tab stop and native radio-group keyboard behavior. */
export function SettingsChoiceGroup<V extends string>({
  options,
  value,
  onChange,
  ariaLabel,
  "data-testid": testId,
}: SettingsChoiceGroupProps<V>) {
  const cardRefs = useRef<Array<HTMLButtonElement | null>>([]);

  const moveSelection = (currentIndex: number, key: string) => {
    const currentCard = cardRefs.current[currentIndex];
    const direction = currentCard ? getComputedStyle(currentCard).direction : "ltr";
    const previousKey = direction === "rtl" ? "ArrowRight" : "ArrowLeft";
    const nextKey = direction === "rtl" ? "ArrowLeft" : "ArrowRight";
    const lastIndex = options.length - 1;
    let nextIndex = currentIndex;

    if (key === "Home") nextIndex = 0;
    if (key === "End") nextIndex = lastIndex;
    if (key === previousKey || key === "ArrowUp") {
      nextIndex = currentIndex === 0 ? lastIndex : currentIndex - 1;
    }
    if (key === nextKey || key === "ArrowDown") {
      nextIndex = currentIndex === lastIndex ? 0 : currentIndex + 1;
    }
    if (nextIndex === currentIndex) return;

    const next = options[nextIndex];
    if (!next) return;
    onChange(next.value);
    cardRefs.current[nextIndex]?.focus();
  };

  return (
    <div
      aria-label={ariaLabel}
      className={CHOICE_GRID_CLASS}
      data-testid={testId}
      role="radiogroup"
    >
      {options.map((option, index) => {
        const checked = option.value === value;
        return (
          <RadioCard
            badge={
              <Check
                aria-hidden="true"
                className={checked ? "size-3.5 text-fg" : "size-3.5 opacity-0"}
              />
            }
            // The card's own selected treatment (selected fill + inset ring)
            // carries the choice.
            className="relative"
            data-testid={testId ? `${testId}-${option.value}` : undefined}
            description={<span className="block leading-snug">{option.description}</span>}
            key={option.value}
            onKeyDown={event => {
              if (
                ["ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown", "Home", "End"].includes(
                  event.key
                )
              ) {
                event.preventDefault();
                moveSelection(index, event.key);
              }
            }}
            onSelect={() => onChange(option.value)}
            ref={element => {
              cardRefs.current[index] = element;
            }}
            selected={checked}
            tabIndex={checked ? 0 : -1}
            title={option.name}
          />
        );
      })}
    </div>
  );
}

/**
 * The choice cards in skeleton form: the real group grid and `RadioCard` shell with placeholder
 * title and description. The cards are disabled and out of the tab order, and the group is hidden
 * from assistive technology; the page's `SkeletonRegion` announces the loading.
 */
export function SettingsChoiceGroupSkeleton() {
  return (
    <div aria-hidden="true" className={CHOICE_GRID_CLASS}>
      {CHOICE_SKELETON_CARDS.map(card => (
        <RadioCard
          className="pointer-events-none"
          description={
            <span className="block leading-snug">
              <SkeletonText lines={2} />
            </span>
          }
          disabled
          key={card}
          onSelect={() => undefined}
          selected={false}
          tabIndex={-1}
          title={<SkeletonText className="w-24" />}
        />
      ))}
    </div>
  );
}
