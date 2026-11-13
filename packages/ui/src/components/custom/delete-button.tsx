"use client";

import { AnimatePresence, m } from "motion/react";
import * as React from "react";

import { useDeleteButton } from "../../hooks/use-delete-button";
import { Check, Trash2, X } from "../../icons";
import { cn } from "../../lib/utils";

export type DeleteButtonPhase = "rest" | "open" | "confirmed";

export interface DeleteButtonProps extends Omit<
  React.ComponentProps<"div">,
  "children" | "onKeyDown"
> {
  /** Names the bin and what it removes, e.g. "Close Review". */
  label: string;
  /** Accessible name of the cancel cross. */
  cancelLabel?: string;
  /** Accessible name of the confirm check; defaults to `Confirm: <label>`. */
  confirmLabel?: string;
  /** Controlled phase; omitted, the button runs itself. */
  phase?: DeleteButtonPhase;
  /**
   * The phase an uncontrolled button mounts in. `"open"` mounts it asking for confirmation, with
   * focus on the cross, as if the bin had just been pressed.
   */
  defaultPhase?: Exclude<DeleteButtonPhase, "confirmed">;
  onPhaseChange?: (phase: DeleteButtonPhase) => void;
  /** Confirm stays unavailable until this is true (for example, until a destination is chosen). */
  canConfirm?: boolean;
  disabled?: boolean;
  onConfirm?: () => void;
  onCancel?: () => void;
  /** How long the confirmed check holds before an uncontrolled button returns to rest (ms). */
  confirmedDuration?: number;
}

function announcement(phase: DeleteButtonPhase, label: string): string {
  if (phase === "open") return `${label}: confirm or cancel`;
  if (phase === "confirmed") return `${label}: confirmed`;
  return "";
}

/**
 * The in-place confirm delete button: the bin opens into a cross that cancels and a check that
 * confirms; Escape cancels. Focus moves to the cross on open and back to the bin on cancel, and a
 * live region announces each phase.
 */
function DeleteButton({
  label,
  cancelLabel = "Cancel",
  confirmLabel,
  phase,
  defaultPhase = "rest",
  onPhaseChange,
  canConfirm = true,
  disabled = false,
  onConfirm,
  onCancel,
  confirmedDuration = 1200,
  className,
  ...props
}: DeleteButtonProps) {
  const { current, transition, binRef, cancelRef, go, cancel } = useDeleteButton({
    phase,
    defaultPhase,
    onPhaseChange,
    onCancel,
    confirmedDuration,
  });
  const open = current === "open";
  const confirmed = current === "confirmed";

  return (
    <div
      role="group"
      aria-label={label}
      className={cn(
        "inline-flex h-7 items-center overflow-hidden rounded-pill text-fg-2",
        open && "bg-destructive-tint shadow-card",
        confirmed && "bg-success-tint text-success",
        className
      )}
      data-phase={current}
      data-slot="delete-button"
      onKeyDown={event => {
        if (event.key !== "Escape" || !open) return;
        // Escape cancels the confirm only; the surface around it stays open.
        event.preventDefault();
        event.stopPropagation();
        cancel();
      }}
      {...props}
    >
      <AnimatePresence initial={false} mode="popLayout">
        {open ? (
          <m.span
            animate={{ opacity: 1, x: 0 }}
            className="flex items-center gap-0.5 px-0.5"
            exit={{ opacity: 0, x: 8 }}
            initial={{ opacity: 0, x: 8 }}
            key="confirm"
            transition={transition}
          >
            <button
              ref={cancelRef}
              aria-label={cancelLabel}
              className="grid size-6 place-items-center rounded-pill text-fg-2 outline-none hover:bg-fg/10 hover:text-fg focus-visible:shadow-focus-ring"
              data-slot="delete-button-cancel"
              onClick={cancel}
              type="button"
            >
              <X aria-hidden="true" className="size-3.5" />
            </button>
            <button
              aria-label={confirmLabel ?? `Confirm: ${label}`}
              className="grid size-6 place-items-center rounded-pill bg-destructive text-destructive-foreground outline-none hover:brightness-110 focus-visible:shadow-focus-ring disabled:opacity-40"
              data-slot="delete-button-confirm"
              disabled={!canConfirm || disabled}
              onClick={() => {
                go("confirmed", null);
                onConfirm?.();
              }}
              type="button"
            >
              <Check aria-hidden="true" className="size-3.5" />
            </button>
          </m.span>
        ) : (
          <m.button
            ref={binRef}
            animate={{ opacity: 1 }}
            aria-label={label}
            className={cn(
              "grid size-7 place-items-center rounded-pill outline-none focus-visible:shadow-focus-ring disabled:cursor-not-allowed",
              !confirmed && "hover:bg-destructive-tint hover:text-destructive"
            )}
            data-slot="delete-button-bin"
            disabled={confirmed || disabled}
            exit={{ opacity: 0 }}
            initial={{ opacity: 0 }}
            key="bin"
            onClick={() => go("open", "cancel")}
            transition={transition}
            type="button"
          >
            {confirmed ? (
              <Check aria-hidden="true" className="size-3.75" />
            ) : (
              <Trash2 aria-hidden="true" className="size-3.75" />
            )}
          </m.button>
        )}
      </AnimatePresence>
      <span aria-live="polite" className="sr-only" role="status">
        {announcement(current, label)}
      </span>
    </div>
  );
}

export { DeleteButton };
