"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

/**
 * `rail` (default): quiet label left, value pushed right — the detail rail.
 * `facts`: a fixed 62px label column with the value start-aligned beside it,
 * in caption type — dense fact lists inside hover cards and popovers.
 */
export type PropertyRowVariant = "rail" | "facts";

export interface PropertyRowProps extends React.ComponentProps<"div"> {
  variant?: PropertyRowVariant;
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

const ROW_CLASS: Record<PropertyRowVariant, string> = {
  rail: "flex min-h-property-row items-center justify-between gap-3 py-property-row-y",
  facts: "grid grid-cols-[62px_minmax(0,1fr)] items-center gap-x-2.5 text-micro",
};

const LABEL_CLASS: Record<PropertyRowVariant, string> = {
  rail: "shrink-0 text-eyebrow text-subtle",
  facts: "min-w-0 truncate text-subtle",
};

const VALUE_CLASS: Record<PropertyRowVariant, string> = {
  rail: "max-w-full text-right font-medium",
  facts: "gap-1.25 text-left",
};

const VALUE_VOICE: Record<PropertyRowVariant, { mono: string; plain: string }> = {
  rail: {
    mono: "font-mono text-mono-id font-normal tabular-nums text-muted",
    plain: "text-form-input text-fg",
  },
  facts: { mono: "font-mono text-mono-id tabular-nums text-fg-2", plain: "text-fg-2" },
};

/**
 * Key/value row: one line, no frame. The rail and fact lists speak entirely
 * through these rows — never bespoke dls.
 */
function PropertyRow({
  variant = "rail",
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
      data-variant={variant}
      className={cn(ROW_CLASS[variant], className)}
      {...props}
    >
      <span data-slot="property-row-label" className={LABEL_CLASS[variant]}>
        {label}
      </span>
      {editor ?? (
        <span
          data-slot="property-row-value"
          data-mono={mono ? "true" : undefined}
          title={recoverableValue}
          className={cn(
            "inline-flex min-w-0 items-center gap-1.5 overflow-hidden whitespace-nowrap",
            VALUE_CLASS[variant],
            mono ? VALUE_VOICE[variant].mono : VALUE_VOICE[variant].plain
          )}
        >
          {hasPrimitiveValue ? <span className="min-w-0 truncate">{children}</span> : children}
        </span>
      )}
    </div>
  );
}

export { PropertyRow };
