"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

export interface PropertyRowProps extends React.ComponentProps<"div"> {
  /** Row label, left column (12px subtle). */
  label: React.ReactNode;
  /** Renders the value in the mono id voice (ids, models, seeds). */
  mono?: boolean;
  /** Value content; ignored when `editor` is provided. */
  children?: React.ReactNode;
  /** Full value exposed when a composed child cannot provide an intrinsic title. */
  valueTitle?: string;
  /**
   * Inline editor trigger replacing the static value (e.g. a DropdownMenu
   * trigger). The slot is rendered flush-right and owns its own semantics.
   */
  editor?: React.ReactNode;
}

/**
 * Detail-rail key/value row: quiet label left, value right, one line, no
 * frame. The rail speaks entirely through these rows — never bespoke dls.
 */
function PropertyRow({
  label,
  mono = false,
  editor,
  valueTitle,
  className,
  children,
  ...props
}: PropertyRowProps) {
  const hasPrimitiveValue = typeof children === "string" || typeof children === "number";
  const recoverableValue = valueTitle ?? (hasPrimitiveValue ? String(children) : undefined);

  return (
    <div
      data-slot="property-row"
      className={cn(
        "flex min-h-property-row items-center justify-between gap-3 py-property-row-y",
        className
      )}
      {...props}
    >
      <span data-slot="property-row-label" className="shrink-0 text-form-label text-subtle">
        {label}
      </span>
      {editor ?? (
        <span
          data-slot="property-row-value"
          data-mono={mono ? "true" : undefined}
          title={recoverableValue}
          className={cn(
            "inline-flex max-w-full min-w-0 items-center gap-1.5 overflow-hidden text-right whitespace-nowrap font-medium",
            mono
              ? "font-mono text-eyebrow font-normal tabular-nums text-muted"
              : "text-small-body text-fg"
          )}
        >
          {hasPrimitiveValue ? <span className="min-w-0 truncate">{children}</span> : children}
        </span>
      )}
    </div>
  );
}

export { PropertyRow };
