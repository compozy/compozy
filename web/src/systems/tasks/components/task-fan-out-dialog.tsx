import {
  Alert,
  AlertDescription,
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  dialogShellClass,
  Field,
  FieldHeader,
  FieldLabel,
  HelpTip,
  Spinner,
  Textarea,
} from "@compozy/ui";

import { useTaskFanOutDialog } from "../hooks/use-task-fan-out-dialog";
import { useTaskFanOutRunResults } from "../hooks/use-task-fan-out-run-results";
import { useRetryTaskRun } from "../hooks/use-task-run-actions";
import type { FanOutTaskRunsRequest, FanOutTaskRunsResponse } from "../types";
import type { WorktreePayload } from "@/systems/workspace";
import { NetworkParticipationFields } from "@/systems/network";

import { TaskFanOutIsolationRow } from "./task-fan-out-isolation-row";
import { TaskFanOutRunResults } from "./task-fan-out-run-results";

export interface TaskFanOutDialogProps {
  taskId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  isPending?: boolean;
  onFanOut: (data: FanOutTaskRunsRequest) => Promise<FanOutTaskRunsResponse | void>;
  /** Per-run isolation needs a git-backed workspace; the row is absent otherwise. */
  gitBacked?: boolean;
  worktrees?: readonly WorktreePayload[];
}

/**
 * Fan-out dialog for scoped assignments under one designation group. It opens
 * from the head overflow and accepts one assignment brief per line.
 */
export function TaskFanOutDialog({
  taskId,
  open,
  onOpenChange,
  isPending = false,
  onFanOut,
  gitBacked = false,
  worktrees,
}: TaskFanOutDialogProps) {
  const state = useTaskFanOutDialog({ onOpenChange, onFanOut });
  const liveRuns = useTaskFanOutRunResults(taskId, state.result?.runs ?? []);
  const retry = useRetryTaskRun();

  return (
    <Dialog onOpenChange={state.handleOpenChange} open={open}>
      <DialogContent
        className={`grid-rows-[auto_minmax(0,1fr)] text-fg ${dialogShellClass("md", { fill: true })}`}
        data-testid="tasks-fan-out-runs-dialog"
        unframed
      >
        <DialogHeader className="border-b border-line px-5 py-4">
          <DialogTitle>Fan out task runs</DialogTitle>
          <DialogDescription>
            Create scoped assignments that run under one designation group.
          </DialogDescription>
        </DialogHeader>

        <form
          className="grid min-h-0 grid-rows-[minmax(0,1fr)_auto_auto]"
          onSubmit={state.handleSubmit}
        >
          <div className="min-h-0 space-y-5 overflow-y-auto p-5">
            {state.formError ? (
              <Alert data-testid="tasks-fan-out-runs-error" variant="danger">
                <AlertDescription>{state.formError}</AlertDescription>
              </Alert>
            ) : null}

            <Field>
              <FieldHeader>
                <FieldLabel htmlFor="tasks-fan-out-designations">Assignments</FieldLabel>
                <HelpTip label="About assignments">
                  One line per run. Each line is injected as that worker&apos;s assignment brief.
                </HelpTip>
              </FieldHeader>
              <Textarea
                data-testid="tasks-fan-out-designations"
                id="tasks-fan-out-designations"
                onChange={event => state.setDesignationsText(event.target.value)}
                placeholder={"Investigate the failing checkout path\nValidate the fix on staging"}
                rows={5}
                value={state.designationsText}
                variant="mono"
              />
            </Field>

            {gitBacked ? (
              <TaskFanOutIsolationRow
                checked={state.worktreePerRun}
                designationCount={state.designationCount}
                disabled={isPending}
                onCheckedChange={state.setWorktreePerRun}
              />
            ) : null}

            {state.result ? (
              <TaskFanOutRunResults
                liveRuns={liveRuns}
                onRetry={runId => retry.mutate({ runId })}
                retryPending={retry.isPending}
                runs={state.result.runs}
                worktrees={worktrees}
              />
            ) : null}

            <NetworkParticipationFields
              allowedStrategies={state.networkStrategies}
              onChange={state.setNetworkParticipation}
              testIdPrefix="tasks-fan-out-network"
              value={state.networkParticipation}
            />
          </div>

          <DialogFooter className="border-t border-line bg-canvas-soft px-5 py-3">
            <Button onClick={() => state.handleOpenChange(false)} type="button" variant="outline">
              {state.result ? "Done" : "Cancel"}
            </Button>
            {state.result ? null : (
              <Button data-testid="tasks-fan-out-runs-submit" disabled={isPending} type="submit">
                {isPending ? <Spinner aria-hidden="true" className="size-4" /> : null}
                Create runs
              </Button>
            )}
          </DialogFooter>
          {gitBacked && state.worktreePerRun ? (
            <p
              className="border-t border-line bg-canvas-soft px-5 pb-3 text-badge text-muted"
              data-slot="task-fan-out-isolation-note"
            >
              Branches land in the run/ namespace.
            </p>
          ) : null}
        </form>
      </DialogContent>
    </Dialog>
  );
}
