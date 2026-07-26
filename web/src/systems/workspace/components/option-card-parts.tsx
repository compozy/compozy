import * as React from "react";

import { cn } from "@agh/ui";

import { useOptionCardSlot } from "../hooks/use-option-card-slot";

export type OptionCardTone = "neutral" | "accent";

export interface OptionCardIconProps extends React.ComponentProps<"span"> {
  tone?: OptionCardTone;
}

export function OptionCardBody({ className, children, ...props }: React.ComponentProps<"div">) {
  useOptionCardSlot("Body");
  return (
    <div
      data-slot="option-card-body"
      className={cn("flex items-start gap-3", className)}
      {...props}
    >
      {children}
    </div>
  );
}

export function OptionCardIcon({
  className,
  tone = "neutral",
  children,
  ...props
}: OptionCardIconProps) {
  useOptionCardSlot("Icon");
  return (
    <span
      data-slot="option-card-icon"
      data-tone={tone}
      aria-hidden="true"
      className={cn(
        "inline-flex size-10 shrink-0 items-center justify-center rounded bg-surface-glaze",
        tone === "accent" ? "text-accent" : "text-fg",
        className
      )}
      {...props}
    >
      {children}
    </span>
  );
}

export function OptionCardContent({ className, children, ...props }: React.ComponentProps<"div">) {
  useOptionCardSlot("Content");
  return (
    <div data-slot="option-card-content" className={cn("min-w-0 flex-1", className)} {...props}>
      {children}
    </div>
  );
}

export function OptionCardTitle({ className, children, ...props }: React.ComponentProps<"p">) {
  useOptionCardSlot("Title");
  return (
    <p
      data-slot="option-card-title"
      className={cn("text-sm font-medium text-fg", className)}
      {...props}
    >
      {children}
    </p>
  );
}

export function OptionCardDescription({
  className,
  children,
  ...props
}: React.ComponentProps<"p">) {
  useOptionCardSlot("Description");
  return (
    <p
      data-slot="option-card-description"
      className={cn("mt-1 text-sm leading-6 text-muted", className)}
      {...props}
    >
      {children}
    </p>
  );
}

export function OptionCardMeta({ className, children, ...props }: React.ComponentProps<"p">) {
  useOptionCardSlot("Meta");
  return (
    <p
      data-slot="option-card-meta"
      className={cn("mt-3 truncate font-mono text-eyebrow text-subtle", className)}
      {...props}
    >
      {children}
    </p>
  );
}

export function OptionCardAction({ className, children, ...props }: React.ComponentProps<"div">) {
  useOptionCardSlot("Action");
  return (
    <div data-slot="option-card-action" className={className} {...props}>
      {children}
    </div>
  );
}
