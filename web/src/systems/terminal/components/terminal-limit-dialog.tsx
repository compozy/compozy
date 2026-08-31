import { useState } from "react";

import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  dialogShellClass,
  DialogTitle,
  Item,
  ItemActions,
  ItemContent,
  ItemTitle,
  MonoId,
} from "@compozy/ui";

import type { TerminalInfo } from "../types";

export interface TerminalLimitDialogProps {
  open: boolean;
  /** The running terminals that occupy this project's slots under this profile. */
  terminals: readonly TerminalInfo[];
  /** The cap, from `[terminal].max_per_workspace`. */
  limit: number;
  onCloseTerminal: (terminalId: string) => void;
  onOpenSettings?: () => void;
  onOpenChange: (open: boolean) => void;
}

/**
 * The project is at its terminal cap.
 *
 * The daemon refuses the create, so the honest answer is the way out rather
 * than a greyed-out plus: the running terminals you could close are the body
 * of the dialog. The cause names the config key that decides the cap, so
 * raising it is a findable action instead of a mystery.
 */
export function TerminalLimitDialog({
  open,
  terminals,
  limit,
  onCloseTerminal,
  onOpenSettings,
  onOpenChange,
}: TerminalLimitDialogProps) {
  const [picked, setPicked] = useState<string | null>(null);
  // A choice only counts while the terminal it named is still there. Closing
  // one and hitting the cap again would otherwise reopen this dialog pointing
  // at something that no longer exists, with its only real action disabled.
  const stillListed = terminals.some(terminal => terminal.id === picked);
  const selected = (stillListed ? terminals.find(t => t.id === picked) : terminals[0]) ?? null;
  const selectedId = selected?.id ?? null;
  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent
        className={`grid-rows-[auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("sm")}`}
        data-testid="terminal-limit-dialog"
        showCloseButton={false}
        unframed
      >
        <DialogHeader className="gap-1 px-5 pt-5 pb-3">
          <DialogTitle>This project is at its terminal limit</DialogTitle>
          <DialogDescription>
            {terminals.length} of {limit} terminals are open. Close one to open another, or raise
            the limit in Settings.
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col border-line border-y" data-testid="terminal-limit-list">
          {terminals.map(terminal => {
            const isSelected = terminal.id === selectedId;
            return (
              <Item
                aria-pressed={isSelected}
                as="button"
                className="rounded-none px-5"
                data-testid={`terminal-limit-row-${terminal.id}`}
                key={terminal.id}
                onClick={() => setPicked(terminal.id)}
                selectable
                selected={isSelected}
                size="sm"
              >
                <ItemContent>
                  <ItemTitle className="min-w-0 truncate font-normal text-muted">
                    {terminal.title}
                  </ItemTitle>
                </ItemContent>
                <ItemActions>
                  <MonoId size="sm" value={terminal.id} />
                </ItemActions>
              </Item>
            );
          })}
        </div>
        <DialogFooter className="justify-between gap-3" variant="ruled">
          <span className="font-mono text-micro text-subtle">
            terminal_limit_reached · terminal.max_per_workspace {limit}
          </span>
          <span className="flex items-center gap-1">
            {onOpenSettings ? (
              <Button onClick={onOpenSettings} size="sm" type="button" variant="ghost">
                Open Settings
              </Button>
            ) : null}
            <Button
              data-testid="terminal-limit-close"
              disabled={selected === null}
              onClick={() => {
                if (selected === null) return;
                onCloseTerminal(selected.id);
                onOpenChange(false);
              }}
              size="sm"
              type="button"
              variant="secondary"
            >
              {selected ? `Close "${selected.title}"` : "Close a terminal"}
            </Button>
          </span>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
