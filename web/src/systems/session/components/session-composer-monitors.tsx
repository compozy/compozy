import { useState } from "react";
import { toast } from "sonner";

import { SessionApiError } from "../adapters/session-api-errors";
import { useSessionMonitors } from "../hooks/use-session-monitors";
import { sessionActiveMonitors } from "../lib/session-monitor-summary";
import type { SessionPayload } from "../types";
import { MonitorComposerControl } from "./monitor-composer-control";

const MONITOR_CANCEL_FAILED = "Could not cancel the monitor.";
const MONITOR_ALREADY_SETTLED = "This monitor already fired or was canceled.";

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
          // The session was already resolved to read the list, so a 404 is a settled monitor.
          if (error instanceof SessionApiError && error.status === 404) {
            toast.info(MONITOR_ALREADY_SETTLED);
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
