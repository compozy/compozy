import { createElement } from "react";
import { ChevronDown, History, PlugZap, TriangleAlert } from "lucide-react";

import { Link } from "@tanstack/react-router";

import { Button, cn, Pill, Time } from "@compozy/ui";

import type { LoopStoryBeat } from "../../lib/loop-run-story-beats";
import { LOOP_STORY_ICONS } from "../../lib/loop-story-icons";
import { LoopSection } from "../loop-section";

/**
 * Backward paging over the durable story; the live tail arrives by stream.
 *
 * One object rather than four loose flags: they describe a single read, they are
 * only ever meaningful together, and half of their sixteen combinations are
 * states the timeline query cannot produce.
 */
export interface LoopRunStoryPaging {
  hasOlder: boolean;
  isLoading: boolean;
  /**
   * True when the timeline read itself failed. Distinct from an empty story:
   * one means the run has done nothing, the other means we cannot say what it
   * has done, and printing the first sentence for the second is a lie.
   */
  isError?: boolean;
  isLoadingOlder: boolean;
  onLoadOlder: () => void;
}

interface LoopRunStoryProps {
  beats: readonly LoopStoryBeat[];
  paging: LoopRunStoryPaging;
  /**
   * The stream dropped, so the beats below are the last reconciled read.
   *
   * The register already says this about itself; the story is a default-read
   * element and a reader who never opens Inspect would otherwise watch a live
   * run quietly stop producing beats with nothing on screen to explain it.
   */
  isReconnecting?: boolean;
}

/**
 * The run's story, from the durable timeline.
 *
 * History used to live in a 500-frame client buffer, which meant a reload lost
 * the beginning of any run long enough to be worth reading about. It now pages
 * from the daemon: the newest window arrives first so the page is useful
 * immediately, and older history loads backward on demand, fenced to a snapshot
 * so appends never shift the ground under a reader.
 */
const TONE_RING = {
  neutral: "ring-line text-muted",
  accent: "ring-accent text-accent",
  success: "ring-success text-success",
  warning: "ring-warning text-warning",
  danger: "ring-danger text-danger",
  info: "ring-info text-info",
} as const;

function LoopRunStoryBeat({ beat }: { beat: LoopStoryBeat }) {
  const icon = LOOP_STORY_ICONS[beat.icon];
  return (
    <li
      className="flex items-start gap-2.5 py-2.25"
      data-kind={beat.kind}
      data-seq={beat.seq}
      data-testid={`loop-run-beat-${beat.seq}`}
    >
      <span
        className={cn(
          "mt-0.5 flex size-5.5 shrink-0 items-center justify-center rounded-full ring-1.5",
          TONE_RING[beat.tone]
        )}
      >
        {createElement(icon, { "aria-hidden": true, className: "size-2.75" })}
      </span>
      <div className="min-w-0 flex-1">
        <p className="text-small-body text-pretty text-fg-strong" data-testid="loop-run-beat-title">
          {beat.title}
          {/* A fork point that names the related run but cannot be opened is
              half a record. The link only appears when the run's own lineage
              resolved the other side, so it never points at a guess
              (US-009.EC-3). */}
          {beat.relatedRunId ? (
            <Link
              className="ml-1.5 font-mono text-mono-id text-info hover:text-fg-strong"
              data-testid={`loop-run-beat-related-${beat.seq}`}
              params={{ runId: beat.relatedRunId }}
              to="/loop-runs/$runId"
            >
              {beat.relatedRunId}
            </Link>
          ) : null}
          {beat.count > 1 ? (
            // A folded run of machinery events. The count is the span the daemon
            // coalesced, so resuming after it replays none of them.
            <span className="ml-1.5 font-mono text-mono-id text-faint">×{beat.count}</span>
          ) : null}
        </p>
      </div>
      <div className="flex shrink-0 items-center gap-2 text-form-hint text-faint">
        {beat.attemptLabel ? <span className="font-mono">{beat.attemptLabel}</span> : null}
        <Time iso={beat.at} />
      </div>
    </li>
  );
}

export function LoopRunStory({ beats, paging, isReconnecting = false }: LoopRunStoryProps) {
  const { hasOlder, isLoading, isError = false, isLoadingOlder, onLoadOlder } = paging;
  // Three different silences, three different sentences. A run that has done
  // nothing, a read still in flight, and a read that failed all show an empty
  // list — only the last one means the history is unknown rather than absent,
  // and the reader has to be told which one they are looking at.
  const emptyLine = isError
    ? "This run's story could not be read. The run itself is unaffected."
    : isLoading
      ? "Reading this run's story…"
      : "Nothing has happened in this run yet.";
  return (
    <LoopSection
      className="mb-0"
      data-testid="loop-run-story"
      gist={beats.length > 0 ? "notable beats" : undefined}
      icon={<History aria-hidden="true" />}
      right={
        // Stale is never painted as current. A failed read is the more specific
        // fact and outranks a reconnect, exactly as it does on the runs roster.
        isError ? (
          <Pill data-testid="loop-run-story-degraded" tone="danger">
            <TriangleAlert aria-hidden="true" className="size-3" />
            unread
          </Pill>
        ) : isReconnecting ? (
          <Pill data-testid="loop-run-story-reconnecting" tone="warning">
            <PlugZap aria-hidden="true" className="size-3" />
            reconnecting
          </Pill>
        ) : null
      }
      title="Story"
    >
      <div className="overflow-hidden rounded-lg border border-line bg-canvas-soft px-4.5 py-2">
        {beats.length === 0 ? (
          <p
            className={cn("py-2 text-small-body", isError ? "text-danger" : "text-muted")}
            data-state={isError ? "error" : isLoading ? "loading" : "empty"}
            data-testid="loop-run-story-empty"
          >
            {emptyLine}
          </p>
        ) : (
          <ul className="flex flex-col divide-y divide-line-soft">
            {beats.map(beat => (
              <LoopRunStoryBeat beat={beat} key={beat.key} />
            ))}
          </ul>
        )}
        {hasOlder ? (
          <div className="border-t border-line-soft pt-2 pb-1">
            <Button
              className="w-full justify-center"
              data-testid="loop-run-story-load-older"
              disabled={isLoadingOlder}
              onClick={onLoadOlder}
              size="sm"
              type="button"
              variant="ghost"
            >
              <ChevronDown aria-hidden="true" />
              {isLoadingOlder ? "Loading earlier beats…" : "Load earlier beats"}
            </Button>
          </div>
        ) : null}
      </div>
    </LoopSection>
  );
}
