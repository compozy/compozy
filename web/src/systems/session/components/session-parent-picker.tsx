import { CornerDownRight, CornerLeftUp } from "@compozy/ui/icons";
import {
  Button,
  Command,
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@compozy/ui";

import { getSessionDisplayTitle } from "../lib/session-display-title";
import { sessionCanMakeTopLevel, sessionMoveUnderRefusal } from "../lib/session-reparent";
import type { SessionPayload } from "../types";

export interface SessionParentPickerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The session choosing a parent; `null` while its read is in flight or failed. */
  source: SessionPayload | null;
  sourceFailed: boolean;
  /** Eligible parents for the current search, already scoped by the owning host. */
  candidates: readonly SessionPayload[];
  search: string;
  onSearchChange: (search: string) => void;
  loading: boolean;
  failed: boolean;
  onRetry: () => void;
  /** A move is in flight; every choice waits for it. */
  pending: boolean;
  onChoose: (parent: SessionPayload) => void;
  onMakeTopLevel: () => void;
}

/**
 * Move under…: pick the session this one nests under, or return a nested child
 * to the top level. Keyboard-first: typing searches the project's sessions and
 * Enter moves under the selected one. An ineligible session states why instead
 * of listing parents the daemon would refuse.
 */
export function SessionParentPicker({
  open,
  onOpenChange,
  source,
  sourceFailed,
  candidates,
  search,
  onSearchChange,
  loading,
  failed,
  onRetry,
  pending,
  onChoose,
  onMakeTopLevel,
}: SessionParentPickerProps) {
  const title = source ? getSessionDisplayTitle(source) : "";
  const refusal = source ? sessionMoveUnderRefusal(source) : null;
  const canMakeTopLevel = source !== null && sessionCanMakeTopLevel(source);
  const choosing = source !== null && refusal === null;
  return (
    <CommandDialog
      className="sm:max-w-lg"
      description={
        title ? `Choose the session “${title}” moves under.` : "Choose a parent session."
      }
      onOpenChange={onOpenChange}
      open={open}
      title="Move under another session"
    >
      <Command data-testid="session-parent-picker" shouldFilter={false} className="h-auto">
        {choosing ? (
          <CommandInput
            aria-label="Search sessions"
            autoFocus
            onValueChange={onSearchChange}
            placeholder={`Move “${title}” under…`}
            value={search}
            variant="quiet"
          />
        ) : null}
        <CommandList className="max-h-[min(24rem,50vh)]">
          {source === null ? (
            <p className="px-3 py-2 text-small-body text-muted-foreground" role="status">
              {sourceFailed ? "Couldn't load this session." : "Loading session…"}
            </p>
          ) : null}
          {refusal ? (
            <p
              className="px-3 py-2 text-small-body text-muted-foreground"
              data-testid="session-parent-picker-refusal"
              role="note"
            >
              {refusal}
            </p>
          ) : null}
          {canMakeTopLevel ? (
            <CommandGroup heading="Top level">
              <CommandItem
                data-testid="session-parent-picker-top-level"
                disabled={pending}
                onSelect={onMakeTopLevel}
                value="top-level"
              >
                <CornerLeftUp aria-hidden="true" className="text-subtle" />
                Make top-level
              </CommandItem>
            </CommandGroup>
          ) : null}
          {choosing ? (
            <SessionParentCandidates
              candidates={candidates}
              failed={failed}
              loading={loading}
              onChoose={onChoose}
              onRetry={onRetry}
              pending={pending}
            />
          ) : null}
        </CommandList>
      </Command>
    </CommandDialog>
  );
}

function SessionParentCandidates({
  candidates,
  loading,
  failed,
  pending,
  onChoose,
  onRetry,
}: Pick<
  SessionParentPickerProps,
  "candidates" | "loading" | "failed" | "pending" | "onChoose" | "onRetry"
>) {
  if (failed) {
    return (
      <div className="flex items-center gap-2 px-3 py-2 text-small-body" role="alert">
        Couldn't load sessions.
        <Button type="button" size="sm" variant="ghost" onClick={onRetry}>
          Try again
        </Button>
      </div>
    );
  }
  if (loading) {
    return (
      <p className="px-3 py-2 text-small-body text-muted-foreground" role="status">
        Loading sessions…
      </p>
    );
  }
  return (
    <>
      <CommandEmpty>No sessions match.</CommandEmpty>
      {candidates.length > 0 ? (
        <CommandGroup heading="Sessions">
          {candidates.map(parent => (
            <CommandItem
              data-testid={`session-parent-picker-${parent.id}`}
              disabled={pending}
              key={parent.id}
              onSelect={() => onChoose(parent)}
              value={parent.id}
            >
              <CornerDownRight aria-hidden="true" className="text-subtle" />
              <span className="min-w-0 flex-1 truncate">{getSessionDisplayTitle(parent)}</span>
              <span className="ms-auto shrink-0 ps-3 text-micro text-subtle">
                {parent.agent_name}
              </span>
            </CommandItem>
          ))}
        </CommandGroup>
      ) : null}
    </>
  );
}
