import type { ReactNode } from "react";

import {
  cn,
  Item,
  ItemActions,
  ItemContent,
  ItemTitle,
  Skeleton,
  SkeletonRegion,
  SkeletonText,
} from "@compozy/ui";

// Terminal-shaped loading placeholders. Static and query-free: they import nothing from the
// terminal system but `@compozy/ui` primitives, so a window can show them before its code has
// loaded. A terminal surface is only ever a placeholder here, never the emulator: the live grid
// (`TerminalPane`) keeps zero animation, and these stand in until it mounts.

/** The emulator surface's padding and type (as `TerminalPane` sets them), so lines sit where output will. */
const TERMINAL_SURFACE_TEXT_CLASS = "px-3.5 pt-2.5 pb-3 font-mono text-code-block tracking-mono";

/** Lines a loading surface stands in with: a prompt and a little output, never a screenful. */
const TERMINAL_LINES = [
  { id: "prompt", width: "w-2/5" },
  { id: "second", width: "w-3/5" },
  { id: "third", width: "w-1/3" },
  { id: "fourth", width: "w-1/2" },
  { id: "fifth", width: "w-2/5" },
] as const;

function TerminalLinesSkeleton({ className }: { className?: string }) {
  return (
    <div aria-hidden="true" className={cn(TERMINAL_SURFACE_TEXT_CLASS, className)}>
      {TERMINAL_LINES.map(line => (
        <SkeletonText className={line.width} key={line.id} />
      ))}
    </div>
  );
}

export interface TerminalWindowSkeletonProps {
  /** The busy status the region announces. Defaults to "Loading terminal". */
  label?: string;
}

/**
 * The Terminal window before its terminal is known: the emulator's dark ground, filling the body
 * the live window fills, with a few mono lines as blocks where the prompt and output go. One busy
 * status for the whole body.
 */
export function TerminalWindowSkeleton({
  label = "Loading terminal",
}: TerminalWindowSkeletonProps) {
  return (
    <SkeletonRegion
      className="flex min-h-0 min-w-0 flex-1 flex-col bg-terminal-bg"
      data-testid="terminal-window-loading"
      label={label}
    >
      <TerminalLinesSkeleton />
    </SkeletonRegion>
  );
}

/** Rows a loading list stands in with, with the width of the title and of the state it trails. */
const AGGREGATE_ROWS = [
  { id: "a", title: "w-48", state: "w-14" },
  { id: "b", title: "w-36", state: "w-14" },
  { id: "c", title: "w-56", state: "w-12" },
  { id: "d", title: "w-40", state: "w-14" },
] as const;

export interface TerminalAggregateSkeletonProps {
  /** The Project filter. Leave it out for a block where the filter goes. */
  filter?: ReactNode;
  /** The "New terminal in" row. Leave it out for a block of text and a button, `null` for none. */
  footer?: ReactNode;
}

/**
 * The all-projects terminal list in its loading shape: the list frame, the filter, rows of the
 * real `Item` with the project, title and state as blocks, and the new-terminal row. Pass the live
 * filter and footer where they do not depend on the list, so they keep their state and place.
 */
export function TerminalAggregateSkeleton({ filter, footer }: TerminalAggregateSkeletonProps) {
  return (
    <div
      className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto px-5 py-4"
      data-testid="terminal-aggregate-loading"
    >
      {filter ?? <Skeleton className="h-button-sm w-16 rounded-md" />}
      <SkeletonRegion className="flex flex-col" label="Loading terminals">
        {AGGREGATE_ROWS.map(row => (
          <Item
            aria-hidden="true"
            className="animate-skeleton-reveal rounded-md"
            key={row.id}
            size="sm"
          >
            <ItemContent className="flex-row items-center gap-2">
              <SkeletonText className="w-20 text-micro" />
              <ItemTitle className="min-w-0 truncate font-normal">
                <SkeletonText className={row.title} />
              </ItemTitle>
            </ItemContent>
            <ItemActions className="text-micro text-subtle">
              <SkeletonText className={row.state} />
            </ItemActions>
          </Item>
        ))}
      </SkeletonRegion>
      {footer === undefined ? (
        <div aria-hidden="true" className="flex flex-wrap items-center gap-2">
          <SkeletonText className="w-40 text-small-body" />
          <Skeleton className="h-button-sm w-14 rounded-md" />
        </div>
      ) : (
        footer
      )}
    </div>
  );
}

/** Lines a loading log stands in with: numbered like the real pane, a little output. */
const LOG_LINES = [
  { id: "a", width: "w-2/3" },
  { id: "b", width: "w-1/2" },
  { id: "c", width: "w-3/4" },
  { id: "d", width: "w-2/5" },
  { id: "e", width: "w-3/5" },
] as const;

/**
 * A pipe terminal's captured output while it is read: the terminal ground with the numbered gutter
 * of the real log pane (`TerminalPipeLogPane`) and lines as blocks.
 */
export function TerminalPipeLogSkeleton() {
  return (
    <SkeletonRegion
      className="flex min-h-0 min-w-0 flex-1 flex-col bg-terminal-bg"
      label="Loading the captured output"
    >
      <div aria-hidden="true" className={cn("min-h-0 flex-1", TERMINAL_SURFACE_TEXT_CLASS)}>
        {LOG_LINES.map(line => (
          <div className="grid grid-cols-[2.125rem_minmax(0,1fr)] gap-2.5" key={line.id}>
            <SkeletonText className="ml-auto w-5" />
            <SkeletonText className={line.width} />
          </div>
        ))}
      </div>
    </SkeletonRegion>
  );
}

/**
 * A recording while its artifact is read: the player's title row, the terminal ground and the
 * transport bar (button, scrubber, clock) as blocks, so the replay opens into the same frames.
 */
export function TerminalRecordingSkeleton() {
  return (
    <SkeletonRegion className="flex min-h-0 flex-1 flex-col" label="Loading the recording">
      <div
        aria-hidden="true"
        className="flex min-h-9 min-w-0 flex-none items-center gap-2 border-line border-b bg-canvas px-4"
      >
        <Skeleton className="size-3 flex-none rounded-xs" />
        <SkeletonText className="w-56 text-eyebrow" />
      </div>
      <div className="flex min-h-0 flex-1 flex-col bg-terminal-bg">
        <TerminalLinesSkeleton />
      </div>
      <div
        aria-hidden="true"
        className="flex min-h-11.5 flex-none items-center gap-3 border-line border-t bg-canvas px-3"
      >
        <Skeleton className="size-button-icon-sm flex-none rounded-md" />
        <Skeleton className="h-1 min-w-0 flex-1 rounded-pill" />
        <SkeletonText className="w-24 font-mono text-eyebrow" />
      </div>
    </SkeletonRegion>
  );
}
