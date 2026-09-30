import {
  CodeBlock,
  LaneTabs,
  MetadataTile,
  MonoId,
  Pill,
  Sheet,
  SheetContent,
  StatusCard,
  TabsContent,
  Time,
  type LaneTabsItem,
  type PillTone,
} from "@compozy/ui";

import { taskRunStatusLabel } from "../lib/task-formatters";
import type { TaskDetailView, TaskInspectView } from "../types";
import { TaskOperatorSheetHeader } from "./task-operator-sheet-header";
import { TaskInspectLoadingSkeleton } from "./task-loading-skeletons";
import { TaskRawPane } from "./task-raw-pane";

import type { TaskInspectTarget } from "../lib/task-detail-search";
import type { TaskStreamState } from "../lib/task-stream-state";

export type TaskInspectDrawerTab = TaskInspectTarget;

export interface TaskInspectDrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  activeTab: TaskInspectDrawerTab;
  onTabChange: (tab: TaskInspectDrawerTab) => void;
  detail: TaskDetailView;
  inspect: TaskInspectView | null;
  inspectLoading?: boolean;
  inspectErrorMessage?: string | null;
  stream: {
    state: TaskStreamState;
    errorMessage: string | null;
    seedSequence: number;
    latestEventSeq: number | null;
  };
}

const TABS: ReadonlyArray<LaneTabsItem<TaskInspectDrawerTab>> = [
  { value: "diagnostics", label: "Diagnostics", testId: "tasks-inspect-tab-diagnostics" },
  { value: "stream", label: "Stream", testId: "tasks-inspect-tab-stream" },
  { value: "raw", label: "Raw", testId: "tasks-inspect-tab-raw" },
];

/**
 * Next-action vocabulary uses plain language first and the enum as microtext.
 *
 * @see docs/design/opendesign/tasks/TASK-DETAILS-REDESIGN-PLAN.md §7.3
 */
const NEXT_ACTION_COPY: Record<string, string> = {
  claim_available: "A worker can pick this up now",
  waiting_for_session: "Waiting for the agent session to attach",
  stranded: "Run lost its worker. Retry or release it",
  recovery_required: "Stuck run needs recovery. Use Recover to requeue",
  running: "Running normally",
  terminal: "Nothing to do — the task is terminal",
};

const SEVERITY_TONE: Record<string, PillTone> = {
  ok: "success",
  info: "info",
  warn: "warning",
  error: "danger",
  critical: "danger",
};

const STREAM_STATE_PRESENTATION: Record<
  TaskStreamState,
  { label: string; pulse: boolean; tone: PillTone }
> = {
  disabled: { label: "Disabled", pulse: false, tone: "neutral" },
  error: { label: "Error", pulse: false, tone: "danger" },
  idle: { label: "Idle", pulse: false, tone: "neutral" },
  receiving: { label: "Receiving", pulse: true, tone: "success" },
};

interface DiagnosticsTile {
  value: string;
  detail?: string;
}

/** Plain-language tiles for the snapshot header, derived from one inspect view. */
function diagnosticsTiles(inspect: TaskInspectView): {
  nextAction: DiagnosticsTile;
  run: DiagnosticsTile;
  session: DiagnosticsTile;
  scheduler: string;
} {
  const run = inspect.current_run ?? null;
  const session = inspect.bound_session ?? null;
  const nextAction = inspect.next_action ?? null;
  return {
    nextAction: {
      value: nextAction ? (NEXT_ACTION_COPY[nextAction] ?? nextAction.replaceAll("_", " ")) : "—",
      detail: nextAction ?? undefined,
    },
    run: run
      ? { value: `Attempt ${run.attempt} · ${taskRunStatusLabel(run.status)}`, detail: run.run_id }
      : { value: "No current run" },
    session: session
      ? { value: session.state ?? "bound", detail: session.session_id }
      : { value: "No bound session" },
    scheduler: inspect.scheduler.paused ? "Paused" : "Active",
  };
}

function DiagnosticsPane({
  taskId,
  inspect,
  isLoading,
  errorMessage,
}: {
  taskId: string;
  inspect: TaskInspectView | null;
  isLoading: boolean;
  errorMessage: string | null;
}) {
  if (isLoading && !inspect) {
    return <TaskInspectLoadingSkeleton label="Loading inspect snapshot" />;
  }
  if (errorMessage && !inspect) {
    return (
      <p className="text-small-body text-danger" role="alert">
        {errorMessage}
      </p>
    );
  }
  if (!inspect) {
    return (
      <p className="text-small-body text-muted" data-testid="tasks-inspect-diagnostics-empty">
        No inspect snapshot is available for this task state.
      </p>
    );
  }
  return <DiagnosticsSnapshot errorMessage={errorMessage} inspect={inspect} taskId={taskId} />;
}

function DiagnosticsSnapshot({
  taskId,
  inspect,
  errorMessage,
}: {
  taskId: string;
  inspect: TaskInspectView;
  errorMessage: string | null;
}) {
  const tiles = diagnosticsTiles(inspect);

  return (
    <div className="flex flex-col gap-4" data-testid="tasks-inspect-diagnostics">
      {errorMessage ? (
        <p className="text-small-body text-danger" role="alert">
          {errorMessage}
        </p>
      ) : null}
      <div className="grid grid-cols-2 gap-2.5">
        <MetadataTile
          label="Next action"
          value={tiles.nextAction.value}
          detail={tiles.nextAction.detail}
        />
        <MetadataTile label="Current run" value={tiles.run.value} detail={tiles.run.detail} />
        <MetadataTile label="Session" value={tiles.session.value} detail={tiles.session.detail} />
        <MetadataTile label="Scheduler" value={tiles.scheduler} />
      </div>

      <DiagnosticsList diagnostics={inspect.diagnostics ?? []} />

      <CodeBlock
        code={`# same view from the CLI\ncompozy task inspect ${taskId} -o json`}
        language="bash"
      />
      <p className="text-form-hint text-subtle">
        Snapshot as of <Time iso={inspect.as_of} mode="relative" />.
      </p>
    </div>
  );
}

function DiagnosticsList({
  diagnostics,
}: {
  diagnostics: NonNullable<TaskInspectView["diagnostics"]>;
}) {
  if (diagnostics.length === 0) {
    return (
      <p className="rounded-md bg-sunken px-3.5 py-3 text-small-body leading-relaxed text-muted">
        No diagnostics were reported in this snapshot.
      </p>
    );
  }
  return diagnostics.map(diagnostic => (
    <StatusCard
      className="border border-line-soft"
      data-testid={`tasks-inspect-diagnostic-${diagnostic.code}`}
      key={diagnostic.id}
      tone={SEVERITY_TONE[diagnostic.severity ?? ""] ?? "neutral"}
    >
      <StatusCard.Header label={diagnostic.title}>
        <MonoId className="ml-auto" size="sm" value={diagnostic.code} />
      </StatusCard.Header>
      <StatusCard.Body>{diagnostic.message}</StatusCard.Body>
      {diagnostic.suggested_command ? (
        <CodeBlock code={diagnostic.suggested_command} language="bash" />
      ) : null}
    </StatusCard>
  ));
}

function StreamPane({ stream }: { stream: TaskInspectDrawerProps["stream"] }) {
  const presentation = STREAM_STATE_PRESENTATION[stream.state];

  return (
    <div className="flex flex-col gap-4" data-testid="tasks-inspect-stream">
      <div className="grid grid-cols-2 gap-2.5">
        <MetadataTile
          label="Connection"
          value={
            <span className="inline-flex items-center gap-1.5">
              <Pill.Dot pulse={presentation.pulse} tone={presentation.tone} />
              {presentation.label}
            </span>
          }
        />
        <MetadataTile label="Latest event seq" value={stream.latestEventSeq ?? "—"} />
        <MetadataTile label="Resume seed" value={stream.seedSequence} />
      </div>
      {stream.errorMessage ? (
        <p className="text-small-body text-danger">{stream.errorMessage}</p>
      ) : null}
      <p className="rounded-md bg-sunken px-3.5 py-3 text-small-body leading-relaxed text-muted">
        The task stream replays durable events for this task subtree, then tails new ones. The page
        refetches on every frame.
      </p>
    </div>
  );
}

/** Operator drawer with Diagnostics, Stream, and Raw panes. */
export function TaskInspectDrawer({
  open,
  onOpenChange,
  activeTab,
  onTabChange,
  detail,
  inspect,
  inspectLoading = false,
  inspectErrorMessage = null,
  stream,
}: TaskInspectDrawerProps) {
  return (
    <Sheet onOpenChange={onOpenChange} open={open}>
      <SheetContent
        className="w-[min(600px,calc(100vw-24px))] sm:max-w-none"
        data-testid="tasks-inspect-drawer"
        side="right"
      >
        <TaskOperatorSheetHeader
          description={
            <>
              Runtime internals for this task. Everything here maps to{" "}
              <span className="font-mono text-eyebrow">compozy task inspect</span>.
            </>
          }
          title="Inspect"
        />
        <LaneTabs<TaskInspectDrawerTab>
          ariaLabel="Inspect views"
          className="flex min-h-0 flex-1 flex-col gap-0 px-4"
          items={TABS}
          listClassName="w-full"
          onChange={onTabChange}
          value={activeTab}
        >
          <div className="min-h-0 flex-1 overflow-y-auto py-4">
            <TabsContent value="diagnostics">
              <DiagnosticsPane
                errorMessage={inspectErrorMessage}
                inspect={inspect}
                isLoading={inspectLoading}
                taskId={detail.task.id}
              />
            </TabsContent>
            <TabsContent value="stream">
              <StreamPane stream={stream} />
            </TabsContent>

            <TabsContent value="raw">
              <TaskRawPane detail={detail} />
            </TabsContent>
          </div>
        </LaneTabs>
      </SheetContent>
    </Sheet>
  );
}
