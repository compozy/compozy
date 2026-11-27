import type { QueryClient } from "@tanstack/react-query";
import { useRef, useState } from "react";

import type { SessionProcessPolicy } from "../adapters/session-processes-api";
import { sessionProcessPorts, sessionsRunningProcesses } from "../lib/session-processes";
import type { SessionTrees } from "../lib/session-tree";
import type { SessionPayload } from "../types";
import { loadSessionProcessChoice, sessionArchiveChoiceState } from "./session-lifecycle-helpers";
import type { SessionBatchAdmission } from "./use-session-batch-admission";
import type {
  SessionLifecycleAction,
  SessionProcessChoice,
  SessionProcessChoiceConfirmation,
} from "./session-lifecycle-types";

/**
 * Stop and archive ask first while their targets have processes running; at zero
 * they run immediately with the daemon default (keep). Archive also asks while
 * its targets have child sessions to include.
 */
export function useSessionProcessChoice(options: {
  queryClient: QueryClient;
  pendingAction: SessionLifecycleAction | null;
  batchRunning: SessionBatchAdmission;
  cancelDeletePrepare: () => void;
}) {
  const { queryClient, pendingAction, batchRunning, cancelDeletePrepare } = options;
  const [processChoice, setProcessChoice] = useState<SessionProcessChoice | null>(null);
  const [archiveChildren, setArchiveChildren] = useState(true);
  const processRead = useRef(0);
  const checkingProcesses = useRef(false);

  const askAboutProcesses = (
    action: "stop" | "archive",
    sessions: readonly SessionPayload[],
    run: (
      processes?: SessionProcessPolicy,
      readErrors?: ReadonlyMap<string, unknown>,
      trees?: SessionTrees
    ) => void
  ) => {
    if (
      sessions.length === 0 ||
      checkingProcesses.current ||
      processChoice !== null ||
      pendingAction !== null ||
      batchRunning.held()
    ) {
      return;
    }
    cancelDeletePrepare();
    checkingProcesses.current = true;
    const generation = ++processRead.current;
    setArchiveChildren(true);
    void loadSessionProcessChoice(
      queryClient,
      action,
      sessions,
      run,
      setProcessChoice,
      () => processRead.current === generation
    ).finally(() => {
      if (processRead.current === generation) checkingProcesses.current = false;
    });
  };

  const { archiveChoice, processTargets } = sessionArchiveChoiceState(
    processChoice,
    archiveChildren,
    setArchiveChildren
  );

  const processDialog: SessionProcessChoiceConfirmation = {
    open: processChoice !== null,
    action: processChoice?.action ?? "stop",
    sessionCount: processChoice?.sessions.length ?? 0,
    processCount: sessionsRunningProcesses(processTargets),
    ports: processTargets.flatMap(session => sessionProcessPorts(session)),
    childSessions: archiveChoice,
    onOpenChange: open => {
      if (!open) {
        processRead.current++;
        checkingProcesses.current = false;
        setProcessChoice(null);
      }
    },
    onChoose: processes => {
      const choice = processChoice;
      if (choice?.read?.pending) return;
      setProcessChoice(null);
      choice?.run(processes, archiveChoice?.included ?? false);
    },
  };

  return { askAboutProcesses, processDialog };
}
