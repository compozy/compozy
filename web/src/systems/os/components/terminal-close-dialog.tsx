import { useAtom } from "@xstate/store-react";
import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@compozy/ui";
import type { TerminalWindowClose } from "../lib/terminal-window-close";

export function TerminalCloseDialog({ controller }: { controller: TerminalWindowClose }) {
  const confirmation = useAtom(controller.confirmation);
  return (
    <Dialog
      open={confirmation !== null}
      onOpenChange={open => {
        if (!open) controller.cancel();
      }}
    >
      <DialogContent showCloseButton={false} data-testid="terminal-close-dialog">
        <DialogHeader>
          <DialogTitle>
            {confirmation?.terminals.length === 1
              ? "Close running terminal?"
              : "Close running terminals?"}
          </DialogTitle>
          <DialogDescription>
            Closing terminates these terminals and their running work, including work visible in
            other views. Saved history is kept.
          </DialogDescription>
        </DialogHeader>
        <ul className="space-y-1">
          {confirmation?.terminals.map(terminal => (
            <li key={terminal.id}>
              {terminal.title} · {terminal.profile_name}
            </li>
          ))}
        </ul>
        <DialogFooter>
          <Button autoFocus variant="outline" onClick={() => controller.cancel()}>
            Cancel
          </Button>
          <Button variant="destructive" onClick={() => confirmation?.answer(true)}>
            Terminate and close
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
