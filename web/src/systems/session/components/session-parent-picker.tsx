import type { KeyboardEvent } from "react";
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
  /** The rows belong to a previous search or page while the next one loads. */
  stale: boolean;
  failed: boolean;
  onRetry: () => void;
  hasNext: boolean;
  hasPrevious: boolean;
  onNextPage: () => void;
  onPreviousPage: () => void;
  /** A move is in flight; every choice waits for it. */
  pending: boolean;
  onChoose: (parent: SessionPayload) => void;
  onMakeTopLevel: () => void;
}

/**
 * Move under…: pick the session this one nests under, or return a nested child
 * to the top level. Keyboard-first: typing searches the project's sessions and
 * Enter moves under the selected one. An ineligible session states why instead
 * of listing parents the daemon would refuse, with Make top-level focused when
 * the session can go back to the top level.
 */
export function SessionParentPicker(props: SessionParentPickerProps) {
  const { open, onOpenChange, source } = props;
  const title = source ? getSessionDisplayTitle(source) : "";
  const choosing = source !== null && sessionMoveUnderRefusal(source) === null;
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
      {choosing ? (
        <SessionParentSearch {...props} title={title} />
      ) : (
        <SessionParentUnavailable {...props} />
      )}
    </CommandDialog>
  );
}

function SessionParentUnavailable({
  source,
  sourceFailed,
  pending,
  onMakeTopLevel,
}: SessionParentPickerProps) {
  const refusal = source ? sessionMoveUnderRefusal(source) : null;
  const canMakeTopLevel = source !== null && sessionCanMakeTopLevel(source);
  return (
    <div className="flex flex-col items-start gap-3 p-3" data-testid="session-parent-picker">
      {source === null ? (
        <p className="text-small-body text-muted-foreground" role="status">
          {sourceFailed ? "Couldn't load this session." : "Loading session…"}
        </p>
      ) : (
        <p
          className="text-small-body text-muted-foreground"
          data-testid="session-parent-picker-refusal"
          role="note"
        >
          {refusal}
        </p>
      )}
      {canMakeTopLevel ? (
        <Button
          autoFocus
          data-testid="session-parent-picker-top-level"
          disabled={pending}
          onClick={onMakeTopLevel}
          size="sm"
          type="button"
          variant="outline"
        >
          <CornerLeftUp aria-hidden="true" />
          Make top-level
        </Button>
      ) : null}
    </div>
  );
}

/** Enter on a page button pages; it must not reach the list and choose the highlighted row. */
function stopCommandEnter(event: KeyboardEvent<HTMLButtonElement>) {
  if (event.key === "Enter") event.stopPropagation();
}

function SessionParentSearch({
  title,
  candidates,
  search,
  onSearchChange,
  loading,
  stale,
  failed,
  onRetry,
  hasNext,
  hasPrevious,
  onNextPage,
  onPreviousPage,
  pending,
  onChoose,
}: SessionParentPickerProps & { title: string }) {
  const locked = pending || stale;
  return (
    <Command
      className="h-auto"
      data-testid="session-parent-picker"
      label="Search sessions"
      shouldFilter={false}
    >
      <CommandInput
        autoFocus
        onValueChange={onSearchChange}
        placeholder={`Move “${title}” under…`}
        value={search}
        variant="quiet"
      />
      <CommandList className="max-h-[min(24rem,50vh)]" aria-busy={stale || undefined}>
        {failed ? (
          <div className="flex items-center gap-2 px-3 py-2 text-small-body" role="alert">
            Couldn't load sessions.
            <Button type="button" size="sm" variant="ghost" onClick={onRetry}>
              Try again
            </Button>
          </div>
        ) : loading ? (
          <p className="px-3 py-2 text-small-body text-muted-foreground" role="status">
            Loading sessions…
          </p>
        ) : (
          <>
            <CommandEmpty>No sessions match.</CommandEmpty>
            {candidates.length > 0 ? (
              <CommandGroup heading="Sessions">
                {candidates.map(parent => (
                  <CommandItem
                    data-testid={`session-parent-picker-${parent.id}`}
                    disabled={locked}
                    key={parent.id}
                    onSelect={() => {
                      if (!locked) onChoose(parent);
                    }}
                    value={parent.id}
                  >
                    <CornerDownRight aria-hidden="true" className="text-subtle" />
                    <span className="min-w-0 flex-1 truncate">
                      {getSessionDisplayTitle(parent)}
                    </span>
                    <span className="ms-auto shrink-0 ps-3 text-micro text-subtle">
                      {parent.agent_name}
                    </span>
                  </CommandItem>
                ))}
              </CommandGroup>
            ) : null}
          </>
        )}
      </CommandList>
      {hasNext || hasPrevious ? (
        <div className="flex justify-between gap-2 border-t border-line p-2">
          <Button
            type="button"
            size="sm"
            variant="ghost"
            disabled={!hasPrevious || stale}
            onClick={onPreviousPage}
            onKeyDown={stopCommandEnter}
          >
            Previous
          </Button>
          <Button
            type="button"
            size="sm"
            variant="ghost"
            disabled={!hasNext || stale}
            onClick={onNextPage}
            onKeyDown={stopCommandEnter}
          >
            Next
          </Button>
        </div>
      ) : null}
    </Command>
  );
}
