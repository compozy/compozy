/**
 * The in-place confirm delete button (S8), prototyped here before it lands in `@compozy/ui` as
 * `DeleteButton`: the bin opens into a panel with a cross to cancel and a check to confirm, and
 * Escape cancels. Interaction reference: Rare UI "Delete button"; implemented here, not installed.
 */
import { Check, Trash2, X } from "@compozy/ui/icons";
import { AnimatePresence, m } from "motion/react";
import { useState } from "react";

import { cn } from "@compozy/ui";

export type DeleteButtonPhase = "rest" | "open" | "confirmed";

export interface DeleteButtonProps {
  label: string;
  /** Forced phase for static states; omitted, the button runs itself. */
  phase?: DeleteButtonPhase;
  /** Confirm is unavailable until this is true (a destination must be chosen first). */
  canConfirm?: boolean;
  onOpenChange?: (open: boolean) => void;
  onConfirm?: () => void;
  onCancel?: () => void;
  className?: string;
}

const SPRING = { type: "spring", stiffness: 520, damping: 34 } as const;

export function DeleteButton({
  label,
  phase,
  canConfirm = true,
  onOpenChange,
  onConfirm,
  onCancel,
  className,
}: DeleteButtonProps) {
  const [own, setOwn] = useState<DeleteButtonPhase>("rest");
  const current = phase ?? own;
  const go = (next: DeleteButtonPhase) => {
    setOwn(next);
    onOpenChange?.(next === "open");
  };
  const open = current === "open";
  const confirmed = current === "confirmed";

  return (
    <div
      className={cn(
        "inline-flex h-7 items-center overflow-hidden rounded-pill text-fg-2",
        open && "bg-destructive-tint shadow-card",
        confirmed && "bg-success-tint text-success",
        className
      )}
      data-phase={current}
      data-slot="delete-button"
      onKeyDown={event => {
        if (event.key === "Escape" && open) {
          go("rest");
          onCancel?.();
        }
      }}
    >
      <AnimatePresence initial={false} mode="popLayout">
        {open ? (
          <m.span
            animate={{ opacity: 1, x: 0 }}
            className="flex items-center gap-0.5 px-0.5"
            exit={{ opacity: 0, x: 8 }}
            initial={{ opacity: 0, x: 8 }}
            key="confirm"
            transition={SPRING}
          >
            <button
              aria-label="Cancel"
              className="grid size-6 place-items-center rounded-pill text-fg-2 outline-none hover:bg-fg/10 hover:text-fg focus-visible:shadow-focus-ring"
              onClick={() => {
                go("rest");
                onCancel?.();
              }}
              type="button"
            >
              <X aria-hidden="true" className="size-3.5" />
            </button>
            <button
              aria-label={`Confirm: ${label}`}
              className="grid size-6 place-items-center rounded-pill bg-destructive text-destructive-foreground outline-none hover:brightness-110 focus-visible:shadow-focus-ring disabled:opacity-40"
              disabled={!canConfirm}
              onClick={() => {
                go("confirmed");
                onConfirm?.();
              }}
              type="button"
            >
              <Check aria-hidden="true" className="size-3.5" />
            </button>
          </m.span>
        ) : (
          <m.button
            animate={{ opacity: 1 }}
            aria-label={label}
            className={cn(
              "grid size-7 place-items-center rounded-pill outline-none focus-visible:shadow-focus-ring",
              !confirmed && "hover:bg-destructive-tint hover:text-destructive"
            )}
            disabled={confirmed}
            exit={{ opacity: 0 }}
            initial={{ opacity: 0 }}
            key="bin"
            onClick={() => go("open")}
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
    </div>
  );
}
