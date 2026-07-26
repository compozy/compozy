import type { ReactNode } from "react";
import { ArrowUpRight } from "lucide-react";
import { Link } from "@tanstack/react-router";

import { formatRelativeTime, LiveBadge } from "@agh/ui";

import type { LoopStoryNow } from "../../lib/loop-run-story";
import type { LoopRunRecord } from "../../types";
import { LoopRunSection } from "./loop-run-section";

interface LoopRunNowCardProps {
  run: LoopRunRecord;
  /** The newest running step from the story adapter; null when nothing runs. */
  now: LoopStoryNow | null;
  /** Parked watch read-model wake stamp (`watch_events.last_wake_at`). */
  watchLastWakeAt?: string;
  /** Poll cadence clause from the watch spec (`checks every 30s`). */
  watchCadence?: string | null;
  /** Ticking elapsed of the current step; null hides the trail clock. */
  stepElapsedLabel?: string | null;
  isLive: boolean;
  /** Turns disclosure slot when the running node is a goal node. */
  children?: ReactNode;
}

interface NowView {
  dotClass: string;
  pulse: boolean;
  label: string;
  sub?: string;
  trail?: string | null;
}

function runningView(
  run: LoopRunRecord,
  now: LoopStoryNow | null,
  stepElapsed?: string | null
): NowView {
  if (!now) {
    return {
      dotClass: "bg-accent",
      pulse: true,
      label: `Round ${Math.max(run.generation, 1)} in progress`,
    };
  }
  const redo =
    run.reattempt_strategy === "failed_only" && now.generation > 1
      ? " — redoing the failed group."
      : ".";
  const child = now.childRunId ? " Waiting on a child run to finish." : "";
  return {
    dotClass: "bg-accent",
    pulse: true,
    label: `Working on ${now.label}`,
    sub: `Round ${now.generation}${redo}${child}`,
    trail: stepElapsed,
  };
}

function watchingView(lastWakeAt?: string, cadence?: string | null): NowView {
  const woke = lastWakeAt ? ` — last woke ${formatRelativeTime(lastWakeAt)}` : "";
  return {
    dotClass: "bg-info",
    pulse: true,
    label: "Watching for the next event",
    sub: `The run wakes when a matching event arrives${woke}.`,
    trail: cadence,
  };
}

function pausedView(run: LoopRunRecord): NowView {
  return {
    dotClass: "border-[1.5px] border-neutral bg-transparent",
    pulse: false,
    label: "Paused",
    sub:
      `Paused by you ${formatRelativeTime(run.last_progress_at)} at a safe point. Nothing is ` +
      `running — Resume continues round ${Math.max(run.generation, 1)} exactly where it left off.`,
  };
}

const QUEUED_VIEW: NowView = {
  dotClass: "bg-neutral",
  pulse: false,
  label: "Waiting for a slot",
  sub: "This loop runs one at a time; the run starts when the current one finishes.",
};

function nowView(props: LoopRunNowCardProps): NowView | null {
  switch (props.run.status) {
    case "running":
      return runningView(props.run, props.now, props.stepElapsedLabel);
    case "watching":
      return watchingView(props.watchLastWakeAt, props.watchCadence);
    case "paused":
      return pausedView(props.run);
    case "queued":
      return QUEUED_VIEW;
    default:
      return null;
  }
}

/** Shared chrome for the "Happening now" outbound child-run / task-run links. */
const NOW_OUTBOUND_LINK_CLASS =
  "mt-2.25 inline-flex items-center gap-1.25 text-badge font-medium text-muted hover:text-fg-strong";

/** Icon + label body shared by both outbound links; the route/testid stay at each call site. */
function NowOutboundLinkContent({ label }: { label: string }) {
  return (
    <>
      <ArrowUpRight aria-hidden="true" className="size-3" />
      {label}
    </>
  );
}

/**
 * The "Happening now" card (§2/§7): the live step while running, the watching
 * card while parked on events, the paused explainer, and the queued slot note.
 * Every rendered value traces to the run projection or streamed frames.
 */
export function LoopRunNowCard(props: LoopRunNowCardProps) {
  const { run, now, isLive, children } = props;
  const view = nowView(props);
  if (!view) return null;
  const showLive = isLive && (run.status === "running" || run.status === "watching");
  return (
    <LoopRunSection
      label="Happening now"
      right={showLive ? <LiveBadge /> : undefined}
      data-testid="loop-run-now"
    >
      <div className="overflow-hidden rounded-lg border border-line bg-canvas-soft">
        <div className="flex items-start gap-3 px-4 py-3.5">
          <span
            aria-hidden="true"
            className={`mt-1.5 size-2 shrink-0 rounded-full ${view.dotClass} ${
              view.pulse ? "animate-pulse motion-reduce:animate-none" : ""
            }`}
          />
          <div className="min-w-0 flex-1">
            <div
              className="text-ws-name font-medium text-fg-strong"
              data-testid="loop-run-now-label"
            >
              {view.label}
            </div>
            {view.sub ? (
              <div className="mt-0.75 max-w-[60ch] text-small-body leading-relaxed text-muted">
                {view.sub}
              </div>
            ) : null}
            {run.status === "running" && now?.childRunId ? (
              <Link
                className={NOW_OUTBOUND_LINK_CLASS}
                data-testid="loop-run-now-child-link"
                params={{ runId: now.childRunId }}
                to="/loop-runs/$runId"
              >
                <NowOutboundLinkContent label="View child run" />
              </Link>
            ) : run.status === "running" && now?.taskLink ? (
              <Link
                className={NOW_OUTBOUND_LINK_CLASS}
                data-testid="loop-run-now-task-link"
                params={{ id: now.taskLink.taskId, runId: now.taskLink.taskRunId }}
                to="/tasks/$id/runs/$runId"
              >
                <NowOutboundLinkContent label="View task run" />
              </Link>
            ) : null}
            {children}
          </div>
          {view.trail ? (
            <span className="pt-0.75 font-mono text-mono-id whitespace-nowrap tabular-nums text-subtle">
              {view.trail}
            </span>
          ) : null}
        </div>
      </div>
    </LoopRunSection>
  );
}
