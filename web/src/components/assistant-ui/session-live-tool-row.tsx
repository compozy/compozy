import { Layers } from "lucide-react";
import { createElement } from "react";

import {
  SessionToolCallRow,
  liveToolLabel,
  parallelToolLabel,
  getToolIcon,
  resolveRegisteredToolName,
} from "@/systems/session";
import type { SessionNavigationReveal } from "./hooks/session-navigation-target-context";
import { toolMessageFromPart } from "./session-timeline-tool-message";

import { cn } from "@/lib/utils";
import { useSessionThreadLiveData } from "./hooks/use-session-thread-live-data";
import { TranscriptDisclosure } from "@compozy/ui";

import type { SessionLiveToolRow, SessionTimelineToolPart } from "./session-timeline.logic";

function LiveToolGlyph({ part }: { part: SessionTimelineToolPart }) {
  return createElement(getToolIcon(resolveRegisteredToolName(part.toolName), part.args), {
    "aria-hidden": true,
    className: "size-3.5 shrink-0 text-subtle",
  });
}

// The one live line: kind glyph in the 20px well, the verb shimmering (plain
// subtle text when still — under reduced motion, or while the window is
// paused — the word "Running" carries the state), then its object as the same
// mono chip the settled row shows, nothing on the right. Plain text, not a
// popover: the raw input is one click away once the call settles into its tool
// row; a truncated object keeps the full text on hover.
function LiveToolLine({ part, still }: { part: SessionTimelineToolPart; still: boolean }) {
  const label = liveToolLabel(part.toolName, part.args, part.toolTitle);
  return (
    <div
      className="flex min-h-transcript-line min-w-0 items-center gap-1.5 px-1 text-small-body"
      data-testid="live-tool-row"
      data-live-kind={part.toolName}
    >
      <span className="flex size-5 shrink-0 items-center justify-center rounded-xs">
        <LiveToolGlyph part={part} />
      </span>
      <span
        className="flex min-w-0 items-center gap-1.5"
        data-testid="live-tool-label"
        // A polite status: assistive tech hears "Running …" when the call
        // starts, the cue the shimmer gives sighted readers.
        role="status"
        aria-label={label.text}
        title={part.toolTitle ?? label.text}
      >
        <span
          className={cn("shrink-0 whitespace-nowrap", still ? "text-subtle" : "session-shimmer")}
        >
          {label.verb}
        </span>
        {label.preview ? (
          <span className="min-w-0 truncate rounded-xs bg-hover px-1.5 py-px font-mono text-transcript-body text-muted">
            {label.preview}
          </span>
        ) : null}
      </span>
    </div>
  );
}

export interface SessionLiveToolRowViewProps {
  row: SessionLiveToolRow;
  reducedMotion: boolean;
  onToggle: () => void;
  reveal?: SessionNavigationReveal | null;
}

/**
 * `SessionLiveToolRow` (ADR-006 rule 1): exactly one live row for the calls
 * still running. A single call reads "Running {tool} — {preview}"; several stay
 * one honest row — "Running N tools…" with the `layers` glyph — that expands to
 * the in-flight list.
 * The shimmer is the only motion in the transcript: it dies under reduced
 * motion and stops while the window is paused (US-018.EC-2) — a view that is
 * not applying frames has no cadence to show.
 */
export function SessionLiveToolRowView({
  row,
  reducedMotion,
  onToggle,
  reveal,
}: SessionLiveToolRowViewProps) {
  const liveData = useSessionThreadLiveData();
  const still = reducedMotion || !liveData;
  const first = row.entries[0];
  if (!first) return null;
  if (row.entries.length === 1) {
    return (
      <div
        data-testid="live-tool"
        data-still={still || undefined}
        className="flex min-w-0 flex-col"
      >
        <LiveToolLine part={first} still={still} />
        {reveal ? (
          <SessionToolCallRow
            message={toolMessageFromPart(first)}
            partIndex={first.partIndex}
            revealOpen
            revealField={reveal.field ?? undefined}
            onRevealRelease={onToggle}
          />
        ) : null}
      </div>
    );
  }
  const detailsId = `${row.id}:entries`;
  return (
    <div
      data-testid="live-tool"
      data-parallel={row.entries.length}
      data-still={still || undefined}
      className="flex min-w-0 flex-col"
    >
      <TranscriptDisclosure
        aria-controls={detailsId}
        data-testid="live-tool-parallel"
        expanded={row.expanded}
        icon={<Layers aria-hidden="true" className="size-3.5 shrink-0 text-subtle" />}
        label={
          <span
            className={cn(still ? "text-subtle" : "session-shimmer")}
            data-testid="live-tool-label"
          >
            {parallelToolLabel(row.entries.length)}
          </span>
        }
        onToggle={onToggle}
      />
      {/* The disclosure label lives inside its button, so the running count is
          announced from a sibling status instead. */}
      <span className="sr-only" role="status">
        {parallelToolLabel(row.entries.length)}
      </span>
      <div
        id={detailsId}
        data-testid="live-tool-entries"
        hidden={!row.expanded}
        aria-hidden={!row.expanded}
        inert={!row.expanded}
        className={
          row.expanded
            ? "ml-transcript-detail-indent flex min-w-0 flex-col gap-0.5 pt-0.5"
            : undefined
        }
      >
        {row.expanded
          ? row.entries.map(part => (
              <div key={part.id} className="min-w-0">
                <LiveToolLine part={part} still={still} />
                {reveal && reveal.partIndex === part.partIndex ? (
                  <SessionToolCallRow
                    message={toolMessageFromPart(part)}
                    partIndex={part.partIndex}
                    revealOpen
                    revealField={reveal.field ?? undefined}
                    onRevealRelease={onToggle}
                  />
                ) : null}
              </div>
            ))
          : null}
      </div>
    </div>
  );
}
