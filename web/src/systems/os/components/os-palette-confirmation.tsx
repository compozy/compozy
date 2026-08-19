import { TriangleAlert } from "lucide-react";
import { useEffect, useRef } from "react";

import { Button } from "@compozy/ui";

import type { CmdPaletteConfirmation } from "../lib/cmd-palette-types";

export interface PaletteConfirmationProps {
  confirmation: CmdPaletteConfirmation;
  /**
   * Non-empty once the target stopped being what it was when the operator
   * triggered the command — renders instead of the confirm control (US-016.EC-2).
   */
  invalidatedReason: string;
  destructive: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}

/**
 * The declared confirmation step (`_uiux.md` S9, US-016).
 *
 * Only the descriptor's own strings render here — the palette never writes
 * consequence copy on a command's behalf, because a confirmation that says
 * something the command does not do is worse than none (BR-13).
 *
 * Cancel takes focus, so the keystroke that opened this dialog cannot also
 * answer it. That is the repeat guard: ⏎ held from the trigger lands on Cancel,
 * and confirming needs a deliberate move to the other control (US-016.EC-1).
 */
export function PaletteConfirmation({
  confirmation,
  invalidatedReason,
  destructive,
  onCancel,
  onConfirm,
}: PaletteConfirmationProps) {
  const cancelRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    cancelRef.current?.focus();
  }, []);

  const invalidated = invalidatedReason.trim() !== "";
  return (
    <div
      aria-describedby="os-palette-confirm-body"
      aria-labelledby="os-palette-confirm-title"
      className="flex flex-col"
      data-testid="os-palette-confirmation"
      role="dialog"
    >
      <div className="flex flex-col gap-2 px-5 pt-5 pb-4">
        <h3 className="text-item-title tracking-tight text-fg-strong" id="os-palette-confirm-title">
          {confirmation.title}
        </h3>
        {invalidated ? (
          <div
            className="flex items-start gap-2 text-warning"
            data-testid="os-palette-confirm-invalid"
            role="alert"
          >
            <TriangleAlert aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
            <p className="text-small-body text-fg" id="os-palette-confirm-body">
              {invalidatedReason}
            </p>
          </div>
        ) : confirmation.body ? (
          <p className="text-small-body text-muted" id="os-palette-confirm-body">
            {confirmation.body}
          </p>
        ) : (
          <p className="sr-only" id="os-palette-confirm-body">
            {confirmation.title}
          </p>
        )}
      </div>
      <div className="flex justify-end gap-2 border-t border-line bg-canvas-tint px-5 py-3">
        <Button
          data-testid="os-palette-confirm-cancel"
          ref={cancelRef}
          size="sm"
          variant="neutral"
          onClick={onCancel}
        >
          Cancel
        </Button>
        {invalidated ? null : (
          <Button
            data-testid="os-palette-confirm-accept"
            size="sm"
            variant={destructive ? "destructive" : "default"}
            onClick={onConfirm}
          >
            {confirmation.confirm}
          </Button>
        )}
      </div>
    </div>
  );
}
