"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

/**
 * Clarification answer vocabulary for the decision dock: bounded choices render
 * as 30px rows with numeric shortcut chips; a question with no choices renders
 * the quiet free-text form instead. Presentational shells only — submission
 * semantics (keys, Enter behavior) belong to the domain host.
 */
function ChoiceList({ children, className, ...props }: React.ComponentProps<"div">) {
  return (
    <div data-slot="choice-list" className={cn("mt-2 flex flex-col gap-0.5", className)} {...props}>
      {children}
    </div>
  );
}

function ChoiceRow({ children, className, type, ...props }: React.ComponentProps<"button">) {
  return (
    <button
      type={type ?? "button"}
      data-slot="choice-row"
      className={cn(
        "flex w-full min-w-0 items-center gap-[9px] rounded-md border border-transparent",
        "min-h-[30px] px-2 py-1 text-left text-small-body text-fg",
        "transition-colors duration-base ease-out motion-reduce:transition-none",
        "hover:border-line hover:bg-hover",
        "aria-pressed:border-line-strong aria-pressed:bg-row-selected",
        "focus-visible:shadow-focus focus-visible:outline-none",
        className
      )}
      {...props}
    >
      {children}
    </button>
  );
}

/** 18px numeric shortcut chip leading a choice row. */
function ChoiceKey({ children, className, ...props }: React.ComponentProps<"kbd">) {
  return (
    <kbd
      data-slot="choice-key"
      className={cn(
        "grid size-[18px] shrink-0 place-items-center rounded-xs border border-line",
        "font-mono text-[10px] text-subtle tabular-nums",
        className
      )}
      {...props}
    >
      {children}
    </kbd>
  );
}

/** Trailing quiet hint on a choice row ("recommended"). */
function ChoiceHint({ children, className, ...props }: React.ComponentProps<"span">) {
  return (
    <span
      data-slot="choice-hint"
      className={cn("ml-auto shrink-0 text-[11px] text-subtle", className)}
      {...props}
    >
      {children}
    </span>
  );
}

/** Free-text answer form shell — rendered when the question ships no choices. */
function ChoiceFree({ children, className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="choice-free"
      className={cn("mt-2 flex flex-col gap-[7px]", className)}
      {...props}
    >
      {children}
    </div>
  );
}

function ChoiceFreeTextarea({ className, ...props }: React.ComponentProps<"textarea">) {
  return (
    <textarea
      data-slot="choice-free-textarea"
      className={cn(
        "min-h-[52px] w-full resize-none rounded-md border border-line bg-elevated",
        "px-2 py-[7px] text-small-body leading-normal text-fg",
        "placeholder:text-subtle focus:border-accent-dim focus:outline-none",
        className
      )}
      {...props}
    />
  );
}

function ChoiceFreeBar({ children, className, ...props }: React.ComponentProps<"div">) {
  return (
    <div data-slot="choice-free-bar" className={cn("flex items-center", className)} {...props}>
      {children}
    </div>
  );
}

export {
  ChoiceFree,
  ChoiceFreeBar,
  ChoiceFreeTextarea,
  ChoiceHint,
  ChoiceKey,
  ChoiceList,
  ChoiceRow,
};
