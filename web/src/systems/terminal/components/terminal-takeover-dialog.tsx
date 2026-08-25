import { ConfirmDialog, MonoId } from "@compozy/ui";

export interface TerminalTakeoverDialogProps {
  open: boolean;
  /** The person who would be displaced, by name. */
  controllerName: string;
  terminalTitle: string;
  terminalId: string;
  onConfirm: () => void;
  onCancel: () => void;
}

/**
 * Displacing another person confirms by name.
 *
 * Taking over an agent is immediate and never asks. Taking over a person does,
 * and no byte reaches the program until the confirmation lands — the dialog is
 * the gate, not a courtesy after the fact.
 */
export function TerminalTakeoverDialog({
  open,
  controllerName,
  terminalTitle,
  terminalId,
  onConfirm,
  onCancel,
}: TerminalTakeoverDialogProps) {
  return (
    <ConfirmDialog
      cancelLabel="Cancel"
      confirmButtonProps={{ "data-testid": "terminal-takeover-confirm" }}
      confirmLabel="Take control"
      contentProps={{ "data-testid": "terminal-takeover-dialog" }}
      description={`${controllerName} is typing in ${terminalTitle} right now. Taking control disconnects their input immediately.`}
      footNote={<MonoId size="sm" value={terminalId} />}
      onConfirm={onConfirm}
      onOpenChange={next => {
        if (!next) onCancel();
      }}
      open={open}
      title={`Take control from ${controllerName}?`}
      tone="neutral"
    />
  );
}
