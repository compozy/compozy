import { useState } from "react";
import { toast } from "sonner";

import type { SessionProcessPayload } from "../adapters/session-processes-api";
import {
  useSessionProcesses,
  type SessionProcessStopRequest,
} from "../hooks/use-session-processes";
import { reportProcessStop } from "../lib/session-process-feedback";
import { sessionRunningProcesses } from "../lib/session-processes";
import type { SessionPayload } from "../types";
import { SessionProcessesComposerControl } from "./session-processes-composer-control";

export interface SessionComposerProcessesProps {
  session: SessionPayload;
}

/**
 * Feeds the composer's processes control. The session's `process_summary`
 * drives the trigger at no cost; the list route is read only while the popover
 * is open. It stays mounted on stopped and archived sessions, whose processes
 * keep running.
 */
export function SessionComposerProcesses({ session }: SessionComposerProcessesProps) {
  const [open, setOpen] = useState(false);
  const count = sessionRunningProcesses(session);
  // The control unmounts at zero without reporting a close, so the read follows the count too.
  const model = useSessionProcesses(
    session.workspace_id ?? "",
    session.id,
    session.profile_name,
    open && count > 0
  );
  const stop = (request: SessionProcessStopRequest) => {
    const listed: readonly SessionProcessPayload[] = model.processes;
    model
      .stop(request)
      .then(response => reportProcessStop(response.results, listed))
      .catch(error => {
        console.error("Could not stop processes.", error);
        toast.error(
          error instanceof Error && error.message ? error.message : "Couldn't stop processes."
        );
      });
  };
  return (
    <SessionProcessesComposerControl
      count={count}
      processes={model.processes}
      supported={model.supported}
      loading={model.loading}
      failed={model.failed}
      missing={model.missing}
      onRetry={model.retry}
      stopping={model.stopping}
      onStop={process => stop({ targets: [process] })}
      onStopAll={() => stop({ all: true })}
      onOpenChange={setOpen}
    />
  );
}
