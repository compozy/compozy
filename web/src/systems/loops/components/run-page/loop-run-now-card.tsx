import type { ReactNode } from "react";
import { ArrowUpRight } from "lucide-react";
import { Link } from "@tanstack/react-router";

import { formatRelativeTime, LiveBadge } from "@compozy/ui";

import type { LoopNodeNowLine } from "../../lib/loop-node-now-view";
import type { LoopNodeLifecycle } from "../../lib/loop-node-lifecycle";
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
  /**
   * Per-node lifecycle lines (retrying / paused / waiting / quarantined /
   * canceling) derived from daemon truth. Each renders as its own row inside
   * this card's anatomy — the card is the one "what is happening" surface.
   */
  nodeLines?: readonly LoopNodeNowLine[];
  /** Lifecycle rows keyed by node id, so a line can host its verb menu. */
  nodesById?: ReadonlyMap<string, LoopNodeLifecycle>;
  /** Renders the verb menu for one lifecycle row; omitted in read-only fixtures. */
  renderNodeActions?: (node: LoopNodeLifecycle) => ReactNode;
  /** ACP session per node; rows link straight into the live agent session. */
  nodeSessions?: ReadonlyMap<string, string>;
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

/** Dot treatment per lifecycle state — signal colour marks state, never category. */
const NODE_LINE_DOT: Record<string, string> = {
  retrying: "bg-warning",
  paused: "border-[1.5px] border-neutral bg-transparent",
  waiting: "bg-info",
  quarantined: "bg-danger",
  requested: "bg-warning",
  delivering: "bg-warning",
  draining: "bg-warning",
};

/** One lifecycle row: headline, sentence, optional chip + provenance, mono trail. */
function NodeNowRow({
  line,
  actions,
  sessionId,
  withDivider,
}: {
  line: LoopNodeNowLine;
  actions?: ReactNode;
  sessionId?: string;
  withDivider: boolean;
}) {
  return (
    <div
      className={`flex items-start gap-3 px-4 py-3.5 ${withDivider ? "border-t border-line-soft" : ""}`}
      data-testid={`loop-run-now-node-${line.nodeId}`}
    >
      <span
        aria-hidden="true"
        className={`mt-1.5 size-2 shrink-0 rounded-full ${NODE_LINE_DOT[line.state] ?? "bg-neutral"}`}
      />
      <div className="min-w-0 flex-1">
        <div className="text-ws-name font-medium text-fg-strong">{line.headline}</div>
        <div className="mt-0.75 max-w-[60ch] text-small-body leading-relaxed text-muted">
          {line.body}
        </div>
        {line.chip ? (
          <span
            className="mt-2 inline-flex items-center rounded bg-warning-tint px-1.5 py-0.5 font-mono text-mono-id text-warning"
            data-testid={`loop-run-now-chip-${line.nodeId}`}
          >
            {line.chip}
          </span>
        ) : null}
        {line.provenance ? (
          <div className="mt-2 font-mono text-mono-id text-subtle">{line.provenance}</div>
        ) : null}
        {sessionId ? (
          <Link
            className={NOW_OUTBOUND_LINK_CLASS}
            data-testid={`loop-run-now-node-session-${line.nodeId}`}
            params={{ id: sessionId }}
            to="/session/$id"
          >
            <NowOutboundLinkContent label="Open session" />
          </Link>
        ) : null}
        <div className="mt-1.5 font-mono text-pill-group-badge text-faint">{line.micro}</div>
      </div>
      <span className="flex shrink-0 items-center gap-2 pt-0.5">
        <span className="font-mono text-mono-id whitespace-nowrap text-subtle">{line.state}</span>
        {actions}
      </span>
    </div>
  );
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
  const { run, now, isLive, children, nodeLines, nodesById, renderNodeActions, nodeSessions } =
    props;
  const view = nowView(props);
  const lines = nodeLines ?? [];
  const nowSessionId = now ? nodeSessions?.get(now.nodeId) : undefined;
  // The card exists when the run has something to say OR when at least one node
  // declares a lifecycle state — a run whose only story is "task_03 is paused"
  // still needs this surface.
  if (!view && lines.length === 0) return null;
  const showLive = isLive && (run.status === "running" || run.status === "watching");
  return (
    <LoopRunSection
      label="Happening now"
      right={showLive ? <LiveBadge /> : undefined}
      data-testid="loop-run-now"
    >
      <div className="overflow-hidden rounded-lg border border-line bg-canvas-soft">
        {view ? (
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
              {run.status === "running" && nowSessionId ? (
                <Link
                  className={`${NOW_OUTBOUND_LINK_CLASS} mr-4`}
                  data-testid="loop-run-now-session-link"
                  params={{ id: nowSessionId }}
                  to="/session/$id"
                >
                  <NowOutboundLinkContent label="Open session" />
                </Link>
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
        ) : null}
        {lines.map((line, index) => {
          const node = nodesById?.get(line.nodeId);
          return (
            <NodeNowRow
              actions={node && renderNodeActions ? renderNodeActions(node) : undefined}
              key={line.nodeId}
              line={line}
              sessionId={nodeSessions?.get(line.nodeId)}
              withDivider={view !== null || index > 0}
            />
          );
        })}
      </div>
    </LoopRunSection>
  );
}
