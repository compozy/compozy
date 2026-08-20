"use client";

import { ArrowRight, SlidersHorizontal } from "lucide-react";
import * as React from "react";

import { DIALOG_TOUCH_TARGET_SEGMENTS_CLASS } from "../../lib/dialog-shell";
import { cn } from "../../lib/utils";
import { EntityDialogToolbar } from "./entity-dialog-toolbar";
import { PillGroup, type PillGroupItem } from "./pill-group";

/** The only disclosure tier an entity editor may have. */
export type EntityMode = "simple" | "advanced";

export interface EntityModeToolbarProps extends Omit<React.ComponentProps<"div">, "onChange"> {
  mode: EntityMode;
  onModeChange: (mode: EntityMode) => void;
  /**
   * Trailing domain content — compact status, never the workspace scope
   * statement. Scope belongs in `EntityDialogFooter`'s hint slot.
   */
  trailing?: React.ReactNode;
  /** Accessible name for the mode group. */
  ariaLabel?: string;
  /** Prefixes the `-mode-simple` / `-mode-advanced` test ids. */
  testIdPrefix?: string;
}

function modeItems(testIdPrefix: string): PillGroupItem<EntityMode>[] {
  return [
    {
      value: "simple",
      label: (
        <span className="flex items-center gap-1.5">
          <ArrowRight aria-hidden="true" className="size-3" />
          Simple
        </span>
      ),
      testId: `${testIdPrefix}-mode-simple`,
    },
    {
      value: "advanced",
      label: (
        <span className="flex items-center gap-1.5">
          <SlidersHorizontal aria-hidden="true" className="size-3" />
          Advanced
        </span>
      ),
      testId: `${testIdPrefix}-mode-advanced`,
    },
  ];
}

/**
 * Full-width chrome strip for Simple/Advanced.
 *
 * Recessed `--color-canvas-tint` against the dialog's `--color-canvas-soft` so
 * the mode switcher reads as chrome, not as the first form row. Simple shows
 * the common path and Advanced is the only disclosure tier — it never hides a
 * required field. Leaving Advanced is the consumer's cue to snap unsupported
 * advanced-only selections back to a Simple-valid default; this control reports
 * the mode and owns no draft state.
 */
function EntityModeToolbar({
  mode,
  onModeChange,
  trailing,
  ariaLabel = "Editor mode",
  testIdPrefix = "entity",
  className,
  ...props
}: EntityModeToolbarProps) {
  return (
    <EntityDialogToolbar
      className={cn("border-b border-line-soft bg-canvas-tint", className)}
      data-slot="entity-mode-toolbar"
      data-mode={mode}
      leading={
        <PillGroup
          aria-label={ariaLabel}
          className={DIALOG_TOUCH_TARGET_SEGMENTS_CLASS}
          items={modeItems(testIdPrefix)}
          onChange={onModeChange}
          size="sm"
          value={mode}
        />
      }
      trailing={trailing}
      {...props}
    />
  );
}

export { EntityModeToolbar };
