import { Target } from "lucide-react";
import { createContext, memo, use } from "react";
import type { ReactNode } from "react";
import { useSelector } from "@xstate/store-react";

import { Marker, MarkerMeta } from "@compozy/ui";
import { Link } from "@tanstack/react-router";
import { useAssistantMessageTimeline } from "./hooks/use-assistant-message-timeline";
import { usePrefersReducedMotion } from "./hooks/use-prefers-reduced-motion";
import {
  notifyDisclosureToggled,
  useOptionalThreadScrollStore,
} from "./hooks/thread-scroll-context";
import {
  useOptionalSessionNavigationTarget,
  useRevealHold,
} from "./hooks/session-navigation-target-context";
import {
  TimelineRowContext,
  toggleTimelineExpansion,
  useTimelineRowContext,
} from "./hooks/use-timeline-row-context";
import { SessionDataEventMarker, SessionMessageText } from "./session-message-parts";
import { SessionChangedFilesRowView } from "./session-changed-files-row";
import { SessionLiveToolRowView } from "./session-live-tool-row";
import { SessionWorkEntryView } from "./session-work-entry";
import { SessionToolGroupRow } from "./session-tool-group-row";
import { rowContainsPart, rowsContainPart } from "./session-timeline-reveal";
import { SessionTurnFoldRowView } from "./session-turn-fold-row";
import {
  type SessionChangedFilesRow,
  type SessionDataRow,
  type SessionLiveToolRow,
  type SessionReasoningRow,
  type SessionRow,
  type SessionTextRow,
  type SessionTurnFoldRow,
  type SessionWorkRow,
  sessionRowEqual,
} from "./session-timeline.logic";
import {
  ClarificationDataPart,
  type CompozyPermissionData,
  type GoalPromptMeta,
  isAgentEventPayload,
  isClarifyEventData,
  PermissionDataPart,
  RuntimeActivityNotice,
  ThinkingBlock,
} from "@/systems/session";

// Rows inside a failed turn's open fold know their turn failed, so the failed
// call earns the danger glyph (ADR-009); everywhere else a failure is absorbed.
const TurnFailedContext = createContext(false);

function isCompozyPermissionData(value: unknown): value is CompozyPermissionData {
  return isAgentEventPayload(value) && typeof value.request_id === "string";
}

// `data-part-index` names the projected part each row renders, so a find jump
// can land on the matched content rather than the message's top edge.
function SessionTextRowView({ row }: { row: SessionTextRow }) {
  return (
    <div className="contents" data-part-index={row.part.partIndex}>
      <SessionMessageText text={row.part.text} streaming={row.part.state === "running"} reveal />
    </div>
  );
}

function SessionReasoningRowView({ row }: { row: SessionReasoningRow }) {
  const hold = useRevealHold(
    row.id,
    reveal => reveal.partIndex !== null && rowContainsPart(row, reveal.partIndex)
  );
  return (
    <ThinkingBlock
      thinking={row.text}
      thinkingComplete={!row.streaming}
      partIndex={row.parts[0]?.partIndex}
      revealOpen={hold.held}
      onRevealRelease={hold.release}
    />
  );
}

function SessionDataRowView({ row }: { row: SessionDataRow }) {
  if (row.part.name === "data-compozy-event" && isClarifyEventData(row.part.data)) {
    return <ClarificationDataPart data={row.part.data} />;
  }
  if (row.part.name === "data-compozy-event" && isAgentEventPayload(row.part.data)) {
    return <RuntimeActivityNotice event={row.part.data} count={row.count} />;
  }
  if (row.part.name === "data-compozy-permission" && isCompozyPermissionData(row.part.data)) {
    return <PermissionDataPart data={row.part.data} />;
  }

  return <SessionDataEventMarker name={row.part.name} />;
}

/** Open mixed work for its exact search match while retaining the reader's disclosure state. */
function SessionWorkRowView({ row }: { row: SessionWorkRow }) {
  const store = useTimelineRowContext();
  const scrollStore = useOptionalThreadScrollStore();
  const turnFailed = use(TurnFailedContext);
  const hold = useRevealHold(
    row.id,
    reveal => reveal.partIndex !== null && rowContainsPart(row, reveal.partIndex)
  );
  if (row.summary) {
    return (
      <SessionToolGroupRow
        row={hold.held && !row.expanded ? { ...row, expanded: true } : row}
        turnFailed={turnFailed}
        onToggle={() => {
          notifyDisclosureToggled(scrollStore);
          if (hold.held) {
            hold.release();
            if (row.expanded) toggleTimelineExpansion(store, "work-group", row.groupId);
            return;
          }
          toggleTimelineExpansion(store, "work-group", row.groupId);
        }}
      />
    );
  }
  return (
    <div data-testid="work-row" className="flex min-w-0 flex-col gap-0.5">
      {row.entries.map(entry => (
        <SessionWorkEntryView
          key={`${entry.kind}:${entry.id}`}
          entry={entry}
          active={row.active}
          turnFailed={turnFailed}
        />
      ))}
    </div>
  );
}

/** Connects live tool rows to the thread's expansion and find-reveal state. */
function SessionLiveToolRowContent({ row }: { row: SessionLiveToolRow }) {
  const navigation = useOptionalSessionNavigationTarget();
  const store = useTimelineRowContext();
  const scrollStore = useOptionalThreadScrollStore();
  const reducedMotion = usePrefersReducedMotion();
  const hold = useRevealHold(
    row.id,
    reveal => reveal.partIndex !== null && rowContainsPart(row, reveal.partIndex)
  );
  return (
    <SessionLiveToolRowView
      row={hold.held && !row.expanded ? { ...row, expanded: true } : row}
      reducedMotion={reducedMotion}
      reveal={hold.held ? navigation?.reveal : null}
      onToggle={() => {
        notifyDisclosureToggled(scrollStore);
        if (hold.held) {
          hold.release();
          if (row.expanded) toggleTimelineExpansion(store, "work-group", row.id);
          return;
        }
        toggleTimelineExpansion(store, "work-group", row.id);
      }}
    />
  );
}

function SessionChangedFilesRowContent({ row }: { row: SessionChangedFilesRow }) {
  const store = useTimelineRowContext();
  const scrollStore = useOptionalThreadScrollStore();
  return (
    <SessionChangedFilesRowView
      row={row}
      onToggle={() => {
        notifyDisclosureToggled(scrollStore);
        toggleTimelineExpansion(store, "changed-files", row.id);
      }}
    />
  );
}

// A find jump opens the fold the matched part sits behind ("opened for a
// match"): the exact part when the daemon located it, the whole turn when an
// older daemon named only the turn. The reader closes it by hand with one
// click, which also releases the jump's hold. While closed, the fold says how
// many matches the daemon found inside it.
function SessionTurnFoldRowContent({ row }: { row: SessionTurnFoldRow }) {
  const store = useTimelineRowContext();
  const scrollStore = useOptionalThreadScrollStore();
  const navigation = useOptionalSessionNavigationTarget();
  const turnId = row.turnId ?? row.id;
  const expandedByReader = useSelector(store, state => state.context.expandedTurns.has(turnId));
  const hold = useRevealHold(
    row.id,
    reveal =>
      reveal.turnId === turnId &&
      (reveal.partIndex === null || rowsContainPart(row.rows, reveal.partIndex))
  );
  const openedForMatch = hold.held;
  const expanded = expandedByReader || openedForMatch;
  const matchesInside = navigation?.foldMatchCounts.get(turnId) ?? 0;
  const note = openedForMatch
    ? "opened for a match"
    : !expanded && matchesInside > 0
      ? `${matchesInside} ${matchesInside === 1 ? "match" : "matches"} inside`
      : null;
  return (
    <TurnFailedContext.Provider value={row.cause === "failed"}>
      <SessionTurnFoldRowView
        row={row}
        expanded={expanded}
        note={note}
        onToggle={() => {
          notifyDisclosureToggled(scrollStore);
          if (openedForMatch) {
            hold.release();
            if (expandedByReader) toggleTimelineExpansion(store, "turn", turnId);
            return;
          }
          toggleTimelineExpansion(store, "turn", turnId);
        }}
      >
        {renderTimelineRows(
          row.rows.map(nested => (nested.kind === "work" ? { ...nested, summary: null } : nested))
        )}
      </SessionTurnFoldRowView>
    </TurnFailedContext.Provider>
  );
}

// Semantic row equality keeps settled subtrees out of streaming updates without
// storing a second derived row list. Interactive variants select their expansion
// state from the stable store handle in `TimelineRowContext`.
const TimelineRowContent = memo(
  function TimelineRowContent({ row }: { row: SessionRow }) {
    switch (row.kind) {
      case "text":
        return <SessionTextRowView row={row} />;
      case "reasoning":
        return <SessionReasoningRowView row={row} />;
      case "data":
        return <SessionDataRowView row={row} />;
      case "working":
        // The one live status line lives under the scroller (SessionThinkingRow);
        // the working part only keeps the live turn from folding.
        return null;
      case "work":
        return <SessionWorkRowView row={row} />;
      case "live-tool":
        return <SessionLiveToolRowContent row={row} />;
      case "changed-files":
        return <SessionChangedFilesRowContent row={row} />;
      case "turn-fold":
        return <SessionTurnFoldRowContent row={row} />;
    }
  },
  (previous, next) => sessionRowEqual(previous.row, next.row)
);

/**
 * Referentially stable renderer: no closure deps, so re-deriving the row list
 * never re-creates the mapping function. Row identity carries the change signal.
 */
function renderTimelineRows(rows: readonly SessionRow[]): ReactNode {
  return rows.map(row => <TimelineRowContent key={row.id} row={row} />);
}

const GOAL_PROMPT_LABELS: Record<GoalPromptMeta["kind"], string> = {
  "goal-work": "Goal work",
  "goal-continuation": "Goal continuation",
  "goal-compaction": "Goal compaction",
};

// `.marker--goal` — the goal prompt as one quiet marker line, mono meta for
// the node/generation facts, the run link in `--info`.
function GoalPromptNotice({ goal }: { goal: GoalPromptMeta }) {
  const turn = goal.turn === null ? "" : ` · turn ${goal.turn}`;
  return (
    <Marker data-testid="goal-prompt-meta" tone="info" icon={<Target strokeWidth={1.8} />}>
      <b>{GOAL_PROMPT_LABELS[goal.kind]}</b>{" "}
      <MarkerMeta>
        {goal.node_id} · generation {goal.generation}
        {turn}
      </MarkerMeta>{" "}
      <Link
        className="text-info transition-colors hover:underline hover:underline-offset-2"
        params={{ runId: goal.run_id }}
        to="/loop-runs/$runId"
      >
        Open run
      </Link>
    </Marker>
  );
}

export function AssistantMessageTimeline({
  queueTraceCount = null,
}: {
  queueTraceCount?: number | null;
}) {
  const { goal, rows, timelineStore } = useAssistantMessageTimeline();

  return (
    <TimelineRowContext.Provider value={timelineStore}>
      {goal ? <GoalPromptNotice goal={goal} /> : null}
      {renderTimelineRows(
        queueTraceCount === null
          ? rows
          : rows.map(row => (row.kind === "data" ? { ...row, count: queueTraceCount } : row))
      )}
    </TimelineRowContext.Provider>
  );
}
