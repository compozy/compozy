"use client";

import type { LucideIcon } from "lucide-react";
import { XIcon } from "lucide-react";
import * as React from "react";

import { DIALOG_CLOSE_BUTTON_CLASS } from "../../lib/dialog-shell";
import { DIALOG_ICON_WELL_TONE, DIALOG_TONE_EYEBROW, type DialogTone } from "../../lib/dialog-tone";
import { Button } from "../button";
import { DialogDescription, DialogHeader, DialogTitle } from "../dialog";
import { Eyebrow } from "./eyebrow";
import { KindIcon } from "./kind-icon";

export interface EntityDialogHeaderProps extends Omit<
  React.ComponentProps<typeof DialogHeader>,
  "title" | "children"
> {
  /** Entity glyph rendered inside the identity well. */
  icon: LucideIcon;
  /**
   * Well and eyebrow tone, the same model `ConfirmDialog` uses. `neutral` (the
   * default) is the mint identity well with a muted eyebrow; keep `danger` /
   * `warning` for dialogs whose subject is destructive or a real warning.
   */
  tone?: DialogTone;
  /**
   * Optional domain path above the title, e.g. `Autonomy · Task`. Omit it when
   * the title already names the entity — a repeated eyebrow is noise.
   */
  eyebrow?: string;
  title: React.ReactNode;
  description?: React.ReactNode;
  /** Renders a trailing close control. Omit when the host owns dismissal. */
  onClose?: () => void;
  closeLabel?: string;
}

/**
 * Canonical entity-editor modal header: the identity well (`KindIcon` well
 * tone) beside an optional eyebrow, the dialog title, an optional description,
 * and a quiet close control. Tone follows the shared dialog tone model, so an
 * entity editor reads as identity (mint), not as something needing attention.
 *
 * Renders `DialogTitle`/`DialogDescription`, so it must be mounted inside a
 * `Dialog`. Surfaces that also render outside a dialog (OS window locations)
 * take this as a slot from their dialog host rather than embedding it.
 */
function EntityDialogHeader({
  icon: Icon,
  tone = "neutral",
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
      <div className="flex items-start gap-3" data-slot="entity-dialog-header">
        <KindIcon
          aria-hidden="true"
          className={DIALOG_ICON_WELL_TONE[tone]}
          data-icon-tone={tone}
          data-slot="entity-dialog-header-icon"
          icon={Icon}
          tone="well"
        />
        <div className="min-w-0 flex-1">
          {eyebrow ? <Eyebrow className={DIALOG_TONE_EYEBROW[tone]}>{eyebrow}</Eyebrow> : null}
          <DialogTitle className={eyebrow ? "mt-1" : undefined}>{title}</DialogTitle>
          {description ? (
            <DialogDescription className="mt-1">{description}</DialogDescription>
          ) : null}
        </div>
        {onClose ? (
          <Button
            aria-label={closeLabel}
            className={DIALOG_CLOSE_BUTTON_CLASS}
            data-slot="entity-dialog-header-close"
            onClick={onClose}
            size="icon-sm"
            type="button"
            variant="quiet"
          >
            <XIcon aria-hidden="true" className="size-4" />
          </Button>
        ) : null}
      </div>
    </DialogHeader>
  );
}

export { EntityDialogHeader };
