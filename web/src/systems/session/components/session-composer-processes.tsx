import { useState } from "react";
import { toast } from "@compozy/ui";

import {
  useSessionProcesses,
  type SessionProcessStopRequest,
} from "../hooks/use-session-processes";
import { reportProcessStop } from "../lib/session-process-feedback";
import {
  processTreeTarget,
  sessionRunningProcesses,
  type SessionProcessTree,
} from "../lib/session-processes";
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
    // One stop at a time; the popover locks every Stop while one is in flight.
    if (model.stopping !== null) return;
    const listed: readonly SessionProcessTree[] = model.trees;
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
      trees={model.trees}
      supported={model.supported}
      loading={model.loading}
      failed={model.failed}
      missing={model.missing}
      onRetry={model.retry}
      stopping={model.stopping}
      onStop={tree => stop({ targets: [processTreeTarget(tree)] })}
      onStopAll={() => stop({ all: true })}
      onOpenChange={setOpen}
    />
  );
}
