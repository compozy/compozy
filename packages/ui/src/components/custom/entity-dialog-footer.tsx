"use client";

import { Info } from "lucide-react";
import * as React from "react";

import { DIALOG_TOUCH_TARGET_CLASS } from "../../lib/dialog-shell";
import { cn } from "../../lib/utils";
import { Button } from "../button";
import { DialogFooter } from "../dialog";
import { Spinner } from "../spinner";

type IconComponent = React.ComponentType<{ className?: string }>;

export interface EntityDialogFooterProps extends Omit<
  React.ComponentProps<typeof DialogFooter>,
  "children" | "showCloseButton"
> {
  /** Consequence note — what persists when the primary action is confirmed. */
  hint?: React.ReactNode;
  /**
   * One low-chrome command on the leading edge — a reset, a view toggle. Ghost
   * or outline only: the footer still carries exactly one primary action.
   */
  leading?: React.ReactNode;
  leadingTestId?: string;
  onCancel: () => void;
  cancelLabel?: React.ReactNode;
  cancelDisabled?: boolean;
  /** Verb + object, switched by mode (`Create agent` → `Save changes`). */
  primaryLabel: React.ReactNode;
  onPrimary?: () => void;
  primaryType?: "button" | "submit";
  primaryDisabled?: boolean;
  /** Leading glyph on the primary action; replaced by a spinner while saving. */
  primaryIcon?: IconComponent;
  /** Blocks duplicate submits and swaps the primary glyph for a spinner. */
  isSaving?: boolean;
  cancelTestId?: string;
  primaryTestId?: string;
  hintTestId?: string;
}

/**
 * Canonical entity-editor modal footer: an optional consequence hint on the
 * left, then Cancel and exactly one primary action.
 *
 * Composes `DialogFooter variant="ruled"` (which owns the ruled chrome) and
 * adds the 52px floor from `--height-editor-footer`. `EditorFooter` is the
 * page/sheet editor footer and stays untouched — this is the modal contract.
 */
function EntityDialogFooter({
  hint,
  leading,
  leadingTestId,
  onCancel,
  cancelLabel = "Cancel",
  cancelDisabled = false,
  primaryLabel,
  onPrimary,
  primaryType = "button",
  primaryDisabled = false,
  primaryIcon: PrimaryIcon,
  isSaving = false,
  cancelTestId,
  primaryTestId,
  hintTestId,
  className,
  ...props
}: EntityDialogFooterProps) {
  return (
    <DialogFooter
      variant="ruled"
      className={cn(
        "min-h-editor-footer items-center gap-3",
        // `DialogFooter` stacks column-reverse on narrow viewports, which would
        // print the consequence note *after* the action it describes. The shell
        // keeps source order below 760px: note, then Cancel, then the primary
        // action closest to the thumb.
        "max-[760px]:flex-col max-[760px]:items-stretch",
        className
      )}
      {...props}
    >
      {leading ? (
        <div
          className={cn("flex shrink-0 items-center", DIALOG_TOUCH_TARGET_CLASS)}
          data-slot="entity-dialog-footer-leading"
          data-testid={leadingTestId}
        >
          {leading}
        </div>
      ) : null}
      {hint ? (
        <div
          className="flex min-w-0 flex-1 items-center gap-2 text-form-hint text-muted"
          data-slot="entity-dialog-footer-hint"
          data-testid={hintTestId}
        >
          <Info aria-hidden="true" className="size-3.5 shrink-0 text-faint" />
          <span className="min-w-0">{hint}</span>
        </div>
      ) : null}
      {leading && !hint ? (
        // `DialogFooter variant="ruled"` justifies to the end, so without a
        // growing cell a leading-only footer parks its command beside Cancel.
        // Hidden once the footer stacks, where it would become an empty row.
        <div aria-hidden="true" className="flex-1 max-[760px]:hidden" />
      ) : null}
      <Button
        className={DIALOG_TOUCH_TARGET_CLASS}
        data-slot="entity-dialog-footer-cancel"
        data-testid={cancelTestId}
        disabled={cancelDisabled}
        onClick={onCancel}
        type="button"
        variant="outline"
      >
        {cancelLabel}
      </Button>
      <Button
        className={cn("min-w-32", DIALOG_TOUCH_TARGET_CLASS)}
        data-slot="entity-dialog-footer-primary"
        data-testid={primaryTestId}
        disabled={primaryDisabled || isSaving}
        onClick={onPrimary}
        type={primaryType}
      >
        {isSaving ? (
          <Spinner className="size-3" />
        ) : PrimaryIcon ? (
          <PrimaryIcon aria-hidden="true" className="size-4" />
        ) : null}
        {primaryLabel}
      </Button>
    </DialogFooter>
  );
}

export { EntityDialogFooter };
