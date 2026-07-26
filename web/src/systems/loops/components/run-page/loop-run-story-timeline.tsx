import type { ComponentType, ReactNode } from "react";
import { Activity, Bell, Check, Eye, Pause, Play, Square, TriangleAlert, X } from "lucide-react";

import { cn, Empty, formatAbsoluteTime, formatRelativeTime, type PillTone } from "@agh/ui";

import type { GoalTurnTimelineItem } from "../../hooks/use-goal-turns";
import type { LoopStoryIcon, LoopStoryRow } from "../../lib/loop-run-story";
import { LoopRunSection } from "./loop-run-section";
import { LoopRunTurnsDisclosure } from "./loop-run-turns-disclosure";

interface LoopRunStoryTimelineProps {
  rows: LoopStoryRow[];
  isLive: boolean;
  /** Goal-node ids from the pinned graph; their newest row hosts the Turns disclosure. */
  goalNodeIds: ReadonlySet<string>;
  goalTurns: readonly GoalTurnTimelineItem[];
  hasMoreGoalTurns?: boolean;
  isLoadingMoreGoalTurns?: boolean;
  onLoadMoreGoalTurns?: () => void;
}

type IconComponent = ComponentType<{ className?: string; "aria-hidden"?: boolean | "true" }>;

const ICONS: Record<LoopStoryIcon, IconComponent> = {
  round: Play,
  "check-pass": Check,
  "check-warn": TriangleAlert,
  "node-done": Check,
  "node-failed": X,
  approval: Bell,
  watching: Eye,
  paused: Pause,
  resumed: Play,
  started: Play,
  stopped: Square,
  done: Check,
};

/** Story dots color state only (§1): neutral rows stay on the faint ring. */
const TONE_RING: Record<PillTone, string> = {
  neutral: "border-faint text-faint",
  accent: "border-accent text-accent",
  success: "border-success text-success",
  warning: "border-warning text-warning",
  danger: "border-danger text-danger",
  info: "border-info text-info",
};

function StoryIssues({ row }: { row: LoopStoryRow }) {
  if (!row.issues || row.issues.length === 0) return null;
  return (
    <span data-testid="loop-story-issues">
      {row.issues.map((issue, index) => (
        <span key={issue.id}>
          {index > 0 ? " · " : " "}
          <span className="font-mono text-mono-id text-subtle">{issue.id}</span>
          {issue.note ? ` — ${issue.note}` : null}
        </span>
      ))}
    </span>
  );
}

function StoryRow({ row, turnsSlot }: { row: LoopStoryRow; turnsSlot: ReactNode }) {
  const Icon = ICONS[row.icon];
  return (
    <div
      className={cn(
        "relative grid grid-cols-[22px_minmax(0,1fr)_auto] items-start gap-3 py-2.25",
        "before:absolute before:top-0 before:bottom-0 before:left-[10.5px] before:w-px",
        "before:bg-line-strong first:before:top-3.5 last:before:bottom-auto last:before:h-3.5"
      )}
      data-kind={row.kind}
      data-testid="loop-story-row"
    >
      <span
        aria-hidden="true"
        className={`relative z-[1] grid size-5.5 place-items-center rounded-full border-[1.5px] bg-canvas ${TONE_RING[row.tone]}`}
      >
        <Icon aria-hidden className="size-2.75" />
      </span>
      <div className="min-w-0">
        <div className="pt-0.5 text-ws-name font-medium text-fg-strong">{row.title}</div>
        {row.sub || (row.issues && row.issues.length > 0) ? (
          <div className="mt-0.5 max-w-[60ch] text-small-body leading-normal text-muted">
            {row.sub}
            <StoryIssues row={row} />
          </div>
        ) : null}
        {turnsSlot}
      </div>
      <div className="flex flex-col items-end gap-0.75 pt-0.75">
        <time
          className="text-form-hint whitespace-nowrap tabular-nums text-subtle"
          dateTime={row.at}
          title={formatAbsoluteTime(row.at)}
        >
          {formatRelativeTime(row.at)}
        </time>
        <span className="font-mono text-pill-group-badge whitespace-nowrap text-faint">
          {row.micro}
        </span>
      </div>
    </div>
  );
}

/**
 * The "What happened" story timeline (§5.3): flat, newest-first, one row per
 * meaningful frame on the prototype's icon spine. Goal-node rows fold the
 * `/turns` audit behind a Turns disclosure on their newest appearance.
 */
export function LoopRunStoryTimeline({
  rows,
  isLive,
  goalNodeIds,
  goalTurns,
  hasMoreGoalTurns = false,
  isLoadingMoreGoalTurns = false,
  onLoadMoreGoalTurns,
}: LoopRunStoryTimelineProps) {
  // Pure pre-pass: the newest row of each goal node hosts the Turns disclosure.
  const turnsHostKeys = new Set<string>();
  const seenGoalNodes = new Set<string>();
  for (const row of rows) {
    if (row.nodeId !== undefined && goalNodeIds.has(row.nodeId) && !seenGoalNodes.has(row.nodeId)) {
      seenGoalNodes.add(row.nodeId);
      turnsHostKeys.add(row.key);
    }
  }
  return (
    <LoopRunSection label="What happened" data-testid="loop-run-story">
      <div className="overflow-hidden rounded-lg border border-line bg-canvas-soft">
        {rows.length === 0 ? (
          <Empty
            className="py-10"
            description="Story rows appear here as the run works."
            icon={Activity}
            title="Nothing yet"
          />
        ) : (
          <div className="px-4 pt-1.5 pb-2.5">
            {rows.map(row => (
              <StoryRow
                key={row.key}
                row={row}
                turnsSlot={
                  turnsHostKeys.has(row.key) ? (
                    <LoopRunTurnsDisclosure
                      hasMore={hasMoreGoalTurns}
                      isLoadingMore={isLoadingMoreGoalTurns}
                      isLive={isLive}
                      onLoadMore={onLoadMoreGoalTurns}
                      turns={goalTurns.filter(turn => turn.nodeId === row.nodeId)}
                    />
                  ) : null
                }
              />
            ))}
          </div>
        )}
      </div>
    </LoopRunSection>
  );
}
