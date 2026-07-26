import { ChevronRight, CircleStop } from "lucide-react";
import { useRef } from "react";
import type { ReactNode } from "react";

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
  useSessionRuntimeRenderContext,
  type AghPermissionData,
  type GoalPromptMeta,
  type UIMessage,
} from "@/systems/session";
import { Button, Eyebrow } from "@agh/ui";
import { Link } from "@tanstack/react-router";
import { useAssistantMessageTimeline } from "./hooks/use-assistant-message-timeline";
import {
  TimelineRowContext,
  type TimelineRowSharedState,
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
  type SessionTurnFoldRow,
  type SessionWorkRow,
  type SessionWorkToggleRow,
  visibleWorkEntries,
} from "./session-timeline.logic";

function isAghPermissionData(value: unknown): value is AghPermissionData {
  return isAgentEventPayload(value) && typeof value.request_id === "string";
}

function SessionTextRowView({ row }: { row: SessionTextRow }) {
  return <SessionMessageText text={row.part.text} streaming={row.part.state === "running"} />;
}

function SessionReasoningRowView({ row }: { row: SessionReasoningRow }) {
  return (
    <ThinkingBlock
      thinking={row.text}
      thinkingComplete={!row.streaming}
      updateCount={row.updateCount}
    />
  );
}

function SessionDataRowView({ row }: { row: SessionDataRow }) {
  const renderContext = useSessionRuntimeRenderContext();
  if (row.part.name === "data-agh-event" && renderContext && isClarifyEventData(row.part.data)) {
    return (
      <ClarificationDataPart
        data={row.part.data}
        sessionId={renderContext.sessionId}
        workspaceId={renderContext.workspaceId}
      />
    );
  }
  if (row.part.name === "data-agh-event" && isAgentEventPayload(row.part.data)) {
    return <RuntimeActivityNotice event={row.part.data} />;
  }
  if (
    row.part.name === "data-agh-permission" &&
    renderContext &&
    isAghPermissionData(row.part.data)
  ) {
    return (
      <PermissionDataPart
        data={row.part.data}
        sessionId={renderContext.sessionId}
        workspaceId={renderContext.workspaceId}
      />
    );
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

function WorkToggleButton({
  row,
  onToggle,
}: {
  row: SessionWorkToggleRow;
  onToggle: (button: HTMLElement | null) => void;
}) {
  const ref = useRef<HTMLButtonElement | null>(null);
  return (
    <Button
      ref={ref}
      type="button"
      variant="ghost"
      size="xs"
      data-testid="work-toggle-row"
      aria-expanded={row.expanded}
      onClick={() => onToggle(ref.current)}
      className="ml-7 w-fit gap-1 px-1 text-subtle hover:text-fg"
    >
      <ChevronRight
        className={cn(
          "size-3 transition-transform duration-base ease-out motion-reduce:transition-none",
          row.expanded ? "rotate-90" : null
        )}
        aria-hidden="true"
      />
      {workToggleLabel(row)}
    </Button>
  );
}

function TurnFoldButton({
  row,
  expanded,
  onToggle,
}: {
  row: Extract<SessionRow, { kind: "turn-fold" }>;
  expanded: boolean;
  onToggle: (button: HTMLElement | null) => void;
}) {
  const ref = useRef<HTMLButtonElement | null>(null);
  return (
    <Button
      ref={ref}
      type="button"
      variant="ghost"
      size="xs"
      data-testid="turn-fold-row"
      aria-expanded={expanded}
      onClick={() => onToggle(ref.current)}
      className="w-fit gap-1 px-1 text-subtle hover:text-fg"
    >
      <ChevronRight
        className={cn("size-3 transition-transform", expanded ? "rotate-90" : null)}
        aria-hidden="true"
      />
      {row.label}
    </Button>
  );
}

function SessionWorkRowView({ row }: { row: SessionWorkRow }) {
  const tools = visibleWorkEntries(row);
  return (
    <div data-testid="work-row" className="flex min-w-0 flex-col gap-0.5">
      {row.grouped ? (
        <Eyebrow className="mb-0.5 px-1 text-subtle">{row.entries.length} tool calls</Eyebrow>
      ) : null}
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
  const { setExpandedWorkGroups } = useTimelineRowContext();
  return (
    <WorkToggleButton
      row={row}
      onToggle={button => toggleTimelineExpansion(setExpandedWorkGroups, row.groupId, button)}
    />
  );
}

function SessionChangedFilesRowContent({ row }: { row: SessionChangedFilesRow }) {
  const { setExpandedChangedFiles } = useTimelineRowContext();
  return (
    <SessionChangedFilesRowView
      row={row}
      onToggle={button => toggleTimelineExpansion(setExpandedChangedFiles, row.id, button)}
    />
  );
}

function SessionTurnFoldRowView({ row }: { row: SessionTurnFoldRow }) {
  const { expandedTurns, setExpandedTurns } = useTimelineRowContext();
  // An interrupted turn keeps its work expanded so the operator keeps their
  // place; the fold row becomes a danger-toned interruption label above the
  // always-visible work (never a collapsing disclosure).
  if (row.interrupted) {
    return (
      <div data-testid="turn-fold-interrupted" className="flex min-w-0 flex-col gap-1">
        <div
          data-testid="turn-fold-interrupt-label"
          className="flex w-fit items-center gap-1.5 px-1 text-small-body text-danger"
        >
          <CircleStop className="size-3" aria-hidden="true" />
          <span>{row.label}</span>
        </div>
        <div className="ml-4 flex min-w-0 flex-col gap-1">{renderTimelineRows(row.rows)}</div>
      </div>
    );
  }
  const expanded = expandedTurns.has(row.turnId ?? row.id);
  return (
    <div className="flex min-w-0 flex-col gap-1">
      <TurnFoldButton
        row={row}
        expanded={expanded}
        onToggle={button => toggleTimelineExpansion(setExpandedTurns, row.turnId ?? row.id, button)}
      />
      {expanded ? (
        <div className="ml-4 flex min-w-0 flex-col gap-1">{renderTimelineRows(row.rows)}</div>
      ) : null}
    </div>
  );
}

// Structural sharing (`computeStableSessionRows`) keeps unchanged row inputs
// referentially stable so the React Compiler can reuse their rendered subtrees.
// Interactive variants read callbacks and expansion from `TimelineRowContext`;
// toggles update their consumers while steady-state streaming leaves the context
// untouched.
export function TimelineRowContent({ row }: { row: SessionRow }) {
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

function GoalPromptNotice({ goal }: { goal: GoalPromptMeta }) {
  const turn = goal.turn === null ? null : `turn ${goal.turn}`;
  return (
    <div
      className="mb-1 flex flex-wrap items-center gap-2 rounded-md border border-line-soft bg-canvas-soft px-2.5 py-1.5"
      data-testid="goal-prompt-meta"
    >
      <Eyebrow className="text-info">{GOAL_PROMPT_LABELS[goal.kind]}</Eyebrow>
      <span className="font-mono text-badge text-subtle">
        {goal.node_id} · generation {goal.generation}
        {turn ? ` · ${turn}` : ""}
      </span>
      <Link
        className="ml-auto text-small-body text-muted hover:text-fg"
        params={{ runId: goal.run_id }}
        to="/loop-runs/$runId"
      >
        Open run
      </Link>
    </div>
  );
}

export function AssistantMessageTimeline() {
  const {
    expandedTurns,
    goal,
    rows,
    setExpandedChangedFiles,
    setExpandedTurns,
    setExpandedWorkGroups,
  } = useAssistantMessageTimeline();
  // The React Compiler stabilizes this context value and its callbacks while
  // their inputs remain unchanged, so steady-state streaming does not invalidate
  // interactive row consumers.
  const sharedState: TimelineRowSharedState = {
    expandedTurns,
    setExpandedWorkGroups,
    setExpandedTurns,
    setExpandedChangedFiles,
  };

  return (
    <TimelineRowContext.Provider value={sharedState}>
      {goal ? <GoalPromptNotice goal={goal} /> : null}
      {renderTimelineRows(rows)}
    </TimelineRowContext.Provider>
  );
}
