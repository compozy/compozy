import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  Eyebrow,
  MonoId,
} from "@compozy/ui";

import type {
  SessionProcessGroupPayload,
  SessionProcessPayload,
} from "../adapters/session-processes-api";
import {
  buildProcessTrees,
  processCountLabel,
  processTreeMembers,
  sortSessionProcesses,
} from "../lib/session-processes";
import { ProcessListNote, SessionProcessList } from "./session-process-list";

export interface ProcessesLeftRunningDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  groups: readonly SessionProcessGroupPayload[];
  supported?: boolean;
  loading?: boolean;
  failed?: boolean;
  onRetry?: () => void;
  onStop: (processes: readonly SessionProcessPayload[]) => void;
  /** Identities whose stop is in flight; any stop in flight locks every Stop. */
  stopping?: ReadonlySet<string> | null;
}

function groupKey(group: SessionProcessGroupPayload) {
  return `${group.workspace_id}/${group.session_id}`;
}

function LeftRunningGroup({
  group,
  stopping,
  onStop,
}: {
  group: SessionProcessGroupPayload;
  stopping: ReadonlySet<string> | null;
  onStop: (processes: readonly SessionProcessPayload[]) => void;
}) {
  const trees = buildProcessTrees(sortSessionProcesses(group.items), group.trees);
  const processes = trees.flatMap(processTreeMembers);
  const name = group.session_name.trim();
  const locked = stopping !== null;
  return (
    <section
      aria-label={name || "Deleted session"}
      className="flex flex-col gap-1"
      data-testid={`left-running-group-${group.session_id}`}
    >
      <div className="flex items-center gap-2 px-0.5">
        <span className="flex min-w-0 flex-1 items-baseline gap-2">
          <span className="truncate text-transcript-meta font-semibold text-fg-strong">
            {name || "Deleted session"}
          </span>
          {name ? null : <MonoId value={group.session_id} />}
          <span className="shrink-0 text-transcript-caption text-faint">
            {processCountLabel(trees.length)}
          </span>
        </span>
        {trees.length > 1 ? (
          <Button
            type="button"
            variant="quiet"
            size="xs"
            disabled={locked}
            onClick={() => onStop(processes)}
            aria-label={`Stop all processes from ${name || "this deleted session"}`}
          >
            Stop all
          </Button>
        ) : null}
      </div>
      <SessionProcessList
        trees={trees}
        stopping={stopping}
        onStop={tree => onStop(processTreeMembers(tree))}
      />
    </section>
  );
}

/**
 * "Processes left running": every process whose session is stopped, archived,
 * deleted, or unknown, grouped by session. CompozyOS never stops these on its
 * own; a dev server may be intentional, so each one waits for a Stop here.
 */
export function ProcessesLeftRunningDialog({
  open,
  onOpenChange,
  groups,
  supported = true,
  loading = false,
  failed = false,
  onRetry,
  onStop,
  stopping = null,
}: ProcessesLeftRunningDialogProps) {
  const all = groups.flatMap(group => group.items);
  const treeCount = groups.reduce(
    (total, group) => total + buildProcessTrees(group.items, group.trees).length,
    0
  );
  const anyStopping = stopping !== null;
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="flex max-h-[calc(100dvh-4rem)] flex-col overflow-hidden sm:max-w-xl"
        data-testid="processes-left-running-dialog"
      >
        <DialogHeader className="shrink-0">
          <DialogTitle>Processes left running</DialogTitle>
          <DialogDescription>
            Started by sessions that are no longer running. They keep running until you stop them.
          </DialogDescription>
        </DialogHeader>
        <div className="flex min-h-0 flex-col gap-4 overflow-y-auto">
          <LeftRunningBody
            groups={groups}
            supported={supported}
            loading={loading}
            failed={failed}
            onRetry={onRetry}
            onStop={onStop}
            stopping={stopping}
          />
        </div>
        {treeCount > 1 && groups.length > 1 ? (
          <div className="flex shrink-0 items-center justify-between gap-2 border-t border-line pt-3">
            <Eyebrow>{processCountLabel(treeCount)} in total</Eyebrow>
            <Button
              type="button"
              variant="destructive"
              size="sm"
              disabled={anyStopping}
              onClick={() => onStop(all)}
              data-testid="left-running-stop-all"
            >
              {anyStopping ? "Stopping…" : "Stop all"}
            </Button>
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}

function LeftRunningBody({
  groups,
  supported,
  loading,
  failed,
  onRetry,
  onStop,
  stopping,
}: Omit<ProcessesLeftRunningDialogProps, "open" | "onOpenChange">) {
  if (!supported || groups.length === 0) {
    // The shared list owns the unsupported, loading, failed, and empty lines.
    return (
      <SessionProcessList
        trees={[]}
        supported={supported}
        loading={loading}
        failed={failed}
        onRetry={onRetry}
        placeholderCount={3}
        emptyLabel="Nothing was left running."
      />
    );
  }
  return (
    <>
      {groups.map(group => (
        <LeftRunningGroup
          key={groupKey(group)}
          group={group}
          stopping={stopping ?? null}
          onStop={onStop}
        />
      ))}
      {failed ? <ProcessListNote message="Couldn't refresh processes." onRetry={onRetry} /> : null}
    </>
  );
}
