import { useState } from "react";
import { toast } from "@compozy/ui";

import { SessionApiError } from "../adapters/session-api-errors";
import { useSessionMonitors } from "../hooks/use-session-monitors";
import { sessionActiveMonitors } from "../lib/session-monitor-summary";
import type { SessionPayload } from "../types";
import { MonitorComposerControl } from "./monitor-composer-control";

const MONITOR_CANCEL_FAILED = "Could not cancel the monitor.";
const MONITOR_NO_LONGER_ACTIVE = "This monitor is no longer active.";

export interface SessionComposerMonitorsProps {
  session: SessionPayload;
}

/**
 * Feeds the composer's monitor control. The session's summary drives the
 * trigger at no cost; the list route is read only while the popover is open.
 */
export function SessionComposerMonitors({ session }: SessionComposerMonitorsProps) {
  const [open, setOpen] = useState(false);
  const active = sessionActiveMonitors(session);
  // The control unmounts at zero without reporting a close, so the read follows the count too.
  const model = useSessionMonitors(
    session.workspace_id ?? "",
    session.id,
    session.profile_name,
    open && active > 0
  );
  return (
    <MonitorComposerControl
      active={active}
      monitors={model.monitors}
      loading={model.loading}
      failed={model.failed}
      missing={model.missing}
      onRetry={model.retry}
      cancelingId={model.cancelingId}
      onCancel={monitorId => {
        model.cancel(monitorId).catch(error => {
          // A 404 means the monitor settled, or its session is gone; either way nothing waits.
          if (error instanceof SessionApiError && error.status === 404) {
            toast.info(MONITOR_NO_LONGER_ACTIVE);
            return;
          }
          console.error(MONITOR_CANCEL_FAILED, error);
          toast.error(MONITOR_CANCEL_FAILED);
        });
      }}
      onOpenChange={setOpen}
    />
  );
}
