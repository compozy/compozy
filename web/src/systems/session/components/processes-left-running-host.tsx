import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { terminateProcesses, type SessionProcessPayload } from "../adapters/session-processes-api";
import { sessionKeys } from "../lib/query-keys";
import { reportProcessStop } from "../lib/session-process-feedback";
import { buildProcessTrees, processIdentity } from "../lib/session-processes";
import { processesLeftRunningOptions } from "../lib/session-processes-query-options";
import { ProcessesLeftRunningDialog } from "./processes-left-running-dialog";

export interface ProcessesLeftRunningHostProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/**
 * Feeds "Processes left running" from the operator aggregate. The list is read
 * (and rescanned) only while the dialog is open; stops go through the
 * identity-checked operator route, which also reaches deleted sessions.
 */
export function ProcessesLeftRunningHost({ open, onOpenChange }: ProcessesLeftRunningHostProps) {
  const queryClient = useQueryClient();
  const query = useQuery({ ...processesLeftRunningOptions(), enabled: open });
  const mutation = useMutation({
    mutationFn: (processes: readonly SessionProcessPayload[]) => {
      const profiles = new Map(
        (query.data?.groups ?? []).flatMap(group =>
          group.items.map(process => [processIdentity(process), group.profile] as const)
        )
      );
      return terminateProcesses(
        processes.map(process => {
          const profile = profiles.get(processIdentity(process));
          if (!profile)
            throw new Error("Process ownership changed. Refresh the list and try again.");
          return { ...process, profile };
        })
      );
    },
    onSettled: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: sessionKeys.processesLeftRunning() }),
        // Stopped and archived owners still have rows whose summary just changed.
        queryClient.invalidateQueries({ queryKey: sessionKeys.lists() }),
      ]),
  });
  const pending = mutation.isPending ? mutation.variables : undefined;
  return (
    <ProcessesLeftRunningDialog
      open={open}
      onOpenChange={onOpenChange}
      groups={query.data?.groups ?? []}
      supported={query.data?.supported ?? true}
      loading={query.isPending && open}
      failed={query.isError}
      onRetry={() => void query.refetch()}
      stopping={pending ? new Set(pending.map(processIdentity)) : null}
      onStop={processes => {
        // One stop at a time; the dialog locks every Stop while one is in flight.
        if (mutation.isPending) return;
        mutation
          .mutateAsync(processes)
          .then(response =>
            reportProcessStop(
              response.results,
              (query.data?.groups ?? []).flatMap(group =>
                buildProcessTrees(group.items, group.trees)
              )
            )
          )
          .catch(error => {
            console.error("Could not stop processes left running.", error);
            toast.error(
              error instanceof Error && error.message ? error.message : "Couldn't stop processes."
            );
          });
      }}
    />
  );
}
