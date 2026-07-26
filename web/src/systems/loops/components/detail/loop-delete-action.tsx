import { Trash2 } from "lucide-react";

import { Button, ConfirmDialog, DialogTrigger } from "@agh/ui";

interface LoopDeleteActionProps {
  loopName: string;
  isPending: boolean;
  error: string | null;
  onConfirm: () => Promise<void>;
  onReset: () => void;
  defaultOpen?: boolean;
  /** Controlled open — use with `hideTrigger` when opened from a menu item. */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  /** Omit the built-in DialogTrigger (caller owns the open control). */
  hideTrigger?: boolean;
}

/** Workspace-shadow deletion with an exact-name confirmation boundary. */
export function LoopDeleteAction({
  loopName,
  isPending,
  error,
  onConfirm,
  onReset,
  defaultOpen = false,
  open,
  onOpenChange,
  hideTrigger = false,
}: LoopDeleteActionProps) {
  return (
    <ConfirmDialog
      defaultOpen={defaultOpen}
      open={open}
      title={`Delete ${loopName}?`}
      description="Delete this workspace-owned definition. If a bundled Loop shares the name, it becomes visible again."
      confirmLabel="Delete loop"
      cancelLabel="Cancel"
      tone="danger"
      confirmTyping={loopName}
      confirmIcon={Trash2}
      isPending={isPending}
      error={error}
      onConfirm={onConfirm}
      onOpenChange={next => {
        onOpenChange?.(next);
        onReset();
      }}
      cancelButtonProps={{ disabled: isPending }}
      contentProps={{ "data-testid": "loop-delete-dialog" }}
    >
      {hideTrigger ? null : (
        <DialogTrigger
          render={
            <Button type="button" variant="destructive" size="sm" data-testid="loop-delete-action">
              <Trash2 aria-hidden="true" className="size-3.5" />
              Delete loop
            </Button>
          }
        />
      )}
    </ConfirmDialog>
  );
}
