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
  Field,
  FieldDescription,
  FieldLabel,
  Spinner,
  Textarea,
} from "@agh/ui";
import { NetworkParticipationFields } from "@/systems/network";

import { useTaskFanOutDialog } from "../hooks/use-task-fan-out-dialog";
import type { FanOutTaskRunsRequest } from "../types";

export interface TaskFanOutDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  isPending?: boolean;
  onFanOut: (data: FanOutTaskRunsRequest) => Promise<void>;
}

/**
 * Fan-out dialog for scoped assignments under one designation group. It opens
 * from the head overflow and accepts one assignment brief per line.
 */
export function TaskFanOutDialog({
  open,
  onOpenChange,
  isPending = false,
  onFanOut,
}: TaskFanOutDialogProps) {
  const state = useTaskFanOutDialog({ onOpenChange, onFanOut });

  return (
    <Dialog onOpenChange={state.handleOpenChange} open={open}>
      <DialogContent
        className="gap-0 p-0 text-fg sm:max-w-2xl"
        data-testid="tasks-fan-out-runs-dialog"
      >
        <DialogHeader className="border-b border-line px-5 py-4">
          <DialogTitle>Fan out task runs</DialogTitle>
          <DialogDescription>
            Create scoped assignments that run under one designation group.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={state.handleSubmit}>
          <div className="space-y-5 p-5">
            {state.formError ? (
              <Alert data-testid="tasks-fan-out-runs-error" variant="danger">
                <AlertDescription>{state.formError}</AlertDescription>
              </Alert>
            ) : null}

            <Field>
              <FieldLabel htmlFor="tasks-fan-out-designations">Assignments</FieldLabel>
              <FieldDescription>
                One line per run. Each line is injected as that worker's assignment brief.
              </FieldDescription>
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

            <NetworkParticipationFields
              allowedStrategies={state.networkStrategies}
              onChange={state.setNetworkParticipation}
              testIdPrefix="tasks-fan-out-network"
              value={state.networkParticipation}
            />
          </div>

          <DialogFooter className="border-t border-line bg-canvas-soft px-5 py-3">
            <Button onClick={() => state.handleOpenChange(false)} type="button" variant="outline">
              Cancel
            </Button>
            <Button data-testid="tasks-fan-out-runs-submit" disabled={isPending} type="submit">
              {isPending ? <Spinner aria-hidden="true" className="size-4" /> : null}
              Create runs
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
