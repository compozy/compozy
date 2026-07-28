"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

export interface EntityDialogToolbarProps extends React.ComponentProps<"div"> {
  /** Leading control — the Simple/Advanced pills, when the surface has a tier. */
  leading?: React.ReactNode;
  /**
   * Trailing domain control — typically the scope selector. A slot so this
   * stays domain-free; consumers own which control belongs here.
   */
  trailing?: React.ReactNode;
  /** Label rendered before `trailing`. */
  trailingLabel?: React.ReactNode;
}

/**
 * The band between a modal header and its body.
 *
 * `.modebar` (`modal-system.css:194`) rules only its bottom edge — a top border
 * would stack against the ruled header's own `border-b` and print a 2px seam.
 *
 * Exists separately from `EntityModeToolbar` because scope and disclosure are
 * independent: the automation editors carry a scope selector with no Simple/
 * Advanced tier, and scope belongs in the same place on every modal that has it.
 */
function EntityDialogToolbar({
  leading,
  trailing,
  trailingLabel,
  className,
  children,
  ...props
}: EntityDialogToolbarProps) {
  return (
    <div
      className={cn(
        "flex flex-wrap items-center gap-3 border-b border-line-soft bg-canvas-tint px-5 py-2.5",
        className
      )}
      data-slot="entity-dialog-toolbar"
      {...props}
    >
      {leading}
      {children}
      {/* The spacer only earns its place when something holds the leading edge.
          With no disclosure tier the trailing control *is* the bar's content,
          so it starts at the gutter instead of floating against the far edge. */}
      {leading || children ? <div className="flex-1" /> : null}
      {trailing ? (
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          {trailingLabel ? (
            <span className="text-form-hint text-subtle">{trailingLabel}</span>
          ) : null}
          {trailing}
        </div>
      ) : null}
    </div>
  );
}

export { EntityDialogToolbar };
