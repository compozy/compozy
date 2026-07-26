"use client";

import { CircleHelp } from "lucide-react";
import * as React from "react";

import { DIALOG_TOUCH_TARGET_SQUARE_CLASS } from "../../lib/dialog-shell";
import { cn } from "../../lib/utils";
import { Tooltip, TooltipContent, TooltipTrigger } from "../tooltip";

export interface HelpTipProps extends Omit<React.ComponentProps<"button">, "children"> {
  /**
   * Accessible name for the icon-only trigger, e.g. `About category path`.
   * Required — an unnamed icon button is invisible to assistive tech.
   */
  label: string;
  /**
   * Explanatory prose only. Anything the runtime will *do* with the current
   * value, any error, and any warning stays visible in the form — a person must
   * never have to hover to learn what will happen when they save.
   */
  children: React.ReactNode;
  side?: React.ComponentProps<typeof TooltipContent>["side"];
}

/**
 * The `(?)` beside a label: explanatory prose on demand instead of a permanent
 * description line under every field.
 *
 * Deliberately a real `<button>` rather than `aria-describedby` on the control:
 * `TooltipContent` mounts conditionally, so a static `aria-describedby` would
 * point at an id that does not exist while the tip is closed; the control's
 * describedby slot is already owned by its error; and a described string is not
 * in the tab order, so removing the visible line would strand keyboard users.
 *
 * Click *opens* in addition to hover and focus — without it the prose is
 * unreachable on touch, and the shell owes a complete mobile path. Dismissal
 * stays with Escape, outside press, and pointer-leave (WCAG 1.4.13); a
 * click-toggle would fight the hover machinery, closing on press and reopening
 * on the pointer that is still inside the trigger.
 *
 * Render it as a *sibling* of the `<label>` (see `FieldHeader`), never a child:
 * nesting a button inside a label folds this trigger's accessible name into the
 * label's own name-from-content computation.
 */
function HelpTip({ label, children, side = "top", className, onClick, ...props }: HelpTipProps) {
  const [open, setOpen] = React.useState(false);

  return (
    <Tooltip open={open} onOpenChange={setOpen}>
      <TooltipTrigger
        render={
          <button
            aria-label={label}
            className={cn(
              // 24px target (WCAG 2.2 SC 2.5.8) around a 14px glyph; the
              // negative block margin keeps it from inflating the label line.
              "inline-flex size-6 -my-1 shrink-0 items-center justify-center rounded-full text-faint",
              "transition-colors duration-base ease-out hover:text-muted",
              "focus-visible:outline-none focus-visible:shadow-focus-ring",
              DIALOG_TOUCH_TARGET_SQUARE_CLASS,
              className
            )}
            data-slot="help-tip"
            onClick={event => {
              setOpen(true);
              onClick?.(event);
            }}
            type="button"
            {...props}
          />
        }
      >
        <CircleHelp aria-hidden="true" className="size-3.5" strokeWidth={1.75} />
      </TooltipTrigger>
      <TooltipContent className="max-w-72 items-start leading-normal" side={side}>
        {children}
      </TooltipContent>
    </Tooltip>
  );
}

export { HelpTip };
