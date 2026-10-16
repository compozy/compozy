import { Network } from "lucide-react";
import { useReducedMotionConfig } from "motion/react";

import {
  Button,
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
  Skeleton,
  SkeletonRows,
  StatusDot,
  cn,
} from "@compozy/ui";

import {
  subagentComposerSignal,
  subagentRosterTitle,
  type SubagentCounts,
} from "./subagent-format";
import { SubagentRoster, type SubagentRosterProps } from "./subagent-roster";
import type { SubagentView } from "./types";

export interface SubagentComposerControlProps {
  subagents: readonly SubagentView[];
  /** What the trigger reports: the rows' counts, or the session's summary before they load. */
  counts: SubagentCounts;
  /** The session reports subagents whose rows have not arrived yet. */
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

/**
 * The composer's subagents control (issue #766): a quiet icon while nothing
 * runs; while subagents work, the icon takes their avatar's live dot and the
 * live count. Its popover hosts the inspector's roster: current rows first,
 * then previous.
 */
export function SubagentComposerControl({
  subagents,
  counts,
  loading = false,
  failed = false,
  onRetry,
  onOpen,
  onStop,
  onOpenChange,
  stale = false,
}: SubagentComposerControlProps) {
  const reduced = useReducedMotionConfig();
  const signal = subagentComposerSignal(counts);

  return (
    <Popover onOpenChange={open => onOpenChange?.(open)}>
      <PopoverTrigger
        render={
          <Button
            type="button"
            variant="quiet"
            size="icon"
            aria-label={signal.ariaLabel}
            data-testid="composer-subagents-button"
            data-state={signal.dot ? (counts.attention > 0 ? "attention" : "running") : "idle"}
            className={cn(
              "shrink-0 text-fg-2 data-[popup-open]:bg-surface-2 data-[popup-open]:text-fg",
              signal.count !== null && "w-auto gap-1.5 px-2"
            )}
          />
        }
      >
        <span className="relative inline-flex">
          <Network aria-hidden="true" />
          {signal.dot ? (
            <StatusDot
              tone={signal.dot.tone ?? "faint"}
              size="sm"
              className={cn(
                "absolute -right-0.5 -bottom-0.5 ring-2 ring-card group-hover/button:ring-surface-2 group-data-[popup-open]/button:ring-surface-2",
                signal.dot.pulse && !reduced && !stale && "animate-pulse"
              )}
            />
          ) : null}
        </span>
        {signal.count !== null ? (
          <span className="font-mono text-mono-id tabular-nums">{signal.count}</span>
        ) : null}
      </PopoverTrigger>
      <PopoverContent side="top" align="start" className="w-80 max-w-[calc(100vw-2rem)] gap-2">
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
        {subagents.length > 0 ? (
          <SubagentRoster
            subagents={subagents}
            onOpen={onOpen}
            onStop={onStop}
            stale={stale}
            hoverSide="right"
            // Nothing runs: the previous rows are the whole answer, so they start open.
            defaultPreviousOpen={counts.live === 0}
          />
        ) : failed ? (
          <div className="flex items-center justify-between gap-2 px-0.5">
            <p className="text-transcript-caption text-muted">Couldn't load subagents.</p>
            {onRetry ? (
              <Button type="button" variant="ghost" size="xs" onClick={onRetry}>
                Try again
              </Button>
            ) : null}
          </div>
        ) : loading ? (
          <SkeletonRows count={2} className="gap-2 px-0.5" rowClassName="h-7.5 justify-center">
            <Skeleton className="h-3.5 w-2/3" />
          </SkeletonRows>
        ) : (
          <div className="flex flex-col gap-0.5 px-0.5 pb-0.5">
            <p className="text-small-body text-fg">Nothing delegated yet.</p>
            <p className="text-transcript-caption text-pretty text-muted">
              When the agent hands work to a subagent, it shows up here.
            </p>
          </div>
        )}
      </PopoverContent>
    </Popover>
  );
}
