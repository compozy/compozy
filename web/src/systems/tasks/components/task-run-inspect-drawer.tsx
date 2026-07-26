import { MetadataTile, Sheet, SheetContent, Time } from "@agh/ui";

import type { TaskRunDetailView, TaskRunInspectView } from "../types";
import { TaskOperatorSheetHeader } from "./task-operator-sheet-header";
import { TaskInspectLoadingSkeleton } from "./task-loading-skeletons";

export interface TaskRunInspectDrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  run: TaskRunDetailView;
  inspect: TaskRunInspectView | null;
  isLoading?: boolean;
  errorMessage?: string | null;
}

/**
 * Run operator drawer for heartbeat, lease window, truncated claim-token hash,
 * and idempotency key. Raw claim tokens do not exist in any DTO and are never rendered.
 *
 * @see docs/design/opendesign/tasks/TASK-DETAILS-REDESIGN-PLAN.md §4.9
 */
export function TaskRunInspectDrawer({
  open,
  onOpenChange,
  run,
  inspect,
  isLoading = false,
  errorMessage = null,
}: TaskRunInspectDrawerProps) {
  const record = run.run;
  const inspectRun = inspect?.current_run ?? null;
  const heartbeatAt = inspectRun?.heartbeat_at ?? null;
  const leaseUntil = inspectRun?.lease_until ?? null;
  const claimHash = inspectRun?.claim_token_hash_truncated ?? null;
  const idempotencyKey = record.idempotency_key ?? null;

  return (
    <Sheet onOpenChange={onOpenChange} open={open}>
      <SheetContent
        className="w-[min(560px,calc(100vw-24px))] sm:max-w-none"
        data-testid="tasks-run-inspect-drawer"
        side="right"
      >
        <TaskOperatorSheetHeader
          description={
            <>
              Claim and lease internals for{" "}
              <span className="font-mono text-eyebrow">{record.id}</span>.
            </>
          }
          title="Inspect run"
        />
        <div className="min-h-0 flex-1 overflow-y-auto px-4 py-4">
          {isLoading && !inspect ? (
            <TaskInspectLoadingSkeleton label="Loading run inspection" />
          ) : errorMessage && !inspect ? (
            <p className="text-small-body text-danger" role="alert">
              {errorMessage}
            </p>
          ) : !inspect ? (
            <p className="text-small-body text-muted">No inspect snapshot is available.</p>
          ) : (
            <>
              <div className="grid grid-cols-2 gap-2.5">
                <MetadataTile
                  className="border border-line-soft bg-input-fill"
                  label="Heartbeat"
                  value={heartbeatAt ? <Time iso={heartbeatAt} mode="relative" /> : "—"}
                />
                <MetadataTile
                  className="border border-line-soft bg-input-fill"
                  label="Lease until"
                  value={leaseUntil ? <Time iso={leaseUntil} mode="absolute" /> : "—"}
                />
                <MetadataTile
                  className="border border-line-soft bg-input-fill"
                  label="Claim token"
                  value={claimHash ? `sha256 · ${claimHash}` : "—"}
                />
                <MetadataTile
                  className="border border-line-soft bg-input-fill"
                  label="Idempotency key"
                  value={idempotencyKey ?? "—"}
                />
              </div>
              <p className="mt-4 rounded-md border border-line-soft bg-canvas-soft px-3.5 py-3 text-small-body leading-relaxed text-muted">
                The claim lease renews on every heartbeat. If heartbeats stop, the scheduler
                escalates this run for recovery.
              </p>
            </>
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}
