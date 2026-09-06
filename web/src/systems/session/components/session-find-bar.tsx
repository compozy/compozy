import { ChevronDown, ChevronUp, X } from "lucide-react";
import { useEffect, useRef, type KeyboardEvent } from "react";

import { Button, Kbd, KbdGroup, SearchInput, Spinner } from "@compozy/ui";

import { cn } from "@/lib/utils";

import { formatMessageTimestamp } from "../lib/format-timestamp";
import {
  findCountLabel,
  findMatchIndex,
  findMatchSpeaker,
  findSnippetSegments,
} from "../lib/session-navigation";
import type { SessionTranscriptSearchMatch } from "../types";
import type { SessionFindModel } from "../hooks/use-session-navigation";

export interface SessionFindBarProps {
  /** The host's `useSessionFind` model: the host owns it so marks and folds read the same matches. */
  find: SessionFindModel;
  agentName: string;
  /** Changes when ⌘F fires while the bar is already open: the field takes focus again. */
  focusKey?: number;
  /** Whether the entry starting at `sequence` sits behind a settled fold right now. */
  isSequenceFolded?: (sequence: number) => boolean;
  /** Esc / close: the host unmounts the bar and returns focus to the composer. */
  onClose: () => void;
  /** The entry's timestamp when the host can resolve one from the loaded transcript. */
  resolveMatchTime?: (match: SessionTranscriptSearchMatch) => number | null;
  className?: string;
}

/** The host's own words for a jump that could not land (the store keeps the error as thrown). */
function jumpErrorText(error: unknown): string {
  return error instanceof Error && error.message.length > 0
    ? error.message
    : "Couldn't jump to that match";
}

function FindKeys() {
  return (
    <span
      className="inline-flex items-center gap-1.5 text-micro text-faint"
      data-testid="find-keys"
    >
      <KbdGroup>
        <Kbd>⏎</Kbd>
        <Kbd>⇧⏎</Kbd>
      </KbdGroup>
      step
      <span aria-hidden="true">·</span>
      <Kbd>Esc</Kbd>
      close
    </span>
  );
}

/**
 * Find in conversation (S8): a 40px bar docked under the window head — never
 * an overlay. The field queries the daemon's full-history search; the count
 * says how many in the whole session; the list gives who/snippet/time and
 * marks hits behind a fold. Enter steps, Shift+Enter steps back, Esc closes;
 * ↑/↓ move the selection without jumping. Jumping loads older history and
 * opens folds through the host's handlers; the reader owns the position.
 */
export function SessionFindBar({
  find,
  agentName,
  focusKey = 0,
  isSequenceFolded,
  onClose,
  resolveMatchTime,
  className,
}: SessionFindBarProps) {
  const inputRef = useRef<HTMLInputElement | null>(null);
  useEffect(() => {
    inputRef.current?.focus();
    inputRef.current?.select();
  }, [focusKey]);

  const activeIndex = findMatchIndex(find.matches, find.activeSequence);
  const hasQuery = find.committedQuery.length > 0 || find.draftQuery.trim().length > 0;
  const hasMatches = find.matches.length > 0;
  const loadingOlder = find.jump?.phase === "loading";
  const activeId =
    find.activeSequence === null ? undefined : `session-find-match-${find.activeSequence}`;

  const handleKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key === "Escape") {
      event.preventDefault();
      onClose();
      return;
    }
    if (event.key === "Enter") {
      event.preventDefault();
      if (hasMatches) find.step(event.shiftKey ? -1 : 1);
      return;
    }
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      if (!hasMatches) return;
      event.preventDefault();
      const delta = event.key === "ArrowDown" ? 1 : -1;
      const next = (activeIndex + delta + find.matches.length) % find.matches.length;
      find.select(find.matches[next]!.sequence);
    }
  };

  // The search landmark is a `<form role="search">`: native semantics in every
  // browser and in the jsdom test environment (which still maps `<search>` to
  // HTMLUnknownElement). The form never submits — Enter steps the matches.
  return (
    <form
      role="search"
      aria-label="Find in conversation"
      data-testid="session-find-bar"
      data-state={
        find.isError
          ? "error"
          : loadingOlder
            ? "loading-older"
            : find.isPending
              ? "searching"
              : !hasQuery
                ? "empty"
                : hasMatches
                  ? "matches"
                  : "no-matches"
      }
      onKeyDown={handleKeyDown}
      onSubmit={event => event.preventDefault()}
      className={cn(
        "relative flex flex-col border-b border-line bg-elevated",
        "shadow-highlight",
        className
      )}
    >
      <div className="flex h-10 items-center gap-2 px-3">
        <SearchInput
          ref={inputRef}
          aria-activedescendant={activeId}
          aria-controls="session-find-matches"
          aria-label="Find in conversation"
          containerClassName="min-w-0 flex-1 max-w-md"
          data-testid="session-find-input"
          onChange={find.setDraftQuery}
          placeholder="Find in conversation"
          value={find.draftQuery}
        />
        <span
          aria-live="polite"
          className="inline-flex shrink-0 items-center gap-1.5 font-mono text-micro text-muted tabular-nums"
          data-testid="session-find-count"
        >
          {!hasQuery ? (
            <FindKeys />
          ) : find.isError ? (
            <span className="text-danger">Couldn&apos;t search</span>
          ) : find.jumpError !== null ? (
            <span className="text-danger" data-testid="session-find-jump-error">
              {jumpErrorText(find.jumpError)}
            </span>
          ) : loadingOlder ? (
            <>
              <Spinner className="size-3" />
              Loading older…
              <span aria-hidden="true">·</span>
              {findCountLabel(activeIndex, find.matches.length, find.truncated)}
            </>
          ) : find.isPending ? (
            <>
              <Spinner className="size-3" />
              Searching…
            </>
          ) : (
            <>
              {findCountLabel(activeIndex, find.matches.length, find.truncated)}
              {find.appended > 0 ? (
                <>
                  <span aria-hidden="true">·</span>
                  <span className="text-faint" data-testid="session-find-appended">
                    +{find.appended} new
                  </span>
                </>
              ) : null}
            </>
          )}
        </span>
        {hasMatches ? (
          <div className="flex shrink-0 items-center">
            <Button
              type="button"
              variant="ghost"
              size="icon-xs"
              aria-label="Previous match (Shift+Enter)"
              data-testid="session-find-previous"
              disabled={loadingOlder}
              onClick={() => find.step(-1)}
            >
              <ChevronUp aria-hidden="true" className="size-3" />
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="icon-xs"
              aria-label="Next match (Enter)"
              data-testid="session-find-next"
              disabled={loadingOlder}
              onClick={() => find.step(1)}
            >
              <ChevronDown aria-hidden="true" className="size-3" />
            </Button>
          </div>
        ) : null}
        {find.isError ? (
          <Button
            type="button"
            variant="ghost"
            size="xs"
            data-testid="session-find-retry"
            onClick={() => {
              find.retry();
              inputRef.current?.focus();
            }}
          >
            Try again
          </Button>
        ) : null}
        <Button
          type="button"
          variant="ghost"
          size="icon-xs"
          aria-label="Close find (Esc)"
          className="text-faint hover:text-fg"
          data-testid="session-find-close"
          onClick={onClose}
        >
          <X aria-hidden="true" className="size-3" />
        </Button>
      </div>
      {hasQuery && !find.isError && (hasMatches || !find.isPending) ? (
        <div
          id="session-find-matches"
          role="listbox"
          aria-label="Matches"
          data-testid="session-find-list"
          className="flex max-h-56 flex-col overflow-y-auto border-t border-line-soft py-1"
        >
          {hasMatches ? (
            find.matches.map(match => (
              <SessionFindMatchRow
                key={match.sequence}
                match={match}
                active={match.sequence === find.activeSequence}
                agentName={agentName}
                folded={isSequenceFolded?.(match.sequence) ?? false}
                query={find.committedQuery}
                timeMs={resolveMatchTime?.(match) ?? null}
                onJump={() => find.jumpTo(match.sequence)}
              />
            ))
          ) : (
            <p
              className="px-3 py-2 text-eyebrow text-muted"
              data-testid="session-find-empty"
              role="option"
              aria-selected={false}
            >
              No matches for &ldquo;{find.committedQuery}&rdquo; in this conversation.
            </p>
          )}
          {find.truncated ? (
            <p
              className="px-3 py-1.5 font-mono text-micro text-faint"
              data-testid="session-find-truncated"
            >
              Showing the first {find.matches.length} matches · refine the search to see the rest
            </p>
          ) : null}
        </div>
      ) : null}
    </form>
  );
}

function SessionFindMatchRow({
  match,
  active,
  agentName,
  folded,
  query,
  timeMs,
  onJump,
}: {
  match: SessionTranscriptSearchMatch;
  active: boolean;
  agentName: string;
  folded: boolean;
  query: string;
  timeMs: number | null;
  onJump: () => void;
}) {
  const time = timeMs === null ? "" : formatMessageTimestamp(timeMs);
  return (
    <button
      type="button"
      role="option"
      id={`session-find-match-${match.sequence}`}
      aria-selected={active}
      data-testid="session-find-match"
      data-sequence={match.sequence}
      data-folded={folded ? "true" : undefined}
      onClick={onJump}
      className={cn(
        "flex min-w-0 items-baseline gap-2 px-3 py-1.5 text-left text-transcript-body",
        "transition-colors duration-fast ease-out hover:bg-btn-default-hover",
        active ? "bg-elevated text-fg-strong" : "text-muted"
      )}
    >
      <span className="w-14 shrink-0 truncate text-eyebrow text-subtle">
        {findMatchSpeaker(match.role, agentName)}
      </span>
      <span className="min-w-0 flex-1 truncate">
        {findSnippetSegments(match.snippet, query).map((segment, index) =>
          segment.match ? (
            <mark
              key={index}
              data-testid="session-find-mark"
              className={cn(
                "rounded-xxs px-px text-inherit",
                active ? "bg-accent-tint-strong ring-1 ring-accent-dim" : "bg-badge-fill"
              )}
            >
              {segment.text}
            </mark>
          ) : (
            <span key={index}>{segment.text}</span>
          )
        )}
        {folded ? (
          <span
            className="ml-1.5 rounded-xxs bg-badge-fill px-1 font-mono text-mono-id text-faint"
            data-testid="session-find-folded"
          >
            folded
          </span>
        ) : null}
      </span>
      {time ? (
        <span className="shrink-0 font-mono text-micro text-faint tabular-nums">{time}</span>
      ) : null}
    </button>
  );
}
