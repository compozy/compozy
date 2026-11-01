import { useRef, useState } from "react";
import { Bot } from "@compozy/ui/icons";
import { useReducedMotionConfig } from "motion/react";

import {
  Button,
  InputGroupButton,
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
  StatusDot,
  cn,
} from "@compozy/ui";

import {
  subagentComposerSignal,
  subagentRosterTitle,
  type SubagentComposerSignal,
  type SubagentCounts,
} from "./subagent-format";
import { SubagentRoster, type SubagentRosterProps } from "./subagent-roster";
import { SubagentRosterSkeleton } from "./subagent-roster-skeleton";
import type { SubagentView } from "./types";

export interface SubagentComposerControlProps {
  subagents: readonly SubagentView[];
  /** What the trigger reports: the rows' counts, or the session's summary before they load. */
  counts: SubagentCounts;
  /**
   * The roster has not answered yet: the popover holds the rows' place (sized by the session's
   * summary) instead of claiming nothing was delegated.
   */
  loading?: boolean;
  /** The rows could not be read; `onRetry` reads them again. */
  failed?: boolean;
  onRetry?: () => void;
  onOpen?: SubagentRosterProps["onOpen"];
  onStop?: SubagentRosterProps["onStop"];
  /** The popover opened or closed; the owner reads the roster while it is open. */
  onOpenChange?: (open: boolean) => void;
  /** Hold the live pulse still (rows the stream has not confirmed). */
  stale?: boolean;
}

/** The subagents icon, carrying the avatar's live dot while any subagent runs. */
function SubagentComposerGlyph({
  signal,
  stale,
}: {
  signal: SubagentComposerSignal;
  stale: boolean;
}) {
  const reduced = useReducedMotionConfig();
  const dot = signal.dot;
  return (
    <span className="relative inline-flex">
      <Bot aria-hidden="true" className="size-4" />
      {dot ? (
        <StatusDot
          tone={dot.tone ?? "faint"}
          size="sm"
          className={cn(
            "absolute -right-0.5 -bottom-0.5 ring-2 ring-card group-hover/button:ring-surface-2 group-data-[popup-open]/button:ring-surface-2",
            dot.pulse && !reduced && !stale && "animate-pulse"
          )}
        />
      ) : null}
    </span>
  );
}

function SubagentComposerFailure({ onRetry }: { onRetry?: () => void }) {
  return (
    <div className="flex items-center justify-between gap-2 px-0.5">
      <p className="text-transcript-caption text-muted-foreground">Couldn't load subagents.</p>
      {onRetry ? (
        <Button type="button" variant="ghost" size="xs" onClick={onRetry}>
          Try again
        </Button>
      ) : null}
    </div>
  );
}

/** No rows: a placeholder while they load, or the empty state. */
function SubagentComposerPlaceholder({ loading, count }: { loading: boolean; count: number }) {
  // The summary sizes the placeholder; with none, a couple of rows hold the place.
  if (loading) return <SubagentRosterSkeleton count={count > 0 ? count : 2} />;
  return (
    <div className="flex flex-col gap-0.5 px-0.5 pb-0.5">
      <p className="text-small-body text-fg">Nothing delegated yet.</p>
      <p className="text-transcript-caption text-pretty text-muted-foreground">
        When the agent hands work to a subagent, it shows up here.
      </p>
    </div>
  );
}

type SubagentComposerPanelProps = Omit<SubagentComposerControlProps, "onOpenChange" | "stale"> & {
  stale: boolean;
};

/** Popover body: the roster rows, a failed read beside any rows that did load, or a placeholder. */
function SubagentComposerPanel({
  subagents,
  counts,
  loading = false,
  failed = false,
  onRetry,
  onOpen,
  onStop,
  stale,
}: SubagentComposerPanelProps) {
  const hasRows = subagents.length > 0;
  return (
    <>
      <div className="flex items-baseline gap-2 px-0.5">
        <PopoverTitle className="text-transcript-meta font-semibold text-fg-strong">
          {subagentRosterTitle(counts.live)}
        </PopoverTitle>
        {counts.total > 0 ? (
          <span className="ml-auto font-mono text-mono-id text-faint tabular-nums">
            {counts.total}
          </span>
        ) : null}
      </div>
      {hasRows ? (
        <SubagentRoster
          subagents={subagents}
          onOpen={onOpen}
          onStop={onStop}
          stale={stale}
          hoverSide="right"
          // Nothing runs: the previous rows are the whole answer, so they start open.
          defaultPreviousOpen={counts.live === 0}
        />
      ) : null}
      {failed ? <SubagentComposerFailure onRetry={onRetry} /> : null}
      {hasRows || failed ? null : (
        <SubagentComposerPlaceholder loading={loading} count={counts.total} />
      )}
    </>
  );
}

/**
 * The composer's subagents control (issue #766): a quiet icon while nothing
 * runs; while subagents work, the icon takes their avatar's live dot and the
 * live count. Its popover hosts the inspector's roster: current rows first,
 * then previous.
 */
export function SubagentComposerControl({
  onOpenChange,
  stale = false,
  ...panel
}: SubagentComposerControlProps) {
  const [open, setOpen] = useState(false);
  const trigger = useRef<HTMLButtonElement>(null);
  const openingDetail = useRef(false);
  const signal = subagentComposerSignal(panel.counts, panel.failed);
  const live = signal.count !== null;

  return (
    <Popover
      open={open}
      onOpenChange={next => {
        if (next) openingDetail.current = false;
        setOpen(next);
        onOpenChange?.(next);
      }}
    >
      <PopoverTrigger
        render={
          <InputGroupButton
            ref={trigger}
            type="button"
            // Icon-only while idle; the live count widens it into the tool pill.
            size={live ? "tool" : "tool-icon"}
            aria-label={signal.ariaLabel}
            data-testid="composer-subagents-button"
            data-state={signal.state}
            className="shrink-0 data-[popup-open]:bg-surface-2 data-[popup-open]:text-fg"
          />
        }
      >
        <SubagentComposerGlyph signal={signal} stale={stale} />
        {live ? <span className="font-mono text-mono-id tabular-nums">{signal.count}</span> : null}
      </PopoverTrigger>
      <PopoverContent
        finalFocus={() => (openingDetail.current ? false : trigger.current)}
        side="top"
        align="start"
        className="w-80 max-w-[calc(100vw-2rem)] gap-2"
      >
        <SubagentComposerPanel
          {...panel}
          stale={stale}
          onOpen={
            panel.onOpen
              ? (subagent, options) => {
                  openingDetail.current = true;
                  setOpen(false);
                  onOpenChange?.(false);
                  trigger.current?.focus();
                  panel.onOpen?.(subagent, options);
                }
              : undefined
          }
        />
      </PopoverContent>
    </Popover>
  );
}
