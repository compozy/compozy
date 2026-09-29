import { useState } from "react";
import { AlertCircle, PauseCircle, PlayCircle, RotateCw } from "lucide-react";

import { Button, ConfirmDialog, Empty } from "@compozy/ui";

import type { SchedulerBacklog, SchedulerStatus } from "../types";
import { SchedulerBacklogPanel } from "./scheduler-backlog-panel";
import { SchedulerPauseDialog } from "./scheduler-pause-dialog";
import { SchedulerStatusSummary } from "./scheduler-status-summary";

export interface SchedulerControlsPanelProps {
  status: SchedulerStatus | null;
  backlog: SchedulerBacklog | null;
  errorMessage?: string | null;
  backlogErrorMessage?: string | null;
  isLoading?: boolean;
  isBacklogLoading?: boolean;
  pending?: {
    pause?: boolean;
    resume?: boolean;
    drain?: boolean;
  };
  onPause?: (reason: string) => void | Promise<void>;
  onResume?: () => void | Promise<void>;
  onDrain?: () => void | Promise<void>;
}

export function SchedulerControlsPanel({
  status,
  backlog,
  errorMessage = null,
  backlogErrorMessage = null,
  isLoading = false,
  isBacklogLoading = false,
  pending,
  onPause,
  onResume,
  onDrain,
}: SchedulerControlsPanelProps) {
  const [pauseOpen, setPauseOpen] = useState(false);
  const [drainOpen, setDrainOpen] = useState(false);

  if (errorMessage && !status) {
    return (
      <section
        className="border-b border-line-soft bg-canvas-soft px-5 py-4"
        data-testid="scheduler-controls-panel-error"
      >
        <Empty
          data-testid="scheduler-controls-panel-empty-error"
          description={errorMessage}
          fill={false}
          icon={AlertCircle}
          title="Couldn't load the task queue"
        />
      </section>
    );
  }

  return (
    <section
      className="border-b border-line-soft bg-canvas-soft px-5 py-4"
      data-testid="scheduler-controls-panel"
    >
      <div className="flex flex-col gap-4 @3xl:flex-row @3xl:items-start @3xl:justify-between">
        <SchedulerStatusSummary isLoading={isLoading} status={status} />
        <SchedulerControlActions
          actionsDisabled={(isLoading && !status) || isAnyActionPending(pending)}
          onDrainRequest={() => setDrainOpen(true)}
          onPause={onPause}
          onPauseRequest={() => setPauseOpen(true)}
          onResume={onResume}
          onDrain={onDrain}
          paused={status?.paused ?? false}
        />
      </div>

      <SchedulerBacklogPanel
        backlog={backlog}
        errorMessage={backlogErrorMessage}
        isLoading={isBacklogLoading}
      />

      <SchedulerPauseDialog
        isPending={pending?.pause ?? false}
        onOpenChange={setPauseOpen}
        onPause={onPause}
        open={pauseOpen}
      />

      <ConfirmDialog
        cancelLabel="Cancel"
        confirmButtonProps={{ "data-testid": "scheduler-controls-drain-confirm" }}
        confirmLabel="Finish current and pause"
        contentProps={{ "data-testid": "scheduler-controls-drain-dialog" }}
        description="New work won't start. Work that is already running gets up to a minute to finish, then the queue stays paused until you resume it."
        isPending={pending?.drain ?? false}
        onConfirm={async () => {
          await onDrain?.();
          setDrainOpen(false);
        }}
        onOpenChange={setDrainOpen}
        open={drainOpen}
        title="Finish current work and pause?"
        tone="warning"
      />
    </section>
  );
}

function isAnyActionPending(pending: SchedulerControlsPanelProps["pending"]): boolean {
  return Boolean(pending?.pause || pending?.resume || pending?.drain);
}

interface SchedulerControlActionsProps {
  paused: boolean;
  actionsDisabled: boolean;
  onPause?: SchedulerControlsPanelProps["onPause"];
  onResume?: SchedulerControlsPanelProps["onResume"];
  onDrain?: SchedulerControlsPanelProps["onDrain"];
  onPauseRequest: () => void;
  onDrainRequest: () => void;
}

function SchedulerControlActions({
  paused,
  actionsDisabled,
  onPause,
  onResume,
  onDrain,
  onPauseRequest,
  onDrainRequest,
}: SchedulerControlActionsProps) {
  return (
    <div className="flex shrink-0 flex-wrap items-center gap-2">
      {paused ? (
        <Button
          data-testid="scheduler-controls-resume"
          disabled={actionsDisabled || !onResume}
          onClick={() => void onResume?.()}
          size="sm"
          type="button"
          variant="neutral"
        >
          <PlayCircle className="size-3" aria-hidden="true" />
          Resume
        </Button>
      ) : (
        <Button
          data-testid="scheduler-controls-pause"
          disabled={actionsDisabled || !onPause}
          onClick={onPauseRequest}
          size="sm"
          type="button"
          variant="neutral"
        >
          <PauseCircle className="size-3" aria-hidden="true" />
          Pause
        </Button>
      )}
      <Button
        data-testid="scheduler-controls-drain"
        disabled={actionsDisabled || !onDrain}
        onClick={onDrainRequest}
        size="sm"
        type="button"
        variant="neutral"
      >
        <RotateCw className="size-3" aria-hidden="true" />
        Finish current and pause
      </Button>
    </div>
  );
}
