import { ChevronRight, Layers } from "lucide-react";
import { createElement } from "react";

import { SessionToolCallRow } from "@/systems/session/components/tool-call-card";
import type { SessionNavigationReveal } from "./hooks/session-navigation-target-context";
import { toolMessageFromPart } from "./session-timeline-tool-message";

import { SessionSummaryDisclosure } from "@/systems/session/components/session-summary-disclosure";

import { cn } from "@/lib/utils";
import { useSessionThreadLiveData } from "./hooks/use-session-thread-live-data";
import { getToolIcon, resolveRegisteredToolName } from "@/systems/session/lib/tool-labels";
import { liveToolLabel, parallelToolLabel } from "@/systems/session/lib/session-tool-visual-state";

import type { SessionLiveToolRow, SessionTimelineToolPart } from "./session-timeline.logic";

function LiveToolGlyph({ part }: { part: SessionTimelineToolPart }) {
  return createElement(getToolIcon(resolveRegisteredToolName(part.toolName), part.args), {
    "aria-hidden": true,
    className: "size-3.5 shrink-0 text-subtle",
    strokeWidth: 1.75,
  });
}

// The one live line: kind glyph in the 20px well, the sentence shimmering
// (plain subtle text when still — under reduced motion, or while the window
// is paused — the word "Running" carries the state), nothing on the right.
function LiveToolLine({ part, still }: { part: SessionTimelineToolPart; still: boolean }) {
  const label = liveToolLabel(part.toolName, part.args, part.toolTitle);
  return (
    <div
      className="flex min-h-transcript-line min-w-0 items-center gap-transcript-inline-gap px-1 text-small-body"
      data-testid="live-tool-row"
      data-live-kind={part.toolName}
    >
      <span className="flex size-5 shrink-0 items-center justify-center rounded-xs">
        <LiveToolGlyph part={part} />
      </span>
      <span
        className={cn(
          "min-w-0 max-w-sm flex-1 truncate font-medium",
          still ? "text-subtle" : "session-shimmer"
        )}
        data-testid="live-tool-label"
      >
        <SessionSummaryDisclosure
          summary={label.text}
          label="Tool details"
          detail={`${part.toolTitle ?? part.toolName}\n\n${JSON.stringify({ tool: part.toolName, ...(part.toolTitle ? { title: part.toolTitle } : {}), input: part.args }, null, 2)}`}
          className="max-w-full font-medium"
        />
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
 * the in-flight list. A running child agent is its own row with the bot glyph.
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
        data-agent={row.agent || undefined}
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
      <button
        type="button"
        aria-expanded={row.expanded}
        aria-controls={detailsId}
        data-testid="live-tool-parallel"
        onClick={onToggle}
        className={cn(
          "group/live inline-flex min-h-transcript-line w-full min-w-0 items-center gap-transcript-inline-gap rounded-md px-1 text-left text-small-body",
          "transition-colors duration-base ease-out hover:bg-hover focus-visible:shadow-focus-ring focus-visible:outline-none"
        )}
      >
        <span className="flex size-5 shrink-0 items-center justify-center rounded-xs">
          <Layers aria-hidden="true" className="size-3.5 shrink-0 text-subtle" strokeWidth={1.75} />
        </span>
        <span
          className={cn(
            "min-w-0 max-w-sm flex-1 truncate font-medium",
            still ? "text-subtle" : "session-shimmer"
          )}
          data-testid="live-tool-label"
        >
          {parallelToolLabel(row.entries.length)}
        </span>
        <ChevronRight
          aria-hidden="true"
          className={cn(
            "size-3 shrink-0 text-faint opacity-0 transition-[opacity,transform] duration-base ease-out group-hover/live:opacity-100 motion-reduce:transition-none",
            row.expanded ? "rotate-90 opacity-100" : null
          )}
          strokeWidth={1.75}
        />
      </button>
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
