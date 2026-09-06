import { useState, type KeyboardEvent } from "react";

import { Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";

import { cn } from "@/lib/utils";

import { formatMessageTimestamp } from "../lib/format-timestamp";
import {
  TRAIL_HIT_HEIGHT_PX,
  TRAIL_MIN_ENTRIES,
  TRAIL_TICK_HEIGHT_PX,
  TRAIL_TICK_WIDTH_ANCHOR_PX,
  trailAnchorSequence,
  trailKeyTarget,
  trailLayout,
  trailTickLabel,
  trailTickState,
  trailTickWidth,
  type SessionSequenceRange,
} from "../lib/session-navigation";
import type { SessionTranscriptOutlineEntry } from "../types";
import { useSessionTranscriptOutline } from "../hooks/use-session-navigation";

export interface SessionMessageTrailProps {
  workspaceId: string;
  sessionId: string;
  /** Pane height the rail may use (its 80% share is derived inside); layout depends only on this and the count. */
  paneHeightPx: number;
  /** Start sequence of the first entry at or under the viewport top; `null` before the host measured. */
  viewportTopSequence: number | null;
  /** Start sequences of the entries currently in view. */
  visibleRange: SessionSequenceRange | null;
  /** See `useSessionTranscriptOutline`: bumps when durable history changed. */
  refreshKey?: string;
  /** Jumps through the host's scroll owner: free mode, bubble 16px under the top (S6). */
  onJumpToSequence: (sequence: number) => void;
  enabled?: boolean;
  className?: string;
}

const TICK_OPACITY: Record<"anchor" | "in-view" | "rest", string> = {
  anchor: "opacity-90",
  "in-view": "opacity-50",
  rest: "opacity-20",
};

/**
 * The message trail (S9): a 28px rail on the scroller's left edge with one
 * 2px tick per operator message from the outline (full history). Ticks rest
 * faint, read stronger for messages in view, the reading anchor is wider and
 * `aria-current`, and width breathes toward the pointer like a dock — colour
 * never moves. Hover/focus shows what was asked and the final reply; click
 * jumps through the host's scroll owner. One roving tab stop; ↑/↓, Home/End,
 * Enter/Space, Esc.
 */
export function SessionMessageTrail({
  workspaceId,
  sessionId,
  paneHeightPx,
  viewportTopSequence,
  visibleRange,
  refreshKey,
  onJumpToSequence,
  enabled = true,
  className,
}: SessionMessageTrailProps) {
  const outline = useSessionTranscriptOutline({ enabled, refreshKey, sessionId, workspaceId });
  const entries = outline.data?.entries ?? [];
  const [hoveredIndex, setHoveredIndex] = useState<number | null>(null);
  const [focusedIndex, setFocusedIndex] = useState<number | null>(null);
  if (!enabled || entries.length < TRAIL_MIN_ENTRIES) return null;

  const layout = trailLayout(entries.length, paneHeightPx);
  const anchorSequence = trailAnchorSequence(entries, viewportTopSequence);
  const anchorIndex = Math.max(
    0,
    entries.findIndex(entry => entry.sequence === anchorSequence)
  );
  const hoveredOffset = hoveredIndex === null ? null : (layout.offsets[hoveredIndex] ?? null);
  const tabIndexFor = (index: number) => (index === (focusedIndex ?? anchorIndex) ? 0 : -1);

  const handleKeyDown = (event: KeyboardEvent<HTMLElement>, index: number) => {
    if (event.key === "Escape") {
      (event.currentTarget as HTMLElement).blur();
      setFocusedIndex(null);
      return;
    }
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      onJumpToSequence(entries[index]!.sequence);
      return;
    }
    const target = trailKeyTarget(event.key, index, entries.length);
    if (target === null) return;
    event.preventDefault();
    setFocusedIndex(target);
    const next = event.currentTarget.parentElement?.parentElement?.querySelector<HTMLElement>(
      `[data-trail-index="${target}"]`
    );
    next?.focus();
  };

  return (
    <nav
      aria-label="Message navigation"
      data-testid="session-message-trail"
      data-compressed={layout.compressed ? "true" : undefined}
      className={cn("relative w-7 shrink-0 select-none", className)}
      style={{ height: layout.height }}
      onMouseLeave={() => setHoveredIndex(null)}
    >
      {entries.map((entry, index) => (
        <SessionTrailTick
          key={entry.sequence}
          entry={entry}
          index={index}
          ordinal={index + 1}
          total={entries.length}
          offset={layout.offsets[index] ?? 0}
          state={trailTickState(entry, anchorSequence, visibleRange)}
          width={trailTickWidth(layout.offsets[index] ?? 0, hoveredOffset, layout.pitch)}
          tabIndex={tabIndexFor(index)}
          onFocus={() => setFocusedIndex(index)}
          onHover={() => setHoveredIndex(index)}
          onJump={() => onJumpToSequence(entry.sequence)}
          onKeyDown={event => handleKeyDown(event, index)}
        />
      ))}
    </nav>
  );
}

function SessionTrailTick({
  entry,
  index,
  ordinal,
  total,
  offset,
  state,
  width,
  tabIndex,
  onFocus,
  onHover,
  onJump,
  onKeyDown,
}: {
  entry: SessionTranscriptOutlineEntry;
  index: number;
  ordinal: number;
  total: number;
  offset: number;
  state: "anchor" | "in-view" | "rest";
  width: number;
  tabIndex: number;
  onFocus: () => void;
  onHover: () => void;
  onJump: () => void;
  onKeyDown: (event: KeyboardEvent<HTMLElement>) => void;
}) {
  const at = Date.parse(entry.at);
  const time = Number.isFinite(at) ? formatMessageTimestamp(at) : "";
  const resolvedWidth = state === "anchor" ? Math.max(width, TRAIL_TICK_WIDTH_ANCHOR_PX) : width;
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <button
            type="button"
            aria-current={state === "anchor" ? "location" : undefined}
            aria-label={trailTickLabel(entry, ordinal)}
            data-testid="session-trail-tick"
            data-trail-index={index}
            data-sequence={entry.sequence}
            data-vis={state}
            tabIndex={tabIndex}
            onClick={onJump}
            onFocus={onFocus}
            onKeyDown={onKeyDown}
            onMouseEnter={onHover}
            className="absolute left-0 flex items-center focus-visible:outline-none"
            style={{ height: TRAIL_HIT_HEIGHT_PX, top: offset }}
          >
            <span
              aria-hidden="true"
              className={cn(
                "block rounded-full bg-fg transition-[width,opacity] duration-fast ease-out motion-reduce:transition-none",
                TICK_OPACITY[state],
                "group-hover:opacity-100 in-[:focus-visible]:opacity-100"
              )}
              style={{ height: TRAIL_TICK_HEIGHT_PX, width: resolvedWidth }}
            />
          </button>
        }
      />
      <TooltipContent side="right" className="w-64 max-w-64 whitespace-normal">
        {/* The popup lays its children out in a row; the card is one column: ask, reply, time · n of N. */}
        <div className="flex min-w-0 flex-col gap-1" data-testid="session-trail-card">
          <div className="line-clamp-2 text-transcript-body font-medium text-fg-strong">
            {entry.preview}
          </div>
          {entry.reply_preview ? (
            <div className="line-clamp-3 text-transcript-body text-muted">
              {entry.reply_preview}
            </div>
          ) : null}
          <div className="font-mono text-micro text-faint tabular-nums">
            {time ? `${time} · ` : ""}
            {ordinal} of {total}
          </div>
        </div>
      </TooltipContent>
    </Tooltip>
  );
}
