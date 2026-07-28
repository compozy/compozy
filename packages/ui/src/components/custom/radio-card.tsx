"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

type IconComponent = React.ComponentType<{ className?: string; size?: number }>;

export interface RadioCardProps extends Omit<React.ComponentProps<"button">, "value" | "title"> {
  selected: boolean;
  onSelect: () => void;
  title: React.ReactNode;
  description?: React.ReactNode;
  icon?: IconComponent;
  badge?: React.ReactNode;
  /** Optional className merged onto the title slot. */
  titleClassName?: string;
}

/**
 * Single radio choice rendered as a card.:
 * - resting state: `--canvas-soft` surface, no border (flat-depth).
 * - selected state: `--surface-glaze` background + `box-shadow: 0 0 0 1px var(--color-line-strong) inset`.
 *   No accent border, no `--accent-tint` fill — accent stays reserved for true CTAs.
 */
function RadioCard({
  selected,
  onSelect,
  title,
  description,
  icon: Icon,
  badge,
  titleClassName,
  className,
  type = "button",
  onClick,
  onKeyDown,
  ...props
}: RadioCardProps) {
  const selectRadioCard = (event: React.MouseEvent<HTMLButtonElement>) => {
    onClick?.(event);
    if (!event.defaultPrevented) onSelect();
  };
  const handleKeyDown = (event: React.KeyboardEvent<HTMLButtonElement>) => {
    onKeyDown?.(event);
    if (event.defaultPrevented) return;
    if (event.key === " " || event.key === "Enter") {
      event.preventDefault();
      onSelect();
    }
  };
  return (
    <button
      type={type}
      role="radio"
      aria-checked={selected}
      data-slot="radio-card"
      data-selected={selected ? "true" : undefined}
      onClick={selectRadioCard}
      onKeyDown={handleKeyDown}
      className={cn(
        "group flex w-full min-w-0 flex-col gap-1.5 rounded bg-canvas-soft px-3 py-2.5 text-left transition-colors duration-base ease-out focus-visible:outline-none focus-visible:shadow-focus-ring",
        selected ? "bg-surface-glaze shadow-inset-strong" : "hover:bg-elevated",
        className
      )}
      {...props}
    >
      <div className="flex min-w-0 items-center gap-2">
        {Icon ? (
          <span
            aria-hidden="true"
            className={cn(
              "inline-flex size-5 shrink-0 items-center justify-center",
              selected ? "text-fg-strong" : "text-muted"
            )}
          >
            <Icon className="size-3" />
          </span>
        ) : null}
        <span
          data-slot="radio-card-title"
          className={cn(
            "min-w-0 flex-1 text-small-body font-medium tracking-eyebrow",
            selected ? "text-fg-strong" : "text-fg",
            titleClassName ?? "truncate"
          )}
        >
          {title}
        </span>
        {badge ? (
          <span data-slot="radio-card-badge" className="ml-auto inline-flex shrink-0 items-center">
            {badge}
          </span>
        ) : null}
      </div>
      {description ? (
        <p data-slot="radio-card-description" className="text-form-label text-muted">
          {description}
        </p>
      ) : null}
    </button>
  );
}

export { RadioCard };
