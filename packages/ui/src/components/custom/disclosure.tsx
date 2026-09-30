"use client";

import { ChevronRight } from "lucide-react";
import * as React from "react";

import { cn } from "../../lib/utils";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "../collapsible";

type DataAttributes = { [key: `data-${string}`]: string | undefined };

export interface DisclosureProps extends Omit<
  React.ComponentProps<typeof Collapsible>,
  "children" | "className"
> {
  /** Toggle text, e.g. "More options", "Technical details", "Advanced". */
  label: React.ReactNode;
  children: React.ReactNode;
  /**
   * `inline` — a quiet text toggle for secondary facts inside a card or dialog.
   * `framed` — a bordered panel whose header row is the toggle (settings
   * "Advanced" layer).
   */
  variant?: "inline" | "framed";
  /** `sm` for form-label density (dialogs, rails); `md` for body-size toggles. */
  size?: "sm" | "md";
  /** Keep the panel in the DOM while closed (preserves form state and anchors). */
  keepMounted?: boolean;
  className?: string;
  triggerProps?: Omit<React.ComponentProps<typeof CollapsibleTrigger>, "children"> & DataAttributes;
  contentProps?: Omit<React.ComponentProps<typeof CollapsibleContent>, "children" | "keepMounted"> &
    DataAttributes;
}

/**
 * Closed-by-default progressive disclosure: a chevron toggle that rotates open
 * and a panel for the secondary layer (calm defaults). One composition for
 * every "More options" / "Technical details" / "Advanced" fold.
 */
function Disclosure({
  label,
  children,
  variant = "inline",
  size = "sm",
  keepMounted = false,
  className,
  triggerProps,
  contentProps,
  ...rootProps
}: DisclosureProps) {
  const framed = variant === "framed";
  const { className: triggerClassName, ...restTrigger } = triggerProps ?? {};
  const { className: contentClassName, ...restContent } = contentProps ?? {};
  return (
    <Collapsible
      data-slot="disclosure"
      data-variant={variant}
      className={cn(
        "flex min-w-0 flex-col",
        framed && "overflow-hidden rounded-lg bg-sunken",
        className
      )}
      {...rootProps}
    >
      <CollapsibleTrigger
        data-slot="disclosure-trigger"
        type="button"
        className={cn(
          "group/disclosure flex items-center text-left font-medium text-muted",
          "transition-colors duration-base hover:text-fg",
          "focus-visible:shadow-focus-ring focus-visible:outline-none",
          size === "sm" ? "gap-1 text-form-label" : "gap-2 text-small-body",
          framed ? "w-full px-4 py-3 hover:bg-row-hover" : "w-fit rounded-sm py-1",
          triggerClassName
        )}
        {...restTrigger}
      >
        <ChevronRight
          aria-hidden="true"
          className={cn(
            "shrink-0 text-faint transition-transform duration-base motion-reduce:transition-none",
            "group-data-panel-open/disclosure:rotate-90",
            size === "sm" ? "size-3" : "size-3.5"
          )}
        />
        {label}
      </CollapsibleTrigger>
      <CollapsibleContent
        data-slot="disclosure-content"
        keepMounted={keepMounted}
        className={cn(framed ? "border-t border-line-soft" : "pt-2", contentClassName)}
        {...restContent}
      >
        {children}
      </CollapsibleContent>
    </Collapsible>
  );
}

export { Disclosure };
