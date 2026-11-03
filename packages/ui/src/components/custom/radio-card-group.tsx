"use client";

import * as React from "react";

import { cn } from "../../lib/utils";
import { RadioCard, type RadioCardProps } from "./radio-card";

export interface RadioCardGroupOption<V extends string> {
  value: V;
  title: React.ReactNode;
  description?: React.ReactNode;
  icon?: RadioCardProps["icon"];
  badge?: React.ReactNode;
  disabled?: boolean;
  className?: string;
  titleClassName?: string;
  "data-testid"?: string;
}

export interface RadioCardGroupProps<V extends string> extends Omit<
  React.ComponentProps<"div">,
  "onChange" | "children" | "defaultValue"
> {
  options: ReadonlyArray<RadioCardGroupOption<V>>;
  /** `null` when nothing is chosen yet; the first enabled card then takes the tab stop. */
  value: V | null;
  onValueChange: (next: V) => void;
  /** Disables every card. */
  disabled?: boolean;
  iconWellSize?: RadioCardProps["iconWellSize"];
}

const MOVE_KEYS = new Set(["ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown", "Home", "End"]);

function nextEnabledIndex(
  enabled: readonly boolean[],
  from: number,
  key: string,
  rtl: boolean
): number {
  const last = enabled.length - 1;
  const back = key === "ArrowUp" || key === (rtl ? "ArrowRight" : "ArrowLeft");
  const forward = key === "ArrowDown" || key === (rtl ? "ArrowLeft" : "ArrowRight");
  if (key === "Home") return enabled.indexOf(true);
  if (key === "End") return enabled.lastIndexOf(true);
  if (!back && !forward) return from;
  const step = back ? -1 : 1;
  for (let offset = 1; offset <= enabled.length; offset += 1) {
    const index = (((from + step * offset) % (last + 1)) + last + 1) % (last + 1);
    if (enabled[index]) return index;
  }
  return from;
}

/**
 * A set of `RadioCard`s with native radio-group keyboard behavior: one tab stop on the chosen card,
 * Arrow keys move and select (wrapping, RTL-aware), Home and End jump to the ends.
 */
function RadioCardGroup<V extends string>({
  options,
  value,
  onValueChange,
  disabled = false,
  iconWellSize,
  className,
  ...props
}: RadioCardGroupProps<V>) {
  const cardRefs = React.useRef<Array<HTMLButtonElement | null>>([]);
  const enabled = options.map(option => !disabled && !option.disabled);
  const selectedIndex = options.findIndex(option => option.value === value);
  const tabStop =
    selectedIndex >= 0 && enabled[selectedIndex] ? selectedIndex : enabled.indexOf(true);

  const move = (from: number, key: string) => {
    const card = cardRefs.current[from];
    const rtl = card ? getComputedStyle(card).direction === "rtl" : false;
    const next = nextEnabledIndex(enabled, from, key, rtl);
    const option = options[next];
    if (next === from || next < 0 || !option) return;
    onValueChange(option.value);
    cardRefs.current[next]?.focus();
  };

  return (
    <div
      className={cn("grid gap-2", className)}
      data-slot="radio-card-group"
      role="radiogroup"
      {...props}
    >
      {options.map((option, index) => (
        <RadioCard
          badge={option.badge}
          className={option.className}
          data-testid={option["data-testid"]}
          description={option.description}
          disabled={!enabled[index]}
          icon={option.icon}
          iconWellSize={iconWellSize}
          key={option.value}
          onKeyDown={event => {
            if (!MOVE_KEYS.has(event.key)) return;
            event.preventDefault();
            move(index, event.key);
          }}
          onSelect={() => onValueChange(option.value)}
          ref={element => {
            cardRefs.current[index] = element;
          }}
          selected={option.value === value}
          tabIndex={index === tabStop ? 0 : -1}
          title={option.title}
          titleClassName={option.titleClassName}
        />
      ))}
    </div>
  );
}

export { RadioCardGroup };
