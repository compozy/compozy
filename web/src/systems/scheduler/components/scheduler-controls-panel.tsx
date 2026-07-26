import { useState } from "react";
import { AlertCircle, PauseCircle, PlayCircle, RotateCw } from "lucide-react";

import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Empty,
  Eyebrow,
  Pill,
  Skeleton,
  Textarea,
  Time,
} from "@agh/ui";

import type { SchedulerBacklog, SchedulerStatus } from "../types";
import { SchedulerBacklogPanel } from "./scheduler-backlog-panel";

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
  const [pauseReason, setPauseReason] = useState("");
  const [pauseError, setPauseError] = useState<string | null>(null);
  const isPausePending = pending?.pause ?? false;
  const isResumePending = pending?.resume ?? false;
  const isDrainPending = pending?.drain ?? false;
  const isActionPending = isPausePending || isResumePending || isDrainPending;
  const isInitialStatusLoading = isLoading && !status;

  const handlePauseOpenChange = (next: boolean) => {
    setPauseOpen(next);
    if (!next) {
      setPauseError(null);
    }
  };

  const handlePauseConfirm = async () => {
    const reason = pauseReason.trim();
    if (!reason) {
      setPauseError("Provide a pause reason.");
      return;
    }
    if (!onPause) {
      return;
    }
    try {
      await onPause(reason);
      setPauseReason("");
      setPauseError(null);
      setPauseOpen(false);
    } catch (error) {
      setPauseError(error instanceof Error ? error.message : "Failed to pause scheduler.");
    }
  };

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
          title="Unable to load scheduler"
        />
      </section>
    );
  }

  return (
    <section
      className="border-b border-line-soft bg-canvas-soft px-5 py-4"
      data-testid="scheduler-controls-panel"
    >
      <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
        <div className="min-w-0">
          <Eyebrow className="text-muted">Scheduler</Eyebrow>
          <div className="mt-1 flex min-w-0 flex-wrap items-center gap-2">
            <h2 className="text-item-title font-medium text-fg-strong">Dispatch controls</h2>
            <Pill
              data-testid="scheduler-controls-state"
              tone={isInitialStatusLoading ? "neutral" : status?.paused ? "warning" : "success"}
            >
              {isInitialStatusLoading ? "Loading" : status?.paused ? "Paused" : "Running"}
            </Pill>
            {isLoading && status ? (
              <Pill data-testid="scheduler-controls-loading" tone="neutral">
                Loading
              </Pill>
            ) : null}
          </div>
          {isInitialStatusLoading ? (
            <div
              aria-label="Loading scheduler status"
              className="mt-2 flex items-center gap-3"
              data-testid="scheduler-controls-meta-loading"
              role="status"
            >
              <Skeleton className="h-3 w-24" />
              <Skeleton className="h-3 w-20" />
              <Skeleton className="h-3 w-28" />
            </div>
          ) : (
            <div
              className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-form-label text-muted"
              data-testid="scheduler-controls-meta"
            >
              <span>{status?.active_claim_count ?? 0} active claims</span>
              <span aria-hidden="true" className="text-faint">
                ·
              </span>
              <span>{status?.queued_run_count ?? 0} queued runs</span>
              <span aria-hidden="true" className="text-faint">
                ·
              </span>
              <span>{status?.paused_task_count ?? 0} paused tasks</span>
              <span aria-hidden="true" className="text-faint">
                ·
              </span>
              <span
                className={(status?.starved_run_count ?? 0) > 0 ? "text-warning" : undefined}
                data-testid="scheduler-controls-starved-count"
              >
                {status?.starved_run_count ?? 0} starved runs
              </span>
              <span aria-hidden="true" className="text-faint">
                ·
              </span>
              <span
                className={
                  (status?.needs_attention_run_count ?? 0) > 0 ? "text-warning" : undefined
                }
                data-testid="scheduler-controls-needs-attention-count"
              >
                {status?.needs_attention_run_count ?? 0} needs attention
              </span>
              {status?.paused_at ? (
                <>
                  <span aria-hidden="true" className="text-faint">
                    ·
                  </span>
                  <span>
                    Paused <Time iso={status.paused_at} mode="relative" />
                  </span>
                </>
              ) : null}
            </div>
          )}
          {status?.paused_reason ? (
            <p
              className="mt-2 max-w-3xl text-form-label text-muted"
              data-testid="scheduler-controls-reason"
            >
              {status.paused_reason}
            </p>
          ) : null}
        </div>

        <div className="flex shrink-0 flex-wrap items-center gap-2">
          {status?.paused ? (
            <Button
              data-testid="scheduler-controls-resume"
              disabled={isInitialStatusLoading || isActionPending || !onResume}
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
              disabled={isInitialStatusLoading || isActionPending || !onPause}
              onClick={() => setPauseOpen(true)}
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
            disabled={isInitialStatusLoading || isActionPending || !onDrain}
            onClick={() => void onDrain?.()}
            size="sm"
            title="Pause dispatch and wait for active claims to finish."
            type="button"
          >
            <RotateCw className="size-3" aria-hidden="true" />
            Drain
          </Button>
        </div>
      </div>

      <SchedulerBacklogPanel
        backlog={backlog}
        errorMessage={backlogErrorMessage}
        isLoading={isBacklogLoading}
      />

      <Dialog open={pauseOpen} onOpenChange={handlePauseOpenChange}>
        <DialogContent
          data-testid="scheduler-controls-pause-dialog"
          showCloseButton={!isPausePending}
          className="max-w-md"
        >
          <DialogHeader>
            <DialogTitle>Pause scheduler?</DialogTitle>
            <DialogDescription>New claims stop; active runs continue.</DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-2">
            <label className="eyebrow text-muted" htmlFor="scheduler-controls-pause-reason">
              Reason
            </label>
            <Textarea
              aria-describedby={pauseError ? "scheduler-controls-pause-error" : undefined}
              aria-invalid={Boolean(pauseError)}
              data-testid="scheduler-controls-pause-reason"
              disabled={isPausePending}
              id="scheduler-controls-pause-reason"
              onChange={event => {
                setPauseReason(event.target.value);
                setPauseError(null);
              }}
              rows={3}
              value={pauseReason}
            />
            {pauseError ? (
              <p
                className="text-form-hint text-danger"
                data-testid="scheduler-controls-pause-error"
                id="scheduler-controls-pause-error"
                role="alert"
              >
                {pauseError}
              </p>
            ) : null}
          </div>
          <DialogFooter className="gap-2">
            <Button
              disabled={isPausePending}
              onClick={() => handlePauseOpenChange(false)}
              size="sm"
              type="button"
              variant="neutral"
            >
              Cancel
            </Button>
            <Button
              data-testid="scheduler-controls-pause-confirm"
              disabled={isPausePending}
              onClick={() => void handlePauseConfirm()}
              size="sm"
              type="button"
            >
              Pause
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  );
}
