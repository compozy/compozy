import { useRef, useState } from "react";
import { Trash2 } from "lucide-react";

import { Button, ConfirmDialog, DialogTrigger } from "@agh/ui";

interface AutomationDeleteActionProps {
  isPending: boolean;
  kind: "jobs" | "triggers";
  name: string;
  onConfirm: () => void | Promise<void>;
  /** Controlled open — use with `hideTrigger` when opened from a menu item. */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  /** Omit the built-in DialogTrigger (caller owns the open control). */
  hideTrigger?: boolean;
}

export function AutomationDeleteAction({
  isPending,
  kind,
  name,
  onConfirm,
  open: openProp,
  onOpenChange,
  hideTrigger = false,
}: AutomationDeleteActionProps) {
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const submissionRef = useRef(false);
  const noun = kind === "jobs" ? "job" : "trigger";
  const pending = isPending || isSubmitting;
  const controlled = openProp !== undefined;
  const open = controlled ? openProp : uncontrolledOpen;

  const setOpen = (next: boolean) => {
    if (pending && next === false) return;
    if (!controlled) setUncontrolledOpen(next);
    onOpenChange?.(next);
    if (!next) setError(null);
  };

  const handleConfirm = async () => {
    if (submissionRef.current) return;
    submissionRef.current = true;
    setIsSubmitting(true);
    setError(null);
    try {
      await onConfirm();
      setOpen(false);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : `Failed to delete automation ${noun}`);
    }
    submissionRef.current = false;
    setIsSubmitting(false);
  };

  return (
    <ConfirmDialog
      cancelButtonProps={{
        "data-testid": "cancel-delete-automation-btn",
        disabled: pending,
      }}
      cancelLabel="Cancel"
      confirmButtonProps={{ "data-testid": "confirm-delete-automation-btn" }}
      confirmIcon={Trash2}
      confirmInputProps={{ "data-testid": "automation-delete-confirm-typing" }}
      confirmLabel={pending ? `Deleting ${noun}…` : `Delete ${noun}`}
      confirmTyping={name}
      contentProps={{ "data-testid": "automation-delete-dialog" }}
      description={
        <>
          This permanently deletes <span className="font-mono text-fg">{name}</span>.{" "}
          {kind === "jobs"
            ? "Its schedule will no longer start the configured target."
            : "Matching events will no longer start the configured target."}
        </>
      }
      error={error}
      errorProps={{ "data-testid": "automation-delete-error" }}
      isPending={pending}
      onConfirm={handleConfirm}
      onOpenChange={next => {
        setOpen(next);
      }}
      open={open}
      title={`Delete ${noun}?`}
      tone="danger"
    >
      {hideTrigger ? null : (
        <DialogTrigger
          render={
            <Button
              data-testid="delete-automation-btn"
              disabled={pending}
              size="sm"
              type="button"
              variant="destructive"
            />
          }
        >
          <Trash2 className="size-3" />
          Delete {noun}
        </DialogTrigger>
      )}
    </ConfirmDialog>
  );
}
