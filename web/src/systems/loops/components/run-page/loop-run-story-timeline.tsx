import { useEffect, useState, type ReactNode } from "react";
import { Activity, History } from "lucide-react";

import {
  cn,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Empty,
  formatAbsoluteTime,
  formatRelativeTime,
  Pill,
  type PillTone,
} from "@compozy/ui";

import type { GoalTurnTimelineItem } from "../../hooks/use-goal-turns";
import { formatLoopScore } from "../../lib/loop-generation-presentation";
import type { LoopStoryRow } from "../../lib/loop-run-story";
import { LOOP_STORY_ICONS } from "../../lib/loop-story-icons";
import { LoopSection } from "../loop-section";
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
  const Icon = LOOP_STORY_ICONS[row.icon];
  const score = formatLoopScore(row.score);
  const anchorId =
    row.kind === "generation_started" && row.generation !== undefined
      ? `loop-generation-${row.generation}`
      : undefined;
  return (
    <div
      className={cn(
        "relative grid grid-cols-[22px_minmax(0,1fr)_auto] items-start gap-3 py-2.25",
        "before:absolute before:top-0 before:bottom-0 before:left-[10.5px] before:w-px",
        "before:bg-line-strong first:before:top-3.5 last:before:bottom-auto last:before:h-3.5"
      )}
      data-kind={row.kind}
      data-testid="loop-story-row"
      id={anchorId}
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
        {row.originLabel || score || row.isBest ? (
          <div
            className="mt-1.5 flex flex-wrap items-center gap-1.5"
            data-testid="loop-story-metadata"
          >
            {row.originLabel ? (
              <Pill size="xs" tone={row.originTone}>
                {row.originLabel}
              </Pill>
            ) : null}
            {score ? (
              <Pill mono size="xs" tone="neutral">
                score {score}
              </Pill>
            ) : null}
            {row.isBest ? (
              <Pill size="xs" tone="success">
                Best
              </Pill>
            ) : null}
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

function readTargetedGeneration(): number | null {
  if (typeof window === "undefined") return null;
  const match = /^#loop-generation-(\d+)$/.exec(window.location.hash);
  return match ? Number(match[1]) : null;
}

function partitionStoryGenerations(rows: readonly LoopStoryRow[]): {
  newest: LoopStoryRow[];
  older: { generation: number; rows: LoopStoryRow[] }[];
} {
  let newestGen = Number.NEGATIVE_INFINITY;
  for (const row of rows) {
    if (row.generation !== undefined && row.generation > newestGen) newestGen = row.generation;
  }
  const newest: LoopStoryRow[] = [];
  const olderMap = new Map<number, LoopStoryRow[]>();
  for (const row of rows) {
    const generation = row.generation;
    if (generation === undefined || generation === newestGen) {
      newest.push(row);
      continue;
    }
    const list = olderMap.get(generation) ?? [];
    list.push(row);
    olderMap.set(generation, list);
  }
  const older = [...olderMap.entries()]
    .sort(([left], [right]) => right - left)
    .map(([generation, groupRows]) => ({ generation, rows: groupRows }));
  return { newest, older };
}

/**
 * The "What happened" story timeline (§5.3): newest generation flat, older
 * generations folded. Goal-node rows fold the `/turns` audit behind a Turns
 * disclosure on their newest appearance.
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
  const { newest, older } = partitionStoryGenerations(rows);
  const [openGens, setOpenGens] = useState(() => {
    const targeted = readTargetedGeneration();
    return targeted === null ? new Set<number>() : new Set([targeted]);
  });
  useEffect(() => {
    const sync = () => {
      const targeted = readTargetedGeneration();
      if (targeted === null) return;
      setOpenGens(current => {
        if (current.has(targeted)) return current;
        const next = new Set(current);
        next.add(targeted);
        return next;
      });
    };
    window.addEventListener("hashchange", sync);
    return () => window.removeEventListener("hashchange", sync);
  }, []);

  const renderRow = (row: LoopStoryRow) => (
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
  );

  return (
    <LoopSection
      className="mb-0"
      data-testid="loop-run-story"
      gist={rows.length === 0 ? undefined : `${rows.length} events`}
      icon={<History aria-hidden="true" />}
      title="What happened"
    >
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
            {newest.map(renderRow)}
            {older.map(group => (
              <Collapsible
                className="border-t border-line-soft"
                data-testid={`loop-story-generation-${group.generation}`}
                key={group.generation}
                open={openGens.has(group.generation)}
                onOpenChange={open => {
                  setOpenGens(current => {
                    const next = new Set(current);
                    if (open) next.add(group.generation);
                    else next.delete(group.generation);
                    return next;
                  });
                }}
              >
                <CollapsibleTrigger className="flex min-h-8 w-full items-center py-2 text-left font-mono text-pill-group-badge text-subtle">
                  Generation {group.generation} · {group.rows.length}{" "}
                  {group.rows.length === 1 ? "event" : "events"}
                </CollapsibleTrigger>
                <CollapsibleContent>{group.rows.map(renderRow)}</CollapsibleContent>
              </Collapsible>
            ))}
          </div>
        )}
      </div>
    </LoopSection>
  );
}
