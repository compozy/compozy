import { cn, Skeleton, SkeletonRegion, SkeletonText } from "@compozy/ui";

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

/** Rows the loading launcher stands in with, by the width of each label. */
const AGGREGATE_ROWS = [
  { id: "a", title: "w-28" },
  { id: "b", title: "w-36" },
  { id: "c", title: "w-24" },
] as const;

/**
 * The All projects Terminal launcher in its loading shape: the centered panel,
 * its search line, and launcher rows as blocks. The lazy window chunk and a
 * window still binding a requested terminal both hold this frame.
 */
export function TerminalAggregateSkeleton() {
  return (
    <div
      className="flex min-h-0 flex-1 justify-center overflow-y-auto px-6 pt-[12vh] pb-6"
      data-testid="terminal-aggregate-loading"
    >
      <SkeletonRegion
        className="flex w-full max-w-lg flex-col self-start rounded-lg border border-line p-1.5"
        label="Loading terminals"
      >
        <div aria-hidden="true" className="flex h-10 items-center gap-2.5 px-3.5">
          <SkeletonText className="w-40 text-body" />
        </div>
        {AGGREGATE_ROWS.map(row => (
          <div
            aria-hidden="true"
            className="flex min-h-8 animate-skeleton-reveal items-center gap-2 px-3.5 py-1.5"
            key={row.id}
          >
            <Skeleton className="size-4 rounded-sm" />
            <SkeletonText className={cn(row.title, "text-small-body")} />
          </div>
        ))}
      </SkeletonRegion>
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
