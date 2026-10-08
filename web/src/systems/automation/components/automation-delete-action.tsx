import { Trash2 } from "lucide-react";
import { useSelector, useStore } from "@xstate/store-react";

import { Button, ConfirmDialog, DialogTrigger } from "@compozy/ui";
import { automationDeleteLogic } from "./automation-delete-store";

interface AutomationDeleteActionProps {
  isPending: boolean;
  name: string;
  /** What stops happening, e.g. "Its schedule will stop asking summarizer. Past runs stay in the log." */
  consequence: string;
  onConfirm: () => void | Promise<void>;
  /** Controlled open — use with `hideTrigger` when opened from a menu item. */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  /** Omit the built-in DialogTrigger (caller owns the open control). */
  hideTrigger?: boolean;
}

export function AutomationDeleteAction({
  isPending,
  name,
  consequence,
  onConfirm,
  open: openProp,
  onOpenChange,
  hideTrigger = false,
}: AutomationDeleteActionProps) {
  const store = useStore(automationDeleteLogic);
  const state = useSelector(store, snapshot => snapshot.context);
  const pending = isPending || state.phase === "submitting";
  const controlled = openProp !== undefined;
  const open = controlled ? openProp : state.open;

  const setOpen = (next: boolean) => {
    if (pending && next === false) return;
    if (!controlled) store.trigger.openChanged({ open: next });
    onOpenChange?.(next);
    if (!next && controlled) store.trigger.openChanged({ open: false });
  };

  const handleConfirm = () => {
    store.trigger.submissionRequested({
      execute: onConfirm,
      fallbackError: "Failed to delete the automation.",
      permitted: !isPending,
      onSucceeded: () => onOpenChange?.(false),
    });
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
      confirmLabel={pending ? "Deleting…" : "Delete automation"}
      confirmTyping={name}
      contentProps={{ "data-testid": "automation-delete-dialog" }}
      description={
        <>
          This permanently deletes <span className="font-mono text-fg">{name}</span>. {consequence}
        </>
      }
      error={state.error}
      errorProps={{ "data-testid": "automation-delete-error" }}
      isPending={pending}
      onConfirm={handleConfirm}
      onOpenChange={next => {
        setOpen(next);
      }}
      open={open}
      title="Delete automation?"
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
          <Trash2 />
          Delete automation
        </DialogTrigger>
      )}
    </ConfirmDialog>
  );
}
