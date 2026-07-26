"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

export type EntityDialogBodyVariant = "default" | "split";

type EntityDialogBodyBaseProps = React.ComponentProps<"div">;

export type EntityDialogBodyProps =
  | (EntityDialogBodyBaseProps & {
      variant?: "default";
      side?: never;
      mainClassName?: never;
      sideClassName?: never;
    })
  | (EntityDialogBodyBaseProps & {
      variant: "split";
      /**
       * Side-pane content for the split layout. It becomes a second,
       * independent scroll owner on `--color-canvas`.
       */
      side: React.ReactNode;
      /** Class merged onto the main pane of a split body. */
      mainClassName?: string;
      /** Class merged onto the side pane of a split body. */
      sideClassName?: string;
    });

/**
 * Shared pane padding keeps the last field off the footer rule and reserves
 * scrollbar space so a body does not reflow horizontally when it starts
 * scrolling.
 */
const PANE_CLASS = "min-h-0 min-w-0 overflow-y-auto px-5 pt-5 pb-6 [scrollbar-gutter:stable]";

/**
 * Modal body region and sole scroll owner of the shell.
 *
 * `variant="split"` is the two-pane host for a body that carries a large
 * expanding component (the workspace directory browser): a `1fr` main pane and
 * a fixed 420px side pane on `--color-canvas` behind a left hairline, each
 * scrolling independently. Below 980px it collapses to one column and the
 * hairline moves to the top edge of the stacked side pane.
 */
function EntityDialogBody({
  variant = "default",
  side,
  mainClassName,
  sideClassName,
  className,
  children,
  ...props
}: EntityDialogBodyProps) {
  if (variant === "split") {
    return (
      <div
        data-slot="entity-dialog-body"
        data-variant="split"
        className={cn(
          "grid min-h-0 grid-cols-[minmax(0,1fr)_420px] max-[980px]:grid-cols-[minmax(0,1fr)]",
          className
        )}
        {...props}
      >
        <div className={cn(PANE_CLASS, mainClassName)} data-slot="entity-dialog-body-main">
          {children}
        </div>
        <div
          className={cn(
            PANE_CLASS,
            "border-l border-line bg-canvas max-[980px]:border-t max-[980px]:border-l-0",
            sideClassName
          )}
          data-slot="entity-dialog-body-side"
        >
          {side}
        </div>
      </div>
    );
  }

  return (
    <div
      data-slot="entity-dialog-body"
      data-variant="default"
      className={cn(PANE_CLASS, "flex-1", className)}
      {...props}
    >
      {children}
    </div>
  );
}

export { EntityDialogBody };
