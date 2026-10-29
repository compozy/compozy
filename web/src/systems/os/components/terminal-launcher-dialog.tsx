import { CommandDialog } from "@compozy/ui";

import { useTerminalLauncher } from "../hooks/use-terminal-launcher";
import { TerminalLauncher } from "./terminal-launcher";

export interface TerminalLauncherDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

function TerminalLauncherDialogBody({ onClose }: { onClose: () => void }) {
  const launcher = useTerminalLauncher({ enabled: true, onOpened: onClose });
  return <TerminalLauncher {...launcher} />;
}

/** The Terminal rail item on All projects: choose where it opens, without a window. */
export function TerminalLauncherDialog({ open, onOpenChange }: TerminalLauncherDialogProps) {
  return (
    <CommandDialog
      className="sm:max-w-lg"
      description="Start a terminal in a project, or go back to a running one"
      onOpenChange={onOpenChange}
      open={open}
      title="Open a terminal"
    >
      {open ? <TerminalLauncherDialogBody onClose={() => onOpenChange(false)} /> : null}
    </CommandDialog>
  );
}
