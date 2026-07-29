import { ChevronDown, ChevronRight, CircleStop, Target } from "lucide-react";
import { useRef } from "react";
import type { ReactNode } from "react";
import { useSelector } from "@xstate/store-react";

import { cn } from "@/lib/utils";
import {
  ClarificationDataPart,
  isAgentEventPayload,
  isClarifyEventData,
  PermissionDataPart,
  resolveToolResult,
  RuntimeActivityNotice,
  SessionToolCallRow,
  ThinkingBlock,
  type CompozyPermissionData,
  type GoalPromptMeta,
  type UIMessage,
} from "@/systems/session";
import { getToolIcon, resolveRegisteredToolName } from "@/systems/session/lib/tool-labels";
import { Marker, MarkerMeta } from "@compozy/ui";
import { Link } from "@tanstack/react-router";
import { useAssistantMessageTimeline } from "./hooks/use-assistant-message-timeline";
import {
  TimelineRowContext,
  toggleTimelineExpansion,
  useTimelineRowContext,
} from "./hooks/use-timeline-row-context";
import { SessionDataEventCard, SessionMessageText } from "./session-message-parts";
import { SessionChangedFilesRowView } from "./session-changed-files-row";
import { SessionWorkingRowView } from "./session-working-row";
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
  visibleWorkEntries,
} from "./session-timeline.logic";

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

  return <SessionDataEventCard name={row.part.name} data={row.part.data} />;
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
function WorkToggleButton({
  row,
  onToggle,
}: {
  row: SessionWorkToggleRow;
  onToggle: (button: HTMLElement | null) => void;
}) {
  const ref = useRef<HTMLButtonElement | null>(null);
  return (
    <button
      ref={ref}
      type="button"
      data-testid="work-toggle-row"
      aria-expanded={row.expanded}
      onClick={() => onToggle(ref.current)}
      className={cn(
        "ml-[25px] inline-flex min-h-[22px] w-fit items-center gap-1.5 rounded-xs px-1",
        "text-[11.5px] text-subtle transition-colors duration-base ease-out hover:text-fg",
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
  onToggle: (button: HTMLElement | null) => void;
}) {
  const ref = useRef<HTMLButtonElement | null>(null);
  const first = row.entries[0];
  const LeadIcon = getToolIcon(resolveRegisteredToolName(first?.toolName ?? "tool"), first?.args);
  return (
    <div data-testid="work-summary-row" data-open={row.expanded} className="flex min-w-0 flex-col">
      <button
        ref={ref}
        type="button"
        aria-expanded={row.expanded}
        onClick={() => onToggle(ref.current)}
        className={cn(
          "inline-flex min-h-6 w-fit items-center gap-[7px] rounded-md px-1 text-left",
          "text-small-body font-medium text-muted",
          "transition-colors duration-base ease-out hover:bg-hover hover:text-fg",
          "focus-visible:shadow-focus-ring focus-visible:outline-none"
        )}
      >
        <span className="flex size-[18px] shrink-0 items-center justify-center">
          <LeadIcon aria-hidden="true" className="size-3 shrink-0 text-subtle" strokeWidth={1.8} />
        </span>
        <span data-testid="work-summary-label" className="min-w-0">
          {summary.label}
        </span>
        <ChevronDown
          aria-hidden="true"
          className={cn(
            "size-3 shrink-0 text-faint transition-transform duration-slow ease-out motion-reduce:transition-none",
            row.expanded ? "rotate-180" : null
          )}
          strokeWidth={1.75}
        />
      </button>
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
        onToggle={button => toggleTimelineExpansion(store, "work-group", row.groupId, button)}
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
      onToggle={button => toggleTimelineExpansion(store, "work-group", row.groupId, button)}
    />
  );
}

function SessionChangedFilesRowContent({ row }: { row: SessionChangedFilesRow }) {
  const store = useTimelineRowContext();
  return (
    <SessionChangedFilesRowView
      row={row}
      onToggle={button => toggleTimelineExpansion(store, "changed-files", row.id, button)}
    />
  );
}

// `.turnfold` — "Worked for Ns", the ONLY border in the transcript. The
// interrupted variant never folds: a quiet danger text line above the
// always-visible work.
function SessionTurnFoldRowView({ row }: { row: SessionTurnFoldRow }) {
  const store = useTimelineRowContext();
  const buttonRef = useRef<HTMLButtonElement | null>(null);
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
          className="flex w-fit items-center gap-1.5 px-1 text-[12px] text-danger"
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
      <button
        ref={buttonRef}
        type="button"
        data-testid="turn-fold-row"
        aria-expanded={expanded}
        onClick={() => toggleTimelineExpansion(store, "turn", turnId, buttonRef.current)}
        className={cn(
          "-ml-0.5 inline-flex items-center gap-[5px] rounded-xs px-1 py-px",
          "text-[12px] text-subtle tabular-nums",
          "transition-colors duration-base ease-out hover:text-fg",
          "focus-visible:shadow-focus-ring focus-visible:outline-none"
        )}
      >
        <ChevronRight
          aria-hidden="true"
          className={cn(
            "size-[11px] shrink-0 text-faint transition-transform duration-slow ease-out motion-reduce:transition-none",
            expanded ? "rotate-90" : null
          )}
        />
        {row.label}
      </button>
      {expanded ? (
        <div className="flex min-w-0 flex-col gap-0.5 pt-1">{renderTimelineRows(row.rows)}</div>
      ) : null}
    </div>
  );
}

// Structural sharing (`computeStableSessionRows`) keeps unchanged row inputs
// referentially stable so the React Compiler can reuse their rendered subtrees.
// Interactive variants select their own expansion state from the stable store
// handle in `TimelineRowContext`.
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
}

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
