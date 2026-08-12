import { CircleStop, Target } from "lucide-react";
import { createElement, memo } from "react";
import type { ReactNode } from "react";
import { useSelector } from "@xstate/store-react";

import { cn } from "@/lib/utils";

import { getToolIcon, resolveRegisteredToolName } from "@/systems/session/lib/tool-labels";
import { Marker, MarkerMeta } from "@compozy/ui";
import { Link } from "@tanstack/react-router";
import { useAssistantMessageTimeline } from "./hooks/use-assistant-message-timeline";
import {
  TimelineRowContext,
  toggleTimelineExpansion,
  useTimelineRowContext,
} from "./hooks/use-timeline-row-context";
import { SessionDataEventMarker, SessionMessageText } from "./session-message-parts";
import { SessionChangedFilesRowView } from "./session-changed-files-row";
import { SessionWorkingRowView } from "./session-working-row";
import { TranscriptDisclosure } from "./transcript-disclosure";
import {
  type SessionChangedFilesRow,
  type SessionDataRow,
  type SessionReasoningRow,
  type SessionRow,
  type SessionTextRow,
  type SessionTimelineToolPart,
  type SessionToolGroupSummary,
  type SessionTurnFoldRow,
  type SessionWorkRow,
  type SessionWorkToggleRow,
  sessionRowEqual,
  visibleWorkEntries,
} from "./session-timeline.logic";
import {
  ClarificationDataPart,
  type CompozyPermissionData,
  type GoalPromptMeta,
  isAgentEventPayload,
  isClarifyEventData,
  PermissionDataPart,
  resolveToolResult,
  RuntimeActivityNotice,
  SessionToolCallRow,
  ThinkingBlock,
  type UIMessage,
} from "@/systems/session";

function isCompozyPermissionData(value: unknown): value is CompozyPermissionData {
  return isAgentEventPayload(value) && typeof value.request_id === "string";
}

function SessionTextRowView({ row }: { row: SessionTextRow }) {
  return <SessionMessageText text={row.part.text} streaming={row.part.state === "running"} />;
}

function SessionReasoningRowView({ row }: { row: SessionReasoningRow }) {
  return <ThinkingBlock thinking={row.text} thinkingComplete={!row.streaming} />;
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

function toolMessageFromPart(part: SessionTimelineToolPart): UIMessage {
  return {
    id: part.toolCallId,
    role: part.result !== undefined || part.isError ? "tool_result" : "tool_call",
    content: "",
    toolName: part.toolName,
    toolInput: part.args,
    toolResult: resolveToolResult(part.result),
    toolError: part.isError,
    isStreaming: part.status === "running",
    timestamp: part.timestamp ? Date.parse(part.timestamp) : Date.now(),
  };
}

function workToggleLabel(row: SessionWorkToggleRow): string {
  if (row.expanded) {
    return "Show fewer tool calls";
  }
  return `+${row.hiddenCount} previous tool call${row.hiddenCount === 1 ? "" : "s"}`;
}

// `.tmore` — bare-text overflow toggle above the visible live tail.
function WorkToggleButton({ row, onToggle }: { row: SessionWorkToggleRow; onToggle: () => void }) {
  return (
    <button
      type="button"
      data-testid="work-toggle-row"
      aria-expanded={row.expanded}
      onClick={onToggle}
      className={cn(
        "ml-transcript-detail-indent inline-flex min-h-transcript-row w-fit items-center gap-1.5 rounded-xs px-1",
        "text-transcript-meta text-subtle transition-colors duration-base ease-out hover:text-fg",
        "focus-visible:shadow-focus-ring focus-visible:outline-none"
      )}
    >
      {workToggleLabel(row)}
    </button>
  );
}

// `.tgroup-sum` — a settled run resting as one semantic summary line; the
// first entry's icon leads, the chevron discloses the folded rows.
function SessionWorkSummaryRow({
  row,
  summary,
  onToggle,
}: {
  row: SessionWorkRow;
  summary: SessionToolGroupSummary;
  onToggle: () => void;
}) {
  const first = row.entries[0];
  const leadIcon = createElement(
    getToolIcon(resolveRegisteredToolName(first?.toolName ?? "tool"), first?.args),
    { "aria-hidden": true, className: "size-3 shrink-0 text-subtle", strokeWidth: 1.8 }
  );
  return (
    <div data-testid="work-summary-row" data-open={row.expanded} className="flex min-w-0 flex-col">
      <TranscriptDisclosure
        expanded={row.expanded}
        onToggle={onToggle}
        icon={leadIcon}
        label={<span data-testid="work-summary-label">{summary.label}</span>}
      />
      {row.expanded ? (
        <div data-testid="work-summary-entries" className="flex min-w-0 flex-col gap-0.5 pt-0.5">
          {row.entries.map(tool => (
            <SessionToolCallRow key={tool.id} message={toolMessageFromPart(tool)} turnSettled />
          ))}
        </div>
      ) : null}
    </div>
  );
}

function SessionWorkRowView({ row }: { row: SessionWorkRow }) {
  const store = useTimelineRowContext();
  if (row.summary) {
    return (
      <SessionWorkSummaryRow
        row={row}
        summary={row.summary}
        onToggle={() => toggleTimelineExpansion(store, "work-group", row.groupId)}
      />
    );
  }
  const tools = visibleWorkEntries(row);
  return (
    <div data-testid="work-row" className="flex min-w-0 flex-col gap-0.5">
      {tools.map(tool => (
        <SessionToolCallRow
          key={tool.id}
          message={toolMessageFromPart(tool)}
          turnSettled={!row.active}
        />
      ))}
    </div>
  );
}

function SessionWorkToggleRowView({ row }: { row: SessionWorkToggleRow }) {
  const store = useTimelineRowContext();
  return (
    <WorkToggleButton
      row={row}
      onToggle={() => toggleTimelineExpansion(store, "work-group", row.groupId)}
    />
  );
}

function SessionChangedFilesRowContent({ row }: { row: SessionChangedFilesRow }) {
  const store = useTimelineRowContext();
  return (
    <SessionChangedFilesRowView
      row={row}
      onToggle={() => toggleTimelineExpansion(store, "changed-files", row.id)}
    />
  );
}

// `.turnfold` — "Worked for Ns", the ONLY border in the transcript. The
// interrupted variant never folds: a quiet danger text line above the
// always-visible work.
function SessionTurnFoldRowView({ row }: { row: SessionTurnFoldRow }) {
  const store = useTimelineRowContext();
  const turnId = row.turnId ?? row.id;
  const expanded = useSelector(store, state => state.context.expandedTurns.has(turnId));
  if (row.interrupted) {
    return (
      <div
        data-testid="turn-fold-interrupted"
        className="mb-2.5 flex min-w-0 flex-col gap-1 border-b border-line pb-1.5"
      >
        <div
          data-testid="turn-fold-interrupt-label"
          className="flex w-fit items-center gap-1.5 px-1 text-transcript-body text-danger"
        >
          <CircleStop className="size-3" aria-hidden="true" />
          <span>{row.label}</span>
        </div>
        <div className="flex min-w-0 flex-col gap-0.5">{renderTimelineRows(row.rows)}</div>
      </div>
    );
  }
  return (
    <div data-open={expanded} className="mb-2.5 min-w-0 border-b border-line pb-1.5">
      <TranscriptDisclosure
        data-testid="turn-fold-row"
        expanded={expanded}
        onToggle={() => toggleTimelineExpansion(store, "turn", turnId)}
        icon={null}
        label={row.label}
        variant="turn"
      />
      {expanded ? (
        <div className="flex min-w-0 flex-col gap-0.5 pt-1">{renderTimelineRows(row.rows)}</div>
      ) : null}
    </div>
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
        return <SessionWorkingRowView row={row} />;
      case "work":
        return <SessionWorkRowView row={row} />;
      case "work-toggle":
        return <SessionWorkToggleRowView row={row} />;
      case "changed-files":
        return <SessionChangedFilesRowContent row={row} />;
      case "turn-fold":
        return <SessionTurnFoldRowView row={row} />;
    }
  },
  (previous, next) => sessionRowEqual(previous.row, next.row)
);

// Referentially stable renderer: no closure deps, so re-deriving the row list
// never re-creates the mapping function. Row identity carries the change signal.
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

export function AssistantMessageTimeline() {
  const { goal, rows, timelineStore } = useAssistantMessageTimeline();

  return (
    <TimelineRowContext.Provider value={timelineStore}>
      {goal ? <GoalPromptNotice goal={goal} /> : null}
      {renderTimelineRows(rows)}
    </TimelineRowContext.Provider>
  );
}
