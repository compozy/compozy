"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

export interface EntityDialogToolbarProps extends React.ComponentProps<"div"> {
  /** Leading control — the Simple/Advanced pills, when the surface has a tier. */
  leading?: React.ReactNode;
  /**
   * Trailing domain content — compact status. Workspace scope belongs in the
   * footer hint, not this row.
   */
  trailing?: React.ReactNode;
}

/**
 * Layout row between a modal header and its body.
 *
 * Unpainted on its own so dialogs without Simple/Advanced do not grow an empty
 * chrome bar. `EntityModeToolbar` adds the recessed `--color-canvas-tint` strip.
 * Workspace scope belongs in `EntityDialogFooter`'s hint slot; `trailing` is
 * compact status only.
 */
function EntityDialogToolbar({
  leading,
  trailing,
  className,
  children,
  ...props
}: EntityDialogToolbarProps) {
  return (
    <div
      className={cn("flex flex-wrap items-center gap-3 px-5 py-2.5", className)}
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
        <div className="flex min-w-0 flex-wrap items-center gap-2">{trailing}</div>
      ) : null}
    </div>
  );
}

export { EntityDialogToolbar };
