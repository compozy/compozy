"use client";

import type { LucideIcon } from "lucide-react";
import { XIcon } from "lucide-react";
import * as React from "react";

import { DIALOG_TOUCH_TARGET_SQUARE_CLASS } from "../../lib/dialog-shell";
import { Button } from "../button";
import { DialogDescription, DialogHeader, DialogTitle } from "../dialog";
import { Eyebrow } from "./eyebrow";

export interface EntityDialogHeaderProps extends Omit<
  React.ComponentProps<typeof DialogHeader>,
  "title" | "children"
> {
  /** Entity glyph rendered inside the accent icon well. */
  icon: LucideIcon;
  /** Domain path above the title, e.g. `Autonomy · Task`. */
  eyebrow: string;
  title: React.ReactNode;
  description?: React.ReactNode;
  /** Renders a trailing close control. Omit when the host owns dismissal. */
  onClose?: () => void;
  closeLabel?: string;
}

/**
 * Canonical entity-editor modal header: a 36px accent icon well beside an
 * accent-strong eyebrow, the dialog title, and an optional description.
 *
 * The icon well is the only accent-tinted surface in the modal shell.
 * `ConfirmDialog` keeps its neutral well and does not use this composition.
 *
 * Renders `DialogTitle`/`DialogDescription`, so it must be mounted inside a
 * `Dialog`. Surfaces that also render outside a dialog (OS window locations)
 * take this as a slot from their dialog host rather than embedding it.
 */
function EntityDialogHeader({
  icon: Icon,
  eyebrow,
  title,
  description,
  onClose,
  closeLabel = "Close",
  className,
  ...props
}: EntityDialogHeaderProps) {
  return (
    <DialogHeader variant="ruled" className={className} {...props}>
      <div className="flex items-start gap-4" data-slot="entity-dialog-header">
        <div
          data-slot="entity-dialog-header-icon"
          className="flex size-9 shrink-0 items-center justify-center rounded-icon-well bg-accent-tint text-accent-strong ring-1 ring-accent-dim ring-inset"
        >
          <Icon aria-hidden="true" className="size-4" />
        </div>
        <div className="min-w-0 flex-1">
          <Eyebrow className="text-accent-strong">{eyebrow}</Eyebrow>
          <DialogTitle className="mt-1">{title}</DialogTitle>
          {description ? (
            <DialogDescription className="mt-1">{description}</DialogDescription>
          ) : null}
        </div>
        {onClose ? (
          <Button
            aria-label={closeLabel}
            className={DIALOG_TOUCH_TARGET_SQUARE_CLASS}
            data-slot="entity-dialog-header-close"
            onClick={onClose}
            size="icon-sm"
            type="button"
            variant="ghost"
          >
            <XIcon aria-hidden="true" className="size-4" />
          </Button>
        ) : null}
      </div>
    </DialogHeader>
  );
}

export { EntityDialogHeader };
