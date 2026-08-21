import { createElement } from "react";
import { ChevronDown, History } from "lucide-react";

import { Button, cn, Time } from "@compozy/ui";

import type { LoopStoryBeat } from "../../lib/loop-run-story-beats";
import { LOOP_STORY_ICONS } from "../../lib/loop-story-icons";
import { LoopSection } from "../loop-section";

interface LoopRunStoryProps {
  beats: LoopStoryBeat[];
  hasOlder: boolean;
  isLoadingOlder: boolean;
  onLoadOlder: () => void;
  isLoading: boolean;
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

export function LoopRunStory({
  beats,
  hasOlder,
  isLoadingOlder,
  onLoadOlder,
  isLoading,
}: LoopRunStoryProps) {
  return (
    <LoopSection
      className="mb-0"
      data-testid="loop-run-story"
      gist={beats.length > 0 ? "notable beats" : undefined}
      icon={<History aria-hidden="true" />}
      title="Story"
    >
      <div className="overflow-hidden rounded-lg border border-line bg-canvas-soft px-4.5 py-2">
        {beats.length === 0 ? (
          <p className="py-2 text-small-body text-muted" data-testid="loop-run-story-empty">
            {isLoading ? "Reading this run's story…" : "Nothing has happened in this run yet."}
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
