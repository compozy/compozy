import { useState } from "react";
import { toast } from "sonner";

import { useSessionMonitors } from "../hooks/use-session-monitors";
import { sessionActiveMonitors } from "../lib/session-monitor-summary";
import type { SessionPayload } from "../types";
import { MonitorComposerControl } from "./monitor-composer-control";

const MONITOR_CANCEL_FAILED = "Could not cancel the monitor.";

export interface SessionComposerMonitorsProps {
  session: SessionPayload;
}

/**
 * Feeds the composer's monitor control. The session's summary drives the
 * trigger at no cost; the list route is read only while the popover is open.
 */
export function SessionComposerMonitors({ session }: SessionComposerMonitorsProps) {
  const [open, setOpen] = useState(false);
  const model = useSessionMonitors(session.workspace_id ?? "", session.id, open);
  return (
    <MonitorComposerControl
      active={sessionActiveMonitors(session)}
      monitors={model.monitors}
      loading={model.loading}
      failed={model.failed}
      onRetry={model.retry}
      cancelingId={model.cancelingId}
      onCancel={monitorId => {
        model.cancel(monitorId).catch(error => {
          console.error(MONITOR_CANCEL_FAILED, error);
          toast.error(MONITOR_CANCEL_FAILED);
        });
      }}
      onOpenChange={setOpen}
    />
  );
}
