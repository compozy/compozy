import { useState } from "react";

import {
  Button,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  Label,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Switch,
  dialogShellClass,
} from "@compozy/ui";

import type { TerminalActorKind, TerminalJournalFilters } from "../types";

const ANY_ACTOR = "any";

interface EditableJournalFilters {
  actor: TerminalActorKind | null;
  since: string;
  failed: boolean;
  terminalId: string;
}

export interface TerminalJournalFilterDialogProps {
  open: boolean;
  value: TerminalJournalFilters;
  onApply: (filters: TerminalJournalFilters) => void;
  onOpenChange: (open: boolean) => void;
}

function editableFilters(value: TerminalJournalFilters): EditableJournalFilters {
  return {
    actor: value.actor ?? null,
    since: value.since ?? "",
    failed: value.failed ?? false,
    terminalId: value.terminalId ?? "",
  };
}

export function TerminalJournalFilterDialog({
  open,
  value,
  onApply,
  onOpenChange,
}: TerminalJournalFilterDialogProps) {
  const [draft, setDraft] = useState<EditableJournalFilters>(() => editableFilters(value));

  const apply = () => {
    const since = draft.since.trim();
    const terminalId = draft.terminalId.trim();
    onApply({
      ...(draft.actor ? { actor: draft.actor } : {}),
      ...(since ? { since } : {}),
      ...(draft.failed ? { failed: true } : {}),
      ...(terminalId ? { terminalId } : {}),
    });
    onOpenChange(false);
  };

  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent
        className={dialogShellClass("sm")}
        data-testid="terminal-journal-filter-dialog"
        unframed
      >
        <DialogHeader className="px-5 pt-5 pb-3" variant="ruled">
          <DialogTitle>Filter command journal</DialogTitle>
        </DialogHeader>
        <form onSubmit={event => { event.preventDefault(); apply(); }}>
        <div className="grid gap-4 px-5 py-4">
          <div className="grid gap-1.5">
            <Label htmlFor="terminal-journal-filter-actor">Who</Label>
            <Select
              onValueChange={actor =>
                setDraft(current => ({
                  ...current,
                  actor: actor === ANY_ACTOR ? null : (actor as TerminalActorKind),
                }))
              }
              value={draft.actor ?? ANY_ACTOR}
            >
              <SelectTrigger
                data-testid="terminal-journal-filter-actor"
                id="terminal-journal-filter-actor"
              >
                <SelectValue>{draft.actor ?? "Anyone"}</SelectValue>
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ANY_ACTOR}>Anyone</SelectItem>
                <SelectItem value="human">Human</SelectItem>
                <SelectItem value="agent">Agent</SelectItem>
                <SelectItem value="system">System</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="terminal-journal-filter-since">Since</Label>
            <Input
              data-testid="terminal-journal-filter-since"
              id="terminal-journal-filter-since"
              onChange={event => setDraft(current => ({ ...current, since: event.target.value }))}
              placeholder="24h"
              value={draft.since}
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="terminal-journal-filter-terminal">Terminal ID</Label>
            <Input
              data-testid="terminal-journal-filter-terminal"
              id="terminal-journal-filter-terminal"
              onChange={event =>
                setDraft(current => ({ ...current, terminalId: event.target.value }))
              }
              value={draft.terminalId}
            />
          </div>
          <div className="flex items-center justify-between gap-4">
            <Label htmlFor="terminal-journal-filter-failed">Failed commands only</Label>
            <Switch
              checked={draft.failed}
              data-testid="terminal-journal-filter-failed"
              id="terminal-journal-filter-failed"
              onCheckedChange={failed => setDraft(current => ({ ...current, failed }))}
            />
          </div>
        </div>
        <DialogFooter className="justify-between" variant="ruled">
          <Button
            onClick={() => {
              onApply({});
              onOpenChange(false);
            }}
            type="button"
            variant="ghost"
          >
            Clear filters
          </Button>
          <Button data-testid="terminal-journal-filter-apply" type="submit">
            Apply filters
          </Button>
        </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
